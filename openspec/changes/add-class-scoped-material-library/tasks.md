## 1. 仓库与应用骨架

- [x] 1.1 确认根目录 README 包含“价值、场景、不做”三行说明，限定当前材料库 MVP；verify：逐行检查说明存在，未将 Non-goals 列为交付功能。
- [x] 1.2 添加 requirements.txt，锁定 Flask 3.x、Werkzeug、gunicorn 及必要测试依赖，SQLite 使用标准库 sqlite3；verify：在干净虚拟环境执行 `python -m pip install -r requirements.txt`，exit code 为 0。
- [x] 1.3 建立 Flask 应用入口、app/ 包、模板及测试骨架，使用应用工厂避免导入时建库；verify：`python -c "import app"` 成功，配置测试环境后应用工厂可创建 Flask 实例。

## 2. 数据模型与种子

- [x] 2.1 实现 scripts/init_db.py，幂等建立 Class/User/Material/KnowledgeEntry 对应结构，包含 password_hash、role、class_id、session_version、文件元数据及 plain_text，设置唯一和外键约束；verify：临时目录中执行 `python scripts/init_db.py` 后数据库存在，检查表结构完整，重复账号、非法角色、无效班级和跨班文本关联均被拒绝。
- [x] 2.2 建立班级、用户、讲义、作业、助手、技能六类核心结构，讲义复用 Material/KnowledgeEntry，作业、助手、技能各仅保留结构和 0～1 条占位数据；verify：查询 SQLite 确认六类结构存在，额外讲义占位不超过一条且不替代两班必需样本材料，没有新增占位业务页面或 API。
- [x] 2.3 初始化 A/B 班、教师 A、学生 A1、教师 B、学生 B1，样本密码通过安全输入或服务端环境提供，使用 Werkzeug 加盐 PBKDF2 写入 password_hash，不内置默认密码；verify：查询角色和班级归属正确，同密码生成不同哈希，正确密码通过、错误密码失败，数据库及初始化输出不含明文密码。
- [x] 2.4 预置两班各至少一条标题和内容可区分的 .txt/.md 材料，保存文件和关联同班的 Material/KnowledgeEntry，重复初始化不覆盖密码或重复造数，第 5 节完成后复用其解析入库方法；verify：初始化两次后核对两班文件、解析文本及关联完整，记录数和原密码哈希不变。
- [x] 2.5 实现 app/db.py，连接启用外键检查，使用参数化 SQL，提供强制 class_id 的列表和 id + class_id 查询方法；verify：数据库测试分别查询两班仅返回本班数据，跨班 ID 无结果，缺少班级参数不能执行无范围查询。

## 3. 登录、会话与密码

- [x] 3.1 实现 Flask 登录页及账号密码登录 API，使用 Werkzeug 验证 password_hash，错误凭据统一返回 401；verify：Flask 测试客户端验证教师和学生登录成功，不存在账号和错误密码响应一致且不建立已认证 session。
- [x] 3.2 使用 signed-cookie session 保存 user_id、role、class_id、session_version，SECRET_KEY 仅从服务端环境读取，登录清空旧身份，按设计设置有效期及 HttpOnly、SameSite=Lax；verify：测试 session 字段、Cookie 标志及过期拒绝，篡改签名不能认证，缺少 SECRET_KEY 时启动失败且无默认密钥。
- [x] 3.3 实现统一登录保护，核对签名、到期及服务端用户信息；verify：测试匿名、伪造和过期会话访问页面重定向 /login，访问材料列表、上传、详情及文件 API 均返回 401，登录页可匿名访问。
- [x] 3.4 实现登出，清除 Cookie 并递增用户 session_version；verify：测试登录、保存 Cookie、退出后重放旧 Cookie，材料 API 返回 401，该用户其他旧 Cookie 同样失效。
- [x] 3.5 为登录、退出和上传提供服务端 CSRF 校验，认证和角色检查先于上传内容处理；verify：测试有效 token 通过，教师缺少或错误 token 返回 403，匿名上传仍为 401、学生上传仍为 403，响应和日志不含密码、哈希或 Cookie。

## 4. 班级隔离

- [x] 4.1 材料列表仅依据已验证 session.class_id 查询，客户端 class_id 参数返回 400；verify：学生 A1 列表和计数不含 B 班标题或记录，学生 B1 只看到 B 班数据，伪造班级不能改变范围，空班列表为 200 和空列表。
- [x] 4.2 详情、解析文本及文件访问执行服务端 id + class_id 校验，跨班及不存在材料统一返回 404；verify：教师 A 和学生 A1 使用 B 班 material id 请求详情及文件均为 404，响应不泄露 B 班标题、正文、路径或存储键，本班读取正常。
- [x] 4.3 私有 uploads 不挂载公开静态路由，下载经授权后返回附件和 nosniff；verify：直接存储路径不能取得文件，合法下载字节一致且包含附件头，含 HTML/脚本的 .txt/.md 不作为页面执行。

## 5. 角色权限与上传解析入库

- [x] 5.1 实现教师上传权限检查，class_id 和 uploader_id 仅来自 session/服务端身份，拒绝客户端覆盖；verify：学生直接上传或伪造 role 返回 403，DB 和 uploads 无新增；教师提交 class_id、uploader_id 或 role 返回 400 且无副作用。
- [x] 5.2 实现文件校验、私有暂存及 .txt/.md 解析为 plain text，扩展名不区分大小写，UTF-8 允许 BOM，拒绝非法编码和 NUL，不渲染 Markdown；verify：合法两类文件解析正确，不支持格式、缺失/空文件或解析失败返回 400，清理暂存文件，双表无脏数据。
- [x] 5.3 串联权限检查 → 文件校验 → 私有暂存 → 文本解析 → 同一事务写入 materials/knowledge_entries → 同卷移动文件 → 提交后返回 201 和 material_id；verify：上传成功后两表各新增一条，material_id、class_id、uploader_id 正确，原始文件和解析文本完整，提交前不返回成功。
- [x] 5.4 保留可配置大小上限（默认 20 MiB）、流式计数、UUID 存储键及安全文件名处理；verify：超限返回 413 且无双表或文件残留，同名文件互不覆盖，路径穿越文件名不能写出私有目录。
- [x] 5.5 实现存储/数据库失败回滚及文件清理、启动前孤立文件恢复；verify：针对性的存储失败和第二张表写入失败测试返回 5xx 且双表回滚，启动清理预置临时及孤立文件，已引用文件或文本记录缺失时拒绝启动，无需复杂故障注入平台。
- [x] 5.6 完成列表页、教师上传表单和学生只读展示，保留稳定排序、元数据展示与文本转义；verify：教师上传后教师及同班学生刷新列表均看到相同材料 ID，学生修改或覆盖尝试不能改变文件或双表，文件名脚本不能执行，不新增修改接口。

## 6. Docker Compose 与 GET /health

- [x] 6.1 添加 Dockerfile、docker-compose.yml、.env.example，服务名 app，gunicorn 监听 8080，持久化 data/app.db 与 uploads，暂存和最终文件位于同卷；verify：`docker compose config` 成功，服务、端口、挂载正确，非 root 应用用户可写持久目录，示例仅含环境变量名及占位说明。
- [x] 6.2 data/app.db 不存在时，在 gunicorn worker 启动前通过 scripts/init_db.py 自动建表和种子初始化，首次密码从预先设置的服务端环境读取，已有数据库不重置；verify：空目录启动后六类结构与种子完整，初始化失败不留下被误判为完成的数据库且可重试，再次启动保持原密码和材料不变。
- [x] 6.3 实现匿名 GET /health 检查数据库及 uploads 读写，并配置 Compose healthcheck；verify：`curl -i http://localhost:8080/health` 返回 HTTP 200 和 `{"status":"ok"}`，容器 healthy；依赖不可用测试返回 503 和 `{"status":"unhealthy"}`，不暴露内部路径或密钥。
- [x] 6.4 README 补充 .env 示例说明、服务端环境变量注入、Compose 启动、首次初始化和样本登录方式，真实 SECRET_KEY 不写入 .env；verify：按文档执行 `docker compose up --build` 成功，http://localhost:8080/login 可登录，http://localhost:8080/health 正常，所有端口一致，缺少必需密钥时启动失败。
- [x] 6.5 验证保留数据的停止和重建流程；verify：上传后执行 `docker compose down` 再执行 `docker compose up --build -d`，不删除卷或宿主数据，原账号仍可登录，列表、文件字节及 KnowledgeEntry 文本和关联仍存在。

## 7. 规约与收尾

- [x] 7.1 执行关键 Scenario 集成验收并记录可复现命令与结果；verify：跨班详情/文件为 404，学生上传为 403 且无新增，教师上传为 201 且 materials/knowledge_entries 均新增，同班学生列表可见，匿名页面跳转 /login、API 为 401，Compose 重启后数据仍存在。
- [x] 7.2 汇总密码、密钥及失败上传验收，执行必要配置与输出检查；verify：密码仅存安全哈希，测试密钥不在版本控制文件、镜像配置、浏览器资源、API 响应或日志出现，400/413/5xx 上传无双表及文件残留，自动化测试通过；不要求镜像层扫描、生产运维或灾备演练。
- [x] 7.3 执行 `openspec validate add-class-scoped-material-library --strict`；verify：命令 exit code 为 0 且无 strict validation error，保留校验结果；规格校验不替代业务 Scenario 验收。
