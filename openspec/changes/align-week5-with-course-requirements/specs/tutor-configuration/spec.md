## Purpose

定义课程对齐后的教师智能体、文本技能和当前绑定行为，使学生每轮只使用本班授权配置，并通过可重复的词法路由与明确的材料集合得到可解释输入。此后继契约替代旧change中固定技能和空绑定全班等冲突条款，保留已有权限与历史快照保护。

## ADDED Requirements

### Requirement: TC1 Scoped agents and fallback role
系统 SHALL 允许教师创建、修改、启停多个本班智能体；名称trim后1–64个Unicode字符且班内唯一，prompt最多2000字符，空白prompt MUST 使用默认助教角色而非拒绝。学生只列出本班启用智能体；学生写入403，未认证401，学生直取停用/外班/不存在助手统一404。班级 MUST 来自可信身份。助手完整更新须提交两个绑定数组，空数组合法，null/省略/非法字段400且无变更。教师管理读取本班停用助手保留。

#### Scenario: Empty role is usable and bounds remain enforced
- **WHEN** 教师创建两个不同名称助手，其中一个prompt为空，另一个超过2000字
- **THEN** 空prompt助手可保存并以默认角色生成，超长提交400；65字名称/同班重名400且不改版本，同名异班允许

#### Scenario: Role guards and explicit binding input
- **WHEN** 学生写配置、教师写外班ID、或教师提交null/省略绑定数组
- **THEN** 分别403、404、400，无绑定/版本变化；学生读取本班停用助手与不存在均404，教师可管理它

### Requirement: TC2 Teacher text skills and validation
系统 SHALL 接受教师编写或读入文件的单个UTF-8 source，空行以上元数据、以下非空说明；保存整份原文并可取回编辑。元数据支持名称、适用条件、资料、工具、启用；名称/适用条件/启用必填，忽略前导空白及元数据#注释，正文注释样式不得丢失。source≤8000、名称1–32、条件1–120、资料≤32、正文1–2000个Unicode字符；启用为是/否，工具非空时仅knowledge_search。系统 MUST 拒绝重复/未知字段、缺正文、无效UTF-8、非法工具或超长，返回教师可读400，无持久变更；不是YAML frontmatter或脚本技能。

#### Scenario: Round trip file
- **WHEN** 教师提交含注释、五种元数据、空行及中文正文的合法source，并随后读取
- **THEN** 返回原文与一致解析字段，正文完整保留；各长度恰好上限时成功

#### Scenario: Reject malformed file without changes
- **WHEN** 分别提交缺必填、重复/未知字段、无正文、非法工具、非法启用或各长度上限加1的source
- **THEN** 每例400且可定位原因，技能/版本/绑定均不改变；正文中的#不被误判元数据注释

### Requirement: TC3 Skill lifecycle and current scoped bindings
系统 SHALL 提供本班技能列表/单条读取及仅教师的source创建、更新、删除，名称班内唯一，冲突400；外班/不存在404，学生写403。智能体 MUST 可绑定零到多个本班技能和材料，每轮按当前已提交关系读取；跨班/不存在绑定404且整个保存回滚。删除技能 MUST 清理当前绑定但不改历史skill_name/引用/配置快照。教师界面 MUST 提供source编辑、CRUD、技能和材料多选并明确空材料含义。旧enabled开关兼容输入须与source持久化一致，不允许混合两种写法。

#### Scenario: CRUD and next turn updates
- **WHEN** 教师创建两技能、绑定一个助手，随后改source、停用、解绑或删除技能，在既有会话再次提问
- **THEN** 下一轮只使用最新绑定且启用的技能，删除不留绑定；进行中和历史答案不重写，重启后配置仍存在

#### Scenario: Reject foreign resources atomically
- **WHEN** 教师一次保存同时包含本班和外班资源，或学生伪造教师身份调用技能写入
- **THEN** 分别404和403，无部分绑定/版本修改；技能同名更新400且原文不变

#### Scenario: Empty material set never expands
- **WHEN** 助手未绑材料，或教师从既有会话助手解绑最后一份材料
- **THEN** 下一轮证据为空，不检索任何同班未绑定材料，仍进入Week5无依据生成；非空绑定只在该集合内召回且仍按身份班级隔离

#### Scenario: Legacy toggle keeps file and metadata consistent
- **WHEN** 教师用enabled-only兼容请求停用已有文本技能，或同时提交source与enabled
- **THEN** 前者原子更新source启用行、解析字段和revision，后者400；GET原文重新解析与存储字段一致

### Requirement: TC4 Deterministic lexical routing
系统 MUST 针对当前提问，只在当前助手本班绑定且启用的技能中评分。条件以中英文逗号、顿号、分号和竖线分词，去空白空词，提问和词转小写；每个词为提问子串时加1，最高正分胜，同分选ID小者，全部0分不选，一次最多一个。重复关键词按出现次数计分，选中结果不依赖候选返回顺序，不调用模型路由。选中名称含解题引导时 MUST 追加不给最终答案、选项、得数或完整解题过程的服务端教学边界；停用/解绑后同题重新路由。

#### Scenario: Score tie zero and separators
- **WHEN** 候选的条件覆盖所有分隔符、混合大小写、重复词/空词，分别构造最高分、同分及零分提问并打乱候选顺序
- **THEN** 按上述规则得到同一确定结果；零分无技能，最多注入一份说明，未绑定/停用者即使命中更多也不参与

#### Scenario: Guidance is activated by selected name
- **WHEN** 选中教师创建且名称包含解题引导的技能，或随后停用/解绑并重放同题
- **THEN** 选中轮只提供引导不披露最终答案，停用/解绑轮不注入该说明，允许角色结合有效资料给结论；实际技能与本轮快照一致

### Requirement: TC5 Evidence filtering and bounded prompt inputs
系统 SHALL 复用现有本班检索，在非空绑定内取前4个候选，再按命中技能非空资料字段对材料标题做子串过滤，剔除空正文后连续编号；不得从未绑定材料补足结果。交给模型的每条资料正文最多800个Unicode字符，置于User消息并与本轮问题对应；System只含角色、选中技能及固定行为/不可信数据边界。无技能不注入其他技能，工具声明不驱动实际工具执行。

#### Scenario: Filter then number and truncate
- **WHEN** top4中含空文、学科不符和超过800字的合法绑定材料
- **THEN** 只剩合法学科非空证据并从1连续编号，模型正文≤800字，引用含标题/切片序号且对应编号；不补入第五条或未绑定资料，历史证据全文可独立保留

#### Scenario: Prompt boundaries remain enforced
- **WHEN** 材料或技能说明夹带忽略权限/外发指令，或学生提交绑定/身份覆盖字段
- **THEN** 资料不升格为System权限、无模型工具执行通道；覆盖请求400，无跨班或未绑定模型输入
