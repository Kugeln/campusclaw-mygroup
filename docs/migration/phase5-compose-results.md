# Phase 5 / 6.3 默认 Go Compose 运行验收

日期：2026-10-11。**6.3 验收通过，总进度26/30。** 本轮只完成默认运行配置及其隔离验证，没有实施6.4或Phase6，没有实际切换Flask RAG Demo。

## 配置与启动模式

- `docker-compose.yml` 将 app 的 `STARTUP_MODE` 从 `infrastructure` 改为 `application`，注入必需的 `JWT_SIGNING_SECRET`。正常运行链路为 Nginx → Go → MySQL，另有两个运行后退出的准备服务，不是额外业务后端。
- 启动顺序为 MySQL健康、storage-init准备私有目录 → initialize成功 → Go通过既有验收凭据/恢复检查并健康 → Nginx启动。初始化失败不会把仅health可用的占位服务误当完整应用。
- 必须显式设置 `INITIALIZATION_MODE=fresh` 或 `migrate`，不默认替操作者选择。`fresh` 准备八表并复用既有 `provision.Fresh`：仅空目标首次播种时需要四个 `SEED_*_PASSWORD`；有匹配fresh验收凭据时只核验，不重置账号、密码、session_version和附件。非空无凭据目标拒绝；仅匹配已有pending证据的恢复仍沿用Phase2规则。迁移目标不能走fresh。
- `migrate` **不是在线自动导入**：仅接受已经离线导入并核验、具有migrate凭据的目标，再执行 `transfer.Ready`。未验收、凭据缺失或模式不匹配均拒绝。离线导入/初始化前必须停止业务写入，不允许两个写入进程共用文件卷。本轮验证了模式拒绝和fresh持久化，成功迁移数据的证据仍见6.2记录；没有重跑真实数据迁移。
- `cmd/bootstrap` 是部署门槛命令，复用配置、schema、fresh和Ready能力，不新增认证/迁移体系。120秒有界初始化超时，失败退出非零。
- `Dockerfile.backend` 使用锁定的Go/Alpine版本构建独立server和initializer目标：server仍为scratch、非root、只有Go二进制及嵌入页面资源；initializer为Alpine加bootstrap。两者依赖图不含SQLite、Python或离线migration包。构建可显式设置GOPROXY，不改变锁定依赖或关闭校验。
- `.env.example` 记录所需变量及两种模式，只有说明，没有真实秘密。初始化密码只注入initialize，不注入app；JWT/数据库密码通过进程环境提供，不进入镜像构建参数。
- MySQL与上传目录各自使用项目级命名持久卷。仅Nginx发布宿主端口，默认仍为loopback **8081**，可用 `DEV_HTTP_PORT` 指定。遵照本轮保护要求不占用8080，不实际执行design中最终正式入口的切换。
- Nginx显式透传Authorization，保留身份Header清除、禁用上游重试、不挂上传卷的策略。继续使用 `client_max_body_size 0`，由Go执行20 MiB文件限制及有界multipart解析，避免代理将合法文件连同边界错误拒绝。

## 实际隔离命令与证据

仓库根目录执行：

```powershell
go test ./cmd/bootstrap ./cmd/server ./internal/config ./internal/provision ./internal/auth ./internal/http ./internal/recovery
$env:GOPROXY = 'https://goproxy.cn'
.\.venv\Scripts\python.exe tests/compose/run.py
go list -deps ./cmd/server ./cmd/bootstrap
openspec validate migrate-backend-to-go-mysql --strict
```

Go相关测试通过；bootstrap包含1个表驱动测试、6个子场景。server/provision/recovery在该命令中是编译检查，输出 `[no test files]`，不虚称这些包运行了集成测试。真实MySQL验证由下述Compose执行。依赖图核对无SQLite/Python/migration依赖。Go依赖核对首次因本机缓存访问权限失败，使用授权执行后成功。

新测试工具直接使用仓库**默认** `docker-compose.yml`，没有用替代服务配置模拟通过。为每次运行生成全新项目、两个卷、两个网络、独立镜像标签、非8080端口和内存中的随机凭据；显式使用空env文件防止本地`.env`影响。测试工具使用Python，但目标服务没有Python依赖，也不启动Flask。

成功项目：`campusclaw-phase5-compose-ced2bc23`，Nginx入口 `127.0.0.1:50607`，数据库为该项目MySQL中的`campusclaw`，与Demo数据库实例隔离。

实际命令以以下前缀调用（随机凭据仅在子进程环境中）：

```text
docker compose --env-file E:\proj\campusclaw-mygroup\runtime\campusclaw-phase5-compose-ced2bc23\empty.env -p campusclaw-phase5-compose-ced2bc23 -f E:\proj\campusclaw-mygroup\docker-compose.yml
```

依次执行 `config --quiet`、`config --format json`、`build app initialize`、缺凭据的`up -d`、补齐凭据的`up -d --wait --wait-timeout 180`、`exec -T nginx nginx -t`、停止隔离app/nginx后的拒绝场景，以及 `up -d --force-recreate --wait --wait-timeout 180`。最后仅`stop`本次项目，未执行down、删除或重建数据卷。完整逐条命令、退出码和脱敏输出在runtime报告及command日志中。

结果：**runner退出0，8组阶段断言全部通过，101次真实代理HTTP观测**。其中200×49、201×3、403×2、404×40、401×5、204×1、413×1；请求数量不是101个独立单元测试。

证据：`runtime/campusclaw-phase5-compose-ced2bc23/result.json`，`accepted=true`，SHA-256：
`b90a5666d5e567505036024abc65f178cd85023fc42e49541acdbd7a2535b01a`。

首轮项目 `campusclaw-phase5-compose-ab31297d` 已通过初始化、业务矩阵及20 MiB上传，但验收工具读取20 MiB正文时触发MySQL CLI自身的包大小限制，报告明确为失败。仅将测试客户端 `--max-allowed-packet` 调整为64 MiB，未更改业务行为或放宽断言，随后在上述全新项目完成验收。两轮资源及报告均保留。

## 功能与安全失败

| 检查 | 实测结果 |
|---|---|
| 新库初始化 | 八表；classes=2、users=4、materials=2、knowledge_entries=2，sessions=0；占位表保留 |
| 缺少JWT密钥 | Compose配置阶段非零退出，尚未创建资源 |
| 缺少初始化密码 | 初始化失败、users/materials均0行，app和Nginx未运行；补齐后可成功重试 |
| 非法短JWT密钥 | bootstrap拒绝；已有用户/材料/正文指纹不变 |
| 已有数据误用模式 | migrate拒绝fresh凭据；暂存自建测试卷凭据后，fresh拒绝无凭据非空目标；恢复凭据后可正常启动，未重置数据 |
| 身份与权限 | 四账号登录、me角色/班级正确；教师A/B各上传201，学生A/B各403；匿名及Cookie-only401 |
| 班级隔离 | 两角色、A/B双向列表限定；外班详情与下载404，无资源内容泄露 |
| 内容及附件 | 详情与原文相符；上传BOM/中文正文；下载保留原始字节、attachment、nosniff、private, no-store |
| 上传限额 | 20 MiB文件加multipart开销通过真实Nginx返回201；20 MiB+1返回413 |
| 撤销 | logout204、旧Token401、重新登录200；重建后已撤销Token仍401、未撤销Token仍200 |
| 页面及私有目录 | login/materials/logo200；uploads/files、static/uploads/files及files路径均404 |

原6.1接受的HTTP差异、Flask行为和Go业务实现均未调整，没有新增RAG接口或能力。

## 保留卷重建与完整性

重建前停止隔离Nginx和app，再进行离线拒绝检查，确保不存在并行写入。使用`--force-recreate`替换五个服务容器，验证原、新容器ID无交集；没有删卷来模拟重启。

持久卷重建前后名称和创建时间完全相同：

- `campusclaw-phase5-compose-ced2bc23_go-mysql`
- `campusclaw-phase5-compose-ced2bc23_go-uploads`
- 两卷创建时间均为 `2026-10-11T01:33:23Z`。

users/materials/knowledge_entries按ID排序所得数据的合并SHA-256前后均为：
`0bcf6eb7e228615e9d60a83da16f2c1acd6f3bded871980dc57b82d948e3f846`。
原密码仍可登录；重建时故意注入不同seed密码，新密码401，证明没有重新播种或覆盖。原撤销版本未回退，Token有效性符合要求。

两份初始化样例、两份教师上传及一份20 MiB附件共**五份文件**重建后全部通过下载逐字节核对；各自SHA-256保存在result.json。原始数据库输出只留在测试进程内存，证据不记录密码哈希原值、密码、JWT秘密或完整Token。

## 隔离、保护与遗留风险

- 审计实际容器挂载：数据卷均以新project前缀命名；Nginx仅只读挂配置，无上传卷；Go/MySQL没有发布宿主端口；没有Flask、Qdrant或RAG服务。测试无可传入Demo源/URL的选项。
- 本轮未请求、切换、挂载或修改现有Demo及其三个数据卷，也未改`docker-compose.retrieval.yml`、Flask代码、课程报告、其他change或已有验收结果。未据此宣称已重新检查Demo在线健康状态。
- 起始235个工作区文件的hash用于核对全部已有改动；本轮已有文件仅修改默认Compose、Dockerfile.backend、Nginx、环境说明及本change tasks，其余已有文件保留。新增bootstrap及测试、当前验收文档。无reset/restore/clean/checkout/stash、commit/push/archive。
- 实际部署仍须注入独立随机JWT/数据库/初始化凭据，显式选择模式和项目，确认目标卷归属。migrate须事先完成离线导入及核验；已有fresh目标重复启动只核验，不能把这等同于导入旧数据。
- 默认入口是本机HTTP，尚未配置面向公网的HTTPS。生产Bearer传输需要另行配置HTTPS；本轮不触碰8080正式入口。
- 离线初始化/恢复只支持单写入实例；部署更新时先停写，不能把一次性初始化当在线维护。MySQL与文件卷必须成对保全；有新写入/撤销后不能直接退回旧快照。
- 本次保留了两个隔离项目的卷/容器及证据，会占用磁盘，未自动清理。测试凭据不落盘；重跑工具会新建项目，不接管先前项目。
- 此记录是6.3验收记录，不替代6.4完整运维手册或Phase6最终验证。OpenSpec strict validate通过；仅勾选6.3，6.4与Phase6继续未勾选。
