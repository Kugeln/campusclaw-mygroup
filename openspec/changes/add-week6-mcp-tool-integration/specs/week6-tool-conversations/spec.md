## Purpose

为 CampusClaw 第六课提供 week6-tool-conversations 增量合同，并保护现有第五课认证、班级隔离、检索和流式行为。本版已亲自核对任务清单与全部九个课件子页，区分官方兼容要求、示范和项目补强；仍为待用户审核、尚未实施的规划文件。

## ADDED Requirements

### Requirement: W6-F46 沿用真实问答入口

系统 SHALL 将MCP接入现有会话消息SSE入口，保留POST /api/ask非流式输入输出和零证据零对话行为。

分类：官方强制/兼容基线（附项目实现选择）。来源映射：O37，原文页/节见 requirements-traceability.md；项目实现细节见 design.md。实施任务：4.7；测试：W6-T46。状态：已完成来源对齐审阅，未实施/未运行本业务测试。

#### Scenario: W6-T46 沿用真实问答入口
- **WHEN** 同题分别走会话和旧ask
- **THEN** 仅会话可产生工具记录；旧ask无MCP且不创建消息

### Requirement: W6-F47 meta前确定性编排

系统 SHALL 认证/班级与原技能路由之后按白名单逐个调用，工具终态备齐后才发送meta，再生成回答；knowledge_search仍由检索负责。

分类：官方强制/兼容基线（附项目实现选择）。来源映射：O09, O18, O20, O21, O40，原文页/节见 requirements-traceability.md；项目实现细节见 design.md。实施任务：4.5；测试：W6-T47。状态：已完成来源对齐审阅，未实施/未运行本业务测试。

#### Scenario: W6-T47 meta前确定性编排
- **WHEN** 门控远端工具与模型并记录RPC/SSE/模型输入时间
- **THEN** 工具返回早于meta、meta早于模型首字，按声明顺序且无模型自主选择，无跨网络长事务

### Requirement: W6-F48 确定性参数与示范差异

系统 SHALL formula_hint传当前问题原文为query；unit_convert由项目明确确定性解析生成value/from/to，课程固定换算参数仅作示范，不引入模型规划。

分类：课程示范+项目选择。来源映射：O18, O19, O20，原文页/节见 requirements-traceability.md；项目实现细节见 design.md。实施任务：4.4；测试：W6-T48。状态：已完成来源对齐审阅，未实施/未运行本业务测试。

#### Scenario: W6-T48 确定性参数与示范差异
- **WHEN** 以1 m转cm命中绑定技能，并用缺单位或多个换算项问题；另用未绑定技能提问要求调用
- **THEN** 合法解析固定100 cm；歧义denied零RPC；无绑定不调用，原生function calling未启用

### Requirement: W6-F49 官方工具元数据

系统 SHALL 有MCP调用尝试时meta.tool_calls每项含ref/status/text，status为ok/denied/error；没有命中技能或无MCP项时省略字段，不发空数组。

分类：官方强制/兼容基线（附项目实现选择）。来源映射：O21, O22, O23, O33，原文页/节见 requirements-traceability.md；项目实现细节见 design.md。实施任务：4.5；测试：W6-T49。状态：已完成来源对齐审阅，未实施/未运行本业务测试。

#### Scenario: W6-T49 官方工具元数据
- **WHEN** 依次触发ok/denied/error、未命中、停用、仅knowledge_search
- **THEN** 前三者都有ref/status/text；后三者断言属性不存在，meta.status及既有第五课字段不变

### Requirement: W6-F50 断开服务降级

系统 SHALL 存在但未连接的服务器调用为denied，返回安全text并追加一行denied审计，原问答继续。

分类：官方强制/兼容基线（附项目实现选择）。来源映射：O10, O22, O28, O33, O40，原文页/节见 requirements-traceability.md；项目实现细节见 design.md。实施任务：4.5；测试：W6-T50。状态：已完成来源对齐审阅，未实施/未运行本业务测试。

#### Scenario: W6-T50 断开服务降级
- **WHEN** 断开后命中技能并检查远端请求计数、meta和审计
- **THEN** RPC0、meta ref/status/text齐全、审计新增1；模型仍按原角色/RAG回答

### Requirement: W6-F51 改名删除失配降级

系统 SHALL 按旧名找不到服务器或仅外班存在时返回denied安全text，不新增调用审计，不泄露外班。

分类：官方强制/兼容基线（附项目实现选择）。来源映射：O12, O22, O28, O33, O40，原文页/节见 requirements-traceability.md；项目实现细节见 design.md。实施任务：4.5；测试：W6-T51。状态：已完成来源对齐审阅，未实施/未运行本业务测试。

#### Scenario: W6-T51 改名删除失配降级
- **WHEN** 改名、成功删除或用外班名/不存在名构造运行失配
- **THEN** RPC0、审计新增0、meta明确denied；不把有审计删除503误算已删除

### Requirement: W6-F52 工具未发现降级

系统 SHALL 项目加固：存在服务的旧引用工具不在当前缓存时拒绝远端调用，denied且写本服务器审计，原问答继续。

分类：项目补强（非老师新增要求）。来源映射：PROJECT，原文页/节见 requirements-traceability.md；项目实现细节见 design.md。实施任务：4.3；测试：W6-T52。状态：已完成来源对齐审阅，未实施/未运行本业务测试。

#### Scenario: W6-T52 工具未发现降级
- **WHEN** connect刷新移除工具后命中旧引用
- **THEN** RPC0，meta denied，审计新增1且detail为安全原因

### Requirement: W6-F53 鉴权失败降级

系统 SHALL MCP 401/403归error/auth_failed，不回传服务凭据或把上游401当学生JWT失效。

分类：官方强制/兼容基线（附项目实现选择）。来源映射：O03, O09, O22, O28，原文页/节见 requirements-traceability.md；项目实现细节见 design.md。实施任务：4.5；测试：W6-T53。状态：已完成来源对齐审阅，未实施/未运行本业务测试。

#### Scenario: W6-T53 鉴权失败降级
- **WHEN** 轮换服务Token使Go持有旧值后提问
- **THEN** 学生身份仍有效，记录error；UI不退出学生登录，问答继续

### Requirement: W6-F54 工具超时降级

系统 SHALL 超时归error/timeout，取消该请求不重试，在总问答预算内继续。

分类：官方强制/兼容基线（附项目实现选择）。来源映射：O09, O22, O28，原文页/节见 requirements-traceability.md；项目实现细节见 design.md。实施任务：4.5；测试：W6-T54。状态：已完成来源对齐审阅，未实施/未运行本业务测试。

#### Scenario: W6-T54 工具超时降级
- **WHEN** 令首工具延迟超3秒
- **THEN** 记录超时且余下合法工具及模型可继续；整轮不被无条件失败

### Requirement: W6-F55 非法返回降级

系统 SHALL RPC畸形/错误id/无文本/不支持content为error；普通文本合法，危险句式及超1200字整条丢弃后仅用固定说明。

分类：官方强制/兼容基线（附项目实现选择）。来源映射：O22, O24, O25，原文页/节见 requirements-traceability.md；项目实现细节见 design.md。实施任务：4.6；测试：W6-T55。状态：已完成来源对齐审阅，未实施/未运行本业务测试。

#### Scenario: W6-T55 非法返回降级
- **WHEN** 注入畸形JSON、错id、正常中文text、含危险句式文本和1201字文本
- **THEN** 普通文本通过；危险或超长正文任何片段均不进入meta.text/审计detail/模型，仅固定说明，问答仍继续

### Requirement: W6-F56 多工具顺序

系统 SHALL 按技能文本声明顺序串行去重计划，最多3次，单工具≤3秒、工具阶段总≤10秒；一个失败不自动停止其他合法工具。

分类：官方强制/兼容基线（附项目实现选择）。来源映射：O18, O20, O38，原文页/节见 requirements-traceability.md；项目实现细节见 design.md。实施任务：4.4；测试：W6-T56。状态：已完成来源对齐审阅，未实施/未运行本业务测试。

#### Scenario: W6-T56 多工具顺序
- **WHEN** 三工具中首失败、次成功、末超时及四项输入
- **THEN** 顺序稳定、总预算可控；末项可因预算不足denied/budget_exceeded，第四项保存时拒绝

### Requirement: W6-F57 外部数据边界

系统 SHALL 安全工具结果置于用户消息的数据位并标明工具引用，不进入system、不覆盖教师或教学边界；拒绝的原文不进入任何模型输入。

分类：官方强制/兼容基线（附项目实现选择）。来源映射：O20, O24, O26, O38，原文页/节见 requirements-traceability.md；项目实现细节见 design.md。实施任务：4.6；测试：W6-T57。状态：已完成来源对齐审阅，未实施/未运行本业务测试。

#### Scenario: W6-T57 外部数据边界
- **WHEN** 返回要求改角色/系统指令的工具文本并捕获输入
- **THEN** 命中危险句式整条丢弃；通过文本只在user且带工具来源，不能触发新工具或外发

### Requirement: W6-F58 危险句式与1200字边界

系统 SHALL 进入上下文前扫描完整工具正文：含忽略前述、ignore previous instructions、你现在是或system:等危险句式，以及超过1200 Unicode字的正文均整条丢弃换固定说明，不删句/截断后使用；先于审计与SSE脱敏。

分类：官方强制/兼容基线（附项目实现选择）。来源映射：O24, O25，原文页/节见 requirements-traceability.md；项目实现细节见 design.md。实施任务：4.6；测试：W6-T58。状态：已完成来源对齐审阅，未实施/未运行本业务测试。

#### Scenario: W6-T58 危险句式与1200字边界
- **WHEN** 分别返回危险句位于开头/中间/尾部、英文大小写变体、1199/1200/1201字、多text块拼接超限和密钥标记
- **THEN** 1199/1200字正常安全正文可用；1201或命中句式全丢弃且无前缀残留，detail≤512/成功摘要≤200，秘密不泄露

### Requirement: W6-F59 零证据与引用

系统 SHALL Week5无知识库依据仍生成，工具仅独立外部依据，不授予数字引用资格；资料引用只能来自合法检索证据。

分类：官方强制/兼容基线（附项目实现选择）。来源映射：O26, O37，原文页/节见 requirements-traceability.md；项目实现细节见 design.md。实施任务：4.6；测试：W6-T59。状态：已完成来源对齐审阅，未实施/未运行本业务测试。

#### Scenario: W6-T59 零证据与引用
- **WHEN** 零证据工具返回[1]，有证据工具伪造[99]
- **THEN** 空证据最终citations=[]且无伪造数字引用；有证据保留原校验，不把工具写成材料引用

### Requirement: W6-F60 历史持久与展示

系统 SHALL 在现有消息配置快照中保存安全ref/status/text并允许本人历史读取，旧消息缺字段兼容且刷新不重放工具；SSE无调用仍省略。

分类：项目补强（非老师新增要求）。来源映射：O23, O37，原文页/节见 requirements-traceability.md；项目实现细节见 design.md。实施任务：4.5；测试：W6-T60。状态：已完成来源对齐审阅，未实施/未运行本业务测试。

#### Scenario: W6-T60 历史持久与展示
- **WHEN** 完成后刷新、读取旧消息及done丢失补读
- **THEN** 安全状态可回看，RPC数不增加，旧快照不改；内部空列表不能使SSE输出空tool_calls

### Requirement: W6-F61 失败恢复与并发

系统 SHALL 保留active_turn并发409、delta先落库、failed/interrupted前缀、取消传播及崩溃恢复，工具副作用不因重试自动重放。

分类：项目补强（非老师新增要求）。来源映射：O37，原文页/节见 requirements-traceability.md；项目实现细节见 design.md。实施任务：4.7；测试：W6-T61。状态：已完成来源对齐审阅，未实施/未运行本业务测试。

#### Scenario: W6-T61 失败恢复与并发
- **WHEN** 并发发送、工具中取消、首delta后断连及进程崩溃恢复
- **THEN** 无额外重复工具执行或孤儿消息；失败不发成功done，锁可释放

### Requirement: W6-F62 审计查询50与100

系统 SHALL GET /api/mcp/tool-calls只读返回本班按时间倒序记录，默认最近50、最多100，含tool_ref/server_name/tool_name/status/detail/user_id/conversation_id/created_at。

分类：官方强制/兼容基线（附项目实现选择）。来源映射：O06, O11, O13, O27, O29, O33，原文页/节见 requirements-traceability.md；项目实现细节见 design.md。实施任务：2.7；测试：W6-T62。状态：已完成来源对齐审阅，未实施/未运行本业务测试。

#### Scenario: W6-T62 审计查询50与100
- **WHEN** 准备120条同班及外班记录，用默认/limit=1/50/100/101请求并交叉身份检查
- **THEN** 默认50、上限100不泄漏外班；limit=101按项目夹至100，学生403、只读无写入、detail安全
