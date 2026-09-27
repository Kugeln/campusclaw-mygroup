## Purpose

将 CampusClaw 的认证状态保存在服务端，使浏览器仅持有不透明会话标识，确保角色和班级来自可信服务端数据。该能力定义会话的建立、轮换、过期、撤销、持久化与故障行为，并保证既有教师学生权限和双向班级隔离在迁移后继续成立。

## ADDED Requirements

### Requirement: Opaque client session identifier
系统 MUST 将身份、角色、班级、会话版本及 CSRF 会话状态保存在服务端，客户端会话 Cookie SHALL 仅包含由服务端安全随机生成、至少具有 256 bit 随机熵的不透明标识，不包含上述字段或其编码/加密载荷。认证 MUST 依赖对应的有效服务端会话记录，禁止退回客户端自包含身份会话。

#### Scenario: Cookie contains only an opaque identifier
- **WHEN** 教师或学生通过正确账号密码及有效 CSRF 登录
- **THEN** 返回 200，服务端保存该用户的会话，客户端 Cookie 仅持有随机标识
- **AND** Cookie 不携带 user_id、role、class_id、session_version 或 CSRF 状态，后续请求凭该标识访问服务端会话

#### Scenario: Unknown altered or legacy cookie
- **WHEN** 请求携带未知、篡改、格式错误的标识，或迁移前的 signed-cookie session 访问受保护接口
- **THEN** API 返回 401，材料页面跳转登录页，不返回材料且不产生上传副作用
- **AND** 系统不以客户端提供的标识创建已认证会话，不尝试兼容解析旧身份 Cookie

#### Scenario: Server record is absent
- **WHEN** Cookie 本身未改变但其对应服务端会话已被删除
- **THEN** 该 Cookie 访问材料 API 返回 401，不能独立恢复身份

### Requirement: Trusted server identity resolution
每次受保护请求 MUST 使用服务端会话关联的现存用户核验身份、角色、单一班级和会话版本。会话与用户记录不一致时 MUST 拒绝认证。客户端在表单、JSON、查询参数、Cookie 身份字段或请求头中声明的 user_id、role、class_id、uploader_id MUST 不能改变身份、授权或上传归属。系统 SHALL 继续只支持 teacher/student。

#### Scenario: Login ignores self declared identity
- **WHEN** 学生使用自己的正确凭据登录，同时提交教师 role、他人的 user_id 或另一班 class_id
- **THEN** 登录后的身份仍为该学生及其数据库记录中的班级，无法获得教师权限

#### Scenario: Server identity becomes inconsistent
- **WHEN** 已登录用户被删除，或服务端会话身份字段、用户角色、班级或 session_version 不再一致
- **THEN** 后续受保护 API 返回 401，材料页面跳转登录页，不沿用旧权限

#### Scenario: Forged upload ownership
- **WHEN** 已登录教师携有效 CSRF 上传，同时附加 user_id、uploader_id、role 或 class_id 字段
- **THEN** 返回 400，不创建材料、解析记录或文件

### Requirement: Session lifecycle and fixation prevention
系统 SHALL 在登录前提供绑定服务端匿名会话的 CSRF token；成功登录 MUST 废止旧 Session ID 并生成新标识和新 CSRF token。已认证会话 MUST 自登录起固定 8 小时过期，普通访问不得续期。退出成功 MUST 保持现有全部设备会话失效行为并清除当前浏览器 Cookie；旧标识不得因后续保存而重新获得授权。Cookie MUST 保持 HttpOnly、SameSite=Lax、明确到期时间及现有 Secure 配置行为。

#### Scenario: Anonymous CSRF and rejected login
- **WHEN** 用户先访问登录页，再以缺失或错误 CSRF 提交登录
- **THEN** 登录页可获取有效 CSRF，错误 CSRF 请求返回 403，不能建立已认证会话
- **AND** 有效 CSRF 搭配错误密码返回统一 401，原认证身份被清除

#### Scenario: Login rotates the session
- **WHEN** 匿名用户登录成功，或已登录用户使用有效 CSRF 再次登录
- **THEN** 新标识和 CSRF 与原值不同，旧标识不再授权任何材料 API
- **AND** 新会话只保留新登录身份和必要会话字段，不保留旧身份或任意遗留字段

#### Scenario: Fixed lifetime
- **WHEN** 用户在 8 小时内持续访问后，在登录满 8 小时或之后访问材料 API
- **THEN** 返回 401，即使浏览器仍发送 Cookie，也不能依靠访问刷新有效期

#### Scenario: Logout revokes all devices
- **WHEN** 同一账号在两个客户端登录后，其中一个以有效 CSRF 退出
- **THEN** 返回 204、清除当前 Cookie，两个客户端之前的标识访问材料 API 均返回 401
- **AND** 后续保存不能重新建立已删除标识的会话或使旧用户会话版本重新获得授权

#### Scenario: Cookie attributes preserved
- **WHEN** 在默认安全配置下登录成功
- **THEN** 会话 Cookie 包含 HttpOnly、SameSite=Lax、Secure 和与会话期限一致的 Expires
- **AND** 仅在现有显式本地 HTTP 配置下允许关闭 Secure

### Requirement: Existing role and class authorization compatibility
服务端会话迁移 MUST 保留材料 API 路径、响应约定和仅两种角色的权限。teacher SHALL 上传、查看、下载本班材料；student SHALL 仅查看、下载本班材料。匿名上传 MUST 为 401；已登录学生直接上传 MUST 为 403，角色判断先于 CSRF 和文件处理。两种角色的列表、详情和下载 MUST 保持 A/B 双向隔离，且客户端班级参数不能扩大范围。

#### Scenario: Both teachers upload within their class
- **WHEN** A/B 班教师分别以有效会话和 CSRF 上传合法材料
- **THEN** 分别返回 201，上传者和班级等于对应服务端身份，本班教师和学生列表/详情返回 200 且可见该材料，下载返回 200 且字节一致

#### Scenario: Both students cannot upload
- **WHEN** A/B 班学生绕过 UI 直接上传，或在表单、JSON、查询参数和请求头中伪造教师角色及身份
- **THEN** 均返回 403，包括携带有效 CSRF 和合法文件的请求；不能新增或改变材料、KnowledgeEntry 和上传文件
- **AND** 缺失 CSRF 时同样优先返回教师角色拒绝，不能只依赖隐藏按钮或 CSRF 阻止上传

#### Scenario: Bidirectional cross class read isolation
- **WHEN** A 班教师/学生访问 B 班材料，或 B 班教师/学生访问 A 班材料
- **THEN** 列表只含自身班级；跨班详情和文件均返回与不存在材料一致的 404，不返回元数据或文件字节

#### Scenario: Forged class scope cannot expand reads
- **WHEN** 任一班教师或学生在列表请求附加 class_id、role 或 user_id 查询参数
- **THEN** 列表返回 400；在跨班详情或文件请求附加这些参数仍返回 404

### Requirement: Persistent session storage and safe initialization
系统 MUST 将会话状态持久化到现有服务端 SQLite，复用当前部署的数据卷，不新增外部会话服务。新旧数据库的会话存储准备 MUST 幂等且不覆盖账号、班级、材料、解析文本和文件。会话有效期 MUST 在每次读取时检查，不能依赖物理删除才失效；启动清理 SHALL 仅删除过期会话，不要求独立维护命令或后台任务。

#### Scenario: Fresh and existing deployment
- **WHEN** 初始化空数据目录，或升级含 A/B 班账号和材料的旧数据目录，并重复初始化
- **THEN** 会话存储就绪，原业务数据及文件保持不变，教师和学生可登录，旧 Cookie 需要重新登录

#### Scenario: Sessions are persisted on the server
- **WHEN** 登录成功后由使用同一数据库的新应用实例读取尚未过期或撤销的会话
- **THEN** 可凭原不透明标识解析该用户，证明状态位于持久化存储而不是进程内存或 Cookie 载荷

#### Scenario: Startup cleanup
- **WHEN** 启动准备清理同时含有效和过期会话的存储
- **THEN** 只删除过期会话，有效会话及全部业务数据和上传文件保持不变

### Requirement: Session storage failures deny authentication safely
会话存储读取或必要写入失败时，系统 MUST 返回通用 5xx，不得授予权限、伪报登录/退出成功或回退客户端身份会话。登录成功响应 MUST 在会话持久化成功后发出。系统 MUST 保持既有健康检查响应契约，不在日志或错误响应中暴露会话标识、密码或密钥。不要求新增统一 503 错误框架。

#### Scenario: Session read failure
- **WHEN** 携会话标识访问材料 API 时，会话存储不可读
- **THEN** 返回通用 5xx，不返回材料内容、不执行上传、不创建替代身份

#### Scenario: Login persistence failure
- **WHEN** 凭据及 CSRF 正确但新会话不能提交到持久化存储
- **THEN** 返回通用 5xx，不返回成功登录或发布新的有效 Cookie

#### Scenario: Anonymous health remains diagnosable
- **WHEN** 会话或业务数据库故障期间请求匿名 GET /health
- **THEN** 仍按现有健康检查规则返回约定的健康 JSON 和 200/503，不创建会话、不泄露内部异常
