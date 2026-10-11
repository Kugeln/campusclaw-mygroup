# Phase 6 最终验收范围核对与最小必要验证

范围：基础Token认证和材料管理，不含RAG。本轮仅只读核对、隔离测试及验收文档；未操作Demo、正式入口、数据库或其三个卷，未修改业务实现。正式上线许可不由任务勾选自动产生。

## 原始三项任务（原样）

- 7.1 通过真实 Nginx/Compose 验证完整Token登录、Authorization透传、页面壳/me、Bearer附件下载、401/403、全设备撤销、上传20 MiB边界、权限、health故障与同密钥重启持久性，检查仅 Nginx 暴露且无上传目录直达路径。
- 7.2 运行当前Token契约适用的Python测试（含tests/token_auth）、`go test ./...`、真实MySQL repository/迁移集成及两后端HTTP契约测试；先明确并记录测试选择与历史Session断言替代映射，不要求无差别重跑旧Session全量基线；记录实际命令/通过数和未运行范围，缺环境报告阻塞，不跳过适用测试后声称通过。
- 7.3 执行 `openspec validate migrate-backend-to-go-mysql --strict`，核对本 change artifacts 一致和第四课 artifacts 未改动，记录最终结果。

## 状态定义与部署边界

- **已通过**：已有有效证据被复核并复用，或本轮实际执行成功；明确区分来源。
- **未通过**：实际执行失败或强制条件不满足。不能把未执行称作失败，也不能把失败当框架差异豁免。
- **未执行**：本轮没有运行；若复用历史通过结果，明确写“历史已通过、本轮未重跑”。
- **需正式部署环境验证**：域名/证书/HTTPS、实际卷与凭据、生产备份恢复/入口切换。未通过这些条件前不能声称生产可用。

spec 的“生产Bearer MUST使用HTTPS”仍强制有效；目前HTTPS**未执行且未具备**，因此生产放行受阻。7.1原文要求真实Nginx/Compose，不要求修改现有正式入口；design验证方法明确全部使用隔离临时库/卷。故本轮真实隔离链路可满足7.1，不能据此把HTTPS写成通过。7.2是适用测试集合，7.3是规约和文件一致性；三项并未要求本轮执行生产灾备恢复。Runbook规定的生产恢复演练仍是正式部署门槛，不将其从手册删除，也不扩成RAG迁移。

## 7.1 验收矩阵

| 原始条件 | 已有证据 | 本轮开始时缺失证据 | 最小验证方法及结论 |
|---|---|---|---|
| 完整Token登录、Authorization透传、页面壳/me | 6.2真实代理登录与可信身份；6.3四账号登录/me及页面200 | 无需重复完整流程 | 复核6.2/6.3报告和当前配置；**已通过（复用）** |
| Bearer附件、401/403、权限 | 6.1双端110；6.2/6.3双向班级隔离、学生拒绝、下载字节及安全头 | 无 | **已通过（复用）**，不把RAG页面算在内 |
| 全设备撤销 | Phase3真实MySQL、6.1多Token；6.3代理只验单旧Token | 同一次真实代理logout撤销两个不同Token | 新隔离项目签发两个不同Token，均先可用，logout204后均401；重新登录200；**已通过（新增）** |
| 上传20 MiB边界 | 6.3真实Nginx合法20 MiB+multipart开销201，20 MiB+1返回413 | 无 | **已通过（复用）**，未无意义重复大文件上传 |
| health故障 | 单元及早期基础设施测试，6.3只验正常health | 最终完整部署下真实依赖故障经Nginx返回503 | 停止本次新MySQL→503 JSON→恢复200；本次上传tmp目录不可写→503→恢复200；**已通过（新增）** |
| 同密钥重启持久性 | 6.3五文件字节/两卷/密码/版本不变，有效及撤销Token状态保留 | 双旧Token重启后仍撤销的代理组合 | 新隔离app同密钥重启，两旧Token401、新Token200；**已通过（复用＋新增）** |
| 仅Nginx暴露、私有目录不可直达 | 6.3真实Mounts/Ports审计及404矩阵 | 无 | 当前Compose/Nginx没有变化；**已通过（复用）** |

**7.1通过。** 新测试 `tests/compose/phase6.py` 只补缺口，不修改默认配置或业务。默认Compose构建并启动全新项目 `campusclaw-phase6-5c763ee0`，端口 **64959**，两个项目级新卷；无可传入Demo目标的参数。故障只施加于本次新MySQL和自有tmp目录，未删除材料、卷或数据库。测试最后停止本次服务，保留全部证据和卷。

实际命令：

```powershell
$env:GOPROXY = 'https://goproxy.cn'
.\.venv\Scripts\python.exe tests/compose/phase6.py
```

退出0，**5组阶段断言通过，21次HTTP观测**（请求数不是单元用例数）。缺失权限通过chmod临时目录模拟，属于真实文件系统权限故障，不冒称磁盘掉电测试；数据库故障为实际停止本次隔离MySQL。

证据 `runtime/campusclaw-phase6-5c763ee0/result.json`，accepted=true，SHA-256：
`4d4003fc7acac4558bbd4201da1dab2384a24a880c231688ad8b2f975db69d4f`。

## 7.2 测试选择、历史映射与矩阵

执行前已检查当前测试：根目录`tests/test_sessions.py`现在测试Bearer，而非旧Session机制，**不能因文件名排除**。选择所有根目录`test_*.py`及整个`tests/token_auth`（含三个独立Edge浏览器用例），分别运行避免同名conftest导入冲突。全部使用临时SQLite/附件、随机HTTP端口/新浏览器上下文；RAG测试目录不在迁移范围，不运行。没有通过修改断言消除失败。

| 历史安全目标 | 当前有效替代断言 |
|---|---|
| SID/Cookie授权、匿名Session | Bearer-only、Cookie-only401、无Set-Cookie、缺/非法Token401 |
| Session固定/轮换/有效期 | 严格HS256与claims、固定28800秒、不滑动续期、重登录不隐式撤销 |
| Session销毁、旧会话重放 | logout提交后204、版本递增、所有旧Token401、重启不复活 |
| CSRF与身份伪造防护 | 不从Cookie/form/query获取认证；同源Bearer、credentials omit、可信principal和teacher guard；不恢复SID/CSRF断言 |
| 角色/班级快照 | 每请求读取当前数据库身份，伪造role/class无效，A/B双向隔离 |
| 会话存储失败 | 用户查询/版本更新失败通用500且无授权、无虚假退出成功、日志脱敏 |

| 原始条件 | 已有证据 | 缺失证据/验证方法 | 本轮结果 |
|---|---|---|---|
| 当前适用Python（含token_auth） | 历史有局部验收，不能替代整个当前选择 | 完整运行选定基础目录＋整个token_auth | **36＋42＝78通过，0失败，0跳过** |
| `go test ./...` | 先前多为按包运行 | 原文明确完整命令，本轮完整运行 | **通过：8个包ok，9个包编译且无测试文件**；部分结果来自Go缓存，非伪称全部重新执行 |
| 真实MySQL repository/迁移集成 | Phase2：10个顶层＋10嵌套迁移/schema/事务场景；Phase3真实认证；Phase4上传/读取/恢复/UI；6.2完整迁移副本 | 相关业务代码没有变化，无需重复全部历史容器测试 | **历史已通过，本轮未重跑这些专用runner**；新7.1也实际使用MySQL，不能代替这些历史repository断言 |
| 两后端HTTP契约 | 6.1完整双端各110通过，80一致、30明确允许差异，0未确认/不兼容 | 已有最终完整报告且代码未变，无需再跑110 | **已通过（复用）**；本轮未重跑，不将局部21次代理请求冒充双端完整测试 |

复用依据：对比6.3开始时和本轮开始时工作区hash，原有169个业务/测试/依赖文件（internal、cmd、app、scripts、tests、go.mod/go.sum/requirements）无变化。6.3新增的bootstrap已经单独验收且纳入本轮完整Go测试；其后变动是部署准备/文档，不改变历史迁移、认证、材料和双端契约用例。6.3默认配置由本轮重新构建启动，health故障缺口单独补验。原始报告accepted及SHA-256再次核对：

文件清单与逐文件比较保存在 `runtime/campusclaw-phase6-5c763ee0/workspace-verification.json`；其中phase6_after为任务勾选前的核对快照，随后仅更新本记录及tasks完成状态，不把历史规划状态误当当前状态。

| 复用报告 | SHA-256 |
|---|---|
| `runtime/campusclaw-phase5-contract-de3a9dfe/report.json` | `bff3831aa1cac620b9d434dbcde897e1d7f95131b3dd663b2af4a531128737d3` |
| `runtime/campusclaw-phase5-cutover-20ce794d/result.json` | `6fbda8797fd00b3b5dbb33ac5e9b9dedbf2ff0a5219ca0e6e1dca136b039cc99` |
| `runtime/campusclaw-phase5-compose-ced2bc23/result.json` | `b90a5666d5e567505036024abc65f178cd85023fc42e49541acdbd7a2535b01a` |

专用MySQL集成使用build tags，不包含在普通`go test ./...`中。本轮没有把`[no test files]`算成迁移集成通过；其依据是 [Phase2](phase2-results.md)、[Phase3](phase3-auth-results.md)、[上传](phase4-upload-results.md)、[读取恢复](phase4-read-recovery-results.md)、[UI](phase4-ui-results.md) 的实际MySQL执行结果与6.2报告。

实际命令（临时目录每次新建，不复用/清空旧目录）：

```powershell
go test ./...
$p='runtime/phase6-python-'+[guid]::NewGuid().ToString('N')
New-Item -ItemType Directory -Path $p | Out-Null
$tests=Get-ChildItem tests/test_*.py | ForEach-Object FullName
.venv\Scripts\python.exe -m pytest -q -p no:cacheprovider --tb=short --basetemp "$p/tmp" --junitxml "$p/results.xml" @tests
$p='runtime/phase6-token-'+[guid]::NewGuid().ToString('N')
New-Item -ItemType Directory -Path $p | Out-Null
.venv\Scripts\python.exe -m pytest -q -p no:cacheprovider --tb=short --basetemp "$p/tmp" --junitxml "$p/results.xml" tests/token_auth
```

两次pytest退出0，基础测试177.91秒、Token测试92.15秒。JUnit证据：

- `runtime/phase6-python-19f4e8c5ff0e4ec0a7fb538104498e75/results.xml`
- `runtime/phase6-token-5b2c41798a254f399096a67e799e5808/results.xml`

**7.2通过**，基于上述本轮完整必需集合与明确复用证据，不代表本轮重复执行所有历史测试。范围外RAG/MySQL-Qdrant问答测试未执行；生产HTTPS和灾备演练未执行；未发现需要业务代码修复的问题。

## 7.3 一致性矩阵

| 原始条件 | 已有证据 | 本轮验证 | 状态 |
|---|---|---|---|
| strict validate | 各阶段历史通过 | 运行原命令并记录最终输出 | 已通过 |
| 本change artifacts一致 | proposal/design有认证修订时的历史时态；spec原验收标准保留 | 标注历史快照，明确原规划8080与当前隔离8081、生产HTTPS的区别；不重定义任务条件或修改spec | 已通过 |
| 第四课artifacts未改动 | 工作区已有第四课未提交修改，不能与HEAD不同就判断本轮修改 | 使用本轮开始/结束hash对比，保留全部原有修改 | 已通过 |

原命令：`openspec validate migrate-backend-to-go-mysql --strict`。
结果：`Change 'migrate-backend-to-go-mysql' is valid`，退出0。**7.3通过**；未归档、同步其他change或自动提交。

## 最终状态与仍受阻的正式部署

三个原始任务条件分别核对后通过，任务进度**30/30**。这是基础Go迁移的开发/隔离验收完成，**不是正式切换或生产就绪声明**。

| 状态 | 项目 |
|---|---|
| 已通过 | 7.1–7.3的上述条件；78项当前Python、完整Go命令、补充真实代理故障与撤销、明确复用的双端和MySQL证据 |
| 未通过 | 本轮执行项无失败，无已发现待修复业务缺陷 |
| 未执行 | 本轮未重复专用历史MySQL runner/双端110；未运行范围外RAG；未做正式切换 |
| 需正式部署环境验证 | HTTPS证书/域名/入口、真实凭据和数据卷归属、生产配对备份恢复演练、停写窗口与回退审批；这些条件仍阻止生产放行 |

Flask RAG Demo既未替换，也未启动/停止/探测；本轮没有证明其当前在线状态。保留全部未提交修改与测试证据。仅新增最小隔离测试和本记录，更新本change proposal/design历史说明及tasks；课程报告、业务代码、默认Compose/Nginx、其他change和旧验收记录保持不变。停止等待下一轮，不commit/push/archive。
