## Purpose

定义第 5 课主要接口的可浏览契约与可重复验收要求，明确官方参考和现有项目差异，并通过接口内容、持久状态及第 4 课回归证明教学助手行为正确而不是仅返回成功状态。

## ADDED Requirements

### Requirement: Explicit local OpenAPI and Swagger contract
系统 SHALL 提供匿名可浏览的 /openapi.json 和 /swagger/index.html，并提供 /swagger/ 同页面。文档 MUST 描述实际实现的路径、方法、参数、必需字段、响应、错误码和 Bearer security；包含提示词、技能启停、会话、历史、检索/问答及 SSE 事件格式。官方与本地差异 MUST 用功能/官方接口/当前接口/差异/建议清单记录，课外接口不得仅因官方存在而加入。

#### Scenario: Browser debugging with authentication
- **WHEN** 浏览器打开本地 Swagger 并通过 Authorize 设置教师或学生测试 Token
- **THEN** 能调试本地登录、教师配置、技能、创建会话、历史和检索主要接口；未设置 Token 调用受保护 API 返回401且文档说明一致，页面和资源不包含真实密钥

#### Scenario: Stream debugging matches documented format
- **WHEN** 使用 Swagger 页面提供的 SSE fetch 调试发送问题
- **THEN** 能在生成完成前看到 delta，记录 meta/done或error；事件核心字段与文档一致，Token不在URL，普通JSON调试仍可用

#### Scenario: Official contract differences are not hidden by local success
- **WHEN** 本地验收使用/api/auth/login、GET /api/search、question/mode问答、enabled技能更新及扩展SSE字段通过
- **THEN** 报告明确未证明官方/api/login、POST /api/search、q/history问答、source技能管理原样兼容，列出路径/字段/状态/引用时机差异；不得宣称全量官方验收通过或自动重构接口，若老师明确要求原样契约则提交用户决定追加规划范围

### Requirement: One script per URL and method with durable assertions
仓库 SHALL 规划并在实施阶段建立 apitest/cases 下按 URL/HTTP方法命名的脚本，同一接口的不同场景放入同一个脚本，提供 apitest/run_all.py 一键执行和 HTML 报告。脚本 MUST 不仅断言HTTP状态，还断言响应字段、文本、权限、流式顺序及数据库状态。所有本课新增接口和既有登录/权限/材料/检索/问答 MUST 在测试清单中有对应脚本。

#### Scenario: Full runner output
- **WHEN** 配置隔离测试环境后执行 python apitest/run_all.py
- **THEN** 自动发现每个接口脚本，按初始化依赖执行，生成包含用例、断言、通过/失败、时间和脱敏错误的HTML，任一失败退出非零；缺必需数据库或网关配置不得报告全通过

#### Scenario: Persistence and authorization evidence
- **WHEN** 执行提示词、技能、会话、消息、引用及拒绝访问场景
- **THEN** 测试同时核对数据库版本、owner/class、消息文本/状态、实际技能和引用，拒绝访问前后行数/字段不变，无跨班模型输入

### Requirement: Sensitive configuration and reports remain local
测试凭据、API Key、签名密钥和测试库连接 MUST 只从本地环境变量或不受版本控制的.env注入；HTML 报告 MUST 位于 Git忽略的 /reports/apitest/，日志与报告 MUST 脱敏。测试 MUST 使用隔离库/文件卷/collection，不能对课堂现有数据故障注入或清理。

#### Scenario: Git and report inspection
- **WHEN** 生成HTML报告并检查待提交文件和报告内容
- **THEN** Git不包含报告、本地.env或真实密钥，报告中没有密码、Token、Key；公开配置示例仅有变量说明

### Requirement: Week four and failure acceptance gate
验收 MUST 包含登录成功/失败、Bearer失效/撤销、teacher/student权限、A/B双向隔离、原文提交/索引失败、keyword/vector/hybrid、引用、单次问答无命中、教师提示词、学生禁止修改、技能启停、会话创建/续问、正常SSE、失败/断连、历史持久化。受控网关 MUST 验证输入组合及调用时序；真实 MySQL/Qdrant 集成和真实模型教学对照结果 MUST 区分记录，不能把mock或历史已通过当当前部署全部通过。

#### Scenario: Week four Go regression
- **WHEN** 默认Go入口运行原第4课检索和材料测试矩阵
- **THEN** 原items、answer/sources、UUID和1-based切片、无命中零模型调用、依赖失败不降级及材料权限契约全部保持；原认证与上传事务测试通过

#### Scenario: Required streaming and teaching coverage
- **WHEN** 运行至少三片段正常流、流前失败、部分输出后失败、断连、重启和同题技能启停案例
- **THEN** HTML记录首片早于生成结束、数据库终态/前缀正确、引用一致、技能实际加载及教学区别；缺少真实模型检查明确标为未执行而非全通过
