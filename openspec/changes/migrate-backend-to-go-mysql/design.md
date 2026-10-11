## Context

原提案基于 Flask/SQLite 服务端 Session。现已核对 `replace-session-auth-with-token-auth` 的 proposal/design/specs/tasks 及 `app/{auth,tokens,routes,__init__}.py`、`app/static/{api,login,materials}.js`：当前 Flask 签发 HS256 JWT，API 仅接受 Bearer，角色/班级每请求查库，logout 递增 session_version；无认证 Session/CSRF 钩子。该契约取代旧 SID、Cookie、CSRF 和身份快照失配规则。材料页为公共页面壳，经 me/材料 API 加载数据。

认证规约修订时的历史快照：Go 当时已有 cmd/server、cmd/admin、cmd/migrate、MySQL 八表 schema、七表迁移/校验和初始化门槛；HTTP 当时仅 /health，尚无 JWT 配置，security 仍为占位。Phase 2 的 PBKDF2 创建、sessions schema、迁移工具及验收记录保留；9/30 是当时的历史进度，不是当前实现状态。后续实现与验收以 tasks 和 docs/migration 的分阶段记录为准。该修订仅涉及本 change，其他 change（包括检索）及其已有工作均不改动；不把检索扩展纳入本迁移范围。

## Goals / Non-Goals

**Goals:** 用一个 Go 单体和一个 MySQL 实例等价实现当前功能；迁移工具可重复验证；认证和文件恢复不依赖旧运行时；服务层仅接受可信身份与显式存储接口。

**Non-Goals:** 不改 UI 功能、不新增角色或材料修改接口；不创建 knowledge_chunks、全文检索、向量/RAG、网关、Redis、队列、微服务、分布式锁或 Session 系统；不进行长期双库运行。为未来留普通模块边界，不建立通用插件/DI 平台。

## Decisions

### 1. Go 单体和接口边界

新增 `cmd/server`、`cmd/admin`（初始化/账号配置）和 `cmd/migrate`；`internal/http` 负责路由、输入、响应与模板，`internal/auth`/`security` 负责密码、JWT 签发校验和可信身份；`internal/material` 负责上传流程，`internal/storage` 封装私有文件操作，`internal/repository/mysql` 使用参数化 SQL 和 database/sql 事务。迁移专用 SQLite 读取仅存在迁移命令的依赖图，server 二进制及最终镜像不包含 SQLite 驱动或 Python。

使用 net/http、html/template、crypto/rand、SHA-256 和成熟 MySQL driver；密码兼容使用必要的 Go crypto 依赖。只在文件/事务/时钟测试需要处引入小接口，构造函数显式传依赖。拒绝 ORM、DI 或 Web 大框架。Apply 锁定依赖与镜像版本并记录，不使用浮动 latest。

认证中间件验证 Bearer 后查询当前用户及 session_version，生成含 user_id/role/class_id 的只读请求 principal；handler 不解析 JWT，也不能从 JSON、query 或自定义身份 header 构造授权身份。材料 service/repository 接受该 scope，所有读取均显式包含 class_id。未来检索可复用该边界，不预建检索模块。

### 2. 当前八表及 MySQL 映射

全部采用 InnoDB、utf8mb4。标识/唯一键使用区分大小写且无尾部空格折叠的二进制语义；使用 utf8mb4 的 NO PAD 二进制排序规则，迁移前测试 `A/a`、尾空格及 Unicode，不能使用默认不区分大小写排序改变用户名唯一性。固定摘要/UUID 使用 ASCII 二进制比较。不自动截断源字段。

| 表 | 当前真实 SQLite 列 | MySQL 目标与约束 |
|---|---|---|
| classes | id INTEGER PK; name TEXT NOT NULL UNIQUE | id BIGINT AUTO_INCREMENT PK; name VARCHAR(255) NOT NULL UNIQUE |
| users | id INTEGER PK; username TEXT UNIQUE NOT NULL; password_hash TEXT NOT NULL; role TEXT CHECK teacher/student; class_id INTEGER NOT NULL FK; session_version INTEGER NOT NULL DEFAULT 0 | id BIGINT AUTO_INCREMENT; username VARCHAR(255) 唯一; password_hash TEXT; role VARCHAR(7) CHECK 两种值; class_id BIGINT FK RESTRICT; session_version BIGINT DEFAULT 0；必填语义不变 |
| sessions | sid_hash TEXT PK NOT NULL; data TEXT NOT NULL; expires_at INTEGER NOT NULL | sid_hash CHAR(64) ASCII PK; data LONGTEXT NOT NULL（保留既有 schema，旧记录不导入；Token 认证不使用）; expires_at BIGINT NOT NULL，Unix 秒；到期索引 |
| materials | id TEXT PK NOT NULL; class_id/uploader_id INTEGER FK RESTRICT; original_name TEXT; size INTEGER CHECK >0; content_type TEXT; storage_key TEXT UNIQUE; created_at TEXT 默认 UTC 毫秒；除注明外均 NOT NULL | id/storage_key CHAR(36) ASCII; class_id/uploader_id BIGINT FK; original_name LONGTEXT; size BIGINT CHECK >0; content_type TEXT; created_at VARCHAR(32) 二进制排序；所有 NOT NULL、storage_key UNIQUE、(id,class_id) UNIQUE 与 (class_id,created_at,id) 索引保留 |
| knowledge_entries | id INTEGER PK; material_id TEXT UNIQUE NOT NULL; class_id INTEGER FK NOT NULL; plain_text TEXT NOT NULL; created_at TEXT NOT NULL 默认 UTC 毫秒；(material_id,class_id) 复合外键 | id BIGINT AUTO_INCREMENT; material_id CHAR(36) ASCII UNIQUE; class_id BIGINT FK; plain_text LONGTEXT; created_at VARCHAR(32)；同班复合 FK RESTRICT 保留 |
| assignments | id INTEGER PK; name TEXT NOT NULL | id BIGINT AUTO_INCREMENT; name LONGTEXT NOT NULL |
| assistants | id INTEGER PK; name TEXT NOT NULL | 同上 |
| skills | id INTEGER PK; name TEXT NOT NULL | 同上 |

整数使用 signed BIGINT 对应 SQLite 64 位整数；显式导入 id 后校验下一自动生成值不冲突。没有现存数据库布尔列；迁移数字解析不能把 ID 变浮点数/截断。sessions.data 不作为新认证载荷。created_at 保留原 ISO UTC 字符串及排序，新增写入由 repository 统一产生 UTC 毫秒格式，不依赖数据库会话时区或 SQLite strftime 默认值。保留 plain_text，不提前重命名 body_text。

源 TEXT 无长度上限，MySQL 索引列需明确界限：预检用户名/班名长度、UUID、时间格式及密码哈希格式。超界/未知格式停止迁移并报告字段位置（不泄露值），不得截断或猜测。如果真实数据要求更宽 schema，Apply 先报告阻塞再修订方案。本轮未读取真实用户数据库，真实数据质量须迁移前预检。

MySQL 必须满足实际使用的 CHECK、InnoDB、utf8mb4 和排序语义，不要求恰好为8.4。Compose/测试当前选8.4系列，Apply 锁定具体补丁版本或镜像摘要以便复现；其他版本须通过相同能力测试。schema 有序版本化，DDL 与数据导入分离，不依赖 DDL 回滚。库连接字符集、严格 SQL mode、包大小应能承载20 MiB正文及协议开销。

### 3. 数据迁移与认证切换

迁移命令仅离线运行，读取停写后的 SQLite 和文件快照，不改源。预检八表结构、FK、角色、班级、材料和容量；完整导入并逐行验证七张业务表，包含 password_hash、role、class_id、session_version 原值。报告包含数量、ID、关系、字段/正文/文件 SHA-256；旧 sessions 的 data、sid_hash、expires_at 不导入，仅留在源备份。明确记录此排除，不要求 sessions 源目标记录数相等。

空目标建立八表，按 classes → users → materials → knowledge_entries、三占位表顺序事务导入（FK有效）。写入/验证在维护窗口进行，报告关联源快照。七表使用确定性类型/长度编码 hash 和逐 ID 内容比较。sessions 初始为空，不导入任何旧匿名、有效或过期会话；重跑不能清空目标已有 sessions 记录，目标开始业务写入后不得重导旧快照。

安全重跑策略：空目标可导入；完整相同目标只验证并成功返回；非空不同目标拒绝写入，不 UPSERT 覆盖。事务失败空库可重试；若提交结果不确定，重新连接核对完整指纹后判断已完成/空库/冲突，不盲目重导。只有显式确认的临时目标可人工重建，不自动 DROP 或删除源 SQLite。导入验证成功后生成离线报告，启动以目标 schema/报告及快照关联为迁移门槛；新安装走独立 fresh 初始化模式，不能把迁移模式误当空库种子。

正式 cutover 后重新登录，旧 Cookie 不授权。保留七表原值导入（包括 session_version）及 sessions 初始为空的 Phase 2 契约；目标使用与旧入口不同的新 JWT_SIGNING_SECRET，使切换前 Token 不能继续授权，不通过重置账号版本改变已验证的导入数据。该密钥在新栈正常重启时保持稳定；同密钥、未过期、未撤销 Token 在重启后仍有效。密钥轮换要求重新登录，不引入多密钥机制。已有 sessions 记录不删除，运行时不读写或清理 sessions 来认证。

### 4. 认证、安全和 HTTP 兼容

直接兼容 Werkzeug `pbkdf2:sha256:1000000$salt$hex`：salt 使用存储字符串的 UTF-8 字节、相同迭代次数和常量时间比较。复用 Phase 2 已有账号哈希创建能力，新账号保持同格式与随机盐；未知算法预检阻止迁移，不依赖 Python 验证密码。Go 不需要 Flask SECRET_KEY，但必须从环境读取独立 JWT_SIGNING_SECRET，缺失、空白或不足 32 字节拒绝启动认证服务，不使用默认值或 SECRET_KEY 兜底。

使用成熟 Go JWT 库，固定 HS256，拒绝 none/其他算法。claims 必须包含 sub（正整数用户 ID 的十进制字符串，signed BIGINT 范围）、ver（非负整数且小于 9223372036854775807）、iat/exp（整数 Unix 秒）；拒绝布尔/浮点替代整数，iat 非负且不在未来，exp = iat + 28800，当前时间必须小于 exp。普通访问不续期，不签入 role/class_id；即使 Token 携带这些字段也不用于授权。Authorization scheme 大小写不敏感，必须恰有一个非空 Bearer 凭据，拒绝重复/歧义 Header，不从 Cookie、query 或 form 取 Token。

每次受保护请求按 sub 查询当前用户及有效班级，对照 ver/session_version。不存在用户、非法角色、无有效班级或版本不匹配返回 401；合法角色/班级变化立即使用数据库新值，不因旧快照失配拒绝。模型没有 enabled 字段，不新增账号状态列。只读 principal 经 request context 传递，业务不读取 Session。用户查询故障返回通用 500，不能误报 401 或回退 Cookie。

logout 在 MySQL 事务中原子递增 users.session_version，确认提交后返回 204；全部设备旧版本 Token 在后续请求中返回 401，重新登录签发新版本。失败返回通用 500，不伪报撤销成功；不删除 sessions 或创建每 Token 记录。已经通过认证的在途请求不承诺回滚。重新登录不隐式撤销其他设备 Token，前端开始登录时仅清理本地旧身份。

| 路径 | 保留契约 |
|---|---|
| GET /login; POST /api/auth/login | 页面不创建匿名 Session；无需 Cookie/CSRF，正确凭据200，返回 access_token、token_type=Bearer、expires_in=28800、redirect=/materials；错误凭据统一401 |
| GET /api/auth/me | Bearer 认证后200，返回当前 user_id/role/class_id/class_name |
| POST /api/auth/logout | Bearer 认证；提交版本撤销后204，全设备失效 |
| GET /materials | 200公共页面壳，不嵌入身份或材料；前端缺 Token 或 API 401 时跳转 /login |
| GET /api/materials | Bearer 认证；匿名401；任意 query400；items 原字段，created_at/id 倒序 |
| POST /api/materials | Bearer认证→teacher→输入校验；学生始终角色403；教师身份字段/额外文件字段/多文件400；无需CSRF；成功201 material_id |
| GET /api/materials/{id} | Bearer 认证；同班200 id/original_name/size/created_at/uploader/plain_text；跨班和不存在相同404 |
| GET /api/materials/{id}/file | Bearer 认证；同班原始字节附件，原 MIME/安全 header；跨班404 |
| GET /health | 匿名独立检查依赖，正常200 status=ok，否则503 status=unhealthy；不读取认证状态、不创建Cookie |

沿用现有 Token 前端：同源 sessionStorage 保存 Token，共享 API 封装仅向同源 API 添加 Authorization，credentials: omit；me、列表、上传、详情、下载、logout 均使用该封装。下载通过 Bearer fetch/Blob/object URL 并及时 revoke，Token 不进入 URL、链接或 Cookie。401/退出清理本地 Token、身份和材料；403 只提示权限不足；退出无论网络结果均清理本地状态，只有204确认撤销，失败提示服务端退出未确认。

移除 login/logout/upload 的认证 CSRF token 要求，不创建匿名 CSRF Session；保持同源部署，不开放任意来源 CORS。登录及 API 响应 Cache-Control: no-store；签名密钥、密码、密码哈希和完整 Token 不写入日志，Token 仅经约定登录响应返回。生产使用 HTTPS。保留模板转义和 textContent，sessionStorage 可被同源脚本读取，不能以 Token 替代 XSS 防护。模板仅作 Go 语法适配，沿用当前页面壳、JS、CSS 和 URL；静态资源经 Nginx 代理 Go。错误 JSON 保持现有明确字符串，认证401携 WWW-Authenticate: Bearer；框架400/413/405不要求 HTML 字节一致。

### 5. 私有文件和事务恢复

沿用 uploads/tmp 与 uploads/files/{UUID}，原文件不转码；plain_text 严格 UTF-8 去 BOM、拒绝 NUL，Markdown 不渲染。文件流计数限制默认 20971520 字节，不信 Content-Length。扩展名与路径净化行为以当前 prepare_upload/persist_prepared 测例对照，兼容 secure_filename 的 Unicode 归一化/安全字符处理和空结果 material.txt；必要时仅引入小型 Unicode 包。存储名永远为服务端 UUID，与用户文件名解耦。

流程：Bearer 认证及教师权限 → 临时文件/解析 → MySQL 事务双表 INSERT → 同文件系统 rename → COMMIT → 201。明确失败回滚双表并删除本次文件，异常中断依靠启动恢复。MySQL 提交连接断开可能结果未知：不能直接删除可能已提交的文件；暂停该写入成功响应，重新连接核查 UUID 的完整双表状态，已提交且文件完整才认定成功，确认未提交才清理。如果数据库持续不可用，返回 500 并保留待恢复文件，不允许展示不完整记录，重启依可信库核对。这是处理远程数据库故障的必要细节，不宣称数据库和文件有分布式原子事务。

恢复在唯一实例监听前执行：验证目录非符号链接、键为规范 UUID、每条材料具有同班正文和普通原文件；不完整即失败且不清理有效资料。通过完整性检查后才清理 tmp 和未引用文件。严禁用旧 SQLite 或尚未验证的空 MySQL 扫描正式文件卷。迁移与演练使用卷副本；最终绑定已验证副本。恢复不与活跃上传并行，不新增锁服务。

### 6. Nginx、Compose 与初始化

最终三服务 nginx、app（Go）、mysql。保持宿主 8080 入口，由 Nginx 发布，app/mysql 仅内部网络；Nginx 不挂上传卷。所有应用路径直接代理 Go，保留 Host 并透传 Authorization；代理覆盖 X-Forwarded-Proto 等转发信息，Go 不根据客户端身份 header 授权。不配置认证 Cookie 或 COOKIE_SECURE，不以 Cookie/Set-Cookie 属性作为验收；生产 Bearer 通过 HTTPS 传输。

上述8080是原规划的正式入口目标，不代表本 change 的隔离验收已切换该入口。当前默认Compose使用127.0.0.1:8081（DEV_HTTP_PORT可配置），另有两个一次性准备服务；正式HTTPS与入口切换仍须单独部署验证，不替换现有Flask RAG Demo。

Nginx 上传 body 限制设为不额外限制，由 Go 的流式文件限额和 multipart 控制管理，以免 20 MiB 文件加边界后被代理拒绝。避免代理重试非幂等上传；超时足以覆盖既有上传和密码验证，测试通过真实代理检查。

MySQL 用独立持久卷；material-data 保留文件持久化且旧 SQLite 不被删除，最终 app 仅挂文件子目录/独立复制卷。环境模板提供 DB 主机/库/用户/密码、MYSQL 初始化密码、JWT_SIGNING_SECRET、UPLOAD_ROOT、MAX_UPLOAD_BYTES、四样例账号密码的占位配置，不在镜像构建注入真实值。Go 镜像只含二进制、模板、静态资源和必要运行文件。

启动顺序：MySQL 健康 → 显式 schema/迁移或 fresh 初始化命令完成 → Go 检查 JWT 配置、schema、数据门槛和文件恢复（不清理 Session） → 开始监听/健康 → Nginx 可用。全新一键 Compose 路径可在 Go 启动前运行 fresh 初始化，但必须显式 fresh 模式；已有数据使用 migrate 模式且未验收不得自动播种。初始化事务提交数据并在失败时清理本次文件，使用初始化完成标记防止半成品就绪，不重置已有账号。提供 Go 账号/班级配置命令替代旧脚本职责。

### 7. 验证方法

保留 Phase 0/1/2 历史基线和验收记录，不将 Flask test_client、session_transaction、SQLite trigger 测试视为 Go 验收。新增独立 HTTP 契约套件，对启用现有 Token 契约的临时 Flask 与 Go/MySQL 执行同一认证和材料矩阵，登录读取 access_token 后显式发送 Bearer；Session/Cookie/CSRF 的已被替代断言仅留作历史证据，不要求新实现通过旧认证机制测试，也不删除历史测试以伪造通过。

Go unit 覆盖 PBKDF2、JWT/claims/固定期限、可信 principal、上传和文件恢复；真实 MySQL repository 覆盖约束、事务、排序及 session_version 撤销故障。认证矩阵包含非法 Header/签名/claims/过期/用户/版本401、旧 Cookie 拒绝、最新角色班级生效、查询或撤销故障500、双设备撤销、重启不复活旧 Token、空 sessions 不影响认证。迁移集成继续覆盖八表结构、七表完整数据及 hash、sessions 初始为空、安全重跑/冲突；切换演练验证新签名密钥拒绝切换前 Token 并允许重新登录。经真实 Nginx 验证 Authorization、页面壳/me、Blob 下载、401/403、20 MiB边界及重启持久化。全部使用隔离临时库/卷和假凭据，不操作现有用户数据库。

## Risks / Trade-offs

- 数据切换窗口需要停写 → 本项目选择离线一次性迁移和完整验证，避免双写复杂度。
- 跨语言密码、JWT/claims、模板与文件名细节偏差 → 旧值测试向量、cutover 重新登录和 HTTP 差异测试阻止带回归切换。
- MySQL 排序/字段容量与 SQLite 不同 → 明确二进制比较、类型预检、完整内容校验，不静默归一化或截断。
- 远程提交结果不确定 → 依据材料 UUID 重查并保留文件证据，启动恢复前不清理。
- 仓库当前文件均未跟踪，不能依赖 git diff 证明完整基线 → Phase 0 记录文件清单/hash和测试结果，后续由操作者正常版本管理，不自动提交或删除文件。

## Migration Plan

0. Baseline：运行旧全量测试，保存 API/安全矩阵、schema 和可复现文件清单。
1. Infrastructure：并行建立 Go/MySQL/Nginx 隔离开发栈，旧服务继续保留。
2. Schema & Migration：实现八表 schema、迁移/验证命令，以副本反复演练，不接正式入口。
3. Authentication & Bearer Token：实现并对照 PBKDF2、JWT 签发验证、当前身份、固定期限、版本撤销和故障拒绝。
4. Material APIs：迁移页面、材料、文件、事务与恢复，验证 A/B 双向隔离。
5. Compatibility & Cutover：所有测试通过后停旧服务（包括账号版本与业务写入）；备份 SQLite 和文件；迁移七表数据至空目标并验证空 sessions，配置新栈独立签名密钥并通知用户重新登录；启动 Go 使用已验证文件副本；先做受控冒烟再将 Nginx 入口切到 Go。旧进程不得与 Go 同时写同一文件卷。
6. Final Verification：验证入口、重启、完整数据和全部安全合约，记录验收；最终 Compose 不启动 Flask、不挂业务 SQLite、不依赖 Python。

切换前或尚未产生新写入时失败：停止 Go，保留失败目标报告，用配对 SQLite/文件快照恢复旧入口；若已正式 cutover，恢复副本须采用新的签名密钥并保持 Bearer-only，不恢复旧 Cookie 授权或已撤销 Token，原备份不改动。Go 已产生新材料、账号变更或版本撤销后，禁止直接退回旧快照丢数据或恢复已撤销授权：停写保全当前 MySQL/文件，优先修复新栈；若必须回旧栈，需另行显式规划反向同步并重新校验安全状态。本 change 不实现自动反向双写。备份不自动删除，不执行部署/真实迁移于 Propose。

## Open Questions

没有需要用户选择的架构问题。实际源数据容量/格式仅在 Apply 的只读预检确认；不符合上述可验证映射时应报告阻塞，不擅改安全或丢弃记录。
