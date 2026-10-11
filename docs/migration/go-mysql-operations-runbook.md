# CampusClaw Go/MySQL 运维 Runbook

适用 change：`migrate-backend-to-go-mysql`，任务6.4。本文只记录操作方法，**本轮未执行部署、停服、备份、恢复或切换**。Phase6尚未执行，本文完成不代表正式上线获准。

## 1. 架构、范围与证据等级

默认 `docker-compose.yml`：浏览器 → Nginx → Go → MySQL。`storage-init` 和 `initialize` 是一次性准备服务；只有Nginx发布loopback端口，默认8081。MySQL和Go没有宿主发布端口。`go-mysql` 保存数据库，`go-uploads` 保存私有附件及初始化凭据；Nginx不挂附件卷。

支持基础Bearer Token认证、teacher/student权限、A/B班隔离、材料上传/列表/原文/附件和页面壳。**不支持Flask RAG的 `/knowledge`、`/api/search`、`/api/ask`、reindex或Qdrant索引迁移。** 不能把默认Go部署称为现有Flask RAG Demo的功能等价替代品，也不能用当前RAG MySQL数据冒充SQLite迁移源。

| 等级 | 含义与依据 |
|---|---|
| 已隔离验证 | [6.1](phase5-http-contract-results.md)：双端各110项通过，80一致、30已接受差异；[6.2](phase5-cutover-results.md)：SQLite/附件副本迁移、失败阻止切换、70次代理观测；[6.3](phase5-compose-results.md)：默认Compose、101次代理观测、保留卷重建 |
| 已核对、待实机执行 | 下文根据真实CLI和配置组合的运维命令；本轮只核对代码/语法，不宣称逐条实机运行过 |
| 待完成 | 正式HTTPS、站点入口切换配置、生产备份/恢复演练、容量与权限确认，以及Phase6最终验收 |

以下PowerShell块均属于“已核对、待实机执行”；其中已验证的底层行为会单独注明。不要整体粘贴执行全文，fresh与migrate是互斥路径。所有示例变量都是本手册的本地变量，只有明确的 `$env:` 项才是程序环境变量。不得代入Demo项目、8080或其三个卷。

## 2. 部署前检查与公共命令上下文

在仓库根目录使用PowerShell。Docker Desktop/Linux容器引擎和Compose V2须可用；Compose需支持`--wait`和健康/完成依赖。检查磁盘空间足够容纳源快照、目标数据、配对备份和恢复演练副本，检查Docker及备份目录权限。默认MySQL8.4.4、Nginx1.28.0-alpine、Go构建1.26.0-alpine3.23已隔离验证，勿任意换成latest。

```powershell
$ErrorActionPreference = 'Stop'
function Invoke-DockerChecked {
    & docker @args
    if ($LASTEXITCODE -ne 0) { throw 'Docker command failed; stop this procedure' }
}
$Repo = (Get-Location).Path
if (-not (Test-Path -LiteralPath "$Repo/docker-compose.yml")) { throw 'Use repository root' }
$Project = Read-Host 'New or explicitly approved Go project name (never Demo)'
if ($Project -notmatch '^campusclaw-go-[a-z0-9-]+$' -or $Project -eq 'campusclaw-go-dev') {
    throw 'Choose an explicit deployment project name'
}
$OpsDir = Join-Path $Repo ('runtime/operations-' + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $OpsDir | Out-Null
$EmptyEnv = Join-Path $OpsDir 'empty.env'
Set-Content -LiteralPath $EmptyEnv -Value '# intentionally empty' -Encoding ascii
$ComposeArgs = @('compose','--env-file',$EmptyEnv,'-p',$Project,'-f',"$Repo/docker-compose.yml")
function Invoke-GoCompose { Invoke-DockerChecked @ComposeArgs @args }
Invoke-DockerChecked version
Invoke-DockerChecked compose version
Invoke-DockerChecked ps -a --filter "label=com.docker.compose.project=$Project"
Invoke-DockerChecked volume ls --filter "label=com.docker.compose.project=$Project"
```

项目名前缀只是防误操作提示，**不能替代人工确认卷归属**。首次fresh/import要求新项目且无遗留目标数据；日常重启则必须使用原项目、原卷、原数据库凭据。查看容器Mounts和卷标签，记录 `${Project}_go-mysql`、`${Project}_go-uploads`；不导出包含秘密的完整`docker inspect`或`compose config`。

由受控秘密管理工具注入下表变量，或在无命令转录的终端安全输入。不得把真实值写进仓库、参数字面量或日志：

| 变量 | 实际用途 |
|---|---|
| `DB_PASSWORD` | Compose创建MySQL业务用户及Go连接密码；已有卷改此变量不会自动修改MySQL用户密码 |
| `MYSQL_ROOT_PASSWORD` | MySQL首次初始化密码；不能用来自动重设已有库root密码 |
| `JWT_SIGNING_SECRET` | 至少32字节随机秘密；无SECRET_KEY兜底，迁移与回退使用独立新密钥；普通重启保持不变 |
| `INITIALIZATION_MODE` | 必须显式fresh或migrate，没有自动选择 |
| 四个 `SEED_*_PASSWORD` | `SEED_TEACHER_A_PASSWORD`、`SEED_STUDENT_A1_PASSWORD`、`SEED_TEACHER_B_PASSWORD`、`SEED_STUDENT_B1_PASSWORD`；首次fresh全部必需 |
| `DEV_HTTP_PORT` | loopback Nginx宿主端口，默认8081；本演练禁用8080 |
| `GO_IMAGE` / `GO_INIT_IMAGE` | 可选独立镜像标签，实际镜像构建文件为Dockerfile.backend |
| `GOPROXY` | 可选构建模块代理；原代理不可达时已有使用goproxy.cn的证据，不关闭TLS/校验和 |

`DB_HOST=mysql`、`DB_PORT=3306`、`DB_NAME=campusclaw`、`DB_USER=campusclaw`、`UPLOAD_ROOT=/uploads`、`STARTUP_MODE=application`由默认Compose固定。`.env.example`只是注释说明，不是可直接使用的凭据文件。Go支持`MAX_UPLOAD_BYTES`，但**当前默认Compose没有映射该变量**，因此此部署使用20 MiB默认值；仅在宿主设置它不会改变容器限额，不在本手册扩大配置。

示例安全输入单个秘密（其余按相同方法输入或由秘密管理工具提供；JWT须来自密码学随机生成器，不能使用手输短语）：

```powershell
$SecretInput = Read-Host 'DB_PASSWORD' -AsSecureString
$env:DB_PASSWORD = [System.Net.NetworkCredential]::new('', $SecretInput).Password
$SecretInput = $null
$env:DEV_HTTP_PORT = '8081'  # 先人工确认空闲；不得使用Demo端口
Get-NetTCPConnection -LocalPort ([int]$env:DEV_HTTP_PORT) -ErrorAction SilentlyContinue
```

若端口已有监听先调查，不停止占用进程。容器环境对Docker管理员可见，这不是秘密保管库；禁止公开其Env。初始密码、JWT秘密和灾备所需凭据在仓库外受限保管。

**HTTPS待完成**：当前配置仅HTTP。正式外部入口必须另行配置证书、HTTPS及同源代理到这个loopback入口，测试Authorization和上传限制。默认配置没有TLS监听、证书或自动续期，不能直接作为公网Bearer入口。正式站点地址/代理配置尚未给出，本文不编造其修改或reload命令。

## 3. Fresh：全新基础材料部署

只适用于不需要导入任何旧数据的空目标。先完成公共上下文和秘密注入，确认新项目没有卷/库冲突。不要通过删除卷“修复”初始化失败。

```powershell
$env:INITIALIZATION_MODE = 'fresh'
Invoke-GoCompose config --quiet
Invoke-GoCompose build app initialize
Invoke-GoCompose up -d --wait --wait-timeout 180
Invoke-GoCompose ps -a
Invoke-GoCompose exec -T nginx nginx -t
$Origin = 'http://127.0.0.1:' + $env:DEV_HTTP_PORT
Invoke-RestMethod "$Origin/health"
```

预期匿名health为200且`{"status":"ok"}`。initialize成功退出后app才启动，app健康后Nginx才启动。首次生成A/B、四个样例账号、两份完整样例材料和八表结构，sessions为空。通过`/login`分别验证teacher_a/student_a1/teacher_b/student_b1；登录响应Token只留浏览器sessionStorage或测试进程内存，不复制到报告。

fresh失败先看initialize日志，保持入口关闭；缺初始化密码不会产生业务数据，补齐后重试已隔离验证。已完成fresh重复运行会核验原凭据/数据，不重置账号密码、附件或session_version；已有migrate凭据或未知非空目标拒绝。不要删除 `.campusclaw-initialization.json`、伪造 `Verified` 或把migrate改成fresh绕过检查。pending文件是失败恢复证据，不是成功标记。

## 4. Migrate：停写、快照、导入和切换

### 4.1 冻结源端并保全

源端必须是确认的**基础材料Flask/SQLite实例**，`MATERIAL_BACKEND=sqlite`，不是当前Flask RAG Demo。登记源进程管理方式、数据库和上传目录、旧入口及责任人。在源端仍在线时，于受控内存中保存一个当时有效的测试Token用于新入口拒绝检查；不落盘或打印。

进入维护窗口，停止源HTTP及所有后台/账号写入，确认退出且无持有源库/附件的写入者。停写包括logout导致的session_version变化。源进程管理方式因部署而异，仓库没有统一“停真实源”命令；必须由操作者先完成并记录，不能只关页面就继续。默认新目标Nginx此时不得启动。

下面快照片段复用6.2的SQLite backup API思路；**本片段只静态核对，未逐条执行**。它要求已停写、使用全新快照目录，拒绝源附件符号链接，不覆盖源。Python仅用于离线备份，不是Go运行依赖。

```powershell
$SourceDB = (Resolve-Path -LiteralPath (Read-Host 'Stopped basic SQLite database path')).Path
$SourceUploads = (Resolve-Path -LiteralPath (Read-Host 'Stopped basic uploads root')).Path
$Snapshot = Join-Path $OpsDir 'snapshot'
@'
from pathlib import Path
import sqlite3, shutil, sys, hashlib, json
db, uploads, dest = map(Path, sys.argv[1:])
assert db.is_file() and uploads.is_dir()
assert not dest.resolve().is_relative_to(uploads.resolve())
assert not uploads.is_symlink()
assert not any(p.is_symlink() for p in uploads.rglob('*'))
dest.mkdir(exist_ok=False)
src = sqlite3.connect(db.resolve().as_uri() + '?mode=ro', uri=True)
out = sqlite3.connect(dest / 'source.db')
try:
    src.backup(out)
    assert out.execute('PRAGMA integrity_check').fetchone()[0] == 'ok'
finally:
    out.close(); src.close()
shutil.copytree(uploads, dest / 'uploads')
def sha(p):
    with p.open('rb') as f: return hashlib.file_digest(f, 'sha256').hexdigest()
manifest = {p.relative_to(dest).as_posix(): sha(p) for p in dest.rglob('*') if p.is_file()}
(dest.parent / 'snapshot-sha256.json').write_text(json.dumps(manifest, indent=2), encoding='utf-8')
'@ | .\.venv\Scripts\python.exe - $SourceDB $SourceUploads $Snapshot
if ($LASTEXITCODE -ne 0) { throw 'Snapshot failed; do not migrate' }
```

Windows还须人工拒绝junction/reparse point和错误目录嵌套。复制全套快照及清单到受限的独立备份位置，记录停写时间、源hash、用户版本、数量、文件清单；验证复制后hash一致。只复制在线SQLite主文件可能遗漏WAL，不可代替一致性快照。备份不自动删除。

### 4.2 创建空目标并使用实际离线工具

为migrate另选一个新项目，重新执行第2节公共上下文和凭据注入；不能沿用已fresh播种的目标。`JWT_SIGNING_SECRET`必须与源不同。目标源快照路径仍使用上一步的完整绝对路径。

默认server镜像只有`/server`，initializer只有`/bootstrap`；不存在`docker compose exec app /migrate`。真实离线命令位于 `cmd/admin` 和 `cmd/migrate`。下例采用6.2已经构建过的 `tests/cutover/Dockerfile.backend` 作为**离线工具镜像**，不把它用作正式HTTP服务镜像。此处手工docker run组合未实机重演，正式使用前须在新副本复核挂载、网络与UID10001权限。

```powershell
$env:INITIALIZATION_MODE = 'migrate'
Invoke-GoCompose config --quiet
Invoke-GoCompose build app initialize
Invoke-GoCompose up -d --wait --wait-timeout 180 mysql
Invoke-GoCompose run --no-deps -T storage-init
$ToolImage = $Project + ':offline-tools'
Invoke-DockerChecked build -f tests/cutover/Dockerfile.backend -t $ToolImage .
$Reports = Join-Path $OpsDir 'reports'
New-Item -ItemType Directory -Path $Reports | Out-Null
$env:DB_HOST='mysql'; $env:DB_PORT='3306'
$env:DB_NAME='campusclaw'; $env:DB_USER='campusclaw'; $env:UPLOAD_ROOT='/uploads'
$ToolArgs = @('run','--network',"${Project}_backend",'--user','10001:10001',
    '--env','DB_HOST','--env','DB_PORT','--env','DB_NAME','--env','DB_USER','--env','DB_PASSWORD','--env','UPLOAD_ROOT',
    '--mount',"type=volume,source=${Project}_go-uploads,target=/uploads",
    '--mount',"type=bind,source=$Snapshot,target=/snapshot,readonly",
    '--mount',"type=bind,source=$Reports,target=/reports")
Invoke-DockerChecked @ToolArgs --entrypoint /admin $ToolImage -mode schema
$MigrationArgs = @('-source','/snapshot/source.db','-source-uploads','/snapshot/uploads')
Invoke-DockerChecked @ToolArgs --entrypoint /migrate $ToolImage -mode preflight @MigrationArgs -report /reports/preflight.json
Invoke-DockerChecked @ToolArgs --entrypoint /migrate $ToolImage -mode import @MigrationArgs -report /reports/import.json
Invoke-DockerChecked @ToolArgs --entrypoint /migrate $ToolImage -mode verify @MigrationArgs -report /reports/verify.json
```

若模块代理不通，工具构建可显式加 `--build-arg GOPROXY=https://goproxy.cn`，这是6.2用过的镜像地址，不强制生产采用。Linux报告目录须预先授予UID10001写权限，源快照仅需读权限，不用全局777。工具不发布端口，源挂载只读，目标只有一个离线写入者。停止的工具容器与证据保留。

每次重试使用新的报告文件名；CLI拒绝覆盖已有report，report必须在源快照之外。**preflight的Verified=false是预期，它不是放行凭据。** import仅接受空目标或可核实的相同目标；失败/提交结果未知时先保留证据、核验目标，不能清库重来或手删文件。`migrate verify`是与指定源快照全字段比较，目标上线新增材料或logout以后不应再要求与旧源指纹相等。

### 4.3 完整性门槛

```powershell
$Preflight = Get-Content -LiteralPath "$Reports/preflight.json" -Raw | ConvertFrom-Json
$Imported = Get-Content -LiteralPath "$Reports/import.json" -Raw | ConvertFrom-Json
$Verified = Get-Content -LiteralPath "$Reports/verify.json" -Raw | ConvertFrom-Json
$SourceHash = (Get-FileHash -LiteralPath "$Snapshot/source.db" -Algorithm SHA256).Hash.ToLowerInvariant()
if (-not $Imported.Verified -or -not $Verified.Verified -or $Verified.Mode -ne 'migrate' -or
    $Verified.SourceSHA256 -ne $SourceHash -or $Verified.Fingerprint -ne $Preflight.Fingerprint -or
    $Imported.Fingerprint -ne $Verified.Fingerprint) { throw 'Cutover gate refused' }
```

此外人工检查八表结构，七表（classes/users/materials/knowledge_entries/assignments/assistants/skills）的Count、IDs、RowsSHA256、正文和FilesSHA256一致；用户密码哈希、role/class_id/session_version原值不变，sessions初始为空。正文按契约去BOM，而附件保留BOM，因此正文hash可能不同于附件hash。对照源清单确认源库与全部附件未被修改。故障后不能拿旧成功报告代替本次verify。

### 4.4 新入口验证与正式切换

门槛全部通过且源保持停写后：

```powershell
Invoke-GoCompose up -d --wait --wait-timeout 180 app
Invoke-GoCompose up -d nginx
Invoke-GoCompose exec -T nginx nginx -t
$Origin = 'http://127.0.0.1:' + $env:DEV_HTTP_PORT
Invoke-RestMethod "$Origin/health"
```

这是启用**候选loopback入口**，不是已经切换正式站点。6.2的维护503→Go配置reload在独立Nginx中验收过，但该维护配置不是默认Compose提供的功能。默认Nginx配置固定上游`app:8080`，首次上线用上述启动；app容器地址改变后宜重建Nginx以更新解析，单纯检查nginx配置成功不证明上游健康。

在候选入口完成第9节清单：旧源Token401、旧Cookie-only401，新密码登录200（保留原账号密码），me/列表/详情/下载正常，学生403、外班404。验证Authorization透传和私有路径404。密码与Token仅放安全客户端内存，不用带明文密码/Token的curl命令或URL。登录不写版本，**logout会写版本**；上传/账号变更同样使回退进入“已有新写入”。如要保留零新写入窗口，先在隔离副本完成这些写操作验收，候选实例只做登录和只读验证。

正式外层HTTPS代理、域名/端口、维护入口和回退配置由部署负责人另行提供并审查；待Phase6及正式清单通过后，才在批准窗口改变其上游。不存在仓库内通用“切正式入口”脚本，不在本文伪造站点路径。正式切换后重复只读/权限验收，记录时间与是否已有新写入；任一安全检查失败立即关闭新入口。

## 5. 故障处理与停止条件

| 情况 | 必须停止什么 | 检查和下一步 |
|---|---|---|
| preflight/import退出非零或无新成功报告 | 停止切换，保持源备份和目标 | 看脱敏错误、挂载/读写权限、schema能力；禁止清空目标或覆盖旧report；提交未知时由工具核实数据库及文件，不手删可能已提交附件 |
| hash/ID/数量/版本不一致 | 禁止启动入口、禁止自动修复源 | 确认快照配对、目标是否已有写入；保留两端证据，修复只能针对批准的副本，重新获得新verify |
| bootstrap或Go启动失败 | 不绕过门槛；停止重启循环 | 查看initialize/app日志；检查密钥、模式、验收凭据、八表、缺正文/缺文件、非法键/符号链接和UID权限；不能切infrastructure假装正常 |
| MySQL不可用、health503 | 停止写入/切换 | 检查MySQL健康、磁盘、内部网络与密码；现有卷改环境密码不会改数据库密码；不能删卷重建“修好” |
| Nginx502 | 关闭对外放行 | 先检查Go健康与启动失败原因，再查内部DNS/app地址、网络和Nginx日志；app换地址后在维护窗口重建Nginx，不能将健康依赖改弱 |
| 学生上传成功、跨班200、旧凭据仍有效 | 立即停止候选入口及业务写入 | 保全日志/数据库/文件，排查密钥、Authorization、可信principal与版本；视为安全故障，不作为框架差异豁免 |
| 上传500或客户端超时 | 不自动重复同一上传/删除文件 | 结果可能已提交；按当前库查询材料及附件，保留UUID/恢复证据；恢复扫描须单实例且数据库已验证可用 |
| 空间不足/备份失败 | 停止后续操作 | 保留已完成快照与报告，补足空间后新位置重试，不自动删历史卷/备份 |

诊断命令（仅已核准Go项目，输出先检查脱敏再归档）：

```powershell
Invoke-GoCompose ps -a
Invoke-GoCompose logs --tail 100 initialize app mysql nginx
Invoke-GoCompose exec -T app /server healthcheck
Invoke-GoCompose exec -T nginx nginx -t
```

health200仅说明当前数据库连接和文件探测通过，不代替完整数据/权限验收。错误日志可能含内部路径或MySQL启动秘密信息，不能原样公开。不得记录登录JSON、Authorization、密码/哈希或完整容器Env。

## 6. 回退决策：三个阶段

| 阶段 | 允许的方向 | 必须满足的条件 |
|---|---|---|
| 切换前，新栈尚未提供业务 | 保持旧入口或恢复被冻结的旧源 | 先停目标写入并保留失败证据；确认源自快照后未发生未保全变更；旧源与目标不共用写卷 |
| 切换后，确证没有新写入 | 停目标，恢复经核验的旧源副本及旧入口 | 不仅材料数相同：还须确认无logout、密码/角色/班级变更、账号创建或后台写入；版本不得回退；使用新的独立签名密钥并要求重新登录 |
| 切换后已有新写入，或无法证明没有 | 停写、保全当前MySQL和附件，优先修复新栈 | **禁止直接用旧快照覆盖新数据或切回旧副本**。必须另行设计并验收反向同步/合并，包括所有新材料、账号状态及不降低的session_version；当前没有自动反向迁移命令 |

所有回退仍为Bearer-only，禁止恢复SID/Cookie授权、重置session_version或重新启用旧签名密钥。换新密钥可拒绝旧Token，但不能掩盖账号密码/权限状态回退；旧密码曾被变更时同样不能恢复旧快照使旧密码重新生效。备份保留不代表可直接上线。恢复操作须对新副本执行，保留原源、失败目标及当前最新状态；正式恢复旧入口命令取决于已审批的旧服务管理/HTTPS配置，不虚构一键回退。

## 7. 持久化、配对备份与恢复演练

必须配对保全 `${Project}_go-mysql` 和 `${Project}_go-uploads`，以及镜像版本、配置、验收报告、初始化模式和安全保管的凭据。附件卷应完整包含`files`、`tmp`、`.campusclaw-initialization.json`及pending/未知提交证据，不仅复制可见文件。MySQL与文件跨系统，不能把不同时点两份在线拷贝当成一致备份。

**禁止对需保留数据的部署执行 `docker compose down -v`、volume rm/prune、数据库清空或卷替换来初始化/恢复。** 日常使用stop/up和保留卷重建。备份不得自动删除。下列冷备份方案**尚未实机演练**，需先在新隔离部署验证；它会停掉所选Go项目，只能在批准维护窗口执行，绝不能代入Demo。

```powershell
Invoke-GoCompose stop nginx app
Invoke-GoCompose stop mysql
Invoke-GoCompose ps -a  # 人工确认所有业务写入者与MySQL已停止；一次性服务也不得仍运行
$Backup = Join-Path $OpsDir 'cold-backup'
New-Item -ItemType Directory -Path $Backup | Out-Null
foreach ($Part in @('go-mysql','go-uploads')) {
    $Volume = "${Project}_$Part"
    Invoke-DockerChecked volume inspect $Volume  # 检查归属；不创建缺失源卷
    Invoke-DockerChecked run --network none --mount "type=volume,source=$Volume,target=/source,readonly" `
        --mount "type=bind,source=$Backup,target=/backup" alpine:3.23.3 `
        tar -cpf "/backup/$Part.tar" -C /source .
}
Get-FileHash -LiteralPath "$Backup/go-mysql.tar","$Backup/go-uploads.tar" -Algorithm SHA256 |
    Export-Csv -LiteralPath "$Backup/archive-sha256.csv" -NoTypeInformation
```

必须等待MySQL正常停止后再读原始数据目录，备份目录只准运维人员访问。归档hash证明归档未变，不单独证明业务一致。使用同MySQL补丁版本恢复，保留UID/GID；跨版本升级不是此恢复方案的已验收能力。

恢复演练（**未实机验收**）：先核对两归档hash，选择另一全新项目和非8080端口。下面只向本次创建的空卷解包；不删除、不覆盖任何原卷：

```powershell
$RestoreProject = 'campusclaw-go-restore-' + [guid]::NewGuid().ToString('N')
$Expected = Import-Csv -LiteralPath "$Backup/archive-sha256.csv"
foreach ($Part in @('go-mysql','go-uploads')) {
    $Archive = Join-Path $Backup "$Part.tar"
    $Row = @($Expected | Where-Object { [IO.Path]::GetFileName($_.Path) -eq "$Part.tar" })
    if ($Row.Count -ne 1 -or (Get-FileHash -LiteralPath $Archive -Algorithm SHA256).Hash -ne $Row[0].Hash) {
        throw 'Backup hash mismatch; stop restore'
    }
    $Volume = "${RestoreProject}_$Part"
    $ExistingVolumes = @(Invoke-DockerChecked volume ls --format '{{.Name}}')
    if ($ExistingVolumes -contains $Volume) { throw 'Restore target already exists' }
    Invoke-DockerChecked volume create --label "com.docker.compose.project=$RestoreProject" `
        --label "com.docker.compose.volume=$Part" $Volume
    Invoke-DockerChecked run --network none --mount "type=volume,source=$Volume,target=/target" `
        --mount "type=bind,source=$Backup,target=/backup,readonly" alpine:3.23.3 `
        tar -xpf "/backup/$Part.tar" -C /target
}
```

确认解包到卷根（不是再嵌套一层目录），隐藏凭据/文件hash和所有者保留。后续使用第2节上下文但令`$Project=$RestoreProject`并重新构造`$ComposeArgs`，改用一个空闲非8080端口；按备份的fresh/migrate凭据选择模式，使用备份数据库的实际密码和**独立新JWT密钥**。启动前只允许一个恢复写入者，不接正式入口。随后运行初始化门槛、health、完整用户/班级/版本/双表清单和逐附件SHA-256比较，再进行A/B权限、下载和撤销检查。

**当前没有通用备份restore或上线后全量hash CLI**；上面是Docker卷解包方法，不是已实现的业务恢复脚本。逐文件/双表审计须复用验收方法或在恢复演练中准备只读核验工具，未完成前不得宣称灾备已验证。`migrate verify`仅适合仍与原SQLite快照相同的目标，不能验证已新增业务数据的冷备份。

## 8. 安全与日常运维

Bearer为HS256、固定28800秒、不滑动续期；身份每次查MySQL，logout事务递增版本，204才确认成功。前端清除Token不等于服务器撤销成功。普通容器重建保留密钥和版本，有效Token继续有效；计划密钥轮换则停止入口/写入、备份当前状态、注入新随机JWT秘密，重建服务并测试旧Token401、新登录200，通知所有用户重新登录。更换密钥不需要也不允许重置版本，不回用旧密钥。

Go/initializer使用UID10001，MySQL业务连接不用root，app/mysql不发布端口，附件不配置静态alias。当前初始化与运行共用同一个数据库用户，尚未拆分DDL和运行DML权限；如生产要求更细权限需另行设计验证，不能声称已做到完全权限分离。生产HTTPS与证书续期仍待完成。Nginx不额外限制body，Go仍执行限额；不开放任意来源CORS。

日常启动、停写和重建（只针对已确认Go项目；保持原模式/凭据/卷）：

```powershell
Invoke-GoCompose up -d --wait --wait-timeout 180
# 维护前先关闭入口和唯一业务写入者
Invoke-GoCompose stop nginx app
# 若需要整体停机
Invoke-GoCompose stop
# 更新镜像时，先完成备份和离线核验；不删卷
Invoke-GoCompose build app initialize
Invoke-GoCompose up -d --force-recreate --wait --wait-timeout 180
Invoke-GoCompose exec -T nginx nginx -t
Invoke-RestMethod ('http://127.0.0.1:' + $env:DEV_HTTP_PORT + '/health')
```

上述“保留卷重建”的底层步骤6.3已验证；不要在旧业务仍写入时并行运行离线初始化、迁移或第二个Go恢复进程。日常例行检查空间/证书到期/健康、访问权限及备份成功，定期在独立项目做恢复演练并记录恢复耗时。检查工作区基线，未经审查的未提交改动不能被误当固定发布版本；禁止通过reset、restore、clean、checkout覆盖或自动stash整理部署目录。

## 9. 正式切换前逐项验收

下表全部勾选之前，不允许正式放行。历史已验证项仍需核对本次部署的配置/数据，不能用演练项目报告替代当前目标报告。

| 正式待签核项 | 隔离证据 / 当前状态 |
|---|---|
| [ ] 负责人确认只迁移基础材料业务，不替代RAG；源不是Demo | 本文边界明确；正式人工确认 |
| [ ] 固定发布版本/镜像、工作区改动清单、项目/端口/两卷归属 | 6.3隔离审计通过；正式人工确认 |
| [ ] HTTPS、证书、域名、外层入口/维护/回退配置 | **待实施及实机验证** |
| [ ] 独立JWT秘密、初始化/数据库凭据、安全保管、时间同步 | 配置拒绝6.3已验证；真实凭据/时钟人工确认 |
| [ ] 备份空间、读写权限、停写窗口、源快照配对与hash | 6.2副本已验证；正式源重新核验 |
| [ ] 八表、七表数量/ID/字段/正文/附件hash，sessions为空、版本原值 | 6.2已验证；当前目标新verify报告必需 |
| [ ] 源及目标无双写、Go恢复只扫描自有附件卷 | 6.2/6.3已验证；实际挂载/进程人工确认 |
| [ ] 正常登录/me、旧Token及Cookie-only401、新登录成功 | 6.1/6.2已验证；当前候选入口复核 |
| [ ] 双向A/B列表/详情/下载隔离；学生403、跨班/不存在404、伪造身份无效 | 6.1/6.2/6.3已验证；当前候选入口复核 |
| [ ] 页面及中文、附件原字节、安全头、私有路径404 | 6.1/6.3已验证；当前候选入口复核 |
| [ ] 上传20 MiB合法/超限拒绝，logout204及旧版本401，重建不复活 | 6.3已验证；写操作在隔离副本执行，实际写入后改用“有新写入”回退策略 |
| [ ] 已接受HTTP差异没有误判为故障或扩大豁免 | 6.1逐项规则：?&、多重错误/大字段、OPTIONS/Allow、条件请求和下载缓存；仅基础业务 |
| [ ] 失败阻止切换、配对冷备份恢复和回退责任人/决策 | 故障门槛6.2已验证；生产冷备恢复及实际回退**未演练** |
| [ ] Phase6适用测试和最终验收完成 | **尚未执行；不因6.4完成而勾选** |

## 10. 本次文档验收

已逐项对照proposal/design/spec/tasks、三份Phase5记录、默认Compose/两类Dockerfile/Nginx/.env.example、`cmd/{admin,migrate,bootstrap}`、`internal/transfer`及6.2/6.3测试命令。特别核对默认镜像不带admin/migrate、report不可覆盖、migrate不是导入开关、MAX_UPLOAD_BYTES未映射、HTTPS未实施等限制。PowerShell代码块只做解析检查，不执行服务命令；证据报告hash与验收记录核对。

6.4是操作文档与既有隔离演练的核对验收；本文未验证的生产环境动作明确标记，不冒称完成恢复/正式切换。仅新增本文并更新本change任务进度。本轮11个PowerShell代码块解析通过；三份历史成功报告的accepted及SHA-256均与验收记录匹配；`openspec validate migrate-backend-to-go-mysql --strict`通过。6.4完成，总进度27/30，Phase5为4/4；Phase6保持未勾选。
