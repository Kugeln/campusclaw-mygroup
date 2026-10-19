## Purpose

定义课程对齐后的私有多轮问答和增量输出，使会话锁定智能体、当前配置和有限历史参与有据或无据生成，并保持已有逐段持久化、引用可追溯与失败中断保护。此契约仅改变Week5会话行为，不改变Week4单次问答语义或认证生命周期。

## ADDED Requirements

### Requirement: SC1 Private fixed agent conversations and title
系统 SHALL 以可信user_id和class_id隔离会话列表/读取/发送，非本人、外班、不存在统一404，匿名401，沿用学生会话接口角色守卫。新建必须选本班enabled助手，外班/不存在/停用404，本班无启用助手400。既有会话agent_id不可变；绑定助手后来disabled MUST 仍可续聊，存在及班级检查不放宽。标题初始为新的提问，首个接受问题trim后前24个Unicode字符更新，后续只刷新updated_at；列表倒序。客户端身份/配置/其他agent覆盖字段400。

#### Scenario: Disabled agent affects only new selection
- **WHEN** 学生选A创建会话，教师停用A后学生继续原会话并尝试新建A会话
- **THEN** 原会话以A当前配置继续，新建按404拒绝（本班已无启用助手时400）；伪造B编号不能换绑，同班他人或外班访问均404

#### Scenario: Unicode title and ownership remain stable
- **WHEN** 首问超过24个含中文字符，随后再次提问、刷新和重启
- **THEN** 标题精确为首问前24字，后续不变，updated_at刷新且列表排序正确；消息/归属和固定agent不丢失

### Requirement: SC2 Six message context including current user
系统 MUST 保存全部历史，仅将按写入顺序末尾6条有效消息作为模型历史窗口，含已保存的本轮user，不能取6轮或6条旧消息后再追加本轮。只允许user/assistant角色，空正文跳过，每条历史正文截到500个Unicode字符。未完成assistant草稿不作为完成历史，完成的无据回答可入窗；本轮user只出现一次，其资料由服务端组装，不接受客户端历史。更早消息及失败前缀仍可在授权历史读取。

#### Scenario: More than three rounds and oversized messages
- **WHEN** 会话超过6条，含长assistant、空正文、非法角色、失败草稿及本轮user
- **THEN** 捕获的模型窗口≤6条、按时间排列、含当前user且不重复，每段历史正文≤500字，非法/空记录和失败assistant不入窗；数据库原文仍完整

### Requirement: SC3 Week five generation with no evidence
系统 MUST 在正常零命中、空绑定、或学科过滤后无证据时继续调用现有模型，按默认/自定义角色和本次命中技能生成，citations为空，输出不得出现虚假数字引用或整段退回旧固定未找到文案。无证据无技能亦合法；有据优先使用本轮证据。检索依赖不可用 MUST 与空结果区分，流前503，不生成成功空结果。Week4 Ask无证据固定文案、空sources和零对话调用 MUST 保持。

#### Scenario: Evidence and skill are independent
- **WHEN** 分别提交有资料有技能、有资料无技能、无资料有技能、无资料无技能四类有效问题
- **THEN** 四类都走Week5模型；只有命中技能轮含其说明，有据轮可引用，无据轮引用[]且无编号，回答符合角色而非旧固定文案

#### Scenario: Cross fragment citation suppression
- **WHEN** 无证据模型将数字引用拆成多个片段，或产生普通括号文本及非编号残留
- **THEN** 虚假数字编号在落库和delta前被去除，普通文字不被误删且持续增量；拼接delta等于持久化正文，空有效回答或整段旧固定文案不能以成功done结束

#### Scenario: Retrieval outage and week four regression
- **WHEN** 非空绑定的检索依赖失败，或向Week4 Ask提交无命中问题
- **THEN** 前者meta前503且无模型调用、已接受轮次如实失败；后者仍固定文案/空sources且模型零调用，不被Week5新分支改变

### Requirement: SC4 Early meta and stable citation identities
发送消息 SHALL 沿用Bearer POST fetch SSE；meta MUST 为首业务事件，在模型调用/首片之前Flush，status恒ok，包含本轮技能和已过滤的完整证据列表，无证据为[]。列表是本轮可引用候选而非预测最终使用来源，编号连续且与模型输入一一对应。delta仅新增文本；done须含消息ID，保留state/skill_name及最终实际引用扩展，最终子集保留meta编号。页面在meta显示候选来源，完成后区分实际引用，失败不得把候选冒称已使用。

#### Scenario: Meta arrives while model has not started
- **WHEN** 受控模型被门控为尚未接受请求，检索和证据准备完成
- **THEN** 客户端已收到并显示status=ok、技能和来源；放行后多次delta增量到达，done实际引用均来自meta，编号不因子集重排

#### Scenario: Empty evidence status and sparse final sources
- **WHEN** 无证据生成，或有证据模型只引用第2条
- **THEN** 前者meta.status仍ok、citations[]；后者meta连续编号，done引用保留number=2并可正确定位，文本与历史编号一致

### Requirement: SC5 Preserve transactional streaming and failure boundaries
系统 MUST 保留单会话并发409、逐段先持久化再发送、正常完成持久化后done、失败/中断前缀与状态、取消上游、崩溃恢复及done丢失查历史能力。user和assistant占位原子写入，保存本轮配置/证据及实际技能审计；在meta前准备失败可返回JSON并终结已接受占位，不能留下永久generating或假成功，初始事务失败无孤儿消息。meta后失败HTTP保持200，用error且无成功done，保存已生成前缀。实际技能不得把未发起模型调用写成成功执行；历史快照不随编辑/重建改写。有据引用越界/缺失仍失败。

#### Scenario: Failure before and after meta
- **WHEN** 分别在初始事务、检索/证据准备、meta后模型连接、部分delta、最终提交注入故障
- **THEN** 初始失败无消息；meta前其他失败返回对应JSON并标记已接受轮次failed；meta后error/HTTP200无done，已发片段均已存储，前缀可读，释放占用或恢复为interrupted

#### Scenario: Cancel restart concurrency and lost done
- **WHEN** 客户端中断、进程重启恢复、并发再发送、或completed已提交但done丢失
- **THEN** 分别取消模型保留interrupted前缀、恢复遗留generating、并发409无多余轮次、历史仍completed；不自动重试收费调用或把失败伪装成功

#### Scenario: Transport and browser protections remain
- **WHEN** 经现有Nginx和浏览器读取分块UTF-8、多行SSE、心跳和故障响应
- **THEN** 解析不丢片段，首段早于末段完成；响应为event-stream/禁止缓存/不缓冲，逐帧Flush，代理buffer/cache关闭且读取超时120秒；原Bearer/401清理与上传路径保持

### Requirement: SC6 Scoped internal conversation deletion cleanup
系统 SHALL 在内部存储删除一个会话时以owner+class限定并原子清理所属全部消息，越权/不存在一致NotFound；生成中会话拒绝Busy，失败时整体回滚，不能产生孤儿。此要求 MUST NOT 自动增加公开DELETE路由、页面删除或后台清理任务，历史数据不被自动批量删除。

#### Scenario: Internal deletion and rollback
- **WHEN** 在隔离库内部删除有多轮消息的本人会话，并分别模拟中途失败、外班/他人ID和进行中轮次
- **THEN** 成功时会话与所属消息同时消失，不影响其他会话；失败整体保留、越权NotFound、活动会话Busy；公开路由清单不新增会话DELETE
