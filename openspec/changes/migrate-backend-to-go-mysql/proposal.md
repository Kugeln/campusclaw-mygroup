## Why

当前实际运行栈是 Flask/Python + SQLite，第三课权限已完成，认证随后由 `replace-session-auth-with-token-auth` 改为 Bearer Token；本迁移沿用该现有契约。为采用后续课程要求的 Go + MySQL 基础，需要独立、可验证地迁移现有功能，避免与第四课检索同时改动导致安全或数据回归。

## What Changes

- 将业务后端等价迁移至 Go，保留全部八表结构，完整迁移除 sessions 外七表数据；保留原 ID、密码哈希、role、class_id、session_version、正文和文件关联。目标 sessions 初始为空，旧会话仅留在源备份，cutover 后重新登录。
- Go 不依赖 Flask SECRET_KEY，但认证必须配置独立 JWT_SIGNING_SECRET；MySQL 按 schema 能力验收，Compose/测试锁定具体版本。
- 增加最小 Nginx 入口，最终链路为 Browser → Nginx → Go → MySQL，上传文件仍使用私有持久卷。
- 保留现有 Bearer Token 的 URL、响应字段、teacher/student、班级隔离、上传事务补偿和启动恢复；采用 HS256 JWT、8 小时固定期限和 session_version 全设备撤销，不恢复 SID Cookie、认证 CSRF 或第二套认证。保留 sessions schema 与迁移工具，Token 认证不读写或清理 sessions。
- **BREAKING（部署层）**：最终启动配置、初始化和数据库运维从 Python/SQLite 切换为 Go/MySQL；必须完成迁移验证后再切换，不支持长期双库写入。
- 保留旧实现作为测试对照和旧数据库作为备份；最终运行无需 Flask、Python 或 SQLite。
- 不实现第四课功能，不修改 `add-traceable-vector-retrieval`。不新增 UI 功能、管理员、Redis、队列、微服务或编排框架。

## Capabilities

### New Capabilities

- `backend-stack-migration`: 定义完整数据迁移、Go 等价安全与材料行为、Nginx/Compose 运行及切换验收。

### Modified Capabilities

无已发布主规约可修改：`openspec/specs/` 当前为空。第三课规约仍在两个已完成但未归档的 change 中。本 change 明确替代其中 Flask/SQLite 的技术限定，其材料权限与数据安全要求继承，认证及页面访问形式以 `replace-session-auth-with-token-auth` 为准；不修改那些历史 artifacts，也不假称它们已有主规约。

## Impact

Apply 将新增 Go handler/service/repository、安全和文件模块、MySQL schema 与显式迁移工具、Nginx 配置，并调整 Docker/Compose、环境模板和运行文档。模板仅作 Go 渲染所必需的语法适配，样式、交互和 API 保持兼容。保留 Python 原测试，并补充 Go、MySQL、迁移和通过 Nginx 的 HTTP 等价测试。历史认证规约修订阶段仅核对并修订本 change 的认证规划，当时不实施 Phase 3、不执行迁移；后续实现进度与验收结果以 tasks 和 docs/migration 的分阶段记录为准。不修改其他 change 或覆盖历史验收记录。
