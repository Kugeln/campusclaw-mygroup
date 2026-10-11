## 1. Token 签发与可信认证

- [x] 1.1 在现有认证栈加入成熟 JWT 库、JWT_SIGNING_SECRET 环境配置与固定 HS256/claims 校验；验证缺失密钥拒绝启动、无默认真实密钥且日志不输出完整 Token。
- [x] 1.2 将 username/password 登录改为签发 sub/ver/iat/exp Token，固定 28800 秒并返回约定字段；验证 teacher/student 正确登录获得 Token、错误凭据 401、无需 Cookie/CSRF 且访问不续期。
- [x] 1.3 实现 Bearer 中间件、当前用户/版本查询、principal 及受保护 /api/auth/me；验证无 Cookie 的成功访问，缺失 Header、非 Bearer、malformed、坏签名、过期、无效用户与版本不匹配均 401，用户查询故障为 5xx，handler 不解析 JWT。

## 2. 撤销与权限兼容

- [x] 2.1 logout 原子递增 session_version，提交后返回 204；验证双客户端旧 Token 均失效、重新登录可用、重启不复活旧 Token，提交失败返回 5xx 而非成功。
- [x] 2.2 将材料身份消费改为 principal，停用认证 Session 和相关 CSRF 钩子；验证教师上传 201、学生上传 403、匿名 401、A/B 双向列表/详情/下载隔离及 forged identity 无效，数据库角色/班级变更立即生效；确认 MySQL sessions schema 和迁移文件未改且空 sessions 不影响 Token 认证。

## 3. 前端与独立验收

- [x] 3.1 引入 sessionStorage 和同源 API 请求封装，登录存 Token，受保护请求添加 Authorization，logout/401 清理身份；验证 403 不误登出、logout 网络失败提示未确认撤销、Cookie 不参与请求、Token 不进入 URL。
- [x] 3.2 将材料页改为无敏感数据的页面壳，以 me/材料 API 加载内容，下载改为 Bearer fetch/Blob；用浏览器验证跳转/刷新、列表/上传/阅读/下载/logout 均携带 Header，匿名无法取得数据，不可信文本不执行脚本。
- [x] 3.3 建立独立 Token HTTP 契约验收与简短配置说明，覆盖 spec 的认证失败矩阵、权限、全设备撤销和前端 Header；以隔离数据验证，记录本 change 的结果，不重跑 Flask 旧 Session baseline、不继续 migration 任务、不改其验收记录或向量检索 change。

以上任务已在当前 Flask 业务栈实现并验收，证据见 `docs/auth/token-auth-results.md`，配置见 `docs/auth/token-auth.md`。未来 Go 采用相同 HTTP/principal 契约，不将补齐 Go 业务或数据库迁移列入本任务清单。
