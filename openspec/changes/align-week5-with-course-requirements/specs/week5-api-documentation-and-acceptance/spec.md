## Purpose

让课程对齐后的接口、注解文档、分层验证和本项目独立账号交叉验证形成一致的可审查证据，防止任务完成或历史成功替代实际行为验收。保留既有Swagger和受控故障测试；官方原始真实同伴要求保持来源真实，本项目由用户明确选择自动化方式，必须披露差异。

## ADDED Requirements

### Requirement: DA1 Annotation generated accurate OpenAPI
系统 SHALL 从处理器注解生成可重复的OpenAPI描述，保留匿名Swagger页面、Authorize和POST fetch SSE调试。文档 MUST 描述实际路由/方法、source CRUD及兼容toggle、请求边界、Bearer/角色、400/401/403/404/409/502/503等适用错误、SSE候选meta与最终done语义及流后200/error。生成检查必须发现代码/注解/产物漂移；不能保留第二份手写接口契约或用固定23操作覆盖新接口。生成方式为课程要求，具体工具不是课程硬约束。

#### Scenario: Generate and debug current interfaces
- **WHEN** 从注解重新生成文档并在Swagger试调技能CRUD、配置、会话、Week4检索/Ask及SSE
- **THEN** 产物可重复、路由和schema一致，角色错误明确；SSE首帧/增量/终态可观察，未提供Bearer时受保护接口401，无真实秘密进入页面

#### Scenario: Contract drift is detected
- **WHEN** 处理器路径/字段或SSE契约变化而注解/生成产物未相应更新
- **THEN** 一致性检查失败并定位差异，不因静态文件可加载或旧23操作数满足而通过

### Requirement: DA2 Course acceptance and protected regression evidence
验收 SHALL 按官方八项DoD逐项证明：智能体可管理、仅绑定集合、会话锁定、引导及停用/解绑区别、零命中仍答、引用对应、真实流式、Swagger可调。测试 MUST 覆盖F01–F12相关边界与DB状态，并保护Bearer、教师/学生、A/B及owner隔离、Week4三模式/阈值/RRF/Qdrant回表、材料/索引、Ask零命中零调用、SSE解析/落库/失败中断、Nginx及Compose入口。单元、隔离真实依赖、受控网关、浏览器、真实模型和历史证据分开，缺失项标未验证；收费模型只在授权限额内执行。

#### Scenario: Full course matrix with real incremental rendering
- **WHEN** 运行课程矩阵和原回归，在受控上游门控多个片段并核对数据库及浏览器
- **THEN** 每个DoD有输入、输出和证据位置，首片早于生成结束、delta拼接与DB一致，权限拒绝无越权副作用；未执行真实模型不冒称教学复验通过

#### Scenario: Week four and sensitive configuration stay protected
- **WHEN** 经默认Go/现有代理入口重跑三模式、故障、无命中Ask及敏感配置检查
- **THEN** 原items/answer/sources、材料UUID/1-based、阈值和故障不降级保持，Ask零命中对话零次；报告脱敏且不入Git，不读取输出真实密钥，不重建课堂数据

### Requirement: DA3 Independent account project verification and preserved history
按用户明确选择，本项目5.4 SHALL 使用独立教师T-A、同班学生S-A1/S-A2、外班学生S-B1身份，在隔离环境完成P1会话归属、P2智能体隔离含停用续聊、P3班级权限边界、P4技能启用/停用/解绑同题变化。项目调整只改变执行与确认方式，MUST NOT 放宽技术行为、错误码、无副作用及证据标准。官方S7及Phase0的真实同伴要求 MUST 如实保留；自动化不得表述为真实同伴操作或确认，项目完成不得表述为原始官方要求全部满足。

P1 MUST 验证本人读取续聊、同班他人列表隔离、直接读取/发送404且owner/agent/消息不变。P2 MUST 验证A/B各自绑定材料与技能、agent覆盖400且绑定不变、停用禁止新会话404（班内无启用助手400）、已有会话按原agent当前配置续聊且历史快照保持。P3 MUST 验证外班会话/助手/技能/材料及文件或引用资源404、身份覆盖字段400、混合跨班绑定整体拒绝且完整配置/版本/关系无部分写入。P4 MUST 在相同材料、问题和其他配置下，每个状态使用新会话，核对实际技能选择与System注入、启用引导/停用解绑差异、合法引用、SSE与数据库一致及历史不重写。

交付 SHALL 保存每个子步骤的实际输入、预期/实际HTTP、响应、SSE事件/完整回答/候选及最终引用、浏览器证据、数据库前后状态、时间/时区、身份角色班级、资源ID、环境/版本及HEAD/工作区摘要。现有28项关键子步骤必须全通过，0失败、0未执行；权限泄露、非法改绑、串用、部分写入、持久化不一致或缺证据等未解决阻塞 MUST 阻止完成。可只读复核真实存在的既有证据，不强制重跑已通过测试。受控模型行为和5.3真实课程模型/用户审核 MUST 分开，不虚构参与者、签名或反馈。

原change32/32、Phase0官方来源与所有原始验证结果 MUST 保留。5.5完成须具备调整后的全部项目必需证据、F/DoD映射、历史失败/修复/复测及迁移风险说明，检查路径/脱敏/历史保护及strict；最终迁移/部署仍需独立授权。

#### Scenario: Independent account evidence satisfies the project method
- **WHEN** 独立教师和不同owner/班级学生的P1～P4现有28项全部通过，实际请求/响应、浏览器/数据库与模型输入/SSE证据可追溯，且无未解决技术阻塞
- **THEN** 本项目5.4可判完成并勾选，记录由Codex自动化执行、未邀请真实同伴，保留官方原始方式与本项目方式的差异，不伪造同伴确认

#### Scenario: Missing evidence or technical failures prevent completion
- **WHEN** 任一P1～P4关键子步骤失败、未执行、身份不独立、证据不可追溯，或有未解决的权限/绑定/引用/持久化阻塞
- **THEN** 5.4及依赖它的5.5保持未完成，不因执行方式调整忽略失败、改预期或伪造响应

#### Scenario: Traceable final handoff
- **WHEN** 调整后的5.4、其他项目必需任务及证据均齐备，最终报告、风险、引用及历史保护检查通过
- **THEN** 本项目5.5可完成，关联F编号/版本/来源/脱敏产物；29/29仅代表调整后的项目标准，官方真实同伴方式未履行仍明确披露，原change及原始验证记录不改写，最终发布/迁移仍需单独授权
