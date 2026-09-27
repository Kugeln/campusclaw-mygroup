## Purpose

在既有班级材料库和可信服务端 Session 上增加可追溯检索与简短问答。材料和切片正文由关系型存储提供，向量只负责召回；所有路径以登录班级限定证据，并在无有效证据时直接拒绝生成回答，保持第 3 课教师学生权限与双向班级隔离。

## ADDED Requirements

### Requirement: Preserve existing authentication and material behavior
系统 MUST 保留现有服务端 Session、teacher/student 两种角色及权限。只有本班教师 SHALL 上传或重建材料索引；学生 SHALL 查看、下载、检索和提问本班材料，上传或重建 MUST 返回 403。材料上传 MUST 保持原文件及材料/原文同事务持久化契约，索引只能在原文可靠提交之后开始，后续索引失败不得撤销上传。

#### Scenario: Upload commits before indexing
- **WHEN** 本班教师上传合法材料
- **THEN** 原文件、材料和正文成功保存后才调用切片/Embedding，返回原有 201 和 material_id
- **AND** 索引失败时仍可通过原列表、详情及下载接口读取完整原材料，不删除正文或回滚已完成上传

#### Scenario: Student permissions remain enforced
- **WHEN** 学生直接上传或重建索引，包括伪造 teacher 角色
- **THEN** 服务端返回 403，不修改材料、切片或向量
- **AND** 未认证用户调用新检索、问答或重建 API 返回 401

### Requirement: Authoritative relational text and identifier only vector payload
课程模式 MUST 使用 MySQL 保存 materials、knowledge_entries.body_text 和 knowledge_chunks.chunk_text，向量结果回表后才能作为摘录。Qdrant 的 campusclaw_chunks MUST 仅保存向量及 class_id、material_id、knowledge_entry_id、chunk_id、chunk_index 五个 payload 字段；point ID MUST 等于 knowledge_chunks.id。MUST NOT 在 payload 保存正文、标题、摘录、问题、答案或密钥。

#### Scenario: Text is resolved from MySQL
- **WHEN** vector 或 hybrid 返回向量命中
- **THEN** 系统使用命中 ID 和当前班级条件查询 MySQL 的 ready 切片，最终标题/正文来自关系型记录
- **AND** Qdrant payload 即使携带伪造正文也不会被用于摘录或发送给对话模型

#### Scenario: Required material migration preserves data
- **WHEN** 将既有 SQLite 材料数据迁入 MySQL
- **THEN** 保留材料/原文 ID、班级、上传者、时间、完整文本及文件关联，plain_text 映射为 body_text
- **AND** 既有详情响应继续提供 plain_text，账号/班级/Session 数据及权限不迁移或重置

### Requirement: Deterministic configurable chunking
系统 SHALL 从已提交 body_text 按顺序切片，默认目标长度 800 个 Unicode 字符、重叠 80 个字符，保存从 1 开始的 chunk_index。SHALL 支持固定长度、自定义字面分隔符及 Markdown 标题边界策略；边界内过长内容继续切片，重叠不得跨明确的段边界。相同正文与策略 MUST 产生相同的切片文本及次序，空白片段不入库。

#### Scenario: Default and boundary strategies
- **WHEN** 对长正文分别应用默认、自定义分隔符和 Markdown 标题策略
- **THEN** 可验证默认窗口长度/80 字重叠、指定边界优先、超长段继续切片和连续序号，正文没有被无故丢弃

#### Scenario: Invalid strategy
- **WHEN** 教师重建时提交未知策略、非法长度/重叠或空的自定义分隔符
- **THEN** 返回 400，旧切片和向量未被删除，不调用外部网关

### Requirement: Independent server side model gateways
Embedding 与对话 MUST 使用各自独立的服务端网关配置和调用模块。Embedding SHALL 接受一段文本并返回配置维度的有限浮点向量，只在索引及 vector/hybrid 查询中调用。对话 SHALL 仅接收问题与已校验的证据标题、序号和正文，不接收向量。浏览器 MUST 仅调用本站 /api，不能取得任何网关密钥或直接访问 Qdrant。

#### Scenario: Invalid embedding response
- **WHEN** 网关超时、返回错误、空向量、非有限数、零范数或维度不匹配
- **THEN** 索引或向量查询按失败处理，不保存无效向量，不调用对话网关补救

#### Scenario: Separate clients and secrets
- **WHEN** 检查浏览器请求、API 响应和构建产物，或用替身记录两个网关调用
- **THEN** 浏览器只访问本站 API，密钥不出现在客户端、错误响应或日志；keyword 无 Embedding 调用，对话输入没有向量

### Requirement: Complete index publication and teacher rebuild
新切片 SHALL 初始为 pending，仅当整份材料所有切片向量写入确认完成后，才在同一 MySQL 事务中全部置为 ready。任一 Embedding 失败 MUST 将本次全部切片置为 failed，且本次不向 Qdrant 发布任何向量。重建 MUST 校验本班教师身份，先使旧索引不可检索，删除并确认旧向量、删除旧切片，再创建新切片，不覆盖原文。

#### Scenario: Embedding fails halfway
- **WHEN** 前几个切片 Embedding 成功而后续一个失败
- **THEN** 所有本次切片标记 failed，没有本次部分向量集合，原材料和 body_text 完整保留

#### Scenario: Rebuild with a new strategy
- **WHEN** 本班教师提交有效的新切片策略
- **THEN** 先确认旧向量和旧切片删除，再建立新切片及向量，成功后全部 ready，旧 ID 不再出现在检索结果
- **AND** 跨班重建与不存在材料统一返回 404，未改变目标材料；这不改变跨班检索的空结果契约

#### Scenario: Qdrant write or cleanup failure
- **WHEN** 向量写入超时、部分写入或补偿删除失败
- **THEN** 本次切片不可成为 ready，不对检索发布半成品；记录失败和需要清理的状态，重建不得跳过清理直接写入新向量
- **AND** Qdrant 恢复后先确认清理再允许重建，不能把未确认清理报告为成功或假称跨库原子事务

#### Scenario: Interrupted indexing
- **WHEN** 应用在切片或向量写入之后、ready 提交之前中断
- **THEN** 恢复后该材料不会被当作完整索引使用，保留原文，并在清理未完成索引后才允许重建

### Requirement: Keyword retrieval with MySQL ngram
keyword MUST 只使用 MySQL FULLTEXT 索引，使用 WITH PARSER ngram 且 ngram_token_size=2；查询 MUST 同时限定可信 class_id 和 index_status=ready，按全文相关性降序返回。该模式 MUST 不调用 Embedding、不访问 Qdrant，不以 LIKE、SQLite FTS 或 Qdrant 文本查询替代。

#### Scenario: Keyword works without vector services
- **WHEN** Qdrant 和 Embedding 网关不可用但 MySQL 有本班 ready 匹配切片
- **THEN** keyword 返回 200、按全文相关性排序的本班摘录，两个外部服务调用次数均为零

#### Scenario: Only ready same class chunks match
- **WHEN** 相同词存在于本班 ready、pending、failed 和另一班 ready 切片
- **THEN** keyword 只返回本班 ready 切片，不暴露其他记录的标题、文本或计数

### Requirement: Vector retrieval with double class filtering
vector MUST 对查询调用 Embedding，在 campusclaw_chunks 使用 Cosine 搜索并以可信 class_id 过滤，丢弃分数低于 0.35 的结果。命中 ID MUST 再按同一 class_id、ready 状态和有效材料关系回 MySQL 获取正文；不存在、不匹配、失效的命中 MUST 丢弃。最终按余弦相似度降序，同分使用稳定 ID 顺序。

#### Scenario: Threshold and authoritative hydration
- **WHEN** 向量命中包含分数 0.34、0.35、0.8 以及无法在本班 MySQL ready 集合解析的 ID
- **THEN** 0.34 与非法 ID 被丢弃，0.35 保留；合法摘录来自 MySQL，并按相似度降序排列

#### Scenario: Untrusted foreign vector hit
- **WHEN** Qdrant 返回另一班 chunk ID，即使 payload 伪称当前班级
- **THEN** MySQL 二次班级过滤丢弃该命中，不能将其正文传给浏览器或模型

### Requirement: Hybrid retrieval through reciprocal rank fusion
默认模式 SHALL 为 hybrid，MUST 执行 keyword 和 vector 两路；各路先完成班级、状态及各自阈值过滤，再以 chunk ID 去重，按 RRF 融合名次。RRF MUST 使用从 1 开始的排名与 k=60：每路存在的命中贡献 1/(60+rank)，缺席该路贡献 0。不得直接相加原始全文分数与余弦分数。

#### Scenario: Ranking rather than score addition
- **WHEN** 同一 chunk 在 keyword 排名 1、vector 排名 3，而另一 chunk 仅在 vector 排名 1
- **THEN** 得分分别为 1/61+1/63 和 1/61，重复 chunk 只返回一次
- **AND** 原始分数尺度变化但排名不变时融合顺序不变，同融合分按 chunk ID 稳定排序

#### Scenario: One route has no matches
- **WHEN** 两路均成功执行但仅一路有本班合法结果
- **THEN** 使用该路排名贡献返回结果，不给缺席一路加分；空结果不等于依赖服务故障

### Requirement: Session scoped search inputs and failures
教师和学生 SHALL 使用 GET /api/search 或 POST /api/ask，班级只能由已认证服务端 Session 注入。客户端 query string、JSON、header、独立身份 Cookie 的 class_id/user_id/role MUST 被忽略，不能改变范围。空白查询或非法 mode MUST 返回 400。Qdrant 不可用时 vector/hybrid MUST 返回 503，不能静默降级 keyword；没有命中不属于 503。

#### Scenario: Bidirectional isolation under forged scope
- **WHEN** A 班教师/学生提交只存在于 B 班的查询且伪造 B 班身份字段，或 B 班用户进行反向操作
- **THEN** 服务正常时三种模式的 search 返回 200 和空 items，ask 返回 200、「资料中未找到相关内容」及空 sources
- **AND** 不通过 403/404、正文、引用、计数或模型输入暴露另一班材料

#### Scenario: Empty input and unavailable dependencies
- **WHEN** 输入空白查询，或 vector/hybrid 依赖的 Qdrant 不可用
- **THEN** 前者返回 400 且不调用网关；后者返回 503、不调用对话网关、不返回编造结果或 keyword 降级结果

### Requirement: Evidence first short answers with traceable sources
POST /api/ask MUST 调用共享检索模块而非重写三种检索，默认 hybrid，仅使用当前班级排名前 4 个有效切片。有效切片为可信回表得到的非空 ready 文本。无切片 MUST 返回精确文本「资料中未找到相关内容」、空 sources，且对话网关调用次数为零。有切片才 SHALL 生成简短、仅依据证据的回答，以 [1] 等编号映射到响应 sources 的材料 ID、标题、chunk ID、chunk_index 和摘录。

#### Scenario: No evidence means no generation
- **WHEN** 查询没有本班有效切片，或所有向量候选均在 MySQL 二次校验时被丢弃
- **THEN** 返回固定无依据文本和空 sources，不调用对话网关，也不使用模型常识补答

#### Scenario: Cited answer from the top four
- **WHEN** 检索获得超过 4 个合法切片
- **THEN** 对话请求只包含排名前 4 个切片的标题、序号、正文和问题，不含向量或其他班级文本
- **AND** 回答的每个来源编号都对应实际提供的证据，sources 中的摘录等于 MySQL 切片文本

#### Scenario: Invalid citations or gateway failure
- **WHEN** 对话网关失败、返回空回答、遗漏来源编号或引用不存在的编号
- **THEN** 返回 502，不发布伪造引用，不自行添加模型未使用的来源掩盖错误
- **AND** 材料中的指令作为证据数据处理，不得覆盖仅使用证据和本班隔离规则
