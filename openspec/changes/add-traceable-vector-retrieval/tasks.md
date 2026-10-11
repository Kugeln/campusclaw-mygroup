## 1. 基线与接入准备

- [x] 1.1 在隔离环境运行按 Bearer 契约等价适配后的全量 `python -B -m pytest -p no:cacheprovider`，记录用例和通过数；保留第 3 课全部认证、Token 撤销、角色、材料事务及班级隔离断言，仅将 Session 专属断言替换为有明确映射的 Bearer 等价断言，不跳过或弱化安全与事务行为。
- [x] 1.2 建立薄 HTTP 层、可信 scope/Evidence、小型 storage 边界及独立检索/网关模块接线；以不创建 Flask request context 的核心测试验证检索/切片/RRF 可独立调用，并检查核心不导入 Flask 或数据库驱动。
- [x] 1.3 为开发和集成验收配置 MySQL、Qdrant 与独立课程网关环境变量，锁定所用依赖/镜像版本；验证 Compose 不公开数据库端口、不增加 Go/Nginx/队列，Qdrant 停机不阻止 keyword 服务启动，缺配置只输出变量名而非密钥。

## 2. 必要的数据存储迁移

- [x] 2.1 实现 MySQL materials、knowledge_entries.body_text 和 knowledge_chunks 模型及复合关系/状态索引；在真实 MySQL 验证幂等建表、跨班关联拒绝、FULLTEXT WITH PARSER ngram 和 ngram_token_size=2，保证 Qdrant point ID 可直接对应 UUID chunk ID。
- [x] 2.2 实现显式 SQLite 材料两表迁移及新库样本初始化；以临时旧库验证 ID/班级/上传者/时间/正文哈希/文件关联保持、重复迁移一致、冲突不覆盖，账号/Token 撤销版本 不迁移，原登录仍有效。
- [x] 2.3 将材料列表/详情/下载/上传/seed/recovery 接入一致的材料 repository，保留 plain_text API 映射及原文件事务补偿；运行旧 SQLite 回归并在 MySQL 重复原文提交、事务失败、文件清理和双向读取权限检查，证明不会用旧库快照删除新文件。

## 3. 切片、Embedding 与向量索引

- [x] 3.1 实现固定 800/80 默认切片、字面分隔符和 Markdown 标题策略；通过 Unicode、长段、短尾、围栏标题、边界重叠、序号和非法策略测试，证明相同输入产生相同文本序列且不丢失非空白正文。
- [x] 3.2 实现独立 Embedding client；通过可配置 openai /embeddings 与 ark_multimodal /embeddings/multimodal 的可控 HTTP 网关测试验证 model/input、固定维度、有限数/非零范数、超时及错误响应处理，密钥/向量不写日志，不依赖对话 client。
- [x] 3.3 实现独立 Qdrant client 和 campusclaw_chunks 初始化；在真实 Qdrant 验证 Cosine/维度检查、五字段 payload 白名单、UUID point ID、class_id 索引/过滤及写入删除完成确认，确认未存正文/标题且查询不返回向量值。
- [x] 3.4 在原文成功提交后接入默认索引，先存 pending chunks、取得全部 Embedding、再写向量并事务性发布全部 ready；验证上传仍 201、索引前原文已可读取，第 N 片 Embedding 失败时所有本次切片 failed、Qdrant 写入次数为零、原文文件不变。
- [x] 3.5 实现本班教师重建 API、旧向量先删确认再删旧切片/写新切片、失败待清理及启动恢复；验证学生 403、跨班/不存在统一 404、非法策略不删除、同材料处理中 409、失败不能跳过清理发布新索引。

## 4. 三种检索和可信班级注入

- [x] 4.1 实现 MySQL keyword repository 查询；真实 ngram 测试确认按相关性降序、只返回当前班级 ready 切片，pending/failed/外班被排除，Embedding/Qdrant 调用次数均为零。
- [x] 4.2 实现 vector 路径的查询 Embedding、Qdrant 班级过滤和 MySQL 同班 ready 二次回表；验证 0.34 丢弃、0.35 保留、分数排序、伪造外班 ID/不存在 ID/失效切片丢弃，所有正文取自 MySQL。
- [x] 4.3 实现纯函数 RRF 与 hybrid 编排；以手算 1/61+1/63、缺席贡献 0、重复 ID、同分稳定序、原始分数缩放和正常空路案例验证 k=60 的名次融合，依赖失败不能伪装为空路降级。
- [x] 4.4 接入 GET /api/search，默认 hybrid，由现有 authenticated 生成并注入 scope；验证空白/非法模式 400 且零网关调用、未登录 401、身份字段被忽略、响应无向量、合法空结果 200/items=[]。

## 5. 基于证据的简短问答

- [x] 5.1 实现独立对话网关 client，仅接受问题和编号证据，使用独立服务端配置及有限输出预算；替身断言输入无向量/外班文本，材料指令作为数据，超时和错误响应不生成本地补答。
- [x] 5.2 实现 POST /api/ask 的 Bearer 鉴权、默认 hybrid 及共享 search(limit=4) 调用；通过调用记录证明不复制三路检索、仅前 4 个有效切片进入对话，零命中及全被回表过滤时精确返回「资料中未找到相关内容」、sources=[]、对话调用次数为零。
- [x] 5.3 实现来源编号映射与响应校验；验证有效 [1] 等编号可追溯到本次 MySQL 标题/切片序号/正文，sources 仅含被引用证据，空回答/缺失或越界编号/对话失败均返回 502，不伪造来源。

## 6. 失败、安全与最终验收

- [x] 6.1 验证 Qdrant 不可用时 keyword 仍可工作、vector/hybrid 返回 503 且不调用对话；覆盖向量部分写入、删除不可确认、ready 提交失败和索引中断，证明原文保留、非 ready 数据不可见、恢复后先清理再重建且不伪报原子成功。
- [x] 6.2 对 A/B 两班各 teacher/student、keyword/vector/hybrid 执行双向隔离矩阵；伪造 query/JSON/header/身份 Cookie 的 class_id/user_id/role，断言无外班摘录、来源或模型输入，外班独有查询返回普通无结果而非 403/404，学生上传/重建保持 403。
- [x] 6.3 运行真实 MySQL/Qdrant 加受控网关的集成链路：上传→切片→向量→三模式→问答→新策略重建；核验正文仅从 MySQL 获取、payload 无正文、两次班级过滤有效、全部 ready 后才可见。缺少必需集成环境时保留任务未完成，不能用 mock 通过代替真实服务验收。
- [x] 6.4 更新 README 的最小架构、服务端配置、材料迁移/备份、失败索引重建和 API 示例；核对无客户端密钥/向量、无无关 UI 改动、无 Go/Nginx 迁移及额外微服务，并记录网关地址/模型/维度作为运行时配置而非源码常量。
- [x] 6.5 运行最终全量 pytest 和新增集成测试命令，逐项对照 spec 记录结果；要求旧安全和事务测试不回归、新测试全部通过，检查 scope 仅涉及必要的材料存储接入和本班检索问答。
- [x] 6.6 执行 `openspec validate add-traceable-vector-retrieval --strict`，确认规划与实际行为一致并交付测试证据；不自动部署、同步或归档 change。

## 历史 Demo 执行说明（2026-09-30，后续由下方确认覆盖）

- 以用户本轮指令为准，保留当前 Bearer Token，不回退 Session/CSRF；最小知识库问答页面属于用户明确授权。proposal/design/spec 的旧认证和 API-only 描述没有重写。
- 不执行 1.1 / 6.5 的全量回归，不运行 SQLite→MySQL 迁移，不推进其他 change。未覆盖完整验收条件的任务保持未勾选，即使其中的 Demo 路径已经实现。
- 第一阶段：核心及独立 HTTP 网关测试 5 passed。第二阶段：加真实 MySQL/Qdrant 的最小套件 6 passed in 9.86s，两份约 1 KB 材料。
- strict validation 命令已通过；这仅证明 OpenSpec 文档结构有效，不代表未勾选任务或真实模型验收完成。真实 Embedding/LLM 配置缺失，页面尚未进行浏览器交互验收。
- 最小闭环到此停止。详细证据及未完成边界见 [verification.md](verification.md)。

## 已确认的继续实施契约

- 用户已确认保留 Bearer，并允许 Session 验收作等价适配；保留原权限、隔离、上传限制、撤销及事务保障。
- 保留当前 vision/2048、MySQL 数据和 Qdrant collection；增加显式协议配置，禁止不匹配维度写入。3.2 因新增兼容路径重新打开，验证后才勾选。
- 继续完成剩余任务；迁移及故障测试使用独立环境，绝不迁移、清空或重建现有课堂数据。未完成任务不因本次契约调整直接勾选。

- 第一阶段继续验证：全量 Bearer 等价基线 84 passed、1 个真实服务入口待 Docker 验证（239.23s）；1.1 完成。两种协议的 HTTP 请求、响应、无效向量和超时用例通过，3.2 完成。

- 2.2 验证完成：test_explicit_migration_and_mysql_seed：保留 ID/时间/文本/文件、重复不变、冲突拒绝、身份和 Token 不变。

- 2.3 验证完成：test_mysql_upload_transactions_and_authoritative_recovery + 原有 SQLite 回归 + 双向权限集成：文件失败/提交失败补偿、原文读取与 MySQL 权威 recovery。

- 3.3 验证完成：test_qdrant_contract_and_dimension_preservation：真实 Cosine/维度/整数索引、UUID/payload 和删除确认，配置不匹配保持原数据。

- 3.4 验证完成：test_upload_embedding_failure_and_committed_original：上传仍 201、原文先可读、第二片失败全部 failed、零 upsert。

- 3.5 验证完成：test_index_failure_recovery + 原链路权限/策略/并发验证：删除失败不替换切片，恢复清理后重建。

- 4.1 验证完成：test_keyword_states_ordering_and_unavailable_vectors：真实 ngram 排序、pending/failed 排除、无向量调用。

- 6.1 验证完成：test_index_failure_recovery：部分写入、删除失败、ready 提交失败、中断；Qdrant 故障无降级。

上述真实服务验证：独立 Compose 项目，16 passed in 23.28s；现有课堂 MySQL/Qdrant 数据未参与迁移、写入或故障注入。

- 1.3 验证完成：配置缺失仅输出名称、无 Qdrant 的 factory 启动、Compose 私有数据库网络，3 passed；真实 MySQL/Qdrant 故障时 keyword 路径在集成测试通过。

- 6.5 最终验证：87 passed / 10 个真实服务用例由隔离 Docker 入口单独执行；Docker 套件 16 passed（含全部 10 个真实服务用例），新增配置检查 3 passed。旧安全/事务按 Bearer 等价映射通过。
- 6.6 最终 strict validation 通过。24/24 指本 change 约定的实现与受控网关验收；本轮真实火山方舟复验发生外部网络超时，不能据此宣称课堂实时问答已恢复。
