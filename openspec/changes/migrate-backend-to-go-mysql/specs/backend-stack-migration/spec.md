## Purpose

定义 CampusClaw 从 Flask 和 SQLite 迁移到 Nginx、Go 与 MySQL 的数据和行为验收契约，保证现有 Bearer Token 认证、班级权限及材料完整性在整体技术栈替换后继续成立，并提供可验证的迁移和切换边界。

## ADDED Requirements

### Requirement: Unified target runtime
最终课程运行 MUST 使用 Browser → Nginx → Go → MySQL；Go SHALL 是唯一业务后端，所有现有关系数据 SHALL 保存于 MySQL，文件 SHALL 使用私有持久存储。运行 MUST 不依赖 Flask、Python 或 SQLite，不新增第四课功能或修改检索 change。

#### Scenario: Start without legacy runtime
- **WHEN** 完成配置和初始化后执行 docker compose up --build，且无 Flask 进程和 SQLite 运行挂载
- **THEN** 经 Nginx 可完成登录和材料流程，MySQL 及 Go 无宿主公开端口，上传文件无静态直达路径

#### Scenario: Durable restart
- **WHEN** 上传后不删除卷地重建并重启 Compose
- **THEN** 用户、班级、session_version、材料、正文和文件保留；签名密钥保持不变时，未过期未撤销 Token 继续有效，已撤销 Token 不得复活；不读写或清理 sessions 来维持认证

### Requirement: Complete verified data migration
迁移 MUST 保留 classes、users、materials、knowledge_entries、assignments、assistants、skills 七表全部数据及约束语义，包括原 ID、密码哈希、角色、班级、session_version、上传者、时间、文件关联和 plain_text。八表结构 MUST 保留，但目标 sessions SHALL 初始为空，旧会话仅留在源备份；cutover 后 MUST 重新登录，不兼容旧 SID；目标 SHALL 使用与旧入口不同的新 JWT_SIGNING_SECRET 拒绝切换前 Token，同时保持 session_version 原值导入。已有 MySQL sessions schema 和迁移工具 MUST 保留，Token 认证 MUST 不依赖 sessions 读写或清理，不引入第二套认证。迁移 SHALL 在停止写入的快照上执行，并逐表验证数量、ID、关键关系和内容完整性。验证失败 MUST 阻止切换；旧数据库和文件备份 MUST 不被自动删除。重复执行 MUST 不覆盖不同的目标数据。

#### Scenario: Preserve complete source
- **WHEN** 迁移包含八表数据、中文正文、有效和过期 Session 的 SQLite 快照
- **THEN** 目标七表数量和 ID 集合一致，文本与文件 hash 一致，外键、班级关系和用户 session_version 一致；八表结构完整，sessions 初始为空
- **AND** 仅旧 Cookie 或切换前 Token 访问受保护 API 返回401；/materials 返回无敏感数据的页面壳，前端缺Token或API401时跳转登录；正确凭据重新登录取得新Token，正常重启且密钥不变时保留未过期未撤销Token的有效性

#### Scenario: Invalid or repeated migration
- **WHEN** 发现缺文件、正文损坏、非法关系、目标冲突或校验不一致，或对已迁移目标重跑
- **THEN** 不报告虚假成功，不自动切换或删除源数据；完全相同目标可重新验证，非相同目标拒绝覆盖并允许修复后显式重试

### Requirement: Bearer authentication compatibility
系统 SHALL 沿用 `replace-session-auth-with-token-auth` 的外部契约，保留 GET /login、POST /api/auth/login、POST /api/auth/logout、GET /api/auth/me、GET /materials 及现有材料 API。密码 MUST 兼容 Werkzeug `pbkdf2:sha256:1000000$salt$hex`（salt 原字符串 UTF-8 字节、相同迭代次数、常量时间比较），新账号保留随机盐与该格式；未知格式预检拒绝，不依赖 Python 验证。登录、退出和上传 MUST 不要求认证 Cookie 或 CSRF token，不创建匿名 Session。签名密钥、密码、密码哈希 MUST 不进入响应、浏览器资源或日志；access_token SHALL 仅由登录响应返回，MUST 不进入日志、URL、页面链接或 Cookie。

#### Scenario: Login and anonymous access
- **WHEN** teacher/student 无 Cookie/CSRF、用正确 username/password 登录
- **THEN** 返回200及 access_token、token_type=Bearer、expires_in=28800、redirect=/materials，Cache-Control为no-store，不创建认证Cookie；错误密码/未知账号统一401，不签发Token
- **AND** 无有效Bearer访问受保护API返回401及WWW-Authenticate: Bearer；/materials返回200公共页面壳，不包含身份/材料，前端无Token或API401时清理状态并跳转/login

#### Scenario: Explicit bearer only
- **WHEN** Authorization缺失、非Bearer、重复/歧义、凭据为空，或Token格式/签名/claims非法、过期、用户失效或版本不匹配
- **THEN** 受保护API返回401且无业务副作用；不从Cookie、query或form获取Token，不以旧Cookie兜底；scheme大小写不敏感，有效Bearer无需Cookie/CSRF即可访问

### Requirement: Fixed token lifecycle and persistent revocation
系统 MUST 使用成熟JWT库、固定HS256，只签发sub/ver/iat/exp；sub MUST 为可映射到正signed BIGINT用户ID的十进制字符串，ver MUST 为非负且小于9223372036854775807的整数，iat/exp MUST 为整数Unix秒，拒绝布尔或浮点替代。iat MUST 非负且不在未来，exp MUST 等于iat+28800，当前时间 MUST 小于exp；普通访问 MUST 不续期。系统 MUST 每请求查询当前用户和有效班级，比较ver与users.session_version，并以当前role/class_id生成只读principal；业务 MUST 不解析JWT或读取Session。

#### Scenario: Fixed expiry and repeated login
- **WHEN** 用户重新登录或持续访问至签发满8小时
- **THEN** 新登录签发固定期限Token，不隐式撤销其他设备；访问不续期，到期返回401；不执行SID/CSRF轮换，不依赖sessions记录

#### Scenario: Current identity and invalid accounts
- **WHEN** Token签名/期限/版本有效，但数据库角色或班级已合法变化，或账号不存在、角色非法、没有有效班级、版本不匹配
- **THEN** 合法变化立即使用当前数据库角色/班级；失效情况返回401，不新增enabled字段、不使用客户端或Token附带的role/class_id授权

#### Scenario: Current identity endpoint
- **WHEN** 用户携有效Bearer请求GET /api/auth/me
- **THEN** 返回200及当前user_id、role、class_id、class_name，不返回密码或密码哈希

#### Scenario: Logout all devices
- **WHEN** 同一账号两个客户端取得Token，其中一个携有效Bearer退出
- **THEN** MySQL事务原子递增session_version，提交后返回204；两端旧版本Token后续请求均401，重新登录Token可用，重启不得复活旧Token；不删除sessions，不创建每Token记录或黑名单

#### Scenario: Storage failure and no session dependency
- **WHEN** 登录/认证读取用户或logout提交发生存储故障，或sessions没有记录
- **THEN** 存储故障返回通用500，不签发Token、不授予权限、不伪报退出成功；sessions为空不影响正确登录、Bearer访问或版本撤销，认证不读取、写入或清理sessions

### Requirement: Browser token flow compatibility
前端 SHALL 沿用同源sessionStorage和共享请求封装，为me、列表、上传、详情、下载和logout显式添加Authorization，使用credentials: omit，MUST 不把凭据发送到外部URL。页面 SHALL 通过受保护API加载身份和数据；材料和文件名 MUST 保持模板转义/textContent，不将隐藏控件视为授权。系统 SHALL 保持同源部署、不开放任意来源CORS，API响应no-store；生产Bearer MUST 使用HTTPS。

#### Scenario: Navigation and download
- **WHEN** 用户登录后进入/刷新材料页并下载附件
- **THEN** 页面通过Bearer me/材料API加载数据，下载通过Bearer fetch/Blob/object URL并及时revoke，不把Token放入链接或Cookie

#### Scenario: Browser expiry and logout failure
- **WHEN** API返回401、用户退出、权限不足返回403或logout失败
- **THEN** 401/退出清理本地Token、身份和材料并回登录；403仅提示权限不足；退出无论网络结果都清理本地状态，但只有204确认服务端撤销，失败提示未确认

### Requirement: Teacher student and class scope equivalence
系统 MUST 仅允许 teacher/student，班级范围 MUST 来自核验后的服务端身份。教师 SHALL 上传本班材料；学生 SHALL 只读。所有列表、详情、下载 MUST 在数据库查询中带可信 class_id。客户端 query、form、JSON、header 和身份 Cookie MUST 不能改变权限。

#### Scenario: Student direct and forged uploads
- **WHEN** A/B 班学生直接上传或伪造 role=teacher、user_id、class_id，携有效 Bearer Token，无需 CSRF
- **THEN** 返回 403 和 {"error":"Teacher role required"}，不改变材料、正文或文件

#### Scenario: Teacher forged ownership
- **WHEN** 教师携有效 Bearer Token 上传并附加身份字段或查询参数
- **THEN** 返回 400，无材料写入；登录时伪造身份字段不能改变实际账号身份

#### Scenario: Bidirectional class isolation
- **WHEN** A 班教师/学生访问 B 班材料，或 B 班教师/学生访问 A 班材料，包括伪造身份 header/Cookie/参数
- **THEN** 列表仅本班；列表附加任何查询参数返回 400；跨班详情和下载与不存在资源相同返回 404
- **AND** 不返回外班数据、计数、路径或文件；修改材料的未支持方法仍返回 405

### Requirement: Material API and file compatibility
系统 SHALL 保留单文件 .txt/.md（忽略扩展名大小写）、UTF-8 可含 BOM、默认 20 MiB 可配置限额。空文件、NUL、无效 UTF-8、格式或输入错误 MUST 返回 400，超限 MUST 返回 413。成功 MUST 返回 201 和 material_id；文件、元数据和正文全部持久化后才能成功。正文 MUST 保留 plain_text 字段，不渲染 Markdown/HTML。

#### Scenario: Upload and same class reads
- **WHEN** A/B 教师上传合法材料后由本班教师和学生查询
- **THEN** 列表 200 且 items 按 created_at、id 倒序，包含 id、original_name、size、created_at、uploader；详情 200 还包含 plain_text，下载 200 且原字节一致
- **AND** 附件使用 text/plain、Content-Disposition attachment、X-Content-Type-Options nosniff、Cache-Control private, no-store；空班列表为 {"items":[]}

#### Scenario: Invalid files and safe names
- **WHEN** 上传缺失/多个文件、空文件、非法文本、不支持格式、超限或路径文件名
- **THEN** 非法内容按规定 400/413 且无残留；可接受路径文件名安全净化并用独立 UUID 存储，同名不覆盖，不能写出存储根

### Requirement: File transaction and recovery equivalence
Material 和 KnowledgeEntry MUST 在同一 MySQL 事务保存；文件或确定的数据库写入/提交失败 MUST 补偿，不能展示不完整材料。启动恢复 SHALL 在接收请求前验证完整性，清理临时和孤立文件；缺正文、缺文件、不安全路径 MUST 拒绝启动。恢复 MUST 仅依照已验证的当前数据库进行。

#### Scenario: Failed upload compensation
- **WHEN** 文件移动或任一表写入或事务提交明确失败
- **THEN** 返回通用 500，双表回滚且本次文件清理，不影响其他材料

#### Scenario: Interrupted process recovery
- **WHEN** 上次中断留下临时/孤立文件，或存在已引用但缺失的正文/文件
- **THEN** 前者在下次启动清理，后者阻止服务启动且保留证据等待恢复，不以不完整数据库清理有效文件

### Requirement: Initialization health and compatibility acceptance
初始化 SHALL 幂等保留已有数据，在新库提供既有 A/B 样例账号和材料，密码只存安全加盐哈希；占位表 SHALL 无新增业务。GET /health MUST 匿名返回依赖正常时 200 {"status":"ok"}，MySQL 或存储不可用时 503 {"status":"unhealthy"}。真正必需的数据库等配置缺失 MUST 拒绝启动；Go SHALL 不依赖 Flask SECRET_KEY，但 MUST 配置独立 JWT_SIGNING_SECRET，缺失、空白或不足32字节拒绝启动认证服务，禁止默认值或SECRET_KEY兜底。MySQL MUST 满足实际 CHECK、InnoDB、utf8mb4 和排序能力；Compose/测试 SHALL 锁定具体版本，不要求恰好8.4。迁移验收 MUST 保留历史测试与记录，并增加 Go/MySQL 与通过 Nginx 的现有 Token 契约验证。已被取代的 SID/Cookie/CSRF 断言 MUST 明确映射为新契约安全断言，不能要求新栈恢复旧认证，也不得删除、跳过或弱化仍适用的安全测试来宣称通过。

#### Scenario: Go runtime configuration
- **WHEN** 未设置 SECRET_KEY，但真实必需配置和数据库能力满足要求
- **THEN** Go 正常启动并完成 Token 签发、验证和撤销流程，测试记录锁定的 MySQL 版本；数据库能力不足不能标记就绪

#### Scenario: Fresh initialization and failed retry
- **WHEN** 全新初始化、重复初始化或种子写入失败后重试
- **THEN** 成功后 A/B 账号和完整样例可用；已有密码和数据不被重置，失败不能标记就绪，可安全重试

#### Scenario: Readiness and proxy compatibility
- **WHEN** 经 Nginx 请求页面、静态资源、API 和健康接口
- **THEN** URL、页面壳、Bearer表单/API交互和Authorization代理透传正常，Token不进入Cookie，客户端身份 header 无授权效力；依赖故障不假装健康，上传限额不被代理错误缩小

#### Scenario: Verified staged cutover
- **WHEN** 准备将入口从旧栈切至新栈
- **THEN** 先通过当前Token契约适用的Python测试、Go单元测试、真实MySQL repository/迁移测试及两后端安全/API对照验证（保留历史Session基线但不作旧机制兼容门槛），再切换；任一门槛失败保留旧数据且不自动切换
