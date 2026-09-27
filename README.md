# 班级材料库 MVP

价值：让教师和学生在各自班级内安全保存、查询和读取教学材料。
场景：教师登录后上传 UTF-8 编码的 .txt/.md 文件，同班教师和学生查看材料列表、解析文本并下载原文件；服务端限制角色权限和班级范围。
不做：多班教师、上传时选择班级、公开注册、管理后台、向量检索、检索问答、对话助手、外部知识库对接、作业提交与批改、SSO、生产高可用（HA）。

## Docker Compose 启动

需要 Docker Desktop（Linux 容器）与 Docker Compose。所有命令在仓库根目录执行，应用端口统一为 **8080**。

`.env.example` 仅说明变量名称和用途。真实 `SECRET_KEY` 和样本密码仅通过服务端进程环境注入，不写入 `.env`、源码或构建参数。首次启动需要下面五个变量；已有数据库只必需 `SECRET_KEY`，密码环境变量不会修改已有账号。

PowerShell 中设置环境变量（密码输入不回显）：

```powershell
# 请将生成的密钥保存在服务端秘密管理设施中，后续启动注入同一值。
$keyBytes = New-Object byte[] 48
$rng = [Security.Cryptography.RandomNumberGenerator]::Create()
$rng.GetBytes($keyBytes)
$env:SECRET_KEY = [Convert]::ToBase64String($keyBytes)
$rng.Dispose()

foreach ($name in @('SEED_TEACHER_A_PASSWORD', 'SEED_STUDENT_A1_PASSWORD', 'SEED_TEACHER_B_PASSWORD', 'SEED_STUDENT_B1_PASSWORD')) {
    $secure = Read-Host $name -AsSecureString
    $ptr = [Runtime.InteropServices.Marshal]::SecureStringToBSTR($secure)
    try {
        [Environment]::SetEnvironmentVariable($name, [Runtime.InteropServices.Marshal]::PtrToStringBSTR($ptr), 'Process')
    } finally {
        [Runtime.InteropServices.Marshal]::ZeroFreeBSTR($ptr)
    }
}
docker compose up --build
```

后台运行可使用 `docker compose up --build -d --wait`。首次启动在 gunicorn worker 启动前自动执行 `scripts/init_db.py --fresh`，创建六类结构、A/B 两班四个账号以及每班一份样本材料。首次配置或初始化失败不会发布不完整的 `data/app.db`，补齐环境后可重试。已有数据库不会重置；启动会检查文件与材料、文本记录的一致性，缺失数据时拒绝启动。

打开 <http://localhost:8080/login>，使用以下账号和首次设置的对应密码登录：

| 账号 | 角色 / 班级 | 首次密码变量 |
| --- | --- | --- |
| teacher_a | 教师 / A | SEED_TEACHER_A_PASSWORD |
| student_a1 | 学生 / A | SEED_STUDENT_A1_PASSWORD |
| teacher_b | 教师 / B | SEED_TEACHER_B_PASSWORD |
| student_b1 | 学生 / B | SEED_STUDENT_B1_PASSWORD |

教师上传 UTF-8 `.txt`、`.md` 后，同班学生可刷新列表、阅读文本及下载附件。默认文件上限 20 MiB，可通过服务端 `MAX_UPLOAD_BYTES` 调整。

```powershell
curl.exe -i http://localhost:8080/health
docker compose ps
```

预期健康响应为 HTTP 200、`{"status":"ok"}`，容器状态为 healthy。数据库或存储不可用时响应 503、`{"status":"unhealthy"}`。缺少 `SECRET_KEY` 时 Compose 和应用均拒绝启动。

Compose 明确使用本地 HTTP 配置 `COOKIE_SECURE=false`；HTTPS 部署应将其设置为 `true`。会话 Cookie 使用 HttpOnly、SameSite=Lax。保持单实例运行，关闭 DEBUG。

## 服务端 Session

Session 状态保存在现有 `DATABASE` 的 SQLite `sessions` 表中，Compose 下为持久卷内的 `data/app.db`。浏览器只保存随机、不透明的 Session ID；数据库保存 ID 摘要、会话 JSON 和过期时间。身份、角色、班级通过服务端会话及用户表核验，不接受客户端自行声明；教师/学生权限和班级隔离保持不变。无需 Redis、ORM、额外服务或新环境变量，`SECRET_KEY` 仍为必填服务端配置。

登录成功会轮换 Session ID 和 CSRF token，会话自登录起固定 **8 小时**过期，普通访问不会续期。退出会删除当前会话、清除 Cookie，并通过用户会话版本使该账号其他设备的旧会话失效。过期时间在每次读取时检查，物理过期记录在启动时清理，无需专用维护命令。

新旧数据库都在现有初始化/启动流程中幂等准备会话表，不重置账号、材料或上传文件。升级前的 signed-cookie 会话不再接受，用户需要重新登录。正常重启沿用同一数据库即可保留未过期会话。若回退到旧 signed-cookie 版本，必须更换服务端 `SECRET_KEY` 并要求重新登录，避免升级前的旧 Cookie 再次有效；备份仍应覆盖整个数据卷。

## 显式初始化与账号创建

首次自动初始化后，无需再次执行种子命令。需要显式执行时：

```powershell
docker compose exec app python scripts/init_db.py --seed
docker compose exec app python scripts/create_account.py --class-name A --username my_teacher --role teacher
```

种子命令幂等，不覆盖已有密码或重复材料。创建新账号时安全交互输入密码；重复用户名会报错。若当前卷已由验收过程初始化，样本密码为验收环境中的随机值，可用第二条命令创建自己知道密码的新账号；不要通过删除卷重新获得账号。

## 保留数据的停止和重建

数据库与私有上传文件保存在 `material-data` 命名卷的 `data/app.db`、`uploads/` 下。上传暂存与最终文件同卷，应用以 UID/GID 10001 运行。

```powershell
docker compose down
docker compose up --build -d --wait
```

后续启动仍需注入原服务端密钥。不要使用 `down -v`，也不要删除卷或宿主数据目录。停止写入后备份整个数据卷，使数据库和文件保持同一时点；维护卷的可用容量。

## 自动化验证

```powershell
python -m venv .venv
.\.venv\Scripts\python.exe -m pip install -r requirements.txt
.\.venv\Scripts\python.exe -m pytest -q
```

测试使用隔离目录与运行时生成的凭据。OpenSpec 验证记录见 `openspec/changes/add-class-scoped-material-library/verification.md`。
