## Context

依据实际 `app/{auth,sessions,csrf,routes,materials,db,recovery,health,accounts,__init__}.py`、两个 SQL schema、`scripts/{init_db,serve,create_account,verify_compose}.py`、现有 tests 和第三课 artifacts。现有 Compose 仅一个 Flask/Gunicorn app、8080 公布端口和 material-data 卷；没有 Go、Nginx 或 MySQL 实现。原认证采用 PBKDF2-SHA256（1,000,000 次），会话为 SQLite 中 sid_hash/data/expires_at。最近已记录 36 项测试通过，但本轮 Propose 不重新宣称执行了测试，Apply 首先重新跑 baseline。

主规约目录为空。历史 `move-auth-to-server-side-session` 的 SQLite 限定和材料 change 的 Flask 运行设计由本次明确授权的技术迁移替代，非业务契约冲突。`add-traceable-vector-retrieval` 仍描述另一未实施方案，本轮保持原样，迁移验收后由用户单独更新。

## Goals / Non-Goals

**Goals:** 用一个 Go 单体和一个 MySQL 实例等价实现当前功能；迁移工具可重复验证；认证和文件恢复不依赖旧运行时；服务层仅接受可信身份与显式存储接口。

**Non-Goals:** 不改 UI 功能、不新增角色或材料修改接口；不创建 knowledge_chunks、全文检索、向量/RAG、网关、Redis、队列、微服务、分布式锁或 Session 系统；不进行长期双库运行。为未来留普通模块边界，不建立通用插件/DI 平台。

## Decisions

### 1. Go 单体和接口边界

新增 `cmd/server`、`cmd/admin`（初始化/账号配置）和 `cmd/migrate`；`internal/http` 负责路由、输入、响应与模板，`internal/auth`/`session`/`security` 负责密码、可信身份、会话、CSRF；`internal/material` 负责上传流程，`internal/storage` 封装私有文件操作，`internal/repository/mysql` 使用参数化 SQL 和 database/sql 事务。迁移专用 SQLite 读取仅存在迁移命令的依赖图，server 二进制及最终镜像不包含 SQLite 驱动或 Python。

使用 net/http、html/template、crypto/rand、SHA-256 和成熟 MySQL driver；密码兼容使用必要的 Go crypto 依赖。只在文件/事务/时钟测试需要处引入小接口，构造函数显式传依赖。拒绝 ORM、DI 或 Web 大框架。Apply 锁定依赖与镜像版本并记录，不使用浮动 latest。

认证中间件生成含 user_id/role/class_id/session_version 的只读请求身份；handler 无法从 JSON、query 或 header 构造授权身份。材料 service/repository 接受该 scope，所有读取均显式包含 class_id。未来检索可复用该边界，不预建检索模块。

### 2. 当前八表及 MySQL 映射

全部采用 InnoDB、utf8mb4。标识/唯一键使用区分大小写且无尾部空格折叠的二进制语义；使用 utf8mb4 的 NO PAD 二进制排序规则，迁移前测试 `A/a`、尾空格及 Unicode，不能使用默认不区分大小写排序改变用户名唯一性。固定摘要/UUID 使用 ASCII 二进制比较。不自动截断源字段。

| 表 | 当前真实 SQLite 列 | MySQL 目标与约束 |
|---|---|---|
| classes | id INTEGER PK; name TEXT NOT NULL UNIQUE | id BIGINT AUTO_INCREMENT PK; name VARCHAR(255) NOT NULL UNIQUE |
| users | id INTEGER PK; username TEXT UNIQUE NOT NULL; password_hash TEXT NOT NULL; role TEXT CHECK teacher/student; class_id INTEGER NOT NULL FK; session_version INTEGER NOT NULL DEFAULT 0 | id BIGINT AUTO_INCREMENT; username VARCHAR(255) 唯一; password_hash TEXT; role VARCHAR(7) CHECK 两种值; class_id BIGINT FK RESTRICT; session_version BIGINT DEFAULT 0；必填语义不变 |
| sessions | sid_hash TEXT PK NOT NULL; data TEXT NOT NULL; expires_at INTEGER NOT NULL | sid_hash CHAR(64) ASCII PK; data LONGTEXT NOT NULL（保存新 Go 会话 JSON，旧记录不导入）; expires_at BIGINT NOT NULL，Unix 秒；到期索引 |
| materials | id TEXT PK NOT NULL; class_id/uploader_id INTEGER FK RESTRICT; original_name TEXT; size INTEGER CHECK >0; content_type TEXT; storage_key TEXT UNIQUE; created_at TEXT 默认 UTC 毫秒；除注明外均 NOT NULL | id/storage_key CHAR(36) ASCII; class_id/uploader_id BIGINT FK; original_name LONGTEXT; size BIGINT CHECK >0; content_type TEXT; created_at VARCHAR(32) 二进制排序；所有 NOT NULL、storage_key UNIQUE、(id,class_id) UNIQUE 与 (class_id,created_at,id) 索引保留 |
| knowledge_entries | id INTEGER PK; material_id TEXT UNIQUE NOT NULL; class_id INTEGER FK NOT NULL; plain_text TEXT NOT NULL; created_at TEXT NOT NULL 默认 UTC 毫秒；(material_id,class_id) 复合外键 | id BIGINT AUTO_INCREMENT; material_id CHAR(36) ASCII UNIQUE; class_id BIGINT FK; plain_text LONGTEXT; created_at VARCHAR(32)；同班复合 FK RESTRICT 保留 |
| assignments | id INTEGER PK; name TEXT NOT NULL | id BIGINT AUTO_INCREMENT; name LONGTEXT NOT NULL |
| assistants | id INTEGER PK; name TEXT NOT NULL | 同上 |
| skills | id INTEGER PK; name TEXT NOT NULL | 同上 |

整数使用 signed BIGINT 对应 SQLite 64 位整数；显式导入 id 后校验下一自动生成值不冲突。没有现存数据库布尔列；sessions.data 中 _permanent 保持 JSON boolean，数字解析不能把 ID 变浮点数/截断。created_at 保留原 ISO UTC 字符串及排序，新增写入由 repository 统一产生 UTC 毫秒格式，不依赖数据库会话时区或 SQLite strftime 默认值。保留 plain_text，不提前重命名 body_text。

源 TEXT 无长度上限，MySQL 索引列需明确界限：预检用户名/班名长度、UUID、时间格式及密码哈希格式。超界/未知格式停止迁移并报告字段位置（不泄露值），不得截断或猜测。如果真实数据要求更宽 schema，Apply 先报告阻塞再修订方案。本轮未读取真实用户数据库，真实数据质量须迁移前预检。

MySQL 必须满足实际使用的 CHECK、InnoDB、utf8mb4 和排序语义，不要求恰好为8.4。Compose/测试当前选8.4系列，Apply 锁定具体补丁版本或镜像摘要以便复现；其他版本须通过相同能力测试。schema 有序版本化，DDL 与数据导入分离，不依赖 DDL 回滚。库连接字符集、严格 SQL mode、包大小应能承载20 MiB正文及协议开销。

### 3. 数据与 Session 迁移

迁移命令仅离线运行，读取停写后的 SQLite 和文件快照，不改源。预检八表结构、FK、角色、班级、材料和容量；完整导入并逐行验证七张业务表，包含 password_hash、role、class_id、session_version 原值。报告包含数量、ID、关系、字段/正文/文件 SHA-256；旧 sessions 的 data、sid_hash、expires_at 不导入，仅留在源备份。明确记录此排除，不要求 sessions 源目标记录数相等。

空目标建立八表，按 classes → users → materials → knowledge_entries、三占位表顺序事务导入（FK有效）。写入/验证在维护窗口进行，报告关联源快照。七表使用确定性类型/长度编码 hash 和逐 ID 内容比较。sessions 初始为空，不导入任何旧匿名、有效或过期会话；重跑不能清空已投入使用的 Go sessions，目标开始业务写入后不得重导旧快照。

安全重跑策略：空目标可导入；完整相同目标只验证并成功返回；非空不同目标拒绝写入，不 UPSERT 覆盖。事务失败空库可重试；若提交结果不确定，重新连接核对完整指纹后判断已完成/空库/冲突，不盲目重导。只有显式确认的临时目标可人工重建，不自动 DROP 或删除源 SQLite。导入验证成功后生成离线报告，启动以目标 schema/报告及快照关联为迁移门槛；新安装走独立 fresh 初始化模式，不能把迁移模式误当空库种子。

正式 cutover 后重新登录，旧 Cookie 访问受保护 API 返回401、页面跳转登录，不实现旧 SID 兼容。新会话仍使用32随机字节 URL-safe SID 和 SHA-256 摘要，Cookie 名/Path保留以覆盖旧值。第三课 server-side-session/spec.md 的 Sessions are persisted on the server 场景仅要求“同一数据库的新应用实例”读取有效会话，并无跨数据库迁移无感登录要求。Go 正常重启仍保留有效会话，仅清理过期记录，不能每次启动清空 sessions。

### 4. 认证、安全和 HTTP 兼容

直接兼容当前 Werkzeug `pbkdf2:sha256:1000000$salt$hex` 存储格式；使用字符串 salt 的原 UTF-8 字节、相同迭代次数和常量时间比较。新账号保持此格式与随机盐，便于对照。未知算法不猜测，预检阻止迁移；不依赖 Python 验证密码。Go 不要求 SECRET_KEY：SID 和 CSRF 由 crypto/rand 生成，MySQL 按 SHA-256 sid_hash 查找，密码采用加盐 PBKDF2，没有使用该共享密钥的真实用途。最终环境模板和启动校验移除此项；旧 Flask 对照环境和原测试仍可使用它，不将 Flask 内部配置断言移植到 Go。

GET /login 创建持久化匿名 CSRF Session。登录 CSRF 成功后验证凭据；错误凭据清除原认证状态并统一 401。成功只保存六字段 user_id、role、class_id、session_version、_permanent、csrf_token；事务删除旧 SID 摘要并插入新摘要，提交后才发 Cookie/200。Expires=登录时间+28800 秒，普通请求不延长、不强制写回。更新已存在会话仅 UPDATE 未过期行，不 UPSERT；零行不复活旧 SID。不新增 revision 或复杂并发协议。

退出在一个 MySQL 事务中增加用户 session_version 并删除当前摘要，提交成功才返回 204/清 Cookie；其他设备在后续认证核验版本时失败。每次请求核验用户存在、角色、班级、版本；会话读取或必要写入故障返回既有通用 500。/health 独立检查依赖，不读取 Session，不创建 Cookie。

| 路径 | 保留契约 |
|---|---|
| GET /login; POST /api/auth/login | 页面 CSRF；header 优先于 form/JSON token；错误 token403，正确凭据200 redirect=/materials，错误凭据401 |
| POST /api/auth/logout | 认证→CSRF；204，全设备失效 |
| GET /materials | 匿名302 /login；本班页面，教师才有上传控件 |
| GET /api/materials | 匿名401；任意 query400；items 原字段，created_at/id 倒序 |
| POST /api/materials | 认证→teacher→CSRF→输入校验；学生始终角色403；教师身份字段/额外文件字段/多文件400；成功201 material_id |
| GET /api/materials/{id} | 同班200 id/original_name/size/created_at/uploader/plain_text；跨班和不存在相同404 |
| GET /api/materials/{id}/file | 同班原始字节附件，原 MIME/安全 header；跨班404 |
| GET /health | 依赖正常200 status=ok，否则503 status=unhealthy |

现有模板的 DOM、JS、CSS 和 URL 保留，仅把 Jinja 控制语法转换为 html/template；自动转义和 `.textContent = material.plain_text` 的安全行为保留。静态资源由 Go 提供经 Nginx 代理，减少双份路径配置。错误 JSON 保持当前明确字符串；框架默认 400/413/405 页面不追求品牌化 HTML 字节一致，但状态、媒体语义及已验收断言必须保持。

### 5. 私有文件和事务恢复

沿用 uploads/tmp 与 uploads/files/{UUID}，原文件不转码；plain_text 严格 UTF-8 去 BOM、拒绝 NUL，Markdown 不渲染。文件流计数限制默认 20971520 字节，不信 Content-Length。扩展名与路径净化行为以当前 prepare_upload/persist_prepared 测例对照，兼容 secure_filename 的 Unicode 归一化/安全字符处理和空结果 material.txt；必要时仅引入小型 Unicode 包。存储名永远为服务端 UUID，与用户文件名解耦。

流程：鉴权及 CSRF → 临时文件/解析 → MySQL 事务双表 INSERT → 同文件系统 rename → COMMIT → 201。明确失败回滚双表并删除本次文件，异常中断依靠启动恢复。MySQL 提交连接断开可能结果未知：不能直接删除可能已提交的文件；暂停该写入成功响应，重新连接核查 UUID 的完整双表状态，已提交且文件完整才认定成功，确认未提交才清理。如果数据库持续不可用，返回 500 并保留待恢复文件，不允许展示不完整记录，重启依可信库核对。这是处理远程数据库故障的必要细节，不宣称数据库和文件有分布式原子事务。

恢复在唯一实例监听前执行：验证目录非符号链接、键为规范 UUID、每条材料具有同班正文和普通原文件；不完整即失败且不清理有效资料。通过完整性检查后才清理 tmp 和未引用文件。严禁用旧 SQLite 或尚未验证的空 MySQL 扫描正式文件卷。迁移与演练使用卷副本；最终绑定已验证副本。恢复不与活跃上传并行，不新增锁服务。

### 6. Nginx、Compose 与初始化

最终三服务 nginx、app（Go）、mysql。保持宿主 8080 入口，由 Nginx 发布，app/mysql 仅内部网络；Nginx 不挂上传卷。所有应用路径直接代理 Go，保留 Host、Cookie 和 Set-Cookie；代理覆盖 X-Forwarded-Proto 等转发信息，Go 不根据客户端身份 header 授权。Secure 由服务器 COOKIE_SECURE 显式配置，本地 HTTP false，默认安全 true，不根据任意转发头关闭。

Nginx 上传 body 限制设为不额外限制，由 Go 的流式文件限额和 multipart 控制管理，以免 20 MiB 文件加边界后被代理拒绝。避免代理重试非幂等上传；超时足以覆盖既有上传和密码验证，测试通过真实代理检查。

MySQL 用独立持久卷；material-data 保留文件持久化且旧 SQLite 不被删除，最终 app 仅挂文件子目录/独立复制卷。环境模板提供 DB 主机/库/用户/密码、MYSQL 初始化密码、COOKIE_SECURE、UPLOAD_ROOT、MAX_UPLOAD_BYTES、四样例账号密码的占位配置，不在镜像构建注入真实值。Go 镜像只含二进制、模板、静态资源和必要运行文件。

启动顺序：MySQL 健康 → 显式 schema/迁移或 fresh 初始化命令完成 → Go 检查 schema、数据门槛、过期 Session 和文件恢复 → 开始监听/健康 → Nginx 可用。全新一键 Compose 路径可在 Go 启动前运行 fresh 初始化，但必须显式 fresh 模式；已有数据使用 migrate 模式且未验收不得自动播种。初始化事务提交数据并在失败时清理本次文件，使用初始化完成标记防止半成品就绪，不重置已有账号。提供 Go 账号/班级配置命令替代旧脚本职责。

### 7. 验证方法

保留全部 Python tests 原样作为旧实现基线，不能声称 Flask test_client、session_transaction、SQLite trigger 测试直接验证 Go。新增独立 HTTP 合约套件对临时 Flask 与 Go/MySQL 各执行同一行为矩阵，CSRF 从页面读取；内部故障注入改由新增 Go 测试覆盖并逐项映射旧测试，而非删除旧断言。

Go unit 覆盖密码/Session/CSRF/上传/文件恢复，真实 MySQL repository 覆盖约束、事务和排序，迁移集成覆盖八表结构、七表完整数据、Unicode、正文/文件 hash、sessions 初始为空、旧 Cookie 拒绝及重新登录、幂等重跑/冲突/缺失数据。新 Go Session 的跨实例持久化、固定期限、轮换、logout/session_version、fail-closed 测试全部保留。经真实 Nginx/Compose 验证 Cookie、20 MiB边界、权限和重启持久化。所有使用隔离临时数据库/卷与假凭据，不对现有用户数据库执行测试迁移。

## Risks / Trade-offs

- 数据切换窗口需要停写 → 本项目选择离线一次性迁移和完整验证，避免双写复杂度。
- 跨语言密码、Cookie、模板与文件名细节偏差 → 旧值测试向量、cutover 重新登录和 HTTP 差异测试阻止带回归切换。
- MySQL 排序/字段容量与 SQLite 不同 → 明确二进制比较、类型预检、完整内容校验，不静默归一化或截断。
- 远程提交结果不确定 → 依据材料 UUID 重查并保留文件证据，启动恢复前不清理。
- 仓库当前文件均未跟踪，不能依赖 git diff 证明完整基线 → Phase 0 记录文件清单/hash和测试结果，后续由操作者正常版本管理，不自动提交或删除文件。

## Migration Plan

0. Baseline：运行旧全量测试，保存 API/安全矩阵、schema 和可复现文件清单。
1. Infrastructure：并行建立 Go/MySQL/Nginx 隔离开发栈，旧服务继续保留。
2. Schema & Migration：实现八表 schema、迁移/验证命令，以副本反复演练，不接正式入口。
3. Authentication & Session：实现并对照账号、CSRF、SID、过期、退出、故障拒绝。
4. Material APIs：迁移页面、材料、文件、事务与恢复，验证 A/B 双向隔离。
5. Compatibility & Cutover：所有测试通过后停旧服务（包括 Session 写入）；备份 SQLite 和文件；迁移七表数据至空目标并验证空 sessions，通知用户重新登录；启动 Go 使用已验证文件副本；先做受控冒烟再将 Nginx 入口切到 Go。旧进程不得与 Go 同时写同一文件卷。
6. Final Verification：验证入口、重启、完整数据和全部安全合约，记录验收；最终 Compose 不启动 Flask、不挂业务 SQLite、不依赖 Python。

切换前或尚未产生新写入时失败：停止 Go，保留失败目标报告，用配对 SQLite/文件快照恢复旧入口；若已正式 cutover，恢复副本须显式作废旧 sessions，不能因回退恢复已失效 Cookie，原备份不改动。Go 已产生新材料或 Session/退出后，禁止直接退回旧快照丢数据或恢复已撤销授权：停写保全当前 MySQL/文件，优先修复新栈；若必须回旧栈，需另行显式规划反向同步并重新校验安全状态。本 change 不实现自动反向双写。备份不自动删除，不执行部署/真实迁移于 Propose。

## Open Questions

没有需要用户选择的架构问题。实际源数据容量/格式仅在 Apply 的只读预检确认；不符合上述可验证映射时应报告阻塞，不擅改安全或丢弃记录。
