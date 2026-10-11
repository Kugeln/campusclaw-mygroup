# replace-session-auth-with-token-auth 验收结果

日期：2026-09-30。范围：已批准的 Flask 业务栈 Token change，8 项任务。

| 验收 | 结果 |
| --- | --- |
| JWT 单元测试 | 31 passed，0.30s；配置 fail closed、HS256、签名、必需 claims/类型及固定 8 小时边界 |
| auth/权限 HTTP 测试 | 8 passed，21.83s；真实密码登录、Bearer、401 矩阵、旧 Cookie-only 拒绝、双客户端撤销/重启、写入及提交故障、当前角色/班级、A/B 双向隔离和 forged identity |
| 真实浏览器测试 | 3 passed，20.44s；Edge Chromium + 临时 HTTP 服务，登录/刷新/上传/阅读/下载/logout 请求 Header、无 Cookie、Blob 字节、sessionStorage、403 保持登录、401 清理、退出网络失败提示、XSS 文本渲染 |
| 最小回归 | 纳入 HTTP 测试：health 200、页面壳无敏感数据、上传大小拒绝且无副作用 |
| 范围核查 | migration change、其验收记录、向量检索 change、MySQL schema 与迁移代码的限定 git diff 为空 |

合计 42 项自动测试通过。各套件分别运行，明确通过后未重复运行无关测试。浏览器套件最初有测试文件字符串语法错误，修正后 3 项全通过；内置浏览器服务模块缺失，改用隔离 Playwright/本机 Edge 完成实际浏览器验收。

环境配置与复现命令见 [token-auth.md](token-auth.md)。测试使用随机凭据、临时数据库和上传目录；未运行旧 Flask Session baseline、Phase 1/2 集成测试、Docker、SQLite→MySQL migration、部署或 archive，未 commit/push。

OpenSpec strict validation：`openspec validate replace-session-auth-with-token-auth --strict` 通过（`Change 'replace-session-auth-with-token-auth' is valid`）。8/8 tasks 已勾选，`git diff --check` 通过，无未完成或阻塞项。
