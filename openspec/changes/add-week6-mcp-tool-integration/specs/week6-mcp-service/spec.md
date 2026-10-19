## Purpose

为 CampusClaw 第六课提供 week6-mcp-service 增量合同，并保护现有第五课认证、班级隔离、检索和流式行为。本版已亲自核对任务清单与全部九个课件子页，区分官方兼容要求、示范和项目补强；仍为待用户审核、尚未实施的规划文件。

## ADDED Requirements

### Requirement: W6-F01 独立网络服务

系统 SHALL 提供仓库内 mcp-server/ 独立进程，Go通过网络调用真实工具，不在主程序伪造结果。

分类：官方强制/兼容基线（附项目实现选择）。来源映射：O01, O33，原文页/节见 requirements-traceability.md；项目实现细节见 design.md。实施任务：1.1；测试：W6-T01。状态：已完成来源对齐审阅，未实施/未运行本业务测试。

#### Scenario: W6-T01 独立网络服务
- **WHEN** 独立启动服务并用独立客户端调用，再停止服务
- **THEN** 启动可发现工具，停止后Go观察到不可用而非返回模拟成功

### Requirement: W6-F02 课程HTTP协议边界

系统 SHALL 以独立HTTP JSON-RPC实现initialize、tools/list、tools/call，握手版本由项目声明，不把课程子集宣称为完整标准Streamable HTTP实现。

分类：官方强制/兼容基线（附项目实现选择）。来源映射：O02, O38，原文页/节见 requirements-traceability.md；项目实现细节见 design.md。实施任务：1.2；测试：W6-T02。状态：已完成来源对齐审阅，未实施/未运行本业务测试。

#### Scenario: W6-T02 课程HTTP协议边界
- **WHEN** 独立客户端握手后发现并调用，另发送不兼容握手和未知方法
- **THEN** 三方法真实可用，返回可读RPC错误；不要求课程未规定的GET SSE/批量/session才能通过

### Requirement: W6-F03 传输与信封

系统 SHALL 提供POST /mcp与UTF8 JSON-RPC 2.0、匹配id的result/error，base_url自动补/mcp；学生会话SSE与MCP HTTP分开。

分类：官方强制/兼容基线（附项目实现选择）。来源映射：O02，原文页/节见 requirements-traceability.md；项目实现细节见 design.md。实施任务：1.2；测试：W6-T03。状态：已完成来源对齐审阅，未实施/未运行本业务测试。

#### Scenario: W6-T03 传输与信封
- **WHEN** 发送正常RPC、畸形JSON、未知方法、错id；以不带/mcp的Base URL连接
- **THEN** 正确端点可用，result/error互斥，错误可读；错id不可算成功

### Requirement: W6-F04 工具发现

系统 SHALL 通过tools/list返回工具名称、说明和inputSchema，连接后以事务完整覆盖缓存，不追加幽灵项；项目限100项。

分类：官方强制/兼容基线（附项目实现选择）。来源映射：O02, O04, O08，原文页/节见 requirements-traceability.md；项目实现细节见 design.md。实施任务：1.3；测试：W6-T04。状态：已完成来源对齐审阅，未实施/未运行本业务测试。

#### Scenario: W6-T04 工具发现
- **WHEN** 连续连接两次，再让远端移除工具或返回重复名/超量列表
- **THEN** 两工具真实可见、行数不翻倍，失踪工具消失；非法列表不发布部分缓存

### Requirement: W6-F05 输入Schema

系统 SHALL 为每个工具发布并执行严格JSON Schema，不接受额外属性、类型错误、过长文本或非有限数值。

分类：官方强制/兼容基线（附项目实现选择）。来源映射：O02, O04，原文页/节见 requirements-traceability.md；项目实现细节见 design.md。实施任务：1.3；测试：W6-T05。状态：已完成来源对齐审阅，未实施/未运行本业务测试。

#### Scenario: W6-T05 输入Schema
- **WHEN** 读取Schema并对缺字段、类型错、额外字段、超长输入发起调用
- **THEN** Schema与执行一致；JSON-RPC -32602，无计算副作用

### Requirement: W6-F06 公式提示工具

系统 SHALL 实现formula_hint(query:string)并通过MCP content返回公式/步骤文本，不直接代算最终数值；具体题型为项目示范。

分类：课程示范+项目选择。来源映射：O04, O19, O39，原文页/节见 requirements-traceability.md；项目实现细节见 design.md。实施任务：1.4；测试：W6-T06。状态：已完成来源对齐审阅，未实施/未运行本业务测试。

#### Scenario: W6-T06 公式提示工具
- **WHEN** 以query传去括号或圆面积问题，再发送缺query/类型错误
- **THEN** 真实返回可用提示文本，输入非法被拒；不强制返回嵌套JSON

### Requirement: W6-F07 单位换算工具命名

系统 SHALL 以unit_convert作为项目最终主名称，参数value/from/to，真实提供同维度换算文本；unit_converter仅为上一版草案名称，不自动注册别名。

分类：课程示范+项目选择。来源映射：O01, O04, O19, O39，原文页/节见 requirements-traceability.md；项目实现细节见 design.md。实施任务：1.4；测试：W6-T07。状态：已完成来源对齐审阅，未实施/未运行本业务测试。

#### Scenario: W6-T07 单位换算工具命名
- **WHEN** tools/list检查名称，再用value=1,from=m,to=cm调用及异维度输入
- **THEN** 发现unit_convert且换算100 cm；不存在必选unit_converter要求，非法维度失败可读

### Requirement: W6-F08 执行返回格式

系统 SHALL tools/call结果接受MCP content的text数组和isError，成功文本先安全处理；不得要求所有远端text本身也是JSON。

分类：官方强制/兼容基线（附项目实现选择）。来源映射：O02, O04，原文页/节见 requirements-traceability.md；项目实现细节见 design.md。实施任务：1.4；测试：W6-T08。状态：已完成来源对齐审阅，未实施/未运行本业务测试。

#### Scenario: W6-T08 执行返回格式
- **WHEN** 执行返回普通中文文本、工具isError及未知工具
- **THEN** 正常文本能用于回答，isError为error且不把失败载荷当成功数据

### Requirement: W6-F09 服务Bearer鉴权

系统 SHALL 独立服务对协议入口校验专用Bearer凭据，缺失/错误为401，不能使用学生JWT代替服务凭据。

分类：官方强制/兼容基线（附项目实现选择）。来源映射：O03, O33，原文页/节见 requirements-traceability.md；项目实现细节见 design.md。实施任务：1.5；测试：W6-T09。状态：已完成来源对齐审阅，未实施/未运行本业务测试。

#### Scenario: W6-T09 服务Bearer鉴权
- **WHEN** 分别携正确、缺失、错误服务Token和学生JWT调用/mcp
- **THEN** 仅正确服务Token可发现或调用；响应无Token明文

### Requirement: W6-F10 健康与启动

系统 SHALL 提供最小无敏感信息健康检查和明确启动命令，缺少服务密钥启动失败；不以health成功替代协议连接成功。

分类：官方强制/兼容基线（附项目实现选择）。来源映射：O02, O03，原文页/节见 requirements-traceability.md；项目实现细节见 design.md。实施任务：1.1；测试：W6-T10。状态：已完成来源对齐审阅，未实施/未运行本业务测试。

#### Scenario: W6-T10 健康与启动
- **WHEN** 缺配置启动，再正确启动并检查/health及initialize
- **THEN** 缺配置非零退出；健康仅暴露状态，连接必须实际握手

### Requirement: W6-F11 工具执行限时

系统 SHALL 单工具执行及网络请求受可取消截止时间约束，不自动重试tools/call。

分类：项目补强（非老师新增要求）。来源映射：PROJECT，原文页/节见 requirements-traceability.md；项目实现细节见 design.md。实施任务：1.5；测试：W6-T11。状态：已完成来源对齐审阅，未实施/未运行本业务测试。

#### Scenario: W6-T11 工具执行限时
- **WHEN** 让工具延迟超过3秒或取消请求
- **THEN** 在限时内取消并返回timeout；实际执行次数至多一次

### Requirement: W6-F12 服务异常隔离

系统 SHALL 网络断开、非200、非法JSON、响应超限或协议错误不能伪装正常结果。

分类：官方强制/兼容基线（附项目实现选择）。来源映射：O02, O09，原文页/节见 requirements-traceability.md；项目实现细节见 design.md。实施任务：2.5；测试：W6-T12。状态：已完成来源对齐审阅，未实施/未运行本业务测试。

#### Scenario: W6-T12 服务异常隔离
- **WHEN** 停止服务及注入畸形/超限响应
- **THEN** 客户端产生稳定脱敏错误，不向模型注入失败载荷

### Requirement: W6-F13 Compose私网集成

系统 SHALL 以独立Compose服务接入Go私网，密钥由环境或secret注入，无默认公开宿主端口，不依赖Flask。

分类：官方强制/兼容基线（附项目实现选择）。来源映射：O01, O40，原文页/节见 requirements-traceability.md；项目实现细节见 design.md。实施任务：1.6；测试：W6-T13。状态：已完成来源对齐审阅，未实施/未运行本业务测试。

#### Scenario: W6-T13 Compose私网集成
- **WHEN** 在独立项目构建启动并从Go容器握手调用两工具
- **THEN** 服务健康且网络调用真实，无现有卷或Demo变动

### Requirement: W6-F14 入站安全

系统 SHALL 验证Origin及Content-Type，限制请求大小，禁止任意脚本、文件读取和命令执行。

分类：项目补强（非老师新增要求）。来源映射：O03，原文页/节见 requirements-traceability.md；项目实现细节见 design.md。实施任务：1.5；测试：W6-T14。状态：已完成来源对齐审阅，未实施/未运行本业务测试。

#### Scenario: W6-T14 入站安全
- **WHEN** 发送不允许Origin、超64KiB请求及脚本样式参数
- **THEN** 分别403、413或参数拒绝，无命令/文件副作用
