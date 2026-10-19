## Purpose

为 CampusClaw 第六课提供 week6-mcp-management 增量合同，并保护现有第五课认证、班级隔离、检索和流式行为。本版已亲自核对任务清单与全部九个课件子页，区分官方兼容要求、示范和项目补强；仍为待用户审核、尚未实施的规划文件。

## ADDED Requirements

### Requirement: W6-F15 登记服务器

系统 SHALL 教师以name/base_url/auth_token登记本班服务器，同班名称唯一且初始connected=false；响应has_auth与tool_count，不含凭据。

分类：官方强制/兼容基线（附项目实现选择）。来源映射：O05, O07, O11, O33，原文页/节见 requirements-traceability.md；项目实现细节见 design.md。实施任务：3.1；测试：W6-T15。状态：已完成来源对齐审阅，未实施/未运行本业务测试。

#### Scenario: W6-T15 登记服务器
- **WHEN** A教师登记、同班重名再登记、B班同名登记，并检查全部JSON字段
- **THEN** 分别201/400/201；响应含base_url/has_auth/connected/tool_count，不含auth_token或token

### Requirement: W6-F16 列表隔离

系统 SHALL 服务器列表仅显示Principal当前班级，不接受客户端class_id覆盖。

分类：官方强制/兼容基线（附项目实现选择）。来源映射：O06, O11, O37，原文页/节见 requirements-traceability.md；项目实现细节见 design.md。实施任务：3.4；测试：W6-T16。状态：已完成来源对齐审阅，未实施/未运行本业务测试。

#### Scenario: W6-T16 列表隔离
- **WHEN** A/B教师分别列表并传伪造class_id
- **THEN** 仅各自班级；覆盖字段400，无外班数据

### Requirement: W6-F17 列表读取与状态

系统 SHALL 通过本班服务器列表提供配置及connected/tool_count/has_auth，不增加单条GET详情作为课程必需接口。

分类：官方强制/兼容基线（附项目实现选择）。来源映射：O05, O11，原文页/节见 requirements-traceability.md；项目实现细节见 design.md。实施任务：3.1；测试：W6-T17。状态：已完成来源对齐审阅，未实施/未运行本业务测试。

#### Scenario: W6-T17 列表读取与状态
- **WHEN** 读取列表，并请求不存在或外班编号的tools接口
- **THEN** 列表只含本班，按编号资源404一致；无需GET /servers/{id}额外路由

### Requirement: W6-F18 修改与改名

系统 SHALL PUT以name/base_url及可选auth_token修改本班服务器，空auth_token保留旧值；修改清空连接与工具缓存，改名不改技能原文。

分类：官方强制/兼容基线（附项目实现选择）。来源映射：O05, O11, O12，原文页/节见 requirements-traceability.md；项目实现细节见 design.md。实施任务：3.1；测试：W6-T18。状态：已完成来源对齐审阅，未实施/未运行本业务测试。

#### Scenario: W6-T18 修改与改名
- **WHEN** 连接后改地址/Token/名称；令牌留空，并用原技能引用提问
- **THEN** 响应200且connected=false/tool_count=0；空令牌保留；旧名引用denied且无新审计行

### Requirement: W6-F19 连接握手

系统 SHALL connect实际完成initialize与tools/list后返回connected/tools，远端不可达、鉴权或握手失败返回502并connected=false；数据库失败503。

分类：官方强制/兼容基线（附项目实现选择）。来源映射：O08, O09, O11, O33，原文页/节见 requirements-traceability.md；项目实现细节见 design.md。实施任务：3.2；测试：W6-T19。状态：已完成来源对齐审阅，未实施/未运行本业务测试。

#### Scenario: W6-T19 连接握手
- **WHEN** 正常连接、错令牌、超时、非法发现及DB发布失败
- **THEN** 正常200含connected=true及工具；远端失败502，DB失败503，无部分新缓存或假成功

### Requirement: W6-F20 断开服务器

系统 SHALL disconnect幂等置connected=false并清空工具缓存，保留技能原文和已有审计。

分类：官方强制/兼容基线（附项目实现选择）。来源映射：O10, O11, O28, O33，原文页/节见 requirements-traceability.md；项目实现细节见 design.md。实施任务：3.2；测试：W6-T20。状态：已完成来源对齐审阅，未实施/未运行本业务测试。

#### Scenario: W6-T20 断开服务器
- **WHEN** 两次断开，查tools再用命中该技能的问题提问
- **THEN** 返回connected=false、tools=[]、零远端call；meta denied且增加一条本服务器denied审计

### Requirement: W6-F21 通过重连重新发现

系统 SHALL 重新发现复用既有connect接口，不新增discover接口；每次都握手并完整覆盖缓存。

分类：官方强制/兼容基线（附项目实现选择）。来源映射：O08, O11，原文页/节见 requirements-traceability.md；项目实现细节见 design.md。实施任务：3.3；测试：W6-T21。状态：已完成来源对齐审阅，未实施/未运行本业务测试。

#### Scenario: W6-T21 通过重连重新发现
- **WHEN** 教师连续两次connect，第二次远端工具发生变化
- **THEN** 通过同一官方接口刷新，缓存不翻倍；不要求POST /discover或单独的409状态

### Requirement: W6-F22 缓存原子刷新

系统 SHALL 重连事务内先删后插，断开/修改时清空；项目revision/CAS防止旧响应覆盖新配置。

分类：官方强制/兼容基线（附项目实现选择）。来源映射：O08, O12，原文页/节见 requirements-traceability.md；项目实现细节见 design.md。实施任务：3.3；测试：W6-T22。状态：已完成来源对齐审阅，未实施/未运行本业务测试。

#### Scenario: W6-T22 缓存原子刷新
- **WHEN** 重连刷新时并发修改地址或断开并让旧响应晚到
- **THEN** 只有当前配置对应列表可用，无部分列表、幽灵工具或旧响应复活

### Requirement: W6-F23 工具列表

系统 SHALL 仅教师通过官方tools接口查询本班服务器最近成功发现的清单，断开后返回空列表。

分类：官方强制/兼容基线（附项目实现选择）。来源映射：O04, O06, O08, O11，原文页/节见 requirements-traceability.md；项目实现细节见 design.md。实施任务：3.3；测试：W6-T23。状态：已完成来源对齐审阅，未实施/未运行本业务测试。

#### Scenario: W6-T23 工具列表
- **WHEN** A教师、A学生和B教师查A服务器工具，再由A断开重查
- **THEN** 分别200/403/404；断开后tools=[]且无凭据

### Requirement: W6-F24 删除与审计外键

系统 SHALL DELETE无审计服务器时物理删除并级联缓存；已有mcp_tool_calls引用则返回503且整体不变，重复成功删除404；技能原文不改。

分类：官方强制/兼容基线（附项目实现选择）。来源映射：O11, O14, O28, O30, O33，原文页/节见 requirements-traceability.md；项目实现细节见 design.md。实施任务：3.1；测试：W6-T24。状态：已完成来源对齐审阅，未实施/未运行本业务测试。

#### Scenario: W6-T24 删除与审计外键
- **WHEN** 分别删除无审计/有审计服务，并对已成功删除的旧引用提问
- **THEN** 无审计204且缓存清空；有审计503且服务/缓存/审计不变；旧引用meta denied无新增审计

### Requirement: W6-F25 教师角色与班级校验

系统 SHALL 所有管理操作复用Bearer当前身份和teacher guard，查询/更新包含class_id；匿名及学生不可访问。

分类：官方强制/兼容基线（附项目实现选择）。来源映射：O06, O11, O37，原文页/节见 requirements-traceability.md；项目实现细节见 design.md。实施任务：3.4；测试：W6-T25。状态：已完成来源对齐审阅，未实施/未运行本业务测试。

#### Scenario: W6-T25 教师角色与班级校验
- **WHEN** 对全部管理URL/方法分别使用匿名、学生、外班教师
- **THEN** 匿名401、学生403、外班资源404，拒绝不改DB或发起网络调用

### Requirement: W6-F26 凭据保密

系统 SHALL API以auth_token接收服务令牌且只返回has_auth；原文不进响应、前端持久存储、Git、镜像或日志；认证加密为项目补强。

分类：官方强制/兼容基线（附项目实现选择）。来源映射：O03, O05, O33，原文页/节见 requirements-traceability.md；项目实现细节见 design.md。实施任务：2.4；测试：W6-T26。状态：已完成来源对齐审阅，未实施/未运行本业务测试。

#### Scenario: W6-T26 凭据保密
- **WHEN** 保存标记Token，检查列表/编辑响应、SSE/审计/报告及镜像构建输入
- **THEN** 仅has_auth布尔返回；密码框写后清空；服务端可用于Bearer，外部无密钥明文

### Requirement: W6-F27 出站地址限制

系统 SHALL 服务器登记和每次连接/call校验管理员允许的scheme/host/port及实际解析IP，禁重定向和云元数据/回环/未允许内网。

分类：项目补强（非老师新增要求）。来源映射：O38，原文页/节见 requirements-traceability.md；项目实现细节见 design.md。实施任务：2.6；测试：W6-T27。状态：已完成来源对齐审阅，未实施/未运行本业务测试。

#### Scenario: W6-T27 出站地址限制
- **WHEN** 登记metadata/localhost或以DNS重绑定及302跳转绕过
- **THEN** 被拒绝且目标零访问；仅显式允许的私网mcp-server可访问

### Requirement: W6-F28 三表存储约束

系统 SHALL 三张MCP表保留base_url/connected/tool_name/schema_json/tool_ref/detail等字段分工，班级唯一、缓存server唯一、审计三状态及外键约束生效。

分类：官方强制/兼容基线（附项目实现选择）。来源映射：O05, O07, O13, O14, O40，原文页/节见 requirements-traceability.md；项目实现细节见 design.md。实施任务：2.1；测试：W6-T28。状态：已完成来源对齐审阅，未实施/未运行本业务测试。

#### Scenario: W6-T28 三表存储约束
- **WHEN** 隔离库检查字段、重复名、重复工具、非法status、断开缓存及有审计删除
- **THEN** 约束拒绝违规，connected仅0/1；审计保留且关联服务删除503，不用软删除替代

### Requirement: W6-F29 显式幂等迁移

系统 SHALL 新schema通过显式离线v4升级及完整catalog/账本/就绪校验，新旧路径、重试及中断可恢复，HTTP不执行DDL。

分类：项目补强（非老师新增要求）。来源映射：O40，原文页/节见 requirements-traceability.md；项目实现细节见 design.md。实施任务：2.2；测试：W6-T29。状态：已完成来源对齐审阅，未实施/未运行本业务测试。

#### Scenario: W6-T29 显式幂等迁移
- **WHEN** 独立库跑fresh、v1/v2/v3到v4、重复及每步中断；漂移后启动
- **THEN** 合法路径就绪；重复不复活数据；漂移/不完整拒绝，无静默修复

### Requirement: W6-F30 旧数据保护

系统 SHALL 迁移保留用户、材料、切片、助手、技能source/绑定、历史消息引用和占位数据，不自动扩大授权。

分类：项目补强（非老师新增要求）。来源映射：O40，原文页/节见 requirements-traceability.md；项目实现细节见 design.md。实施任务：2.3；测试：W6-T30。状态：已完成来源对齐审阅，未实施/未运行本业务测试。

#### Scenario: W6-T30 旧数据保护
- **WHEN** 升级前后比较ID、数量、source/hash及旧消息快照
- **THEN** 旧内容逐项一致；没有给旧技能补MCP绑定或改变空材料语义

### Requirement: W6-F31 审计归属与例外

系统 SHALL 找到本班服务器时对ok、未连接denied、远端失败error各追加一行，含tool_ref/server/user/conversation/time/detail；改名或删除导致找不到旧名时仅返回denied而无审计行。

分类：官方强制/兼容基线（附项目实现选择）。来源映射：O13, O27, O28, O33，原文页/节见 requirements-traceability.md；项目实现细节见 design.md。实施任务：2.7；测试：W6-T31。状态：已完成来源对齐审阅，未实施/未运行本业务测试。

#### Scenario: W6-T31 审计归属与例外
- **WHEN** 触发成功、断开、改名、无审计服务删除、工具失败并比较审计增量
- **THEN** 各存在服务分支各加一条终态行；找不到旧名零新增，detail脱敏且≤512字，成功摘要≤200字

### Requirement: W6-F32 终态追加及持久化失败

系统 SHALL mcp_tool_calls只追加终态ok/denied/error，不写pending后改行；审计写失败不得伪报完整成功或重放远端动作。

分类：官方强制/兼容基线（附项目实现选择）。来源映射：O13, O27, O30，原文页/节见 requirements-traceability.md；项目实现细节见 design.md。实施任务：2.7；测试：W6-T32。状态：已完成来源对齐审阅，未实施/未运行本业务测试。

#### Scenario: W6-T32 终态追加及持久化失败
- **WHEN** 故障注入终态INSERT失败，并在远端返回后模拟进程退出
- **THEN** meta前503/failed且不重放；披露崩溃可能无审计，不伪造全路径可追踪保证

### Requirement: W6-F33 历史审计保护

系统 SHALL 不随服务/工具/技能/会话清理级联删除调用审计；有审计服务删除受限；课程未规定保存天数，不虚构要求。

分类：官方强制/兼容基线（附项目实现选择）。来源映射：O13, O14, O30，原文页/节见 requirements-traceability.md；项目实现细节见 design.md。实施任务：2.7；测试：W6-T33。状态：已完成来源对齐审阅，未实施/未运行本业务测试。

#### Scenario: W6-T33 历史审计保护
- **WHEN** 尝试删除有审计服务、删除内部会话并查询历史
- **THEN** 服务删除503；历史仍在，审计无公开删除API；期限作为项目运维事项而非缺课程来源
