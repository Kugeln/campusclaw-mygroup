## Purpose

为 CampusClaw 定义独立于后端语言及数据库迁移的 Bearer Token 认证契约，覆盖账号密码登录、认证期限、服务端可信身份与退出撤销，并确保教师学生权限及双向班级隔离在认证方式替换后保持有效。

## ADDED Requirements

### Requirement: Password login issues an expiring token
系统 SHALL 保持现有 username/password 和密码哈希验证，正确凭据返回 200 及 `access_token`、`token_type: Bearer`、`expires_in: 28800` 和 `redirect: /materials`；Token MUST 自签发起固定 8 小时过期，普通访问不得续期。

#### Scenario: Teacher and student login
- **WHEN** 教师或学生向 POST /api/auth/login 提交正确凭据，不携带 Cookie 或 CSRF token
- **THEN** 返回上述 Token 响应，不建立认证 Cookie；错误密码或不存在账号统一返回 401，不签发 Token、不透露账号是否存在

#### Scenario: Fixed expiry
- **WHEN** 用户持续访问后在签发满 8 小时或之后重用 Token
- **THEN** 受保护 API 返回 401，必须重新登录

### Requirement: Explicit bearer authentication
所有受保护 API MUST 仅使用 `Authorization: Bearer <token>` 认证，失败时返回 401 且不返回受保护内容或产生业务写入；有效身份但角色权限不足 MUST 返回 403。跨班资源继续按既有隐藏资源约定返回 404。

#### Scenario: Bearer succeeds without cookies
- **WHEN** 用户仅携有效 Bearer Token 请求本班材料列表、详情或下载
- **THEN** 返回 200 和其有权访问的数据，无需 Cookie 或 CSRF token

#### Scenario: Invalid authentication matrix
- **WHEN** Authorization 缺失、scheme 非 Bearer、Token malformed、签名无效、已过期、关联用户不存在或失效、版本不匹配或已撤销
- **THEN** 均返回 401，不回退到 Cookie、查询参数或表单中的 Token

#### Scenario: Legacy cookie cannot authorize
- **WHEN** 请求只携旧有效认证 Cookie，或 Cookie 与无效 Bearer 同时存在
- **THEN** 受保护 API 返回 401，旧 Cookie 不能恢复身份

### Requirement: Current server identity controls authorization
系统 MUST 在每个受保护请求中确认当前用户及有效版本，使用服务端当前 role/class_id 授权。客户端提交的 user_id、role、class_id、uploader_id 或 Token 中附带的旧角色班级值 MUST 不能提升权限、改变上传归属或扩大读取范围。

#### Scenario: Server role or class changes
- **WHEN** 用户 Token 未过期且版本匹配，但服务端角色或班级已经改变
- **THEN** 后续请求立即使用当前角色和班级；无效角色或无有效班级的用户返回 401

#### Scenario: Current identity for browser display
- **WHEN** 客户端携有效 Bearer 请求 GET /api/auth/me
- **THEN** 返回 200 及当前服务端 user_id、role、class_id、class_name，不返回密码、密码哈希或其他用户身份

#### Scenario: Forged identity
- **WHEN** 用户登录或访问时在表单、JSON、查询参数或自定义 header 伪造身份字段
- **THEN** 登录身份只由正确凭据确认；学生上传仍为 403；教师上传附加身份字段返回 400，列表附加身份查询参数返回 400，跨班详情或下载仍为 404；其他伪造字段不改变可信身份

### Requirement: Preserve role and class permissions
系统 SHALL 仅支持 teacher/student；教师可上传及读取本班材料，学生只能读取本班材料。拒绝上传 MUST 不产生材料记录、解析记录或持久化文件。

#### Scenario: Upload authorization
- **WHEN** A/B 班教师各自携有效 Token 上传合法文件，或 A/B 班学生携有效 Token 直接上传
- **THEN** 教师分别返回 201，归属其当前服务端身份；学生均返回 403，包括伪造教师字段的请求；匿名上传返回 401

#### Scenario: Bidirectional isolation
- **WHEN** A/B 班教师和学生分别请求列表、本班及另一班的详情和文件
- **THEN** 列表及计数仅含本班，本班详情/下载为 200 且文件字节正确，跨班详情/下载与不存在资源同为 404，不泄露元数据或内容

### Requirement: Logout revokes previously issued tokens
POST /api/auth/logout MUST 要求有效 Bearer Token；成功返回 204 前必须持久化撤销该账号全部此前签发版本的 Token。撤销写入失败 MUST 返回通用 5xx，不宣称退出成功。

#### Scenario: All devices revoked
- **WHEN** 同一账号两个客户端登录，其中一个成功退出
- **THEN** 两个客户端旧 Token 的后续受保护请求均返回 401；重新登录获得的新 Token 可用，服务重启不能恢复旧 Token

#### Scenario: Revocation failure
- **WHEN** logout 的撤销状态无法提交
- **THEN** 返回 5xx，客户端清除本地认证状态并提示服务端退出未确认，不能宣称旧 Token 已被撤销

### Requirement: Browser sends tokens explicitly
前端 SHALL 保存登录 Token，统一为身份查询、材料列表、上传、详情、下载及 logout 添加 Authorization Header。退出或收到 401 MUST 清除本地 Token 和身份数据并回到登录页。Token MUST 不出现在 URL、页面链接或认证 Cookie 中。

#### Scenario: Page navigation and download
- **WHEN** 登录后进入或刷新 /materials 并使用列表、上传、阅读和下载
- **THEN** 页面通过携带 Bearer 的请求加载身份和材料，下载请求同样带 Authorization；无 Token 时只显示无敏感数据的页面壳并跳转登录页

#### Scenario: Browser logout and expiry
- **WHEN** 用户退出或任一受保护请求收到 401
- **THEN** 当前页面不再保留 Token、用户信息或材料数据；403 仅提示权限不足，不视为认证失效

### Requirement: Fail closed and preserve storage boundary
系统 MUST 从环境读取必要签名密钥，缺失时拒绝启动认证服务；不得硬编码或提交真实密钥、记录完整 Token 或输出密码/密码哈希。当前用户状态不可读时 MUST 返回通用 5xx 且拒绝授权。现有 MySQL sessions 表及 schema MUST 保留，不删除或重新迁移，不作为 Token 认证来源。

#### Scenario: Configuration and state failure
- **WHEN** 必要签名密钥缺失，或请求时当前用户状态不可读
- **THEN** 分别拒绝启动或返回通用 5xx，不签发替代凭据、不执行受保护业务、不泄露秘密

#### Scenario: No session dependency
- **WHEN** Token 认证启用且 sessions 表没有会话记录
- **THEN** 正确登录、有效 Bearer 访问和版本撤销仍可工作，sessions schema 保持原样
