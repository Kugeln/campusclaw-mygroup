## Context

核查日期：2026-10-13。OpenSpec CLI 1.13.0，`openspec/config.yaml` 使用 spec-driven；主 specs 为空，5 个旧 change 均有完成任务但未归档。未发现 AGENTS.md（排除无关 runtime 临时目录后检查）。旧 Session change 是历史需求，现行认证以 Bearer change 和实际代码为准。本轮未执行业务测试、访问数据库、操作容器或验证当前部署在线状态；历史验证文档仅作证据参考。

| 能力 | 实际证据与可复用边界 |
|---|---|
| 默认栈 | `docker-compose.yml`：Browser → Nginx → Go → MySQL 8.4.4；`go.mod` Go 1.26、net/http、MySQL/JWT；不是 Flask 默认部署 |
| 认证与角色 | `internal/auth/`、`internal/http/auth.go`：Bearer HS256、sub/ver/iat/exp、8 小时；每请求查当前角色/班级，logout 全设备版本撤销。Go 可直接复用 |
| 班级材料 | `internal/material/`、MySQL repository：上传、原文、列表、详情、附件、补偿与恢复已实现；UUID，跨班 ID 404，学生上传 403 |
| 第 4 课 RAG | `app/retrieval/`：800/80 切片、keyword ngram、vector ≥0.35 双重过滤、RRF k=60、top4、引用校验；`app/storage/retrieval.sql` 与 Qdrant 存储。完整 Python 实现已存在，但 Go 路由中没有 search/ask/reindex |
| 无命中 | `app/retrieval/answering.py`：精确“资料中未找到相关内容”、sources=[]、对话调用零次。跨班独有查询也是普通空结果 |
| 网关 | `app/clients/dialogue.py`：同步 `/chat/completions`，固定 system prompt、max_tokens=350、无 history/skill/stream；Embedding 支持 ark_multimodal/openai |
| 数据差异 | 默认 Go MySQL `knowledge_entries.plain_text`、无 chunks；assistants/skills 仅 id/name 占位。Flask RAG 身份、users/classes 在 SQLite，材料/chunks 在另一 MySQL，无 assistants/skills 业务路由 |
| 前端 | Go 内嵌 login/materials 模板与统一 Bearer API；Flask `knowledge.js` 有单次问答，等待完整 JSON；均无多轮/教师配置/SSE |
| 配置 | Go 使用 DB_*、JWT_SIGNING_SECRET；RAG 使用 MYSQL_*、QDRANT_URL、EMBEDDING_*、DIALOGUE_*；默认 Compose 无 Qdrant/网关配置，nginx 当前未关闭 buffering |
| 测试文档 | `tests/token_auth`、Go HTTP/repository、`tests/retrieval`、HTTP 对照/Compose 验证已有；无 OpenAPI/Swagger 路由，无 apitest/run_all.py。`.gitignore` 已忽略根 reports/runtime 与本地秘密 |

已阅读相关旧 proposal/design/tasks/spec 和验证记录：材料库、Session→Bearer、可追溯检索、Go 迁移。Go 迁移明确未包含 RAG；第 4 课旧规约中的 Python-only 实现边界不再适用于本次用户明确选择的 Go 接入，但原可观测安全/检索行为保持。

## Goals / Non-Goals

**本轮执行边界：** 只允许修改本变更规约文件及已有辅助规划文档。以下代码、数据模型、代理与部署内容均为未来实施设计，本轮不执行、不编写实现或测试脚本，不修改 Docker 配置及已有迁移规约。用户确认方案 2 仅确定 Go 目标架构，验证完成后停止等待审核和新的实施指令。

**Goals:** 默认 Go 独立运行；把已有第 4 课行为移植接入一个共享检索服务，而非为每种 API 重写检索。采用最小班级助手配置、固定技能、同步请求生命周期、MySQL 消息持久化与真实上游流式输出。

**Non-Goals:** 不把 Flask SQLite 身份库、RAG MySQL 和 Go MySQL 自动合并；不替换材料 UUID、Token 或认证 sessions 表；不创建通用 Agent 引擎。proposal 所列课外功能全部排除。`/api/agents` 只是官方接口名称，表示普通助手配置记录，不引入 Agent 框架。

## Decisions

### 1. 用户已决定在 Go 补接第 4 课

**前置依赖与新增范围分界：** Go 缺失的切片/索引、Embedding/Qdrant、三模式检索、引用校验、单次问答、无命中旁路及相应运行配置属于第 4 课接入前置项。现有 Go 身份、角色、班级与材料能力直接复用。提示词、技能、多轮会话、流式网关和消息审计是第 5 课新增项。Phase 1 必须单独证明第 4 课在默认 Go 入口可用；Phase 2 起依赖该验收结果，不能仅以 Python 历史测试通过代替。Phase 1 中第 5 课数据模型准备单独标识，不混入检索前置项的完成结论。

运行路径固定 Browser → Nginx → Go → MySQL，向量召回使用现有 Qdrant 协议，网关配置只在服务端。先将 Python 第 4 课的算法、失败语义和 HTTP 契约移植成 Go 检索/索引/网关模块，search、ask、会话依赖同一个服务；Python 留作参考和回归比较，不作为 Go 的 HTTP sidecar，不通过内部 HTTP 绕一圈调用 `/api/search`。

直接复用 Go principal、teacher guard、材料 repository、上传原文事务和恢复；在上传提交之后接索引，保留索引失败不回滚原文。保留三种检索、FULLTEXT ngram_token_size=2、ready 发布、重建清理确认、Qdrant 五字段白名单、既有 vision/2048 维度保护。向量服务失败不静默降级。助手非空 material_ids 必须在 keyword 查询、Qdrant 召回与 MySQL 回表中同时作为本班内的附加过滤，在排名和 limit 之前执行，不能先取 top4 再过滤导致遗漏合法证据。旧 `/api/ask` 仍单次同步、固定证据约束，不自动持久化为会话或注入教师技能；新会话才使用三者组合。

选择 Go 而非扩展 Flask 来自用户明确答复；这是必要新增工作，不能把 Python 已实现等同于 Go 已可直接调用。

### 2. 教师配置范围与技能最小加载

提示词作用于一个本班助手及其所有会话，不是全校全局，也不存浏览器。允许本班不同名称助手，以满足官方 agent_id 契约；课堂只需创建一个即可，不建设管理平台。每次 PUT 事务更新配置并递增 revision；收到消息时读取最新提交的助手及技能状态快照。既有会话不锁死初始提示词，在下一轮生效；进行中快照和旧回答不变。未启用助手不能创建/继续生成，返回隐藏资源 404。

复用 assistants、skills 占位表，明确新增 class_id、配置/版本字段并隔离旧无班级占位行；不凭 name 推断归属。技能定义采用服务端受版本控制常量或静态资源：名称、短说明、完整指令和版本；每班 skills 行保存启用状态。名称“解题引导”，短说明“以问题和步骤提示帮助学生自行推导”。完整指令要求分析已知与目标、提供下一步思路、追问理解、不给最终答案；关闭时只保留教师风格和证据约束，允许直接解释结论。安全/班级/证据约束优先，其次启用技能的不给最终答案约束，再教师提示词，历史和检索作为非指令数据。

渐进加载第一层名称/短说明常驻。检索前根据教师配置快照确定技能是否选中：本班 enabled 且被助手绑定则选中，否则不选中；此步骤不读取完整指令。第二层仅在检索取得合法本班证据、准备生成时加载已选中技能的完整指令；第三层本课没有脚本/参考资源，不加载也不执行。未来注册表可增加稳定 key、metadata、loadInstructions(version) 及只读参考资料列表，不把此扩展列为任务。不引入 LLM 技能路由。

配置快照中的 selected_skill_id 表示检索前选择，实际 skill_id/name/version/hash 表示生成时已加载并用于模型请求的技能。两者必须区分：无命中时可以保留选中记录用于解释决策，但实际技能为空，完整指令加载和模型调用均为零；SSE及历史的 skill_name 只展示实际技能。

### 3. 数据模型与持久化边界

| 表/字段 | 预计调整 |
|---|---|
| knowledge_entries | 保留 plain_text 及旧响应；为 Go 加索引状态、策略、cleanup_required，与现有正文建立明确映射；不盲目执行 RAG CREATE IF NOT EXISTS 期待旧表自动升级 |
| knowledge_chunks | 新增 UUID、entry/material/class 复合关系、chunk_index 从 1、chunk_text、状态及 FULLTEXT ngram；默认库目前没有此表 |
| assistants | 保留 id/name，增加 class_id、prompt、enabled、revision、updated_by、updated_at，班级名称唯一；skill_ids 绑定和 material_ids 范围存 JSON 并在事务校验。空 material_ids 表示全部本班材料，有值只缩小本班范围 |
| skills | 保留 id/name，增加 class_id、固定 skill_key、enabled、instruction_version、revision、updated_by/at；每班固定 key 唯一。不允许学生或任意 source 改完整指令 |
| conversations | BIGINT id、user_id、class_id、agent_id、title、created_at/updated_at、active_turn_id；索引(user_id,class_id,updated_at,id)，助手关系带 class_id |
| conversation_messages | BIGINT id、conversation_id、class_id、turn_id、序号、role、content LONGTEXT、status、created_at；assistant 保存 prompt 文本/版本快照、配置快照、skill_id/name/version/hash、选取原因、检索证据快照、最终 citations JSON、failure_reason |

user 消息 status=completed；assistant 状态 generating/completed/failed/interrupted，生成结果 status 单独为 ok/no_evidence。JSON 中不保存 API Key、Token 或密码。唯一会话序号、一个 active_turn 和行锁维护顺序、并发 409；不持有数据库事务跨模型调用。引用快照保存 number/material_id/title/chunk_id/chunk_index/chunk_text，不强制引用 FK 到可重建删除的 chunks，保留历史追溯。消息归属由 conversation 复合关系保证；读取仍以 principal owner/class 过滤。

Go 统一身份和教学数据在 MySQL，便于事务和 FK；不沿用 RAG 分裂身份库。现有 classroom 数据不在本 change 自动导入。若正式目标发现已存在外部 RAG 库，先核对 class/user/UUID/文件 hash 与 collection 关联，禁止仅以数值 ID 相同认定可合并。

### 4. 每轮链路、历史与引用

Bearer 当前身份 → owner/class/agent 检查 → 配置原子快照 → 根据教师配置确定技能是否选中（仅元信息） → 按本班检索 → 有合法证据且准备生成时加载已选中技能完整指令 → 服务器历史 → 同一次模型调用 → SSE → 终态和来源保存。选择不等于实际使用；无命中仍返回精确“资料中未找到相关内容”、空实际技能、空引用，完整指令加载和模型调用均为零，无数据分支也持久化。受控调用记录必须证明选择先于检索、完整加载后于合法证据确认。

追问检索使用“上一条已完成用户问题 + 当前问题”（首轮仅当前问题），仅为查询上下文，不把旧引用当本轮证据；必要时裁剪到服务端查询上限。模型历史取最近最多 6 个 completed 且生成结论 ok 的完整轮次，设 6000 Unicode 字符历史预算，按完整旧轮次从早到晚移除；系统/教师/技能/当前问题/本轮 top4 证据不得被历史挤出。若剩余必需输入仍超网关预算，明确失败而非任意截掉证据。failed/interrupted/no_evidence 不进入可依据的模型历史。

上游新增真正 stream=true 解析，保留独立非流式 ask；不能先取得完整字符串再拆分。在逐段显示期间文本为未完成草稿；最终扫描 [n]，要求至少一条合法引用且全部编号合法，才生成最终 citations。引用失效/缺失则 failed，不能伪造引用补齐。

### 5. SSE 与消息事务

沿用官方事件名称和核心字段，增加终态所需可选字段；详见接口对照。POST `/api/conversations/{id}/messages` JSON `{content}`，content trim 后 2–500 字符。额外 prompt/skill_ids/history/class_id/agent_id 等配置/身份字段返回 400（官方 agent_id 忽略，差异有记录）；身份只来自 principal。

流前完成权限、参数、检索、上游连接/首包就绪，失败仍是 JSON；暂不写消息。成功后事务插入 user 和 generating assistant，提交配置、证据快照并占用会话，然后发送 headers/meta；事务失败取消上游并返回 503。无命中不连接上游，直接同事务写入固定回答，再发送 meta/delta/done。

```
event: meta
data: {"status":"ok","citations":[],"skill_name":"解题引导","message_id":12}

event: delta
data: {"text":"先分析题目给出的条件。"}

event: done
data: {"message_id":12,"citations":[{"number":1,"material_id":"uuid","title":"教材.md","chunk_id":"uuid","chunk_index":1,"chunk_text":"原文"}],"skill_name":"解题引导","state":"completed"}

```

meta citations=[] 为尚未最终校验的来源；done 才给最终实际引用。HTTP Content-Type `text/event-stream; charset=utf-8`、Cache-Control `no-cache, no-store`、`X-Accel-Buffering: no`，每帧以空行结束，delta 文本 JSON 转义。每 15 秒空闲发 `: heartbeat` 注释，不算业务事件；done 唯一，之后关闭。官方没有 [DONE]，不增加第二种结束标志。

每个输出帧（可聚合上游小 token，目标不超过 100ms）先提交累计 content 再刷新；客户端看见的每段都已持久化。引用校验和最终 completed/citations/释放 active_turn 在事务成功后才能 done。流中异常发送 `event:error`、`data:{"error":"回答失败"}` 并关闭，不再 done；error 消息仅人类可读字符串，HTTP 仍 200。失败状态/前缀/技能留库，数据库失败无法即时标状态则保持 generating 并由恢复标 interrupted，不能报告完成。

浏览器断连取消上下文和网关请求，用短时独立恢复写入标 interrupted、释放占用；崩溃恢复对遗留 generating 做同样处理，不引入队列。单实例课程服务内管理活动请求，启动恢复只触及自身已失去活动连接的轮次；未来多实例需租约，本课不承诺多副本。完成提交后 done 传输丢失，历史仍是 completed，前端重新读取确认，不回退。自动断线重连/重放不支持，用户重发是新轮次。

前端复用 CampusAPI 同源 Bearer、credentials omit，以 fetch POST + ReadableStream + streaming TextDecoder 解析 SSE，不能使用原生 EventSource 把 Token 放 URL。显示草稿、最终引用、实际技能、失败/中断标签；401 清身份，403 不退出，重新登录取消正在显示的旧流。Nginx 对该 SSE 路径关闭 proxy_buffering/cache、保持 Header 透传并匹配心跳超时；普通上传行为保持。

### 6. API、Swagger 与测试

具体差异、路径/字段/状态和范围见 [api-differences.md](api-differences.md)；脚本路径/数据库断言/报告见 [apitest-plan.md](apitest-plan.md)。新增 API 优先采用官方 agent/conversation 形状，技能采用固定内置 toggle 最小接口，明确不支持任意技能 source CRUD。旧路径/响应/UUID 保留，不为严格复制官方整站重构认证。

本地 `/openapi.json`、`/swagger/index.html`（`/swagger/` 同页面）匿名可读；OpenAPI 3.0.3 用 bearerAuth，主要 API Try it out，SSE 增加小型 fetch 调试展示（普通 Swagger 对无限流 UI 不足）。文档按仓库 Go 选择一个单一来源的注解/生成或静态契约加一致性校验，不能复制官方 JSON 冒称已实现。文档覆盖既有及本课 API，并明确选用哪个实际响应和兼容差异。

默认 Compose 后续接 Qdrant 私网及外部网关所需出站网络、ngram=2、持久卷和环境变量；保留现有 DB_*、JWT_SIGNING_SECRET，新增 QDRANT_URL、EMBEDDING_*、DIALOGUE_*，不改为官方 SESSION_SECRET。首次 Go 检索初始化不能自动清理旧 collection。配置模板只列变量名及说明，不包含真实值。

## Risks / Trade-offs

- [Go 尚无检索，工作量高于直接扩展 Flask] → Phase 1 完成 Go 第 4 课闭环回归后才进入生成；照现有算法和契约，不再发明另一检索架构。
- [两个 MySQL schema 不同、class ID 可能碰撞] → 对 Go 库显式增量迁移，保留 plain_text/UUID；不自动合并外部课堂库，索引指向的数据身份必须匹配。
- [逐帧落库增加写入] → 有限帧聚合、课程单实例；优先保证已显示文本可恢复，不引入队列或仅在结束时存储。
- [LLM 引导行为不完全确定] → 测试捕获实际请求、受控网关检查组合输入，真实模型按固定题目启停对照复验；关键词断言不能替代教学效果检查。
- [生成中提示注入及不合法引用] → 固定权限/技能约束优先，历史和材料为数据；最终引用校验失败不可 completed。
- [Swagger 默认可能等待 SSE 结束] → 提供文档事件样例和实际 fetch 流调试面板，验证首片出现时间，不宣称只看到完整响应就是流式验收。

## Migration Plan

此节只规划，当前不执行。实施时在隔离 Go 测试库/文件卷/Qdrant collection 验证增量 schema、旧 Go 八表/原文/UUID/Token 版本不变；更新 schema catalog/显式初始化流程使既有版本可升级，重复升级幂等，失败不发就绪。旧无班级助手/技能占位数据保留并排除可用列表，由明确管理步骤分配，不能猜测归属。

接入 Go 索引和检索、受控独立网关验证第 4 课 → 配置与会话 → 流式前端 → Swagger/全量相关验收。正式环境部署另行授权；仅新增列/表不自动删旧字段。回退应用前备份新会话与配置并确认旧版本 schema 校验是否接受增量结构，采用明确兼容版本或只读停用新入口，不能回退旧快照覆盖新消息；不删除课堂向量或原文件。

## Review decisions

Phase 1 的第4课 Go 接入必须有独立验收记录，列明默认 Go 入口、实际命令/结果、隔离 MySQL/Qdrant 与受控网关证据及未执行项；Flask历史、规划完整或strict验证通过均不能替代。Phase 1 全部完成并通过该门槛前不得实施 Phase 2。此门槛不得推迟到 Phase 6 最终回归。

官方兼容的剩余验收风险见 api-differences.md 的风险表：当前缺少“老师是否要求官方脚本原样运行”的证据，不能承诺原样通过，也不据此新增别名、POST检索或技能source CRUD。若验收明确要求这些契约，必须由用户决定是否追加兼容适配规划，保留既有接口，不更改认证生命周期或材料ID。

后端目标已由用户确认 Go。建议本次评审接受接口差异清单中的最小兼容策略：保留认证和旧 search/ask、UUID/1-based 切片，新增会话采用官方形状，技能只 toggle。如果要求老师的全量官方测试原样通过，则登录生命周期、整数 ID、技能 source CRUD、检索响应及会话并发语义都需扩展为另一个兼容变更；当前不擅自加入。本次规约不是实施授权。
