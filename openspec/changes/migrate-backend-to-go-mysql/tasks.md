## 1. Phase 0 — Baseline

- [x] 1.1 运行 `python -m pytest -q` 全量旧测试，记录环境、命令、结果和工作区文件/hash 清单；任何失败先定位，不删除或弱化测试。
- [x] 1.2 固化 design 的路由/状态/字段/权限矩阵与八表 schema 清单，建立原测试到新增 Go/HTTP 验收用例映射，核对每项第三课安全契约有对应断言。

## 2. Phase 1 — Infrastructure

- [ ] 2.1 建立 Go cmd/internal 边界和锁定依赖，实现配置校验；以 build/unit 验证缺少必需配置拒绝启动且不输出秘密，server 不依赖 SQLite/Python，未设 SECRET_KEY 仍可启动并认证。
- [x] 2.2 增加 MySQL 连接与匿名 health 端点，通过依赖可用/不可用及文件读写探测测试验证 200/503 JSON 和无 Session 副作用。
- [x] 2.3 规划落地隔离的 Nginx/app/mysql Compose、环境模板及持久卷；用 `docker compose config`、镜像构建和 Nginx 配置检查验证仅入口暴露、健康依赖和无真实 secret，保留旧运行方式供对照；MySQL 锁定补丁版本/摘要并验证所需能力，不以恰好8.4作为业务门槛。

Phase 0/1 验收记录：见 `docs/migration/baseline.md`、`docs/migration/phase1-results.md`。2.1 的项目骨架、配置、build/unit 与无 SECRET_KEY 启动部分已完成；“并认证”依赖 Phase 3，按本轮范围保持未勾选。Phase 2 的 3.1–3.5 已实现并通过验证，验收及收尾记录见 `docs/migration/phase2-results.md`。当前完成 9/30；Phase 3 及以后未开始。

## 3. Phase 2 — MySQL Schema and Migration

- [x] 3.1 实现八表 MySQL schema、版本准备及类型映射；真实 MySQL repository 测试验证 PK/FK/UNIQUE/NOT NULL/CHECK、同班复合关联、大小写/尾空格唯一性、时间排序和 20 MiB 正文。
- [x] 3.2 实现离线 SQLite/文件快照只读预检和清单；用缺文件、非法 FK、超长键、未知密码格式及正常八表样本验证明确拒绝与源数据未改变。
- [x] 3.3 实现仅空目标事务导入及安全重跑，复制七表所有 ID、密码哈希、session_version 与正文，创建空 sessions 不导入旧会话；验证成功、回滚、相同目标只验证、不同目标拒绝及提交结果不确定后的核对。
- [x] 3.4 实现逐表数量/ID/字段、关系、正文和文件 SHA-256 验证报告及切换门槛；注入内容损坏证明验证失败不标成功、不删除源、不启动未验证目标。
- [x] 3.5 实现 fresh 幂等初始化、种子和 Go 账号/班级命令；验证四样例账号、A/B 完整样例材料、占位结构、非法角色/班级拒绝，以及失败重试不重置已有密码或文件。

## 4. Phase 3 — Authentication and Session

- [ ] 4.1 实现兼容现存 PBKDF2 格式的密码验证/创建与登录逻辑；旧哈希测试向量验证正确/错误密码、未知账号统一401、随机盐及无秘密输出。
- [ ] 4.2 实现 MySQL Session repository 与随机 SID/摘要 Cookie；验证仅存 SHA-256、六字段正确类型、跨实例持久化、未知/篡改/删除记录拒绝；新 Go Session 安全测试完整保留，不要求旧 SQLite Cookie 兼容。
- [ ] 4.3 实现登录 SID/CSRF 轮换和固定 8 小时期；使用可控时钟测试持续请求不续期、边界到期拒绝、旧 SID 不可用及 Cookie 属性。
- [ ] 4.4 实现 logout 事务、session_version 核验与到期清理；验证204、清 Cookie、全部设备失效、旧请求不复活和启动仅清理过期记录。
- [ ] 4.5 实现认证/teacher/CSRF 中间件和可信 scope；验证匿名401/页面302、学生优先角色403、教师CSRF403、header/form/JSON token 优先级及身份参数不能授权。
- [ ] 4.6 测试 Session 读/登录写/退出事务故障和用户角色/班级/版本变化；确认通用500或失效401、无成功 Cookie/上传副作用、health 独立且日志脱敏。

## 5. Phase 4 — Material APIs

- [ ] 5.1 实现私有文件存储和流式解析；单元测试覆盖扩展名、UTF-8/BOM/NUL、空/多文件、限额、同名、路径及文件名净化，不覆盖已有文件。
- [ ] 5.2 实现材料与正文同一 MySQL 事务和 rename/提交流程；验证201仅在完整持久化后返回，上传者/班级来自可信 scope。
- [ ] 5.3 验证文件移动、双表写入、确定提交失败及提交结果未知处理；确认回滚/补偿和保留待恢复证据，不删除已提交材料文件。
- [ ] 5.4 实现列表/详情/附件 repository/service/handler；真实 MySQL 与 HTTP 测试核对原字段、plain_text、排序、空列表、文件原字节及安全 header。
- [ ] 5.5 实现 Go 启动文件恢复；测试临时/孤立文件清理，缺正文/原文件、非法键/符号链接拒绝启动，未验证库不能触发正式文件清理。
- [ ] 5.6 适配现有模板渲染和静态路径；验证登录/退出/上传/详情/下载交互、teacher/student 控件差异、HTML 转义及正文 textContent，不改无关 UI。
- [ ] 5.7 执行 A/B 两方向、两角色的完整安全矩阵；覆盖学生直接/伪造上传403、教师伪造字段400、列表参数400、跨班详情/下载404、身份header/Cookie无效、修改方法405且无数据副作用。

## 6. Phase 5 — Compatibility and Cutover

- [ ] 6.1 建立独立 HTTP 合约套件并分别运行临时 Flask/SQLite 与 Go/MySQL；逐项比较既有 API/页面行为，所有差异依据 spec 解决（Go 不继承 Flask SECRET_KEY 必填断言，cutover 允许重新登录），保留原 Python 测试和安全断言。
- [ ] 6.2 在隔离副本演练停写、备份、八表结构和七表数据导入、验证和 Nginx 切换；验证原 ID、正文/文件 hash、sessions 初始为空、旧 Cookie 拒绝、重新登录成功、session_version 不回退以及失败阻止切换。
- [ ] 6.3 完成最终默认 Compose → Nginx → Go → MySQL 运行配置；验证无需 Flask/Python/业务 SQLite，MySQL 和文件持久卷重建后仍可读取原材料。
- [ ] 6.4 编写初始化/迁移/验证/切换/回退操作文档并按隔离演练核对；说明新写入后不能直接退旧快照、备份不自动删除、真实部署需单独执行，不修改第四课 change。

## 7. Phase 6 — Final Verification

- [ ] 7.1 通过真实 Nginx/Compose 验证完整登录、Cookie、上传20 MiB边界、附件、权限、health故障与重启持久性，检查仅 Nginx 暴露且无上传目录直达路径。
- [ ] 7.2 运行最终 `python -m pytest -q`、`go test ./...`、真实 MySQL repository/迁移集成和两后端 HTTP 合约全量测试；记录命令与通过数，缺环境作为阻塞报告，不跳过后声称通过。
- [ ] 7.3 执行 `openspec validate migrate-backend-to-go-mysql --strict`，核对本 change artifacts 一致和第四课 artifacts 未改动，记录最终结果。
