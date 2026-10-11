# Phase 5 / 6.2 隔离迁移与 Nginx 切换演练

**6.2 验收通过，总进度25/30。** 本次只演练隔离副本，没有实施6.3、6.4或Phase6，没有迁移RAG或切换正式入口。

## 环境与实际执行

成功项目：`campusclaw-phase5-cutover-20ce794d`。

- 独立Flask/SQLite源服务：`127.0.0.1:53914`，明确 `MATERIAL_BACKEND=sqlite`；数据来自共享测试夹具，并增加非空占位表、非零账号版本及一个历史sessions行。没有使用真实Demo作为源。
- 独立Nginx入口：`127.0.0.1:54546`。初始为维护503，验证完成后在同一隔离端口reload到 `Nginx → Go → MySQL`。源Flask在快照前已经停止，不是双写切换。
- MySQL8.4.4、Nginx1.28.0-alpine、Go1.26.0/alpine3.23、运行工具Alpine3.23.3。构建专用镜像 `campusclaw-phase5-cutover-20ce794d:rehearsal`，未改默认Dockerfile或Compose。
- 全新卷：`campusclaw-phase5-cutover-20ce794d_mysql-data`、`campusclaw-phase5-cutover-20ce794d_target-files`；全新frontend/backend网络，backend为internal。
- 实际审计14个本次容器的挂载/端口元数据：卷均属于新project、host挂载均位于新runtime，Nginx仅挂配置，app/MySQL无宿主发布端口。未挂载Demo三个卷；Go只收到新目标文件卷，无法扫描Demo上传目录。
- 导入工具、ownership初始化和Go服务顺序运行，未让两个写入进程同时共享上传卷。HTTP验证后先停止Go，再离线执行最终verify。

命令（仓库根目录，PowerShell）：

```powershell
.\.venv\Scripts\python.exe -m unittest discover -s tests/cutover -p test_gate.py -v
$env:GOPROXY = 'https://goproxy.cn'
go test -mod=readonly ./internal/migration ./internal/transfer
$env:CUTOVER_GOPROXY = 'https://goproxy.cn'
.\.venv\Scripts\python.exe tests/cutover/run.py
openspec validate migrate-backend-to-go-mysql --strict
```

- Gate单元测试1项通过，覆盖未验证、陈旧源hash、错误指纹、源未停写、目标不健康等拒绝条件。
- Go migration/transfer包编译通过，输出均为 `[no test files]`；**不把编译描述成真实迁移集成测试通过**。真实数据库/事务/迁移证据来自下述Compose演练。
- 完整演练退出0、10个阶段断言通过；记录70次隔离Nginx HTTP请求，包括4个维护503、32个200、4个401、25个404、2个400、3个403。请求数量不是70个独立测试用例。
- OpenSpec strict validate通过。

首个项目 `campusclaw-phase5-cutover-e0447bec` 在构建时因默认 `proxy.golang.org` 连接失败停止，尚未启动目标数据库/入口；其源快照和失败记录保留。主机迁移依赖首次下载也失败。使用可达镜像后重试成功，**依赖版本、go.mod/go.sum、TLS和校验和验证均未改变**，仅为该进程/演练构建设置代理，不修改全局Go配置。

## 停写、备份与完整性

源端先执行真实Bearer登录、logout和重新登录，证明撤销版本有实际变化。随后停止源HTTP监听并验证端口不可访问；以SQLite backup API生成独立一致性 `source.db`，复制匹配的上传文件，确认无WAL/SHM/journal旁文件。迁移容器以只读方式挂载快照。快照全部文件SHA-256在迁移与HTTP验证后再次比对不变。

实际复用原工具：`admin -mode schema`，`migrate -mode preflight/import/verify`，不另写数据导入实现。目标八表结构经原ValidateSchema检查；七表均有测试数据，包含所有原ID、字段指纹、关联、正文与文件hash。

| 表 | 源数量 | 目标数量 | 原ID |
|---|---:|---:|---|
| classes | 3 | 3 | 1,2,3 |
| users | 5 | 5 | 1,2,3,4,5 |
| materials | 3 | 3 | `11111111-1111-4111-8111-111111111111`、`22222222-2222-4222-8222-222222222222`、`33333333-3333-4333-8333-333333333333` |
| knowledge_entries | 3 | 3 | 1,2,3 |
| assignments | 1 | 1 | 101 |
| assistants | 1 | 1 | 102 |
| skills | 1 | 1 | 103 |
| sessions | 1 | **0** | 源历史行仅留备份，不导入、不清理源 |

用户ID1–5的 `session_version` 为 **8、8、9、10、11**，目标逐项相等且最终再次验证，无重置/回退。密码哈希作为迁移字段保持一致，原密码通过目标登录验证；报告仅保存字段指纹，不打印密码/密码哈希原值。

| 材料 / 正文ID | 附件SHA-256（源=目标） | 正文SHA-256（源=目标） |
|---|---|---|
| A / 1 | `45e9c05859e66181cf36f512728bbe7645a2170399d7ecf623c92c25f4911c9f` | 同附件 |
| B / 2 | `317cf9c9f6d3939ecb118151003186e6174e8bdaff3660aaa98055323df2e083` | 同附件 |
| 中文BOM文件 / 3 | `b7740688df2fe6a2b08fdf8728f71345cb0baf5aae9f932115caba456df2eea9` | `9871245e762c99a8f287e193109ca88a8df38330f6e3e644a03c79fd5b76e385` |

第三项正文按原契约去除开头BOM，附件仍保留原字节，因此两个hash不同是预期。

源快照SHA-256：`3144d6d4e4cfb252853404aadbe22c43a75d8c6123a05d4fb1cfbdbb88f07ef5`。
七表/文件总体指纹：`4faade8ffaf3b08dfa5b3c1ab9279b981fb5b03f9d5c98e84a2726c1bd5fc79f`。
导入、verify、修复故障副本后reverified、代理HTTP验证后after-http均匹配原preflight指纹；preflight本身 `Verified=false`，不能直接授权切换。

## 故障注入与停止条件

| 故障 | 实测结果 | 切换状态 |
|---|---|---|
| 损坏独立fault-snapshot附件 | preflight退出1，`file size mismatch`，不生成成功报告 | Nginx保持503 |
| 在新库knowledge_entries插入前设置拒绝触发器 | import退出1，`insert failed in knowledge_entries`；七表和sessions均0行，tmp/files为空、无成功初始化凭据 | 正常Go启动拒绝未验证目标，Nginx保持503 |
| 成功导入后改坏新库一条正文 | verify退出1，`body content mismatch`，不生成验证成功报告 | 不切换；仅从隔离夹具恢复该正文并重新验证 |

触发器仅在本次新库创建并移除；未删除/重建数据库、卷或Demo文件。失败导入的pending证据不是成功状态。已有旧成功报告也不能替代本次新验证：必须重获与当前源hash、指纹匹配的verify结果，再检查源已停写及Go健康，才写入Nginx upstream配置。

实际执行 `nginx -t -c /rehearsal/nginx.conf` 成功后reload；入口由503变200。任何阶段异常都会结束演练并将隔离入口置回维护配置、停止本次服务，保留全部证据，不回退真实数据。

## Token/Cookie 与真实代理验证

- 源/目标使用独立随机JWT签名密钥。切换前曾在源端验证有效的Token在新入口返回401；目标数据库用户ID和version仍相同，所以没有通过重置账号版本伪造密钥切换效果。
- 历史SID样式Cookie及cookie中的access_token均401；目标sessions为空，认证不恢复Session/CSRF。源sessions行仅用于证明历史会话排除，并未启用旧Session登录体系。
- 五个账号重新登录成功，经Nginx携Authorization调用me返回可信user_id/role/class_id/class_name；匿名401。证明Header确实透传，而非仅查看Nginx文本配置。
- A/B两班教师/学生及空班账号的列表仅本班；详情正文和附件原始字节一致；中文Content-Disposition语义正确，保留nosniff及private,no-store。
- 跨班详情/附件404，缺失资源404；伪造role字段上传时教师400、学生403，无数据写入。页面壳、Logo可访问，私有上传路径和静态上传猜测URL均404。
- 停止Go后再执行offline verify，七表及文件仍与快照相等，证明读操作/拒绝请求未产生持久化副作用。

本轮没有追加新的目标材料或执行目标logout；迁移验证后保持目标数据不变以便最终全字段对照。完整业务写入与logout语义已在6.1及Phase3/4验收，本轮不冒称重新执行所有110项或Phase6全矩阵。

## 证据、回退限制与工作区保护

完整结果：`runtime/campusclaw-phase5-cutover-20ce794d/result.json`，SHA-256
`6fbda8797fd00b3b5dbb33ac5e9b9dedbf2ff0a5219ca0e6e1dca136b039cc99`。
该目录还保留Compose文件、维护Nginx配置、源库/快照/上传副本、故障副本、所有迁移报告与HTTP观测。report不含真实密码、JWT密钥或完整Token。保留的测试容器环境含生成的演练凭据，不能将docker inspect Env导出到公开记录。

14个本次容器均已退出，镜像、容器、网络、卷与备份均保留；未运行down/rm/volume删除或自动清理。没有两个运行中的上传卷写入者。失败构建的独立runtime同样保留。

回退限制：本演练只验证停写快照到新目标及失败不切换，**没有执行真实回退或宣称新写入后能直接切回旧快照**。若目标已有新材料、账号变更或logout版本递增，先停止写入并协调数据与版本，防止数据丢失或撤销Token复活；切换和回退都保持Bearer-only，换签名密钥并要求重新登录。保留旧快照不代表它仍是可安全恢复的最新状态。正式TLS/入口配置、生产备份与回退操作文档分别留在6.3/6.4及后续验收，不在本轮实现。

现有Flask RAG Demo的 `/health` 和 `/login` 在演练前后均为 **200**，health正文正确、登录页含CampusClaw；仅只读GET，无Demo登录、上传、RAG请求或数据库操作。这证明两个要求的端点保持正常，不等于重新验收整个RAG流程。

本轮新增 `tests/cutover/{Dockerfile.backend,gate.py,test_gate.py,run.py,README.md}`、本记录，并仅更新该change的tasks完成状态。正式Compose/Nginx、Flask/RAG与Go业务、课程报告、其他change、原有迁移工具和历史验收文件保持原样。未commit、push或archive；**完成6.2后停止，不推进后续任务**。
