## Context

见 proposal.md。当前 `app/auth.py` 以 Session 查询用户，`app/routes.py` 仍直接读取 session 身份；模板嵌入班级/材料，使用 Cookie fetch、CSRF 表单和下载链接。`users` 已有 id、username、password_hash、role、class_id、session_version；未见独立 enabled 字段。MySQL schema 同样有 session_version 和 sessions；Go HTTP 目前仅实现 health。

现有 `move-auth-to-server-side-session`、`add-class-scoped-material-library` 及 migration 规划要求 SID Cookie/CSRF。本 change 明确替代其认证机制、页面访问形式和角色/班级快照失配行为，不修改那些文件或历史验收结论；材料权限和 400/403/404 语义保持不变。当前主 specs 为空，因此新增独立 bearer-token-auth 能力。

## Goals / Non-Goals

**Goals:** 建立跨 Flask/Go 的 HTTP 认证契约，业务只消费可信请求身份，撤销可持久化且不增加表。

**Non-Goals:** 不实现 Go 尚缺的业务接口、不继续 migration、不做 Phase 3、不涉及向量检索；不新增账号状态字段、Refresh Token、blacklist、Redis 或认证服务。当前可执行实现落点是已有 Flask 业务栈；未来 Go 适配复用本契约，不作为本 change 完成前提。

## Decisions

### 1. JWT 与签发契约

已读取材料未规定其他 Token 格式，采用 JWT HS256（成熟库签名/校验，不自写密码算法）。与 opaque Token 相比无需引入新的凭据存储；与非对称签名相比适合当前单体规模。只允许固定 HS256，拒绝 none、其他算法及重复/歧义 Authorization；scheme 大小写不敏感，必须恰有一个非空凭据。

最小 claims：`sub` 为用户 id 的十进制字符串，`ver` 为非负整数 session_version，`iat`/`exp` 为整数 Unix 秒，`exp = iat + 28800`。四项必填且严格验型，校验签名、sub 可映射、时间顺序、iat 不在未来及当前时间小于 exp；无滑动续期。不签入 role/class_id；即使外来已签 Token 带这些字段也不用于授权。

POST `/api/auth/login` 保持凭据输入，返回 `{access_token, token_type: "Bearer", expires_in: 28800, redirect: "/materials"}`，响应 `Cache-Control: no-store`。错误凭据统一 401。前端开始新登录时清掉旧本地身份，不隐式撤销其他设备（全设备撤销由 logout 负责）。

环境变量 `JWT_SIGNING_SECRET` 提供至少 32 字节随机秘密，启动检查缺失/空值/长度不足并 fail closed；不回退 Flask SECRET_KEY 或默认值。样例只给占位说明，不提供可用默认值；日志过滤 Authorization、Token 响应和异常中的原始凭据。生产通过 HTTPS 传输，代理透传 Authorization；轮换密钥要求重新登录，不增加多密钥系统。

### 2. 可信 principal 与请求顺序

中间件：Header → JWT 验签与 claims 校验 → 按 sub 查当前用户及班级 → 比较 ver/session_version → 生成只读 `AuthenticatedPrincipal {user_id, role, class_id}`。现有模型中“失效”指不存在、非法角色、无有效班级或版本不匹配，不虚构 enabled 列。角色/班级合法变更立即使用数据库新值，不因旧快照而保留权限。

Flask 用请求上下文承载 principal；未来 Go 用 request context。handler/service 使用 principal，不解析 JWT、不读取 session、不以客户端字段覆盖身份。上传顺序为认证 → teacher → 输入/文件校验，保持学生 403 优先；查询持续按 principal.class_id 限定。缺失/非法认证统一 401，权限不足 403，跨班资源保持 404，用户查询故障返回通用 5xx，不误报为无用户或回退 Cookie。

### 3. 复用版本撤销

`ver` 直接映射 `users.session_version`，不新增或重命名 token_version 列。logout 以当前 principal 执行原子 `session_version = session_version + 1`，确认事务提交才返回 204；失败返回 5xx。所有旧版本 Token（全部设备）在后续认证时 401，重新登录签发新版本。并发登录若读取了旧版本，只会得到随后被拒绝的 Token；不通过缓存绕过数据库版本检查。已经通过认证的在途请求不承诺回滚。

保留 MySQL sessions 表、schema、迁移脚本与已有数据，不以其读写或清理作为 Token 运行前提。关闭此运行路径上的认证 Session/匿名 CSRF Session 钩子；旧 Cookie 忽略且绝不授权，可在响应中清除。无需黑名单或每 Token 记录；代价是每请求查询用户，符合当前规模和即时角色/班级要求。

### 4. 前端与页面边界

选择同源 `sessionStorage` 保存 Token：跨当前多页面跳转和刷新可用，通常随标签页关闭结束；相比 localStorage 减少长期留存，相比仅内存无需重写为 SPA。它不是 HttpOnly，也不提供 XSS 防护；各标签页独立，其他设备/标签页在下一次 401 清理。角色显示只取受保护 `GET /api/auth/me` 返回的 `{user_id, role, class_id, class_name}`，不解码 Token 作为权限依据。

抽取小型共享请求封装，仅对本应用同源 API 添加 Bearer，使用 `credentials: omit`，防止凭据转发到外部 URL。登录表单由 JS 提交；所有受保护请求（含 FormData 上传、me、logout、下载）走此封装。401 清理本地状态/已显示材料并跳转登录；403 仅提示无权限。退出先取当前 Token 发请求，随后无论网络结果均清理本地状态；只有 204 确认服务端撤销，失败明确提示未确认。

浏览器普通导航及 `<a>` 下载不能自动加 Header，故 `/materials` 改为不含身份/材料的公共页面壳，启动时调用 me 和材料 API；无 Token 则回登录。下载改为 Bearer fetch → Blob → 临时 object URL 下载并及时 revoke，绝不把 Token 放 URL。保留页面布局和既有材料 API 路径；新增 me 仅用于身份展示。此处明确取代旧“匿名材料页面必须服务端 302”的形式，受保护数据仍完全由服务端鉴权。

### 5. CSRF 与 XSS

移除仅为认证 Cookie 服务的 login/logout/upload CSRF token 要求；显式 Bearer 不随跨站请求自动发送，因此不能机械保留依赖匿名 Session 的 CSRF 流程。保持同源部署，不开放任意来源 CORS；登录不写 Cookie、不把 Token 放重定向 URL，只由本站 JS 保存成功响应，跨站表单不能建立浏览器认证状态。若未来恢复 Cookie 认证，必须重新设计 CSRF，不能套用本结论。

XSS 可读取 sessionStorage 并窃取 Token，8 小时和 logout 撤销只能限制影响，不能替代防护。继续用模板转义及 textContent 展示不可信材料/文件名，禁止注入原始 HTML、不加载不必要的第三方脚本，响应不缓存 Token，不输出完整 Token；不将客户端隐藏按钮视为授权。

## Risks / Trade-offs

- JWT 并非完全无状态 → 每请求查询当前用户/版本，换取即时授权与全设备撤销。
- sessionStorage 可被同源脚本读取、Blob 下载占用内存 → 严格文本渲染与同源请求，沿用现有上传大小限制，及时释放 Blob。
- 两套 change 的认证描述存在历史冲突 → 本文件明确 Token 契约的替代范围；不回写迁移任务或验收，后续切换后端必须独立遵守该契约。
- 登出网络/提交失败无法保证旧 Token 撤销 → 本地清理并告知未确认，禁止把仅删除本地 Token 当作服务端退出成功。

## Migration Plan

仅为后续 Apply 的认证切换安排，并非数据库迁移阶段：配置独立 signing secret，完成现有栈 Token 中间件及前端后协调切换，用户重新登录；不执行 DDL、数据迁移或 sessions 删除。验证 Bearer 契约后启用，禁止 Cookie/Bearer 双轨兜底。若需回退，暂停访问并经独立授权恢复旧应用，先持久化使已有凭据失效、要求重新登录，避免旧 sessions 恢复授权；本轮不执行切换、部署或回退。
