# Apply 验证记录

## 2026-09-23：恢复任务 6.1

- 读取 proposal.md、design.md、specs/ 下全部四份 spec.md、tasks.md。
- tasks.md 实际进度：22/30；第一个未完成任务为 6.1。此前已完成任务未重新执行。
- `docker compose version`：exit 0，Docker Compose v5.5.1。
- 使用宿主权限和仅保存在进程环境中的临时 SECRET_KEY 执行 `docker compose config --quiet`：exit 0。
- `docker compose config --format json` 在内存解析并断言：仅有 app 服务，8080:8080，数据库路径为 /var/lib/material-library/data/app.db，uploads 路径为 /var/lib/material-library/uploads，二者位于同一命名卷；通过。未输出展开后的密钥值。
- `.env.example` 检查：仅包含注释、变量名称和占位说明；通过。
- `docker compose build`：exit 1。拉取 python:3.12-slim 的元数据时，访问 auth.docker.io 的匿名 token 端点发生 TCP 443 连接超时。
- `curl.exe -4 -I --connect-timeout 10 --max-time 15 https://auth.docker.io/`：curl 错误 28，IPv4 同样连接超时。
- `docker image ls`：无可复用本地镜像；Docker registry mirrors 为空。
- 因镜像未构建成功，非 root 用户对持久目录的真实写入及同卷验证尚未执行，6.1 保持 `[ ]`。未进入 6.2 或之后任务。
- 未删除、重置或覆盖既有应用数据；未修改 openspec/specs/ 主规格；未修改宿主 DNS、代理或 Docker 全局配置。

恢复条件：Docker 能成功拉取 `python:3.12-slim`。恢复后重新执行 6.1 的镜像构建及非 root 同卷读写验证，通过后才勾选并进入 6.2。

## 2026-09-23：本地基础镜像就绪后继续 Apply

此前 Docker Hub 网络阻塞已解除，以下均为实际执行结果。没有重置或删除卷；验收生成的材料保留在项目数据卷中。

| 任务 | 可复现命令 / 检查 | 结果 |
| --- | --- | --- |
| 6.1 | `docker compose config --quiet`、`docker compose build`；`docker compose run --rm --no-deps app python -c "import os,pathlib; assert os.getuid()!=0; roots=[pathlib.Path(os.environ['DATABASE']).parent,pathlib.Path(os.environ['UPLOAD_ROOT'])/'tmp',pathlib.Path(os.environ['UPLOAD_ROOT'])/'files']; [(p/'write-probe').write_text('ok') for p in roots]; [(p/'write-probe').unlink() for p in roots]; assert len({p.stat().st_dev for p in roots})==1"` | exit 0；镜像成功构建，非 root 用户可写，同卷成立；端口、挂载及示例检查通过 |
| 6.2 | `.venv/Scripts/python.exe -m pytest tests/test_startup.py -q`；`docker compose run --rm --no-deps -v "${PWD}/tests:/checks:ro" app python -m pytest /checks/test_startup.py -q -p no:cacheprovider` | 本地和真实容器各 1 passed；首次失败无 app.db、补齐后重试成功、六类结构及两班种子完整、重复启动无变化。首次容器运行未禁用 cacheprovider，只有只读测试目录缓存警告，不影响通过 |
| 6.3 | `docker compose up --build -d --wait --wait-timeout 90`；`curl.exe -i http://localhost:8080/health`；`docker compose ps`；`.venv/Scripts/python.exe -m pytest tests/test_health.py -q` | exit 0；HTTP 200 / `{"status":"ok"}`，app healthy，端口 8080；依赖不可用返回 503 / `{"status":"unhealthy"}`，1 passed |
| 6.4 | `.venv/Scripts/python.exe scripts/verify_compose.py startup` | exit 0；真实 Compose 重建启动、教师及学生 HTTP 登录、缺少 SECRET_KEY 的 Compose 和应用启动拒绝均通过 |
| 6.5 | `.venv/Scripts/python.exe scripts/verify_compose.py persistence` | exit 0；真实上传后 down / up --build，原账号登录成功，数据库摘要及所有文件摘要不变，本班列表、解析文本、文件字节保持完整 |
| 7.1 | `.venv/Scripts/python.exe scripts/verify_compose.py scenarios` | exit 0；匿名页面 302 / API 401、教师和学生跨班详情及文件 404、学生上传 403 且无变化、教师上传 201 且双表各增一条、同班可见、实际 Compose 重建后完整保留 |
| 7.2 | `.venv/Scripts/python.exe scripts/verify_compose.py security` | exit 0；运行凭据不在仓库文件、镜像配置、页面/API 和日志中出现；数据库仅保存 PBKDF2 密码哈希；真实 HTTP 400/413 无双表和文件残留；容器内 24 passed（67.08s），覆盖存储失败、第二张表写入失败及提交阶段失败的 5xx 回滚 |
| 7.3 | `openspec validate add-class-scoped-material-library --strict` | exit 0；`Change 'add-class-scoped-material-library' is valid`，无 strict validation error |

复现前按 README 注入环境并启动服务。`verify_compose.py` 从当前容器读取服务端环境到内存，复用验收账号密码，不输出或写入凭据；要求当前容器环境中的样本密码与最初初始化密码一致。脚本仅用于本地验收，会增加验收材料，persistence/scenarios 阶段会停止并重建当前 Compose 服务，保留命名卷。

7.2 验收过程修复了容器测试运行配置：测试应挂载至 `/app/tests` 以保持仓库相对路径；测试子进程移除服务级 `DATABASE`、`UPLOAD_ROOT` 和 `COOKIE_SECURE`，让各测试使用隔离目录和自身配置。修正前三个失败用例为会话配置、数据库结构初始化、种子初始化；修正后针对性复验为 3 passed。应用服务的环境、持久目录和已有数据没有为此更改。

最终 tasks.md 为 30/30，均在对应 verify 通过后逐项勾选。本次仅调整此 change 的 design.md 以明确任务 6.2 已要求的首次自动初始化，并统一 Compose 文件名称；没有修改 openspec/specs/ 主规格，没有实现 Non-goals，也未归档 change。服务保留运行，登录地址为 http://localhost:8080/login，健康地址为 http://localhost:8080/health。样本凭据为运行环境中生成的验收值，未写入本文；使用自己的账号可按 README 的安全交互命令创建。
