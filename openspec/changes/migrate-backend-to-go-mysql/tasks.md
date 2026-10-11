## 1. Phase 0 — Baseline

- [x] 1.1 运行 `python -m pytest -q` 全量旧测试，记录环境、命令、结果和工作区文件/hash 清单；任何失败先定位，不删除或弱化测试。
- [x] 1.2 固化 design 的路由/状态/字段/权限矩阵与八表 schema 清单，建立原测试到新增 Go/HTTP 验收用例映射，核对每项第三课安全契约有对应断言。

## 2. Phase 1 — Infrastructure

- [x] 2.1 建立 Go cmd/internal 边界和锁定依赖，实现配置校验；以 build/unit 验证缺少必需配置拒绝启动且不输出秘密，server 不依赖 SQLite/Python，配置有效 JWT_SIGNING_SECRET 且未设 SECRET_KEY 仍可启动并认证。
- [x] 2.2 增加 MySQL 连接与匿名 health 端点，通过依赖可用/不可用及文件读写探测测试验证 200/503 JSON 和无 Session 副作用。
- [x] 2.3 规划落地隔离的 Nginx/app/mysql Compose、环境模板及持久卷；用 `docker compose config`、镜像构建和 Nginx 配置检查验证仅入口暴露、健康依赖和无真实 secret，保留旧运行方式供对照；MySQL 锁定补丁版本/摘要并验证所需能力，不以恰好8.4作为业务门槛。

Phase 0/1 验收记录：见 `docs/migration/baseline.md`、`docs/migration/phase1-results.md`。Phase 2 的 3.1–3.5 验收记录见 `docs/migration/phase2-results.md`，原历史勾选和记录保留。4.1–4.3 验收见 `docs/migration/phase3-login-results.md`；4.4–4.6及2.1补充验收见 `docs/migration/phase3-auth-results.md`。5.1–5.3验收见 `docs/migration/phase4-upload-results.md`；5.4–5.5验收见 `docs/migration/phase4-read-recovery-results.md`；5.6–5.7验收见 `docs/migration/phase4-ui-results.md`。当前30/30，Phase 3为6/6、Phase 4为7/7；Phase 5的6.1–6.4完成（4/4）。完整110项双端契约及五项明确边界见 `docs/migration/phase5-http-contract-results.md` 第10节，隔离迁移与Nginx切换演练见 `docs/migration/phase5-cutover-results.md`。6.3默认Compose验收见 `docs/migration/phase5-compose-results.md`。6.4运维手册见 `docs/migration/go-mysql-operations-runbook.md`；Phase 6三项基础迁移验收完成，矩阵及证据见 `docs/migration/phase6-final-verification.md`；正式HTTPS、生产恢复演练及入口切换未执行，历史验收记录保留。

## 3. Phase 2 — MySQL Schema and Migration

- [x] 3.1 实现八表 MySQL schema、版本准备及类型映射；真实 MySQL repository 测试验证 PK/FK/UNIQUE/NOT NULL/CHECK、同班复合关联、大小写/尾空格唯一性、时间排序和 20 MiB 正文。
- [x] 3.2 实现离线 SQLite/文件快照只读预检和清单；用缺文件、非法 FK、超长键、未知密码格式及正常八表样本验证明确拒绝与源数据未改变。
- [x] 3.3 实现仅空目标事务导入及安全重跑，复制七表所有 ID、密码哈希、session_version 与正文，创建空 sessions 不导入旧会话；验证成功、回滚、相同目标只验证、不同目标拒绝及提交结果不确定后的核对。
- [x] 3.4 实现逐表数量/ID/字段、关系、正文和文件 SHA-256 验证报告及切换门槛；注入内容损坏证明验证失败不标成功、不删除源、不启动未验证目标。
- [x] 3.5 实现 fresh 幂等初始化、种子和 Go 账号/班级命令；验证四样例账号、A/B 完整样例材料、占位结构、非法角色/班级拒绝，以及失败重试不重置已有密码或文件。

## 4. Phase 3 — Authentication and Bearer Token

- [x] 4.1 实现兼容现存 PBKDF2 格式的密码验证与登录，复用 Phase 2 哈希创建能力；旧哈希向量验证正确/错误密码、未知账号统一401、随机盐及无秘密输出；登录无需 Cookie/CSRF。
- [x] 4.2 引入成熟 Go JWT 库和 JWT_SIGNING_SECRET 配置，固定 HS256，严格校验 sub/ver/iat/exp 类型、范围与签名；验证缺失/空白/不足32字节密钥拒绝启动、无 SECRET_KEY 兜底，非法算法/签名/claims拒绝；保留 sessions schema/迁移工具且认证不依赖其读写或清理。
- [x] 4.3 实现 Token 签发响应 access_token/token_type=Bearer/expires_in=28800/redirect=/materials 和 no-store；用可控时钟验证固定8小时、访问不续期、到期401、重新登录不隐式撤销其他设备；不签发认证 Cookie、不实现 SID/CSRF 轮换。
- [x] 4.4 实现 MySQL logout 原子递增 session_version，提交后204；验证双客户端全部旧版本 Token 失效、新登录可用、同密钥正常重启保留未过期有效 Token 且不复活已撤销 Token；不删除或清理 sessions。
- [x] 4.5 实现 Bearer/teacher 中间件、只读 principal 和 /api/auth/me；验证缺失/非Bearer/重复歧义Header401、Cookie/query/form不兜底、匿名401、学生优先403、教师无需CSRF、身份参数不能授权；当前角色/班级和版本查库，保持A/B双向隔离。
- [x] 4.6 测试登录/认证用户查询及退出事务故障、无效用户/角色/班级/版本；确认故障通用500、无效身份401、合法角色/班级变化立即采用新值，无Token签发/上传成功副作用；health独立、日志脱敏，空sessions仍可认证。

## 5. Phase 4 — Material APIs

- [x] 5.1 实现私有文件存储和流式解析；单元测试覆盖扩展名、UTF-8/BOM/NUL、空/多文件、限额、同名、路径及文件名净化，不覆盖已有文件。
- [x] 5.2 实现材料与正文同一 MySQL 事务和 rename/提交流程；验证201仅在完整持久化后返回，上传者/班级来自可信 scope。
- [x] 5.3 验证文件移动、双表写入、确定提交失败及提交结果未知处理；确认回滚/补偿和保留待恢复证据，不删除已提交材料文件。
- [x] 5.4 实现列表/详情/附件 repository/service/handler；真实 MySQL 与 HTTP 测试核对原字段、plain_text、排序、空列表、文件原字节及安全 header。
- [x] 5.5 实现 Go 启动文件恢复；测试临时/孤立文件清理，缺正文/原文件、非法键/符号链接拒绝启动，未验证库不能触发正式文件清理。
- [x] 5.6 适配现有 Token 页面壳和静态路径，沿用 sessionStorage、同源 Bearer 封装及 credentials: omit；验证 me/列表/上传/详情/Blob下载/logout 携带 Header、无Token或401清理并跳转、403不登出、退出失败提示未确认；保留角色控件差异、HTML转义与textContent，不改无关UI。
- [x] 5.7 执行 A/B 两方向、两角色的完整安全矩阵；覆盖学生直接/伪造上传403、教师伪造字段400、列表参数400、跨班详情/下载404、身份header/Cookie无效、修改方法405且无数据副作用。

## 6. Phase 5 — Compatibility and Cutover

- [x] 6.1 建立独立 HTTP 契约套件，对现有 Token Flask/SQLite 与 Go/MySQL 比较登录字段、Bearer失败矩阵、me/页面壳、下载、权限及撤销；必需 JWT_SIGNING_SECRET 而非 SECRET_KEY，cutover重新登录；保留历史测试/记录，但已被替代的SID/Cookie/CSRF断言不作为新栈门槛，继续覆盖对应有效安全目标。
- [x] 6.2 在隔离副本演练停写、备份、八表结构和七表数据导入、验证和 Nginx 切换；验证原 ID、正文/文件 hash、sessions 初始为空、旧 Cookie 拒绝、新栈独立签名密钥拒绝切换前 Token、重新登录成功、session_version 原值迁移且不回退以及失败阻止切换。
- [x] 6.3 完成最终默认 Compose → Nginx → Go → MySQL 运行配置；验证无需 Flask/Python/业务 SQLite，MySQL 和文件持久卷重建后仍可读取原材料。
- [x] 6.4 编写初始化/迁移/验证/切换/回退操作文档并按隔离演练核对；说明切换/回退均保持Bearer-only、换签名密钥要求重新登录、撤销版本不得回退，新写入后不能直接退旧快照、备份不自动删除、真实部署需单独执行，不修改第四课 change。

## 7. Phase 6 — Final Verification

- [x] 7.1 通过真实 Nginx/Compose 验证完整Token登录、Authorization透传、页面壳/me、Bearer附件下载、401/403、全设备撤销、上传20 MiB边界、权限、health故障与同密钥重启持久性，检查仅 Nginx 暴露且无上传目录直达路径。
- [x] 7.2 运行当前Token契约适用的Python测试（含tests/token_auth）、`go test ./...`、真实MySQL repository/迁移集成及两后端HTTP契约测试；先明确并记录测试选择与历史Session断言替代映射，不要求无差别重跑旧Session全量基线；记录实际命令/通过数和未运行范围，缺环境报告阻塞，不跳过适用测试后声称通过。
- [x] 7.3 执行 `openspec validate migrate-backend-to-go-mysql --strict`，核对本 change artifacts 一致和第四课 artifacts 未改动，记录最终结果。
