## Purpose

定义 CampusClaw 从 Flask 和 SQLite 迁移到 Nginx、Go 与 MySQL 的数据和行为验收契约，保证第三课认证、服务端会话、班级权限及材料完整性在整体技术栈替换后继续成立，并提供可验证的迁移和切换边界。

## ADDED Requirements

### Requirement: Unified target runtime
最终课程运行 MUST 使用 Browser → Nginx → Go → MySQL；Go SHALL 是唯一业务后端，所有现有关系数据 SHALL 保存于 MySQL，文件 SHALL 使用私有持久存储。运行 MUST 不依赖 Flask、Python 或 SQLite，不新增第四课功能或修改检索 change。

#### Scenario: Start without legacy runtime
- **WHEN** 完成配置和初始化后执行 docker compose up --build，且无 Flask 进程和 SQLite 运行挂载
- **THEN** 经 Nginx 可完成登录和材料流程，MySQL 及 Go 无宿主公开端口，上传文件无静态直达路径

#### Scenario: Durable restart
- **WHEN** 上传后不删除卷地重建并重启 Compose
- **THEN** 用户、班级、Session、材料、正文和文件保留，有效未撤销 Session 继续有效

### Requirement: Complete verified data migration
迁移 MUST 保留 classes、users、materials、knowledge_entries、assignments、assistants、skills 七表全部数据及约束语义，包括原 ID、密码哈希、角色、班级、session_version、上传者、时间、文件关联和 plain_text。八表结构 MUST 保留，但目标 sessions SHALL 初始为空，旧会话仅留在源备份；cutover 后 MUST 重新登录，不兼容旧 SID。迁移 SHALL 在停止写入的快照上执行，并逐表验证数量、ID、关键关系和内容完整性。验证失败 MUST 阻止切换；旧数据库和文件备份 MUST 不被自动删除。重复执行 MUST 不覆盖不同的目标数据。

#### Scenario: Preserve complete source
- **WHEN** 迁移包含八表数据、中文正文、有效和过期 Session 的 SQLite 快照
- **THEN** 目标七表数量和 ID 集合一致，文本与文件 hash 一致，外键、班级关系和用户 session_version 一致；八表结构完整，sessions 初始为空
- **AND** 旧 Cookie 访问受保护 API 返回401、页面跳转登录；正确凭据重新登录建立安全的新 MySQL Session，正常重启保留有效新会话

#### Scenario: Invalid or repeated migration
- **WHEN** 发现缺文件、正文损坏、非法关系、目标冲突或校验不一致，或对已迁移目标重跑
- **THEN** 不报告虚假成功，不自动切换或删除源数据；完全相同目标可重新验证，非相同目标拒绝覆盖并允许修复后显式重试

### Requirement: Authentication and CSRF compatibility
系统 SHALL 保留 GET /login、POST /api/auth/login、POST /api/auth/logout、GET /materials 及现有材料 API。密码 MUST 兼容已存加盐哈希；未知账号和错误密码 SHALL 返回相同 401。CSRF MUST 使用服务端会话 token，支持既有 header/form/JSON 传递方式和优先级。秘密 MUST 不进入响应、浏览器资源或日志。

#### Scenario: Login and anonymous access
- **WHEN** teacher/student 携有效 CSRF 和正确密码登录
- **THEN** 返回 200 和 {"redirect":"/materials"}；错误密码返回统一 401 并清除旧认证身份
- **AND** 无效或缺失 Session 访问材料 API 返回 401，访问 /materials 返回 302 至 /login

#### Scenario: CSRF rejection
- **WHEN** 登录缺失或错误 CSRF，或已认证教师上传/用户退出缺失或错误 CSRF
- **THEN** 返回 403，无业务副作用；上传验证顺序仍为认证、教师角色、CSRF、文件校验

### Requirement: Persistent opaque session lifecycle
Session MUST 保存在 MySQL，仅以 SHA-256 SID 摘要为持久化键；浏览器仅保存至少 256 bit 安全随机 SID。成功登录 MUST 轮换 SID 和 CSRF，清除旧字段。期限 SHALL 固定为登录起 8 小时，普通访问不得延长。Cookie SHALL 保留 session 名、Path=/、HttpOnly、SameSite=Lax、Expires 和显式 Secure 配置。每次受保护请求 MUST 对照用户记录核验 user_id、role、class_id、session_version。

#### Scenario: Rotation and fixed expiry
- **WHEN** 从匿名或已有登录会话重新成功登录，再持续访问至登录满 8 小时
- **THEN** 登录产生新 SID/CSRF，旧 SID 不能授权，普通请求不续期，到期访问 API 返回 401
- **AND** Cookie 不包含身份或 CSRF 载荷，MySQL 不存原始 SID

#### Scenario: Trusted identity and tampering
- **WHEN** Cookie 未知/篡改/记录已删除，或用户不存在、角色/班级/版本与 Session 不一致
- **THEN** 受保护 API 返回 401，页面跳转登录，不采用客户端身份或恢复旧授权

#### Scenario: Logout all devices and stale save
- **WHEN** 用户携有效 CSRF 退出后重放任一设备旧 SID 或保存旧请求会话
- **THEN** 退出返回 204 并清除 Cookie；所有旧会话失效，旧保存不能重新创建已删除或过期会话

#### Scenario: Cleanup and storage failure
- **WHEN** 服务启动，或 Session 读取/登录写入/退出事务发生存储故障
- **THEN** 启动仅清理过期 Session；故障返回通用 500，不授予权限、不发布成功登录 Cookie、不伪报退出成功、不回退客户端会话

### Requirement: Teacher student and class scope equivalence
系统 MUST 仅允许 teacher/student，班级范围 MUST 来自核验后的服务端身份。教师 SHALL 上传本班材料；学生 SHALL 只读。所有列表、详情、下载 MUST 在数据库查询中带可信 class_id。客户端 query、form、JSON、header 和身份 Cookie MUST 不能改变权限。

#### Scenario: Student direct and forged uploads
- **WHEN** A/B 班学生直接上传或伪造 role=teacher、user_id、class_id，包括有效或缺失 CSRF
- **THEN** 返回 403 和 {"error":"Teacher role required"}，不改变材料、正文或文件

#### Scenario: Teacher forged ownership
- **WHEN** 教师携有效 CSRF 上传并附加身份字段或查询参数
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
初始化 SHALL 幂等保留已有数据，在新库提供既有 A/B 样例账号和材料，密码只存安全加盐哈希；占位表 SHALL 无新增业务。GET /health MUST 匿名返回依赖正常时 200 {"status":"ok"}，MySQL 或存储不可用时 503 {"status":"unhealthy"}。真正必需的数据库等配置缺失 MUST 拒绝启动；Go SHALL 不依赖无实际用途的 SECRET_KEY。MySQL MUST 满足实际 CHECK、InnoDB、utf8mb4 和排序能力；Compose/测试 SHALL 锁定具体版本，不要求恰好8.4。迁移验收 MUST 保留旧测试并增加 Go/MySQL 与通过 Nginx 的等价验证，不删除、跳过或弱化安全测试。

#### Scenario: Go runtime configuration
- **WHEN** 未设置 SECRET_KEY，但真实必需配置和数据库能力满足要求
- **THEN** Go 正常启动并完成认证和 Session 流程，测试记录锁定的 MySQL 版本；数据库能力不足不能标记就绪

#### Scenario: Fresh initialization and failed retry
- **WHEN** 全新初始化、重复初始化或种子写入失败后重试
- **THEN** 成功后 A/B 账号和完整样例可用；已有密码和数据不被重置，失败不能标记就绪，可安全重试

#### Scenario: Readiness and proxy compatibility
- **WHEN** 经 Nginx 请求页面、静态资源、API 和健康接口
- **THEN** URL、表单交互和 Cookie 正常，客户端身份 header 无授权效力；依赖故障不假装健康，上传限额不被代理错误缩小

#### Scenario: Verified staged cutover
- **WHEN** 准备将入口从旧栈切至新栈
- **THEN** 先通过原测试、Go 单元测试、真实 MySQL repository/迁移测试及安全/API 对照验证，再切换；任一门槛失败保留旧数据且不自动切换
