# Bearer Token 认证运行说明

本说明对应独立 change `replace-session-auth-with-token-auth`，实现位于当前 Flask/SQLite 业务栈。Go/MySQL migration 的进度、schema 和验收记录未改变；未来 Go 业务接口须采用相同 HTTP 契约。

## 配置与使用

安装 `requirements.txt`。在服务器进程环境设置 `JWT_SIGNING_SECRET`（至少 32 字节随机秘密）、`DATABASE`（已有 SQLite 数据库）和 `UPLOAD_ROOT`（对应材料目录）。JWT 密钥应由安全随机源生成并保持稳定，不提交到仓库、写进示例或输出到日志；替换密钥会使旧 Token 无效。缺失/空白/过短密钥阻止应用启动；无需 SECRET_KEY 或 COOKIE_SECURE。

本地可用 `.venv/Scripts/python.exe -m flask --app app run --host 127.0.0.1 --port 8080` 启动，再访问 `/login`；该命令不初始化账号或迁移数据库。现有 Flask Compose 配置也已改为传入 JWT_SIGNING_SECRET，本 change 未运行 Docker 或部署。生产必须使用 HTTPS，代理透传 Authorization，不记录该 Header 或登录响应正文。

- `POST /api/auth/login`：JSON 或表单提交 username/password，返回 access_token、token_type=Bearer、expires_in=28800、redirect=/materials。
- 受保护请求只接受 `Authorization: Bearer <token>`；旧 Cookie、URL/form Token 均不能认证。
- JWT 固定 HS256，sub/ver/iat/exp 必填且验型，8 小时不续期；role/class_id 每次从当前数据库查询。
- `GET /api/auth/me`：返回当前 user_id、role、class_id、class_name。
- `POST /api/auth/logout`：提交 session_version 递增后才返回 204，全设备旧 Token 随后 401；失败为通用 5xx。
- 无效认证 401，学生上传 403，跨班详情/下载 404；用户状态读取失败 5xx 且不执行业务。

前端用 sessionStorage，所有 API 经 `app/static/api.js`；受保护请求带 Bearer，credentials=omit，并拒绝外部 URL 和请求重定向。材料页只提供公共壳；身份、列表和下载分别通过受保护请求获取，下载使用 fetch/Blob 临时 URL。退出或 401 清理本地状态；退出网络失败会提示服务端撤销未确认，403 不误登出。

Cookie 认证及其 CSRF 流程已停用，同源 API 不开放任意来源 CORS。sessionStorage 可被 XSS 读取，因此材料/文件名以 textContent 渲染，并限制同源脚本的 CSP；Token 从不进入 URL，响应不缓存。现有 sessions 表和旧 Session 模块保留，但不参与认证或正常启动时的会话准备。

签名/验签采用 [PyJWT 官方 API](https://pyjwt.readthedocs.io/en/2.10.1/api.html)，固定允许算法，应用额外验证版本、ID、整数时间和固定期限。

## 独立验收

使用临时 SQLite、临时文件目录和每次随机生成的测试密码/密钥，不操作现有数据。命令：

```text
.venv/Scripts/python.exe -m pytest tests/token_auth/test_tokens.py -q
.venv/Scripts/python.exe -m pytest tests/token_auth/test_http.py -q
.venv/Scripts/python.exe -m pip install -r tests/token_auth/requirements-browser.txt
.venv/Scripts/python.exe -m pytest tests/token_auth/test_browser.py -q
openspec validate replace-session-auth-with-token-auth --strict
```

浏览器套件需要本机 Microsoft Edge，运行无界面隔离 Chromium 上下文和临时 loopback HTTP 服务，结束后自动关闭。Playwright 是验收依赖，不加入生产依赖。旧 Session 测试描述已被替换的认证契约，保持原文件且不作为本 change 的 Token 验收；不运行 migration baseline。
