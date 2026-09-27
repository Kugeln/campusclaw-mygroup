## Context

仓库只包含 OpenSpec 配置及工作流文件，未发现业务源码、测试、部署文件或既有规格；配置没有项目 context 字段。此设计是新增实现方案，不描述任何已存在功能。动机见 proposal.md，行为契约见 specs/ 下四份规格。

知识库保存原始文件、材料元数据及解析后的纯文本记录，每个教师或学生只属于一个班级，上传自动使用教师的班级。MVP 仅接收 .txt、.md，不引入检索问答。设计涉及认证、数据模型和部署，需在实施前统一约束。

## Goals / Non-Goals

**Goals:**

- 用一个同源应用集中处理页面、API、会话和文件授权，减少认证边界。
- 将班级限制下沉到材料查询入口，覆盖列表、详情和文件读取。
- 给出从空仓库到可启动、可验证应用的明确结构和数据恢复策略。

**Non-Goals:**

- 不构建独立前端服务、分布式存储、多实例部署或生产 HA；仅做文本解析入库，不引入 RAG、向量检索、智能问答或索引流水线。
- 不做 SSO、对话助手、作业提交与批改。
- 不提供用户自行修改角色或班级、班级管理页面；初始化账号使用运维命令。

## Decisions

### 1. 同源单体与本地持久化

采用 Python、Flask 3.x 服务端模板、JSON API、标准库 sqlite3 与私有文件目录，单容器单应用实例。服务端统一负责页面、API、认证和文件访问，Compose 内使用 gunicorn 启动。建议目录为 config/（配置）、accounts/（身份与初始化）、materials/（数据访问及解析）、templates/、tests/；实现时锁定实际依赖版本。SQLite 数据库和文件放在同一个命名数据卷的独立子目录中。

选择理由：项目从零开始且范围是简单材料库，Flask 签名会话、独立 CSRF 校验和 Werkzeug 密码哈希工具可以集中处理安全逻辑，sqlite3 无需引入 ORM。备选 React + 独立 API 会增加构建和跨域会话边界；PostgreSQL 与对象存储本次无需引入。SQLite 的并发写入限制通过短事务、单实例和受限上传规模接受。

### 2. 单班数据模型与初始化

| 实体 | 主要字段和约束 |
| --- | --- |
| Class | id、name |
| User | id、唯一 username、password_hash、role（teacher/student）、非空 class_id 外键、session_version（默认 0） |
| Material | UUID id、class_id 外键、uploader_id 外键、original_name、size、content_type、storage_key、created_at |
| KnowledgeEntry | id、唯一 material_id 外键、非空 class_id 外键、plain_text、created_at；每份材料对应一条解析文本记录 |

通过幂等 SQL 初始化 classes、users、materials、knowledge_entries，sqlite3 每个连接启用外键检查，使用参数化查询。角色以 CHECK 约束限定，班级外键不可为空。建立材料 class_id/created_at 索引；materials 的 (id, class_id) 唯一约束与 knowledge_entries 的复合外键保证文本记录和材料属于同班。上传者与材料班级由已验证 session 赋值，外键删除采用保护策略。本次不支持更改已创建用户的班级，不建立独立 Session 表。

提供幂等创建班级及创建账号的管理命令，账号命令明确接收一个班级和角色，密码通过无回显交互或标准输入读取，并统一调用密码哈希流程。重复账号报错，不隐式覆盖密码。不内置通用演示密码，不开放注册或默认管理后台；测试在隔离数据库中创建 A/B 两班各一名教师和学生。

另提供显式执行的幂等种子命令，预置 A/B 两个班级、每班一名教师和一名学生样本账号，以及名称和内容分别标明 A 班、B 班的 .txt 或 .md 样本材料。样本账号密码由操作者安全输入并哈希保存；样本材料复用解析和双表入库流程，重复执行不重复创建、不覆盖既有密码或材料。种子数据仅用于验证登录、角色权限、班级隔离和本班材料查询，不在普通启动时自动重置数据。

备选多对多班级关系与管理后台不符合本轮确认的简单范围。

### 3. 认证、会话和密码

使用 Werkzeug 提供的加盐 PBKDF2 密码哈希及验证工具，不自行实现密码学。采用 Flask signed-cookie session，SECRET_KEY 仅从服务端环境变量读取。登录成功清空旧 session，写入 user_id、role、class_id、session_version，设置 8 小时有效期。Cookie 签名用于防篡改，不加密内容，禁止存入密码、哈希或材料正文。

每次受保护请求先验证签名和有效期，再核对用户存在、session_version 及角色/班级与数据库一致；不一致即视为未认证。通过校验的 session 是授权及班级范围来源，忽略客户端自报身份。为保留退出后旧 Cookie 不可重用的行为，退出时递增该用户的 session_version 并清除 Cookie；这会同时使该用户其他已登录设备失效。该字段不是独立数据库会话，不维护 Session 实体。

会话 Cookie 使用 HttpOnly、SameSite=Lax；HTTPS 部署启用 Secure，本地 HTTP Compose 通过明确的开发配置关闭 Secure。所有改变状态的浏览器请求，包括登录和退出，都校验 CSRF。认证接口对不存在账号与密码错误给出统一 401；日志禁用请求体、Cookie、密码及哈希输出。备选浏览器 localStorage JWT 增加凭据暴露和退出撤销复杂度，本次不采用。

页面保护将未登录请求跳转 /login，API 独立返回 JSON 401，不复用会产生 HTML 跳转的页面装饰器。API 先识别身份并检查教师权限，再进行 CSRF 和上传内容处理，保证匿名上传为 401，已登录学生上传为 403；教师的缺失 CSRF 请求返回 403。采用服务端校验的 CSRF token，Flask 签名会话本身不替代 CSRF 防护。上传处理不能在权限检查前写入应用材料目录。

### 4. 路由与授权契约

| 路由 | 行为 |
| --- | --- |
| GET /login | 匿名可访问登录页 |
| POST /api/auth/login | 账号密码及 CSRF，成功 200 并写会话 Cookie，错误凭据 401 |
| POST /api/auth/logout | 已认证且 CSRF 有效时退出，返回 204 |
| GET /materials | 受保护列表页，教师可使用上传表单，学生只有读取操作 |
| GET /api/materials | 本班列表，200；拒绝 class_id 等客户端范围字段，返回 400 |
| POST /api/materials | 单文件 multipart 上传，教师成功 201，匿名 401，学生 403 |
| GET /api/materials/{id} | 同班详情 200；跨班或不存在 404 |
| GET /api/materials/{id}/file | 同班附件下载；跨班或不存在 404 |
| GET /health | 匿名健康检查 |

登录成功固定进入 /materials，不接收任意外部跳转 URL。列表返回 items，按 created_at 和 id 稳定降序排列，展示材料 ID、原始文件名、大小、上传时间和上传者，不暴露 storage_key 或绝对路径。详情同样不暴露存储地址。

材料列表必须使用已验证 session 中的 class_id 过滤，详情和下载执行 id 与该 class_id 联合查询；读取 knowledge_entries 时同样限制 material_id 与 class_id。不能先读取任意班级记录再由前端判断。上传的 class_id 只能来自当前 session，uploader_id 来自 session.user_id；客户端提交 class_id、uploader_id、role 等字段一律拒绝，学生始终先因角色失败而返回 403。班级不由表单选择，完整入库流程见下一节。

选用统一 404 代替跨班 403，避免材料存在性泄露；这仍是服务端拒绝访问。页面隐藏按钮仅改善体验，直接 API 测试是权限验收依据。

### 5. 文件与记录一致性

默认单文件上限 20 MiB，通过服务端 MAX_UPLOAD_BYTES 配置，按流读取累计大小而非仅信任 Content-Length。MVP 仅接收非空 .txt、.md（扩展名大小写归一化），其他类型返回 400。文件以 UTF-8（允许 BOM）严格解码为 plain text；.md 保留 Markdown 文本，不渲染 HTML、不执行内容。无法解码或包含 NUL 的内容视为解析失败，返回 400。原文件名只作为经过清理的显示和附件名，文件路径使用服务端随机 UUID storage_key，存储目录不配置公开静态服务。

处理流程：权限检查（认证、教师角色、CSRF）→ 文件校验（字段、扩展名、非空及大小）→ 私有目录暂存并持续检查流大小 → parse 为 plain text → 在同一数据库事务中写入 materials 和 knowledge_entries → 将原文件原子移动至同卷最终目录 → 提交成功后返回 201，本班列表立即可查。双表使用相同的 session.class_id，knowledge_entries.material_id 关联本次材料 ID，原文件字节保持不变。

解析失败时清理暂存文件，不写入任何数据库记录；后续任一表写入、文件移动或提交失败时回滚整个事务并清理本次文件，服务端存储错误返回 5xx。数据库与文件系统没有共同事务，因此进程中断可能留下未提交文件。应用在接收请求前执行恢复：删除暂存文件及未被材料记录引用的孤立文件，仅操作受控存储目录；若发现材料缺少文件或对应文本记录，启动失败并要求恢复数据，不静默展示不完整材料。

下载在联合查询成功后由应用流式返回，设置 Content-Disposition: attachment 和 X-Content-Type-Options: nosniff。备选公开静态 URL 会绕过认证，禁止使用；预签名 URL 与对象存储不在本次架构内。

### 6. 部署配置与健康检查

交付 Dockerfile、docker-compose.yml、.env.example、忽略规则和 README。Compose 在运行时从操作者的服务端环境注入 SECRET_KEY，不经 build args、不写进镜像；真实密钥只放服务端环境变量，不写入 .env 文件，示例只记录变量名与占位说明。应用缺少密钥即失败，不启用默认密钥；生产禁用 DEBUG。数据卷保存 SQLite 和私有文件，容器使用可写该卷的非 root 用户。

README 包含“价值、场景、不做”三行说明、Docker 启动方式以及显式执行种子命令的步骤。启动顺序为配置验证、首次自动初始化、文件与双表一致性恢复检查、gunicorn 启动 Flask 应用；初始化和恢复在启动 worker 前只执行一次。data/app.db 不存在时，scripts/init_db.py 从服务端环境读取四个样本密码，在暂存数据库中建表和生成种子，文件就绪后才发布数据库；失败不留下被误认为完成的 app.db，可修正配置后重试。已有数据库不重置、不覆盖密码或重复创建材料。显式种子命令仍可独立使用。GET /health 检查数据库查询和存储读写（受控临时探测文件随即清理），健康返回 200/{"status":"ok"}，依赖异常返回 503/{"status":"unhealthy"}。Compose healthcheck 调用此端点。存储就绪检查优于仅检查进程存活，能发现无法上传材料的部署。

## Risks / Trade-offs

- [SQLite 写入竞争与单机磁盘容量有限] → 保持短事务，限制上传大小，在 README 明确单实例范围和卷容量维护要求。
- [文件系统与数据库无法原子提交] → 同卷原子移动、异常补偿、启动前清理，以及故障注入测试。
- [上传内容不做恶意内容扫描] → 私有存储、强制附件下载和禁止执行；本轮材料库不保证内容安全。
- [错误部署 Cookie 或密钥配置导致认证风险] → 区分本地 HTTP 与 HTTPS 配置，生产 Secure Cookie、关闭 DEBUG，启动配置校验和泄露检查。
- [班级过滤遗漏造成越权] → 共用受限查询入口，以两班两角色的直接 API 测试覆盖每个材料入口。
- [签名 Cookie 内容可见且复制后可能重放] → 仅保存最小身份字段，校验到期时间与用户 session_version；接受退出会同时注销该用户所有设备的限制。
- [文本编码或解析失败] → 限定 .txt/.md 与 UTF-8，严格解码，失败清理文件且不留下双表记录；不扩展为通用文档解析或 RAG。

## Migration Plan

1. 空库通过 SQL 初始化班级、用户、materials 和 knowledge_entries 表及约束；不存在旧数据迁移或会话表。
2. 操作者设置 SECRET_KEY 和四个样本密码环境变量，执行 docker compose up --build；空卷由 worker 启动前的初始化命令自动创建 A/B 班、样本教师和学生账号及两班可区分的样本材料。已有数据库保持不变。
3. 验证 GET /health、两类账号登录、教师上传后文件与双表记录一致、本班查询、学生上传拒绝和跨班读取拒绝，并验证解析失败无残留。
4. 停止写入后备份整个数据卷，数据库与文件保持同一备份时点；普通重启不得删除卷。
5. 后续版本上线前备份，回滚使用兼容镜像；若迁移不兼容则停止服务后恢复成套卷备份。禁止以 docker compose down -v 作为普通回滚手段。
