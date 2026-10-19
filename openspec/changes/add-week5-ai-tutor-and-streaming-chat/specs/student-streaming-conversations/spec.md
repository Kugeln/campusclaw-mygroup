## Purpose

定义学生私有多轮会话和逐段流式回答的完整行为，将本班检索、教师配置及必要历史共同用于有证据回答，并保证失败、断连及重启后消息和来源仍能正确追溯。

## ADDED Requirements

### Requirement: Private durable student conversations
学生 SHALL 创建、列出和读取自己的会话及历史消息。创建绑定本班已启用助手；user_id/class_id MUST 来自当前 Bearer 身份。每次列出、读取、发送 MUST 同时校验所有者和当前班级；教师不能读取学生私有会话。同班其他学生、跨班用户和不存在会话 ID MUST 统一返回 404；教师调用学生会话操作返回 403，未认证返回 401。持久数据 MUST 与认证 sessions 表分离。

#### Scenario: Create and reload
- **WHEN** 学生创建会话并完成两轮问题后刷新页面或应用重启
- **THEN** 仍能读取自己的会话、按顺序排列的消息、实际技能、状态与引用，ID 和文本保持

#### Scenario: Ownership and class attacks
- **WHEN** A1 使用 A2 或 B1 的会话 ID 读取或发送消息，或 A1 已转班后访问原班会话
- **THEN** 返回 404，无消息写入、检索或模型调用，不透露会话元数据

### Requirement: Evidence precedes combined generation
每轮 MUST 先验证 Bearer、会话及助手权限，加载配置快照并据教师配置确定技能是否选中，再检索当前学生班级知识库；选择阶段只使用元信息，有合法证据且准备生成时才加载已选中技能的完整指令，将教师提示词、实际技能、必要历史、当前问题及本轮前 4 条合法证据共同发送给模型。历史 MUST 来自该会话服务端存储，只使用完成的有效轮次，按确定的预算保留最近上下文；客户端不得提交历史。历史和材料中的指令 MUST 作为数据，不能覆盖权限、引用或技能教学约束。

#### Scenario: Follow up uses all inputs
- **WHEN** 学生第一轮讨论移项，第二轮问“那下一步呢”，配置和解题技能已启用且本班有证据
- **THEN** 调用记录包含当前教师提示词、完整技能、第一轮完成历史、第二轮问题及第二轮本班证据，其他会话和外班内容均不存在

#### Scenario: No evidence retains week four behavior
- **WHEN** 当前轮无本班有效 ready 切片，包括外班独有数据或全部回表过滤，即使历史曾有命中
- **THEN** SSE 返回精确文本“资料中未找到相关内容”、空引用并完成持久化，模型调用次数为零，不使用历史旧引用或常识补答

#### Scenario: Retrieval outage is not an empty result
- **WHEN** hybrid/vector 所需依赖不可用
- **THEN** SSE 开始前返回 503 JSON，不降级 keyword、不生成无数据成功回答、不调用对话模型

### Requirement: Same class verified traceable citations
回答引用 MUST 指向本轮真实提供的证据编号，保存材料 UUID、标题、chunk UUID、从 1 开始的 chunk_index 及文本快照。最终 citations MUST 仅含实际引用且校验通过的证据，不能编造页码或位置；材料链接仍执行本班权限校验。重建索引后历史快照仍可展示，但不得绕过当前材料访问权限。

#### Scenario: Citation matches relational evidence
- **WHEN** 本轮输出包含 [1] 且引用校验成功
- **THEN** citations 对应本轮第 1 条 MySQL 证据的标题、材料/切片 ID、序号及原文；流式最终引用与数据库历史一致

#### Scenario: Missing or out of range citation
- **WHEN** 有证据生成的完整回答缺少编号、含越界编号或引用并非本轮来源
- **THEN** 该助手消息标为 failed，发布 error 事件且无成功 done；不补造引用掩盖错误

### Requirement: Incremental POST SSE with a terminal contract
发送消息 SHALL 使用携 Bearer 的 POST，正常为 200 text/event-stream、no-cache 并逐段刷新，不能等待完整回答后切分伪装流式。事件名称和 JSON 数据 MUST 按 design 的官方对照契约明确，正常完成只能发送一次 done；开始前 400/401/403/404/409/502/503 使用 {error:string} JSON，开始后的异常使用 error 事件并关闭流，不改 HTTP 200。前端 MUST 使用支持 POST Header 的 fetch 流解析，正确处理 UTF-8、跨网络分片、多行及结束事件。

#### Scenario: Normal stream before full completion
- **WHEN** 受控网关延迟发送至少三个回答片段
- **THEN** 浏览器在最后片段生成前显示第一片段，顺序拼接文本；最终引用和技能可见，done 后文本与历史存储一致

#### Scenario: Failure after a partial answer
- **WHEN** 模型已发送若干片段后超时或返回错误
- **THEN** 收到 error 且没有成功 done，前端显示失败和已收到的未完成文本，历史标记 failed 并保存已可靠提交的前缀

### Requirement: Transactional turn persistence and disconnect recovery
验证和检索成功后、开始 SSE 前 MUST 原子保存 user 消息、assistant generating 占位、配置及证据快照。每个待推送片段 MUST 先可靠存入回答前缀，再刷新给客户端；正常回答与引用校验成功后提交 completed，再发送 done。断连 MUST 取消上游调用并标记 interrupted，保留 user、已提交前缀及实际技能，不假装完成、不自动重试；进程崩溃遗留 generating 在恢复时转 interrupted。完成已提交后客户端未收到 done 的情况 MUST 由历史查询确认 completed，不能回退已完成状态。

#### Scenario: Client disconnects midstream
- **WHEN** 客户端收到首段后中断连接
- **THEN** 上游请求取消、助手状态 interrupted，无 completed 或伪造引用；刷新历史能看到原问题、已提交前缀和技能，可重新发送新一轮

#### Scenario: Persistence fails
- **WHEN** 初始事务或流中回答/最终状态写入失败
- **THEN** 初始失败返回 503 且无孤立消息；流中失败停止输出及模型，不发送 done；恢复后非完成轮次可辨认，客户端已收到片段均来自已提交前缀

#### Scenario: Concurrent sends and restart
- **WHEN** 同一会话已有 generating 轮次时又提交消息，或应用在生成中崩溃后重启
- **THEN** 并发请求返回 409 且不新增轮次；重启将遗留轮次标为 interrupted 并释放占用，completed 不改变，重发作为新轮次不承诺幂等重放

### Requirement: Preserve week four capabilities in the Go runtime
本需求是第 4 课既有能力在 Go 上的接入前置依赖，MUST 在第 5 课新增功能实施前单独验收，不作为本课新教学能力或本轮实施授权。
第 5 课 SHALL 运行于默认 Go/MySQL；Go MUST 补接现有第 4 课切片、索引、检索、引用及单次问答行为，三种检索只能有一个共享实现供 search、ask 和会话调用，MUST 不依赖 Flask/Python 业务运行时。既有材料、认证及 Token 撤销契约 MUST 保持；已有课堂 MySQL/Qdrant 数据不得自动合并、删除或重建。

#### Scenario: Old APIs remain available
- **WHEN** 经默认入口调用 GET /api/search?q=...&mode=...、POST /api/ask {question,mode} 及原材料 API
- **THEN** Go 返回既有 items 或 answer/sources 契约，默认 hybrid、无命中固定文本、非法输入 400、检索故障 503、问答引用/网关错误 502均保持，search/ask/会话调用共享检索

#### Scenario: Prerequisite gate remains unmet
- **WHEN** Python 第4课已通过，但默认Go入口尚未通过检索、引用及单次问答前置验收
- **THEN** 前置依赖保持未完成，Phase 1 必须交付独立 Go 入口及真实 MySQL/Qdrant 验收记录，Phase 1 未全部完成前不得实施 Phase 2；Flask历史测试或strict规约验证不作为替代，本轮不补写Go业务实现或修改已有迁移规约

#### Scenario: Index and security regression
- **WHEN** 教师上传并索引材料、学生尝试上传/重建，或 A/B 两班双向检索并伪造身份
- **THEN** 原文先提交且索引失败不丢文件；学生写入 403，跨班材料 ID 404，外班独有查询 200 空结果；keyword 不调用向量服务、vector 双重过滤、hybrid RRF 行为与第 4 课一致
