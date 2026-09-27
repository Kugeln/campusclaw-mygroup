## Why

当前实际运行栈是 Flask/Python + SQLite，第三课权限和服务端 Session 已完成。为采用后续课程要求的 Go + MySQL 基础，需要独立、可验证地迁移现有功能，避免与第四课检索同时改动导致安全或数据回归。

## What Changes

- 将业务后端等价迁移至 Go，保留全部八表结构，完整迁移除 sessions 外七表数据；保留原 ID、密码哈希、role、class_id、session_version、正文和文件关联。目标 sessions 初始为空，旧会话仅留在源备份，cutover 后重新登录。
- Go 无实际用途的 SECRET_KEY 不列为必需配置；MySQL 按 schema 能力验收，Compose/测试锁定具体版本。
- 增加最小 Nginx 入口，最终链路为 Browser → Nginx → Go → MySQL，上传文件仍使用私有持久卷。
- 保留 URL、响应字段、CSRF、teacher/student、班级隔离、上传事务补偿和启动恢复；Session 继续使用随机 SID、摘要持久化、8 小时固定期限与全设备退出失效。
- **BREAKING（部署层）**：最终启动配置、初始化和数据库运维从 Python/SQLite 切换为 Go/MySQL；必须完成迁移验证后再切换，不支持长期双库写入。
- 保留旧实现作为测试对照和旧数据库作为备份；最终运行无需 Flask、Python 或 SQLite。
- 不实现第四课功能，不修改 `add-traceable-vector-retrieval`。不新增 UI 功能、管理员、Redis、队列、微服务或编排框架。

## Capabilities

### New Capabilities

- `backend-stack-migration`: 定义完整数据迁移、Go 等价安全与材料行为、Nginx/Compose 运行及切换验收。

### Modified Capabilities

无已发布主规约可修改：`openspec/specs/` 当前为空。第三课规约仍在两个已完成但未归档的 change 中。本 change 明确替代其中 Flask/SQLite 的技术限定，其业务与安全要求全部继承；不修改那些历史 artifacts，也不假称它们已有主规约。

## Impact

Apply 将新增 Go handler/service/repository、安全和文件模块、MySQL schema 与显式迁移工具、Nginx 配置，并调整 Docker/Compose、环境模板和运行文档。模板仅作 Go 渲染所必需的语法适配，样式、交互和 API 保持兼容。保留 Python 原测试，并补充 Go、MySQL、迁移和通过 Nginx 的 HTTP 等价测试。当前 Propose 仅生成本 change 文档，不修改业务或执行迁移。
