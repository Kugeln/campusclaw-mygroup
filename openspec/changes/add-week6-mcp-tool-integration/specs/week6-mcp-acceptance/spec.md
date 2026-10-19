## Purpose

为 CampusClaw 第六课提供 week6-mcp-acceptance 增量合同，并保护现有第五课认证、班级隔离、检索和流式行为。本版已亲自核对任务清单与全部九个课件子页，区分官方兼容要求、示范和项目补强；仍为待用户审核、尚未实施的规划文件。

## ADDED Requirements

### Requirement: W6-F63 教师MCP页签

系统 SHALL 最小扩展现有教师tutor页面提供MCP页签、服务器列表/登记/修改/删除/连接发现/断开/工具数/最近调用，复用8个官方操作。

分类：官方强制/兼容基线（附项目实现选择）。来源映射：O05, O31, O40，原文页/节见 requirements-traceability.md；项目实现细节见 design.md。实施任务：5.1；测试：W6-T63。状态：已完成来源对齐审阅，未实施/未运行本业务测试。

#### Scenario: W6-T63 教师MCP页签
- **WHEN** 教师完成全链路并查看有审计删除503、改名失效及连接502
- **THEN** UI反映真实状态和失败，has_auth反馈但无令牌回填，不依赖额外详情/discover接口

### Requirement: W6-F64 学生最小反馈

系统 SHALL 学生正常流式问答显示脱敏工具成功/拒绝/失败反馈，无管理入口或服务器凭据。

分类：官方强制/兼容基线（附项目实现选择）。来源映射：O22, O23, O31, O40，原文页/节见 requirements-traceability.md；项目实现细节见 design.md。实施任务：5.2；测试：W6-T64。状态：已完成来源对齐审阅，未实施/未运行本业务测试。

#### Scenario: W6-T64 学生最小反馈
- **WHEN** 学生走三种工具状态及无工具流
- **THEN** 工具反馈与meta一致，delta仍增量，不能打开管理操作

### Requirement: W6-F65 视觉交互兼容

系统 SHALL 保留CampusClaw品牌、Logo、导航、知识库、材料页、历史及原SSE解析/异常交互。

分类：官方强制/兼容基线（附项目实现选择）。来源映射：O31, O37，原文页/节见 requirements-traceability.md；项目实现细节见 design.md。实施任务：5.2；测试：W6-T65。状态：已完成来源对齐审阅，未实施/未运行本业务测试。

#### Scenario: W6-T65 视觉交互兼容
- **WHEN** 浏览器比较各角色页面并测试UTF8跨块、EOF和401
- **THEN** 原页面功能及身份处理保持，无破坏布局或无done假成功

### Requirement: W6-F66 OpenAPI官方兼容

系统 SHALL 沿用注解生成链，覆盖官方8个MCP URL/方法及base_url/auth_token输入、has_auth输出、tool_ref/detail、ref/status/text和无调用省略；Swagger可试调。

分类：官方强制/兼容基线（附项目实现选择）。来源映射：O05, O11, O22, O23, O32, O33, O40，原文页/节见 requirements-traceability.md；项目实现细节见 design.md。实施任务：5.3；测试：W6-T66。状态：已完成来源对齐审阅，未实施/未运行本业务测试。

#### Scenario: W6-T66 OpenAPI官方兼容
- **WHEN** 生成及-Check、动态对照8操作，再浏览器Authorize执行connect/tools/技能保存
- **THEN** 无漏项/漂移或手写第二份契约；无额外详情/discover必需操作，不重写第五课JWT或材料UUID

### Requirement: W6-F67 自动化真实工具矩阵

系统 SHALL 以隔离MySQL/Qdrant和独立MCP真实网络调用覆盖正常/错误鉴权、Schema、缓存、权限、迁移、审计与全部拒绝/故障。

分类：官方建议、项目要求。来源映射：O01, O33, O34, O40，原文页/节见 requirements-traceability.md；项目实现细节见 design.md。实施任务：6.1；测试：W6-T67。状态：已完成来源对齐审阅，未实施/未运行本业务测试。

#### Scenario: W6-T67 自动化真实工具矩阵
- **WHEN** 执行逐条W6测试并故意注入一次失败
- **THEN** 正常全通过，注入失败非零；不能用模拟Go返回替代两个真实工具

### Requirement: W6-F68 双向隔离与真实同伴

系统 SHALL 自动验证A/B双向和同班owner边界，并由真实同伴用本人账号完成服务器/工具隔离、令牌不回传、断开denied审计及本班审计四项，留请求响应证据。

分类：官方强制/兼容基线（附项目实现选择）。来源映射：O06, O34, O36，原文页/节见 requirements-traceability.md；项目实现细节见 design.md。实施任务：6.2/6.7；测试：W6-T68。状态：已完成来源对齐审阅，未实施/未运行本业务测试。

#### Scenario: W6-T68 双向隔离与真实同伴
- **WHEN** 执行自动负测，再请真实同伴按四项操作并记录本人确认、账号角色及脱敏证据
- **THEN** 自动权限全部通过且真实同伴四项证据齐备；自动账号不可冒充同伴，缺人不得勾选人工任务

### Requirement: W6-F69 第五课回归

系统 SHALL 回归Bearer/撤销、材料/索引、三种检索、旧Ask零命中、技能启停/解绑/路由、6条窗口、引用、SSE及恢复。

分类：官方强制/兼容基线（附项目实现选择）。来源映射：O37，原文页/节见 requirements-traceability.md；项目实现细节见 design.md。实施任务：6.3；测试：W6-T69。状态：已完成来源对齐审阅，未实施/未运行本业务测试。

#### Scenario: W6-T69 第五课回归
- **WHEN** 在新隔离环境运行当前align验收及相关Go/browser测试
- **THEN** 原保护矩阵全部通过；不沿用旧无据零调用断言到Week5

### Requirement: W6-F70 报告与官方建议分类

系统 SHALL 项目继续交付脱敏JSON/HTML与完整测试元信息；课程任务清单第五节将apitest/run_all.py及HTML列为建议，不能称老师强制。

分类：官方建议、项目要求。来源映射：O35, O40，原文页/节见 requirements-traceability.md；项目实现细节见 design.md。实施任务：6.4；测试：W6-T70。状态：已完成来源对齐审阅，未实施/未运行本业务测试。

#### Scenario: W6-T70 报告与官方建议分类
- **WHEN** 执行完整/故意失败/缺配置报告分支，核对来源分类
- **THEN** 报告真实区分执行状态和模型性质、HTML不入Git；官方建议与项目必交付分开

### Requirement: W6-F71 真实模型预算人工验收

系统 SHALL 真实模型验证需新的明确上限/费用授权，默认禁用、禁止自动重试；真实教学安全需人工审读，不把受控模型算真实。

分类：项目补强（非老师新增要求）。来源映射：O38，原文页/节见 requirements-traceability.md；项目实现细节见 design.md。实施任务：6.5；测试：W6-T71。状态：已完成来源对齐审阅，未实施/未运行本业务测试。

#### Scenario: W6-T71 真实模型预算人工验收
- **WHEN** 无授权尝试real阶段，再在授权额度内执行有限场景
- **THEN** 无授权零收费请求；账本逐次扣减，人工审核工具依据/公式/换算与注入边界，未执行明确列出

### Requirement: W6-F72 来源审核与后续门槛

系统 SHALL 保留现有change及本轮文档边界，对已读取任务清单/概览/9子页/官方OpenAPI建立双向映射，明确来源冲突与项目扩展；完成后等待审核不自动实施。

分类：官方强制/兼容基线（附项目实现选择）。来源映射：O38, O40，原文页/节见 requirements-traceability.md；项目实现细节见 design.md。实施任务：6.6；测试：W6-T72。状态：已完成来源对齐审阅，未实施/未运行本业务测试。

#### Scenario: W6-T72 来源审核与后续门槛
- **WHEN** 核对11份课程正文读取、8接口及72项双向追踪、文档修改范围和strict
- **THEN** 来源核对记录真实可定位，未宣称浏览器已调试或业务已验收；待决冲突保留，实施需新指令
