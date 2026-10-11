# Demo 验证记录（2026-09-30）

## 本轮授权与边界

用户要求最小课堂闭环、保留现有 Bearer Token，并明确不跑全量 Token/迁移测试、不迁移数据、不 commit/push/archive。旧规划中的 Session/CSRF、仅 API、全量基线要求与此次指令冲突，按用户指令实施；规划正文没有重新 Propose 或改写。

## 已运行

- `.venv/Scripts/python.exe -B -m pytest -p no:cacheprovider tests/retrieval/test_core.py tests/retrieval/test_clients.py -q`：5 passed in 1.16s。
- `.venv/Scripts/python.exe scripts/check_retrieval_demo.py`：真实 MySQL 8.4.4、Qdrant 1.13.4、受控 HTTP Embedding/对话网关，6 passed in 9.86s。只构建一次镜像；临时项目已停止并清理自身测试卷。
- `node --check app/static/knowledge.js`、`node --check app/static/materials.js`：通过。
- `openspec validate add-traceable-vector-retrieval --strict`：`Change 'add-traceable-vector-retrieval' is valid`。
- `git diff --check`：通过（只有既有 Windows LF/CRLF 提示）。

## 闭环证据

真实 Token 登录 A/B 两班 teacher/student → 教师分别上传约 1 KB 的 UTF-8 md/txt → 原文提交后进入 Embedding → Qdrant 写入确认 → MySQL ready → 三种检索 → `/api/ask` 返回回答和 `[1]`，来源文件名 `demo-a.md`、摘录等于 MySQL 原文。

MySQL 幂等建表与 ngram=2、复合关系拒绝跨班关联、Qdrant 五字段 payload/UUID point ID、双重班级过滤均检查。受控网关只用三维测试向量，答案是测试响应，不代表真实 LLM 已验证。

核心验证包括 800/80 Unicode 切片、两种边界策略、RRF、0.34/0.35 阈值、HTTP 无效向量和网关错误、最多四条证据、无证据零对话调用、非法来源 502。真实服务验证包括双向四账号三模式隔离、伪造 query/JSON/header/cookie 身份、学生上传/重建 403、跨班资源 404、无效策略 400、并发重建 409、第二片 Embedding 失败无向量写入、原文件仍可下载、成功重建与 MySQL 权威 recovery。

## 已实现但保留任务未完成的部分

- 1.3：提供独立 Compose/配置；未做完整停机启动和配置错误矩阵。
- 2.3：列表/详情/下载/上传/recovery 已接入同一 MySQL authority；未完成迁移/样本 seed 适配和旧 SQLite 全量事务回归。旧 seed 在 MySQL 模式明确拒绝执行。课堂使用新库手动上传小材料。
- 3.3：真实 Qdrant 路径与 payload 已验；collection 配置不符拒绝逻辑已实现，尚未真实构造维度/距离冲突验收。
- 3.4：上传提交后索引、失败状态与全体发布已实现；第二片失败验证在重建入口完成，尚未针对上传入口做同一故障测试。
- 3.5：教师重建及失败/恢复流程已实现；尚未做删除失败与启动中断的完整故障注入。
- 4.1：真实 MySQL ngram 正常匹配、跨班及 failed 排除已验；尚未单独验收多条相关性排序和 pending 状态矩阵。

1.1、2.2、6.1、6.5 保持未完成：全量基线、材料迁移、大范围故障注入、最终全量回归不属于本轮已执行范围。

## 课堂就绪状态

接口最小闭环通过；问答页面已实现，模板与 JS 语法已检查，尚未进行浏览器交互验收。

### 2026-09-30 真实模型链路追加验证

在受控网关验收之外，本轮使用真实火山方舟模型完成了一次端到端课堂链路：

- Embedding：`doubao-embedding-vision-251215`，多模态接口 `POST /embeddings/multimodal`，实际返回维度 2048（替代原规划的 `doubao-embedding-text-240715`/2560，因账户未开通该 Embedding 模型权限）。
- Dialogue：`doubao-seed-2-0-lite-260428`，OpenAI 兼容 `POST /chat/completions`。
- 实现层最小改动：`app/clients/embedding.py` 改为多模态端点与 `input=[{type,text}]` 结构，并按真实返回把 `data[0]['embedding']` 调整为 `data['embedding']`（多模态响应 `data` 为对象而非数组）；测试网关与两处断言同步更新，5 passed。
- 真实链路：teacher_a 登录 → 上传 demo-a.md → 真实 Embedding 写入 Qdrant（collection dimension=2048）→ hybrid 检索命中 demo-a.md 切片（score≈0.033）→ 真实 Dialogue 回答“星河读书活动将在周三下午三点开始，举行地点是图书馆二楼[1]。”→ sources[0].title=`demo-a.md`、number=1。
- 无依据问题“木星探测器的燃料配方是什么？”：检索零命中 → `answer='资料中未找到相关内容'`、sources=[]、对话调用次数为 0，符合 spec。
- 真实模型 base URL/key/model/dimension 仅通过进程环境注入运行容器，未写入任何 Git 跟踪文件、源码或 OpenSpec artifact；`git status`/全树 grep 确认无密钥泄漏。

配置方法、启动命令、1213 字节示范材料与问题见 [Demo 说明](../../../docs/retrieval/README.md)。


## 确认契约后的最终实施与验证（2026-09-30）

本节覆盖上文历史待完成列表。用户明确授权 Bearer 等价验收与保留 vision/2048；仅修改本 change 冲突规约。新增标准 openai 协议通过显式配置选择，默认保持 ark_multimodal。认证等价映射见 docs/retrieval/auth-test-mapping.md。

### 验证结果

- 规约修订后及最终 strict validation 均通过。
- 首次等价基线：84 passed、1 个真实服务入口待 Docker 验证，239.23s。
- 最终全量 pytest：87 passed、10 skipped，239.56s。10 项都是有环境门槛的真实服务用例，并已全部在下述独立 Docker 套件实际执行成功，不是以 skip 替代验收。
- 独立 MySQL 8.4.4 / Qdrant 1.13.4 套件：16 passed，23.28s。包括原真实链路及新增 9 项实际服务测试。
- 配置检查追加 3 passed：缺配置只输出名称、不依赖 Qdrant 的 factory、Compose 不公开数据库端口。该组随后也纳入最终 87 项。
- Git diff --check 通过；没有 commit/push/archive。

### 任务证据

| 任务 | 证据 |
| --- | --- |
| 1.1 | tests/test_auth.py、test_sessions.py 与材料页面断言的 Bearer 等价映射；全量基线通过 |
| 1.3 | test_configuration.py；真实 keyword 在向量依赖不可用时可用；原有服务正常启动 |
| 2.2 | test_explicit_migration_and_mysql_seed：只读源库/文件、原 ID/关联/时间/正文、重复一致、冲突不覆盖、Token 保留；迁移只跑临时库 |
| 2.3 | 原 SQLite 权限/事务/补偿回归；test_mysql_upload_transactions_and_authoritative_recovery；原真实集成双向访问 |
| 3.2 | 两种协议的 HTTP 请求/响应、维度/有限数/零范数、超时与错误响应检查 |
| 3.3 | test_qdrant_contract_and_dimension_preservation：真实维度/Cosine/索引/删除确认；不匹配配置保留旧索引；实时课堂向量摘要不变 |
| 3.4 | test_upload_embedding_failure_and_committed_original：上传提交先于模型、第二片失败无 upsert、全部 failed、原文件可读 |
| 3.5 / 6.1 | test_index_failure_recovery 的 partial/cleanup/ready/interrupted；原集成教师/学生/跨班/非法策略/409；恢复先清理再重建 |
| 4.1 | test_keyword_states_ordering_and_unavailable_vectors：真实 ngram、排序、状态过滤、零 Embedding/Qdrant 调用；原链路双向跨班过滤 |
| 6.5 / 6.6 | 最终全量与真实集成结果、strict validation、工作区与秘密检查 |

### 现有课堂数据保护

所有有写入、迁移或故障注入的测试都使用新建的独立测试 Compose 项目及临时 SQLite/文件目录，不连接课堂实例。仅清理该临时测试项目自身资源。

课堂实例仅恢复原容器并读取：MySQL 原有 2 份材料均 ready；Qdrant 原有 collection 保持 Cosine/2048、2 个点。读出全部 point ID/payload/vector 后排序计算摘要，操作前后均为 `ea42260dc50d5bd0dd09965a36d962186ce737d274afa7021393e1b5d5cc86cb`。App 恢复前确认无暂存/孤立文件，因此未触发文件删除。没有迁移或重新初始化课堂账号、修改密码、上传或重建原材料。

待提交文件检查共 74 个，真实 API Key 匹配为 0。Key/Token 未写入本文或源码。

### 当前真实模型复验限制

App /health 与 /login 均 200，teacher_a 使用原有密码登录 200。真实 hybrid 问答返回 503；独立最小 Embedding 诊断在容器内约 15 秒超时，宿主机同样超时，未收到能判断凭据有效性的 HTTP 响应。没有用受控网关替换课堂配置，没有调整阈值或伪造答案。

内置 Browser 连接失败：缺少 browser-service.mjs。既有 Edge/Playwright 最小真实交互未取得完成证据，等待已终止；最终全量中的隔离浏览器测试通过，但不能代替当前课堂真实模型页面验收。

24/24 表示当前 change 的约定实现、等价行为和真实 MySQL/Qdrant＋受控网关验收完成；外部火山方舟网络恢复后仍需完成两道课堂问题的实时复验。

### 2026-09-30 minimal live network follow-up

- No business code, runtime configuration, material or vector writes performed; prior 87-test regression and 10 real-dependency tests were reused, not rerun.
- App was running. Embedding base URL matched the configured Ark v3 endpoint. Container had no proxy environment/settings; Windows urllib detected registry proxy schemes despite no proxy environment variables.
- Container DNS and TCP succeeded; the initial TLS handshake timed out. Host direct TLS/HTTP succeeded; a credential-free request returned 401. Host direct real multimodal embedding returned 200, dimension 2048, in 0.75 seconds.
- Container DNS matched host DNS. A TLS 1.2 diagnostic received HTTP 401. A subsequent unchanged/default container embedding request returned 200, dimension 2048, in 14.62 seconds. This demonstrates intermittent reachability, not a verified permanent network repair or proof of a TLS-version-specific defect.
- One live teacher_a login succeeded (200); the supported existing-material question returned 503 after 16.31 seconds. No answer or citation was obtained. The end-to-end classroom acceptance remains blocked by intermittent container-to-provider connectivity; no timeout increase or implementation workaround was applied.
- Read-only data check: two materials remain ready; collection remains Cosine/2048 with two points; full sorted point/payload/vector fingerprint matches the previous baseline.
- Task checklist remains 24/24 with no unchecked items. This does not imply that the current live classroom acceptance passed. Existing strict validation remains applicable because no planning contracts changed.
