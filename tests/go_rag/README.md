# Go Phase 1 验收

首次运行请先阅读[可复现测试镜像准备](../IMAGE_PREPARATION.md)，从本仓库构建
`campusclaw-week5-rag-test:local`，核对基础镜像与新隔离资源；不要依赖旧机器的镜像、
连接描述文件或历史 runtime。离线导入镜像不等于已导入构建缓存。

课程网关真实验证使用已被 Git 忽略的 `.env.course.local`：

```powershell
powershell -NoProfile -File tests/go_rag/run_phase4.ps1 -Stage realmodel -ModelEnvFile .env.course.local
```

配置使用 `EMBEDDING_BASE_URL`、`EMBEDDING_API_KEY`、`EMBEDDING_MODEL=course-embedding`、`EMBEDDING_DIMENSION=2048`、`EMBEDDING_API_FORMAT=openai`，以及独立的 `DIALOGUE_BASE_URL`、`DIALOGUE_API_KEY`、`DIALOGUE_MODEL=course-chat`。两个 Base URL 均为 `https://ai-gateway.devops.hello1023.com/v1`。Embedding 只发送 model/input。真实测试仍限制最多 8 次 Embedding 和 4 次对话；每次使用新 collection 和已有独立测试库，只新增测试记录，不执行 schema 升级。证据写入被忽略的 runtime 目录。

Go 服务直接启动时需先将本地配置导入进程环境；Go 不自动加载 dotenv。Compose 可通过 `--env-file .env.course.local` 选择配置，但真实验证应使用上面的隔离 runner，避免启动默认项目连接其他数据库。Qdrant URL 中的 qdrant 服务名需在对应 Docker 网络内解析。

回退 Doubao 时可给同一 runner 传入已被 Git 忽略的 `.env.doubao.local`，保持原有模型、维度和独立 collection，并设置 `EMBEDDING_API_FORMAT=ark_multimodal`。不传 `-ModelEnvFile` 时仍保留从原 Demo 容器读取配置的旧路径；该容器必须存在。切换供应商时不要跨模型复用已有向量 collection，不删除旧索引。本次不创建或修改 Demo，也不保存不存在的旧密钥。

此目录是第四课 Go 接入的独立验收，不是第五课 Phase 6 的 apitest 实现。

已有测试项目可使用 `powershell -File tests/go_rag/run.ps1` 重跑。默认连接信息位于被忽略的 `runtime/week5-phase1-preflight/test-connection.json`，包含测试项目名和临时测试密码，不提交。runner检查项目命名、网络Compose所有权及服务运行状态；不创建、删除或停止现有服务，只创建新测试运行容器并保留其证据。

首次准备必须先只读核对 Docker 网络、宿主/VPN路由，并使用新的随机项目名及不冲突的 `WEEK5_TEST_SUBNET`；`compose.yml` 只有新内部网络和项目卷，不使用external课堂资源。当前已核对项目为campusclaw-week5-rag-ee706bec，子网10.254.241.0/24。不要复制这个子网用于第二个同时存在的项目。

服务不发布宿主端口，测试Go容器连接同一内部网络，通过mysql/qdrant服务名访问。文件卷uploads-test及编译缓存均只属于该项目，受控模型网关在Go测试进程中提供；业务运行不依赖Python/Flask/SQLite。测试需按包串行 `-p 1`，因为schema漂移测试会在该测试库短时加入并移除专用测试列，不能与业务启动校验并发。

catalog开发捕获测试仅用于首次批准的DDL；不要在未知数据库上捕获后接受漂移。正式严格校验使用已纳入源码的schema_rag_catalog.json，不从运行库自动重新生成。

Go RAG保持原GET search q/mode和POST ask question/mode；不增加官方login/search别名。默认Go配置RAG_ENABLED=false，便于个人本地准备不影响旧入口。开启须提供服务端网关变量、运行显式离线RAG schema升级（`go run ./cmd/admin -mode rag-schema`，停止目标库写入并备份后），再启用配置；初始化bootstrap在明确RAG_ENABLED=true时也可完成升级。HTTP启动绝不执行DDL，失败不就绪。默认Compose的rag profile提供私网Qdrant；本次没有启动默认Compose或正式部署。

回退应用前保留新索引及教学预备表；旧应用v1严格catalog不会接受v2，不能直接降级然后删除新增表以绕过检查。保持RAG_ENABLED=false并使用本分支兼容旧行为的版本可暂时停用RAG，无需覆盖数据库或文件快照。第5课预备表尚无业务API。
