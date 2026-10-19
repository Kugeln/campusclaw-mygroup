## Purpose

定义学生解题助手所使用的班级教师提示词和内置解题引导技能，保证配置由服务端按可信身份读取、更新仅影响后续轮次，并留下可核对的版本与实际技能记录。

## ADDED Requirements

### Requirement: Teacher owned class assistant configuration
系统 SHALL 提供本班助手的读取、新建和更新接口，提示词包含助手角色、回答风格及教学原则。只有本班教师 MUST 能写入；学生只能读取可用助手并使用。助手 MUST 绑定服务端班级，同班名称唯一；名称去空白后 1–64 字符、提示词 1–2000 字符，非法输入返回 400。跨班助手与不存在助手统一返回 404；未认证返回 401，学生写入返回 403，拒绝时没有数据库变更。

#### Scenario: Teacher saves and student uses
- **WHEN** A 班教师创建并保存助手提示词，A 班学生选择该助手提问
- **THEN** 服务端使用保存的提示词，B 班不能列出或读取该助手，重启后配置仍存在

#### Scenario: Forbidden and invalid edits
- **WHEN** 学生伪造教师字段更新提示词，或教师提交空白/超长提示词、同班重复名称或跨班 ID
- **THEN** 分别返回 403、400 或 404，原提示词与版本不改变；无 Bearer 的写入返回 401

### Requirement: Configuration revisions apply to the next turn
系统 MUST 在接受每轮问题时原子读取已提交配置及版本并形成快照，不采信消息请求中的 prompt、skill_ids、role、class_id。教师保存后接受的下一轮 MUST 使用新版本，包括既有会话的追问；正在生成的轮次继续使用其快照。历史回答、引用及配置快照 MUST 不因配置更新而重写。

#### Scenario: Edit during generation
- **WHEN** 第一轮以版本 1 开始，教师保存版本 2 后学生在同一会话发出第二轮
- **THEN** 第一轮保留版本 1，第二轮使用版本 2，两轮文本及版本可从持久记录核对

#### Scenario: Forged per message configuration
- **WHEN** 学生发送 content 并额外提交自定义 prompt 或技能开关
- **THEN** 返回 400 且不创建消息，不调用模型，客户端不能覆盖教师配置

### Requirement: Minimal server selected guidance skill
系统 SHALL 至少提供一个内置“解题引导”技能；只有教师可为本班启用或关闭。服务端 MUST 常驻技能名称和简短说明，检索前根据教师配置快照确定是否选中：启用且被助手绑定则选中，此时不读取完整指令；仅有合法本班检索证据、准备生成时 MUST 加载已选中技能的完整指令。启用且实际用于生成时 MUST 引导分析、给出思路或下一步问题，不直接给最终答案；关闭时 MUST 不加载该指令，允许依据本班证据直接解释和给出结论。系统 MUST 不执行技能脚本、MCP 或多 Agent 编排。

#### Scenario: Selection precedes retrieval and loading follows evidence
- **WHEN** 教师已启用并绑定技能，学生提交有效问题且检索有合法本班证据
- **THEN** 调用顺序为配置选择、班级检索、合法证据确认、完整技能加载、模型生成；检索开始时完整指令尚未加载，实际技能记录与模型输入一致

#### Scenario: Toggle produces a teaching difference
- **WHEN** 教师对同一有证据的解题题目分别启用与关闭技能，并各创建一轮回答
- **THEN** 启用回答包含分析提示/步骤或引导问题且不披露最终答案，关闭回答可以直接给证据支持的结论；受控网关断言完整技能指令仅在启用轮次出现，真实模型按教学检查清单复验

#### Scenario: Student cannot toggle
- **WHEN** 学生直接调用技能更新或助手绑定更新，包括伪造 role 或指定外班 skill_id
- **THEN** 返回 403，无技能、助手或版本变更；教师指定外班技能返回 404

### Requirement: Actual skill audit per answer
每轮助手消息 MUST 持久化实际加载技能的 ID、名称及指令版本或摘要哈希，并向历史消息提供 skill_name；关闭时记录空技能，不能把“已启用但未执行”标记成实际使用。无检索命中时 MUST 不调用模型、不加载完整技能，实际技能为空，同时记录配置快照与 no_evidence 原因；失败或断连轮次也保留实际执行记录。

#### Scenario: Used skill survives later toggle
- **WHEN** 已完成一轮启用技能的回答后教师关闭技能并重启应用
- **THEN** 原回答仍记录原技能和版本，新轮次记录空技能，历史文本不改变

#### Scenario: No evidence audits no execution
- **WHEN** 技能启用但本班检索无有效命中
- **THEN** 检索前已记录配置选中决策，但保存精确“资料中未找到相关内容”、空引用和空实际技能，记录 no_evidence；不能把选中当实际使用，完整技能与对话网关调用次数均为零
