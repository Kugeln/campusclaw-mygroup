## Why

课程要求受保护请求显式携带 `Authorization: Bearer <token>`，当前 Session/Cookie 认证不满足要求。本 change 独立替换认证机制，保留账号密码、教师/学生权限和班级隔离。

## What Changes

- **BREAKING**：登录签发固定 8 小时 JWT，受保护 API 仅接受 Bearer Header，旧认证 Cookie 不再授权。
- 中间件验证 Token 并查询当前用户，向业务提供可信 principal；复用 `users.session_version` 实现 logout 全设备撤销。
- 前端统一发送 Authorization，调整页面数据加载与下载，移除认证 Cookie 依赖并重新界定 CSRF/XSS 防护。
- 保留现有 MySQL sessions schema，不读写它来建立新认证，不删除或重新迁移。

## Capabilities

### New Capabilities
- `bearer-token-auth`: 登录签发、Bearer 校验、可信身份、期限、撤销及前端行为。

### Modified Capabilities
无。当前 `openspec/specs/` 无已发布能力；相关认证与权限规约位于其他 change。新增能力明确替代其中 Session/Cookie 认证行为，沿用权限约束，不修改其他 change。

## Impact

未来实现涉及当前 Flask 认证入口、请求身份消费、模板及请求封装、环境配置和认证验收；增加维护中的 JWT 库。Go 目前仅有 health HTTP 入口，本 change 不补齐 Go 业务 API、不继续迁移任务。外部认证契约和 principal 与 Flask/Go 解耦，后续后端适配沿用该契约。

与 `migrate-backend-to-go-mysql` 完全独立，不是其 Phase 3；不改该 change、已完成验收记录或 `add-traceable-vector-retrieval`。不引入 Redis、blacklist、OAuth/OIDC、认证微服务或 Refresh Token；本轮仅 Propose。
