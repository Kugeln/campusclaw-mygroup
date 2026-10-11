# Phase 5 / 6.1 共享 HTTP 契约差异复核

最新完整验收结果见第 10 节：用户确认五项边界后完整110项通过，6.1已验收，总进度24/30。第1–9节保留历史报告、修复与裁定过程；其中待确认/未验收状态已被第10节取代，不回写历史证据。

复核日期：2026-10-10。本轮仅阅读现有证据并补充本文，不重新执行完整套件、不修改业务代码、测试或规约。**6.1 未验收，保持未勾选，总进度 23/30。**

## 1. 报告来源与数量校正

用户提供的 `runtime/campusclaw-phase5-contract-dedb1c85/report.json` 是较早的 **102 项**报告，并非 110 项报告。两份 JSON 均可完整解析，`complete=true`，逐项计数与汇总相符，不存在为获取证据而重跑的必要。额度中断不等于测试崩溃；套件退出码 1 表示差异未解决。

| 报告目录后缀 | 用例数/端 | Flask 通过/失败/跳过 | Go 通过/失败/跳过 | 一致/已接受/待确认/不兼容 |
|---|---:|---|---|---|
| `dedb1c85` | 102 | 99/3/0 | 90/12/0 | 67/1/31/3 |
| **`8bcdb72b`** | **110** | **109/1/0** | **98/12/0** | **77/1/31/1** |

以后一份为当前验收证据：`runtime/campusclaw-phase5-contract-8bcdb72b/report.json`。110 项包括 106 项共享 HTTP 用例与 4 项启动配置拒绝检查，并非 110 个独立缺陷或 HTTP 请求。每个用例可能发送多次请求。

SHA-256：

- `dedb1c85/report.json`：`26ddf2b10985af481b76815880f6a5740adb1573b5cc890ac490a941a53f3fa0`
- `8bcdb72b/report.json`：`72c6412b3031439edd1fbc297220071f9b3d711c1fc2763667344a0d5babdfd7`

既有 [结果记录](phase5-contract-results.md)、[逐项比较 JSON](phase5-contract-comparison.json)、[测试说明](../../tests/http_contract/README.md) 均保留。本文补充按根因和规约强制性复核，不回写或改造历史报告分类。

## 2. 根因去重与规约边界

当前 **31 pending + 1 incompatible = 32 个未解决比较用例，归为 6 个根因组**。失败数、差异数和根因数不可相加。下列“强制”判断依据现有文件，不新增验收范围。

| 组 | 用例数 | 根因与实际行为 | 规约判断及处理 |
|---|---:|---|---|
| R1 | 1 | 截断 multipart 的读取错误被 Go 归入存储故障：Flask 400 JSON，Go 500 JSON | **明确强制不兼容**。spec 的 Material API and file compatibility 要求格式/输入错误 MUST 400。需修复错误分类，不能豁免 500。 |
| R2 | 1 | Flask 检查解析后的 `request.args`，Go 检查非空 `RawQuery`；`?&` 分别 200/400 | 普通非空查询参数 400 为强制且已通过。空分隔符是否算“任何查询参数”存在 design“任意 query”与实际解析语义的边界，需明确，不将 Flask 判成权限漏洞。 |
| R3 | 3 | Flask 先解析全部 form/files，再查额外字段/数量；Go 边读取第一文件边验证，尚未见后续字段即返回 | **其中两项 JSON→HTML 已触及强制兼容**：design §4 明确“错误 JSON 保持现有明确字符串”。第三项超限+身份字段 400/413 涉及两个强制规则的优先级，应确认顺序，不擅自选一个。 |
| R4 | 1 | >1 MiB 非文件 role 字段触发 Flask/Werkzeug form 限额 413；Go 识别身份字段后直接 400 | 不等于单文件 20 MiB 限额问题。spec 未单列非文件 form 大小阈值与优先级；属需确认的框架解析限额差异，不能自动豁免状态差异。 |
| R5 | 20 | Flask 自动提供 OPTIONS；Go 只注册明确方法。15 个 405 的 Allow 集合不同，5 个 OPTIONS 为 200/405 | “未支持修改方法 405”为强制，15 项状态已通过。spec/design 未明确要求 OPTIONS 自动响应、Allow 包含 OPTIONS；属于本轮要求观察但尚无明确迁移门槛的框架差异。**需决定是否对齐，当前不自动接受**。 |
| R6 | 6 | Go 的附件和嵌入静态文件用 `ServeContent(..., time.Time{}, ...)`，未设 ETag；Flask send_file 提供验证器 | 2 项验证器差异引出 4 项 304/200，不是 6 个独立缺陷。spec 强制原字节/MIME/安全头/no-store，未明列 ETag、Last-Modified、304。需确认是否保留这些 Flask 能力，不能从 no-store 推导可免除条件请求。 |

R3 两项指 `upload.bad_extension_then_identity` 和 `upload.invalid_text_then_second_file`。报告原始分类仍为 pending，本轮人工复核将它们标识为**现有 JSON 契约需对齐的差异**，不是新增两个根因；“框架400/413/405不要求 HTML 字节一致”只豁免框架错误页的字节一致性，不豁免明确业务 JSON 消失。关于多重错误中应选择哪条错误，后续处理必须与现有 Flask 基准及 design 一起确认。

可接受/已解决项另计：

- **已确认差异**：下载 Flask `no-store` / Go `private, no-store`，共 1 项；后者符合 migration spec。
- **表示层允许差异**：框架自动 HTML 错误页不逐字比较；附件 Content-Disposition 按解码文件名与 attachment 语义比较。该豁免不涵盖 400→500、400→413、JSON→HTML 或 Allow/条件请求行为。
- **测试解析器问题，已在上一轮修复**：早期 Flask A 班教师/学生 read-isolation 两项失败均源于测试选择 ASCII `filename`，未优先采用 `filename*`；不是两个班级隔离缺陷，也不是种子数据损坏。当前解析器及回归测试已存在，最终报告四个角色/班级组合均通过。本轮未重修或重跑。
- 405 的框架错误表示还表现为 Flask HTML / Go plain text；其表示层是否额外约束没有明文，不把它再重复计算为独立业务缺陷。Allow 差异仍单独可见、未豁免。

## 3. 截断 multipart：优先处理的明确不兼容

已有复现用例：[cases.py](../../tests/http_contract/cases.py) 的 `multipart.truncated`。教师携有效 Bearer，发送 `multipart/form-data; boundary=contract-boundary`，包含一个 `file` / `a.txt` part、正文 `partial`，**不发送 multipart 结束边界**。请求本身有完整 Content-Length，不是测试网络中断。

已有观测：

```text
Flask: 400 application/json {"error":"Exactly one file is required"}
Go:    500 application/json {"error":"Material storage unavailable"}
```

代码路径核对（推断与观测分开）：

1. [Go UploadHandler](../../internal/http/upload.go) 的首个 `NextRawPart` 能读到 part，随后调用 `Store.Prepare(name, part, limit)`。
2. [PrivateStore.Prepare](../../internal/storage/upload.go) 的 `io.CopyBuffer` 同时从 multipart reader 读入、向临时文件写入；除超限外，任何复制错误都被转换为 `ErrUploadStorage`，丢失“客户端 multipart 读取异常”和“磁盘写入异常”的区别。
3. handler 的 Prepare 错误分支只将 ErrLimit→413、ErrText/ErrFileType→400，其余→500。这也解释为何后面的 `NextRawPart` / `Invalid multipart body` 400 分支未处理本次异常。
4. 因此代码与 500 观测一致，最可能原始 reader 错误是缺失终止边界导致的 unexpected EOF。**报告未保存底层 Go error 值，本轮未执行新诊断，不将该具体 error 值声称为实测。**
5. Flask [material_upload](../../app/routes.py) 在访问 form/files 后检查文件数量，现有证据显示其未得到完整文件并返回上述 400。

该失败发生在 `Uploader.Save` 之前，不是 MySQL COMMIT 未知结果；不应调整事务恢复策略来解决。用例的 `no_effect` 断言核对了两张材料表及 tmp/files 字节摘要，报告失败原因是状态不符、不是副作用不符；已有证据支持未遗留材料写入。未来修复应保留真实磁盘错误 500，不能把所有 `ErrUploadStorage` 一律改为 400；需最小针对性测试区分客户端读异常、真实存储写异常及临时文件清理。本轮不实施修复。

## 4. 命令、复现与环境

上一轮实际执行并保留证据的命令：

```powershell
.\.venv\Scripts\python.exe -m unittest discover -s tests/http_contract -p test_harness.py -v
go test -mod=readonly ./tests/http_contract/fixture
.\.venv\Scripts\python.exe tests/http_contract/run.py
openspec validate migrate-backend-to-go-mysql --strict
```

结果分别为：3 个 harness 单元测试通过；Go fixture 包编译成功（`no test files`，不可计成集成测试通过）；共享套件 110 项/端、退出 1；strict validate 通过。真实 MySQL 运行证据来自共享 runner，不能用仅编译夹具替代。

后续获得实施指令后，可用同一 runner 复现上述请求；它没有按 case 过滤的 CLI，本轮不新增过滤器、不假称已执行单项复测。每组的准确输入由下方完整用例索引和 `cases.py` 定义：R2 为 `?&`；R3 按用例名顺序放置两种错误；R4 的 role 字段为 1 MiB+1；R5 对五个现有路由发指定方法；R6 先 GET，再用响应验证器发条件 GET。不要对正式 8080 Demo 发送这些变更请求。

隔离 runner 每次新建随机名称 MySQL 8.4.4 容器/网络/卷、空库、SQLite 与独立上传目录；Flask 明确 `MATERIAL_BACKEND=sqlite`，两个 HTTP 服务使用不同随机回环端口和独立 JWT 密钥。测试账号/材料来自同一 fixtures.json，不访问 RAG、Qdrant、三个 Demo 卷。运行结束停止自建服务但保留资源。原报告所列端口是历史端口，不能视为现在仍存活的目标。

## 5. 验收结论与保护范围

**不能勾选 6.1，也不能以本轮记录作为进入 6.2 的验收依据。** R1 违反明确 400 要求；R3 的明确 JSON 兼容差异尚未解决；R2/R4/R5/R6 以及多重错误优先级仍需确认。未明确要求的框架行为不自动扩大为强制实现，也不因来源是框架就自动接受。

本轮只新增本文：不修改此前测试/报告/文档、不改 tasks/design/spec、Flask/Go 业务、课程报告、Compose 或 Nginx；不运行完整套件或启动测试服务，不操作数据库/卷，不提交、推送或归档。上一轮 Demo `/health`、`/login` 均为 200；**本轮没有重新探测，不把历史结果描述为本轮实时健康结论**。

下一轮建议先确认 R3–R6 的必要对齐范围，再授权最小业务修复及对应回归测试。R1 错误分类修复范围已经明确，但仍留待用户下一轮指令。

## 6. 完整失败与待确认用例索引

以下从最终 110 项报告逐项提取，保留原报告分类；根因判断以第 2 节为准。通过但存在差异的用例也列出，不把它们算作跳过。

| 用例 ID | Flask | Go | 原报告分类 |
|---|---|---|---|
| `query.&` | fail / HTTP 200 | pass / HTTP 400 | pending |
| `upload.bad_extension_then_identity` | pass / HTTP 400 | pass / HTTP 400 | pending |
| `upload.invalid_text_then_second_file` | pass / HTTP 400 | pass / HTTP 400 | pending |
| `upload.oversize_then_identity` | pass / HTTP 400 | fail / HTTP 413 | pending |
| `multipart.truncated` | pass / HTTP 400 | fail / HTTP 500 | incompatible |
| `multipart.large_form_field` | pass / HTTP 413 | fail / HTTP 400 | pending |
| `method.PUT./api/materials` | pass / HTTP 405 | pass / HTTP 405 | pending |
| `method.PUT./api/materials/11111111-1111-4111-8111-111111111111` | pass / HTTP 405 | pass / HTTP 405 | pending |
| `method.PUT./api/materials/11111111-1111-4111-8111-111111111111/file` | pass / HTTP 405 | pass / HTTP 405 | pending |
| `method.PUT./api/auth/login` | pass / HTTP 405 | pass / HTTP 405 | pending |
| `method.PUT./login` | pass / HTTP 405 | pass / HTTP 405 | pending |
| `method.PATCH./api/materials` | pass / HTTP 405 | pass / HTTP 405 | pending |
| `method.PATCH./api/materials/11111111-1111-4111-8111-111111111111` | pass / HTTP 405 | pass / HTTP 405 | pending |
| `method.PATCH./api/materials/11111111-1111-4111-8111-111111111111/file` | pass / HTTP 405 | pass / HTTP 405 | pending |
| `method.PATCH./api/auth/login` | pass / HTTP 405 | pass / HTTP 405 | pending |
| `method.PATCH./login` | pass / HTTP 405 | pass / HTTP 405 | pending |
| `method.DELETE./api/materials` | pass / HTTP 405 | pass / HTTP 405 | pending |
| `method.DELETE./api/materials/11111111-1111-4111-8111-111111111111` | pass / HTTP 405 | pass / HTTP 405 | pending |
| `method.DELETE./api/materials/11111111-1111-4111-8111-111111111111/file` | pass / HTTP 405 | pass / HTTP 405 | pending |
| `method.DELETE./api/auth/login` | pass / HTTP 405 | pass / HTTP 405 | pending |
| `method.DELETE./login` | pass / HTTP 405 | pass / HTTP 405 | pending |
| `method.OPTIONS./api/materials` | pass / HTTP 200 | fail / HTTP 405 | pending |
| `method.OPTIONS./api/materials/11111111-1111-4111-8111-111111111111` | pass / HTTP 200 | fail / HTTP 405 | pending |
| `method.OPTIONS./api/materials/11111111-1111-4111-8111-111111111111/file` | pass / HTTP 200 | fail / HTTP 405 | pending |
| `method.OPTIONS./api/auth/login` | pass / HTTP 200 | fail / HTTP 405 | pending |
| `method.OPTIONS./login` | pass / HTTP 200 | fail / HTTP 405 | pending |
| `validator.headers./api/materials/33333333-3333-4333-8333-333333333333/file` | pass / HTTP 200 | pass / HTTP 200 | pending |
| `validator.If-None-Match./api/materials/33333333-3333-4333-8333-333333333333/file` | pass / HTTP 304 | fail / HTTP 200 | pending |
| `validator.If-Modified-Since./api/materials/33333333-3333-4333-8333-333333333333/file` | pass / HTTP 304 | fail / HTTP 200 | pending |
| `validator.headers./static/logo.png` | pass / HTTP 200 | pass / HTTP 200 | pending |
| `validator.If-None-Match./static/logo.png` | pass / HTTP 304 | fail / HTTP 200 | pending |
| `validator.If-Modified-Since./static/logo.png` | pass / HTTP 304 | fail / HTTP 200 | pending |

共 32 行：31 pending、1 incompatible；Flask fail 1 行，Go fail 12 行。早期报告额外的两个 Flask fail 为 `material.read_isolation.teacher_a`、`material.read_isolation.student_a1`，均为已修复的同一测试解析器问题。

本轮收尾检查：`openspec validate migrate-backend-to-go-mysql --strict` 通过。对开始时 225 个已有工作区文件的 SHA-256 清单复核，无已有文件变化；仅新增本文。

## 7. 截断 multipart 局部修复与回归（2026-10-10）

仅修改 Go 上传输入错误分类及必要测试。HTTP 层将 multipart reader 的 unexpected EOF 标记为 `storage.ErrUploadInput`；存储层仅保留这个明确来源的输入标记，其他复制/文件异常仍归入存储错误。handler 将该输入标记返回既有 JSON 400，不把全部存储错误改成 400。原单文件格式/文本错误的 HTML 400、超限 413 规则保留。

| 用例 | 旧 Go | 本次 Go | Flask 基准 | 结果 |
|---|---|---|---|---|
| `multipart.truncated` | 500 application/json，`{"error":"Material storage unavailable"}` | 400 application/json，`{"error":"Exactly one file is required"}` | 400 application/json，同一 error 字符串 | 两端一致，已修复 |
| `upload.bad_extension_then_identity` | 400 text/html | 本轮暂未修改 | 400 application/json，`{"error":"Client identity fields are not accepted"}` | 待明确两种组合的优先级授权 |
| `upload.invalid_text_then_second_file` | 400 text/html | 本轮暂未修改 | 400 application/json，`{"error":"Exactly one file is required"}` | 待明确两种组合的优先级授权 |

后两项并非只改 Content-Type 就能兼容：Go 在第一文件错误处已退出，尚未读取后续身份字段或第二文件。选择 Flask 原 JSON 字符串需要调整这两种组合的检查顺序。用户本轮同时要求“不擅自修改多重错误优先级”，因此已提出范围澄清，不凭默认选项或沉默视为授权；尚未修改这两项，更未替换为新的 JSON 错误字符串来声称通过。

实际执行顺序与结果：

1. `go test -mod=readonly ./internal/http ./internal/storage`：通过。新增回归检查带正文/空正文截断均为 JSON 400、关闭存储根仍为 JSON 500、错误不写材料且不留 tmp/files；存储单测验证明确输入标记与未分类 I/O 错误不混淆。
2. `.\.venv\Scripts\python.exe -m unittest discover -s tests/http_contract -p test_harness.py -v`：4 项通过。新增精确用例选择测试，不更改共享用例断言。
3. 下列共享用例使用新建独立 MySQL 8.4.4 与 Flask/SQLite，通过真实 HTTP 执行，**两端均 10 通过 / 0 失败 / 0 跳过，10 项一致**：

```powershell
.\.venv\Scripts\python.exe tests/http_contract/run.py --case multipart.truncated --case multipart.missing_boundary --case upload.unsupported --case upload.invalid_utf8 --case upload.nul --case upload.empty --case upload.over_limit --case upload.missing_file --case upload.success.txt --case upload.success.quoted_boundary
```

报告：`runtime/campusclaw-phase5-contract-e544ab9c/report.json`。此报告明确 `full_suite=false`、`not_executed_count=100`、`selection_accepted=true`、`accepted=false`；退出码 0 只表示局部选择通过。旧 110 项结果保持原样，**不外推新的全套通过数量**。

4. `openspec validate migrate-backend-to-go-mysql --strict`：通过。

新增 `--case` 是原 runner 的精确选择参数，同一原始用例函数仍在两个独立服务执行；未知 ID 在创建资源前拒绝。未放宽断言、归一化或跳过失败。测试用自建 `campusclaw-phase5-contract-e544ab9c` 资源，结束仅停止自身服务并保留容器/网络/卷、SQLite 与上传目录，不触碰 Demo 卷。

剩余待确认比较用例仍为历史报告中的 31 项，包括上述两个 JSON 差异；OPTIONS/Allow、条件请求、form 限额及其他多重错误优先级未修改。**6.1 未勾选，总进度仍为 23/30；未进入 6.2–6.4。**

## 8. 已授权的两种 JSON 组合修复

用户明确确认方案 1：仅允许两种已确认组合对齐 Flask 的检查顺序。第 7 节“待授权”状态已被此次确认取代；不扩展到其他多重错误、OPTIONS、条件请求或表单限额。

| 共享用例 | 修复前 Go | 修复后 Go 与 Flask（实测一致） |
|---|---|---|
| `upload.bad_extension_then_identity` | 400 `text/html`，文件格式错误页 | 400 `application/json`，`{"error":"Client identity fields are not accepted"}` |
| `upload.invalid_text_then_second_file` | 400 `text/html`，文本错误页 | 400 `application/json`，`{"error":"Exactly one file is required"}` |

实现范围：`internal/http/upload.go` 在非法扩展名/非法 UTF-8 后执行有界的后续 part 检查，只在匹配两种组合且完整结束、没有第三个 part 时使用上述既有 JSON。其他组合或无法确认完整性时保留原错误。`internal/storage/upload.go` 区分非法 UTF-8 与空文本/NUL，仍通过 `errors.Is(..., ErrText)` 保持原文本错误语义及原错误字符串，单独发生时继续 HTML 400。

后续检查使用流式丢弃、现有文件限额及请求上限，不将整个请求读入内存。对身份字段的大输入不应用新增优先级：采用本地 Flask 默认 `MAX_FORM_MEMORY_SIZE=500000` 作为“停止此次特殊处理”的边界，超过则保留原格式错误，**不是新增全局 form 限额或调整既有大表单响应**。原超限分支不进入这段检查，继续原 413；此前截断输入标记和 400 分支保留。

针对性测试：在 `internal/http/upload_input_test.go` 新增 11 个子用例，明确断言两种组合的状态、Content-Type、唯一 error 字段及原字符串，并验证单独格式/编码错误、空/NUL 加第二文件、非法扩展名加第二文件、非法编码加身份字段、超限加身份字段、大身份字段、第三个 part 等不会套用新的 JSON 规则。所有拒绝检查 no-store、无材料写入、无 tmp/files 残留。

实际执行：

```powershell
go test -mod=readonly ./internal/http ./internal/storage
.\.venv\Scripts\python.exe tests/http_contract/run.py --case upload.bad_extension_then_identity --case upload.invalid_text_then_second_file --case multipart.truncated --case upload.unsupported --case upload.invalid_utf8 --case upload.empty --case upload.nul --case upload.over_limit --case upload.two_files --case upload.success.txt
openspec validate migrate-backend-to-go-mysql --strict
```

- Go HTTP / storage 包测试通过，含此前截断及真实存储失败分类回归。
- 共享 HTTP 使用新建隔离 Flask/SQLite 与 MySQL 8.4.4，**Flask 10/0/0、Go 10/0/0（通过/失败/跳过），10 项一致**；没有修改任何共享断言或响应归一化。
- 报告：`runtime/campusclaw-phase5-contract-8ee6bf26/report.json`。`full_suite=false`、100 项未执行、`selection_accepted=true`、全套 `accepted=false`。未重跑 110 项、未改写旧报告，也不推算新的全套通过数。
- OpenSpec strict validate 通过。
- 隔离服务已停止，自建容器/网络/卷及 runtime 文件保留；未操作 Demo 卷。Flask 业务、课程报告、默认 Compose、正式入口、tasks/spec/design 保持本轮前状态。

基于历史报告并扣除局部已验证修复，原截断不兼容和两项 JSON 差异均已解决；**仍有 29 个历史待确认比较用例**：`?&` 1 项、超限+身份字段 1 项、大表单 1 项、OPTIONS/Allow 20 项、验证器/条件请求 6 项。该数量是问题跟踪状态，不是本轮完整套件的新统计。**6.1 未勾选，仍为 23/30；未进入 6.2。**

## 9. 剩余 29 项的兼容边界分类（只读分析）

本节依据当前 spec/design/tasks、Flask/Go 路由代码、本地锁定 Flask/Werkzeug 源码、原 `8bcdb72b` 报告及两个局部修复报告。没有启动服务、重跑 110 项、修改测试或业务代码。原始 JSON 的 pending 分类不回写；本节是人工裁定与待确认建议。

### 9.1 去重与分类总数

原始 32 个未解决比较用例减去已实测修复的截断 multipart 和两个 JSON 组合，剩余 **29 项、5 个根因组**。不再把已解决的三个用例或早期 filename* 解析器问题算入。

| 分类 | 用例数 | 裁定 |
|---|---:|---|
| M：必须修复的明确契约不兼容 | 0 | 剩余项目尚无不依赖边界选择、可直接要求 Go 修改的独立根因。不是宣称所有剩余行为已验收。 |
| A：可以接受的框架差异 | 15 | PUT/PATCH/DELETE 的 405 已满足强制要求；Allow 如实列出该后端支持方法，差异仅 OPTIONS。接受其“无需精确同集”的结论，不等于批准 OPTIONS 200/405 的差异。 |
| T：测试工具或夹具错误 | 0 | 数据和观测有效；部分断言预设了尚未批准的兼容目标，属 D，不应删除用例或称为误报。 |
| D：规约未明确、待用户确认 | 14 | OPTIONS 5、验证器/条件请求 6、空查询分隔符 1、超限+身份 1、大表单 1。 |

**必须修复的新增根因数量：0；待确认边界：5 个。** 对 A 类的判断限于已观测的 405 与 Allow，不扩展到响应中的所有头，也不自动将原 suite 的比较结果改成通过。若决定提供 OPTIONS，Allow 必须同步与实现一致；不能为消除差异仅把 OPTIONS 字符串加到 Allow。

### 9.2 逐组依据、建议与影响

**G1：空查询分隔符（1 项，D）。** `query.&`：Flask 200 scoped JSON / Go 400 JSON。Flask `request.args` 为空，Go `RawQuery` 非空。spec 的 Bidirectional class isolation 规定“列表附加任何查询参数返回400”，design §4 写“任意 query400”；两者没有定义没有键值的 `?&`。这不是跨班数据泄露，正常 role/class_id/uploader_id/q 参数均拒绝。建议保留 Go 的严格 400，并明确裸分隔符不要求两端等价；若目标是 Flask URL 解析完全兼容，则需另授权只对空解析结果放行。前者无需业务修改，但需要明确允许的状态差异，不能把两端 200/400 归一化为同一结果。

**G2：超限文件后附身份字段（1 项，D）。** `upload.oversize_then_identity`：Flask 400 JSON identity 错误 / Go 413 HTML。spec 同时要求身份字段400（Teacher forged ownership）和文件超限413（Material API and file compatibility），未规定两者同时存在时的先后。Flask 先解析 form；Go 在第一文件超限时停止。此前获准的两种组合没有授权第三种组合。建议保留流式超限优先413、早停和无写入，再明确该组合例外；若选择严格沿用 Flask 400 JSON，需要设计有界后续检查，不能为找到身份字段无限读取超限正文。无论选择哪种，都必须保留学生403优先、单独超限413、单独身份字段400与无持久化副作用。

**G3：大非文件表单字段（1 项，D）。** `multipart.large_form_field`：role 字段为1 MiB+1，后有正常文件；Flask 413 HTML / Go 400 JSON identity 错误。锁定 Flask 默认 `MAX_FORM_MEMORY_SIZE=500000`，Werkzeug 在 form 解析时检查字段/缓冲限额，先于业务拒绝。这个限制不同于20 MiB文件限额，也不能把500000字节说成1 MiB。Go 读到字段头即拒绝，不解析或持久化字段正文；未因返回400放行大字段。spec 没要求复制该非文件字段阈值。建议允许 Go 提前400，保留总请求有界和禁止身份字段；若要精确413，应另行定义非文件字段阈值、请求 framing 开销、解析缓冲与业务检查顺序，不能顺带缩小合法20 MiB文件上传能力。第8节的500000保护条件仅限制特殊组合检查，不是已经实现了全局Flask表单限额。

**G4：路由方法能力（20 项，其中 A=15、D=5）。** spec Bidirectional class isolation 和 task5.7 明确不支持修改方法405；15项都已满足，且无数据副作用。两端 Allow 的 GET/HEAD/POST 集合一致，Go 没宣称支持实际不支持的 OPTIONS；集合顺序本来就已排序比较，不是排序噪音。design §4 不要求框架405错误页字节一致。故15项无需仅为 Allow 多一个 OPTIONS 而修改业务。

另外5个 OPTIONS 是独立能力边界：Flask 自动200、Go405。spec 的路径表没有要求 OPTIONS；同源 Bearer 也不能被解释成必须开放跨域 CORS。建议保持 Go405及真实Allow作为明确框架差异。若有客户端方法发现需求，也可选择仅实现同源OPTIONS200；这是新增兼容能力，需确认后再实施，不能自动增加 Access-Control-Allow-Origin 或绕过受保护 GET/POST。接受15个Allow差异不预先替用户选择这5个状态差异。

**G5：验证器与条件响应（6 项，D）。** 附件与logo各1项头比较、各2项条件GET，共6项同源问题：Flask send_file 生成 ETag/Last-Modified，Go ServeContent 使用零修改时间且未设ETag。原文/附件字节与安全头属于spec明确要求，现有验证已通过；spec未列ETag、Last-Modified或304为强制能力。`private, no-store` 也不等于禁止接收条件请求，不能用已接受的缓存头差异豁免304语义。

测试有一个需明确的前提：当Go没有返回ETag时，If-None-Match用例会发 `"absent-validator"`；没有Last-Modified时发送夹具时间。Go200并不证明“忽略了自己宣告的匹配验证器”，证明的是当前缺少Flask的验证器能力。Flask侧用自己的真实验证器，因此其304是有效观测；两端原始ETag无需字面相等。这里是验收目标尚未批准，不是夹具损坏。

建议本迁移范围暂不要求复制验证器/304，允许Go200完整内容；代价是重复传输附件及静态资源、带宽效率较低。若要求条件下载兼容，需明确附件与静态资源是否都纳入、稳定ETag来源、修改时间精度和重启行为；优先保证认证/班级校验发生在任何304判断之前，错误/跨班/撤销Token不得经304绕过权限。不能伪造Last-Modified或固定ETag凑过测试。若批准“不要求验证器”，后续仍保留原始头及状态观测，并针对宣告验证器的实现检验真实匹配语义，不盲目把所有200/304视作同义。

### 9.3 五个需要用户确认的选择

1. `?&`：保留Go400并允许Flask200，还是要求Go按Flask解析语义返回200？建议前者。
2. 超限+身份：允许Go413优先，还是扩展授权对齐Flask400 JSON？建议前者，保留流式早停。
3. 大role字段：允许Go提前400，还是复制Flask表单阈值与413顺序？建议前者，不新增全局表单限制。
4. OPTIONS：允许Flask200/Go405并各自如实Allow，还是新增Go OPTIONS200？建议前者，同源业务没有已证实的依赖。
5. 验证器/条件GET：允许Go不提供验证器并返回完整200，还是要求附件和/或静态资源提供ETag/Last-Modified及304？建议本轮不扩大能力，差异必须显式记录。

这些建议都**尚未成为已接受差异**，没有修改spec/design或测试期望；需用户确认后再落地对应边界。对15项A类也应在后续测试比较政策中保留方法集合、状态、无副作用检查，只显式处理被确认的OPTIONS能力差异，不能笼统忽略Allow。

### 9.4 是否具备完整回归条件

隔离runner在技术上可运行，但**用于验收的完整回归条件尚未齐备**：上述5项选择未定，当前 suite 将仍按其旧预设报错/pending。先确认边界，再按明确决定更新必要比较政策/实现与针对性回归，最后运行完整110项。不得简单修改所有预期为Go现状，也不得用本轮分类冒充新的通过数量。任务6.1仍未勾选，总进度23/30；未进入6.2。

### 9.5 剩余29项逐项索引

以下每一行都源自原110项报告中pending且未被局部修复覆盖的用例。分类 A/D 和根因 G1–G5 见上文；没有隐藏失败项。

| # | Case ID | Group | Class | Flask / Go HTTP |
|---:|---|---|---|---|
| 1 | `query.&` | G1 | D | 200 / 400 |
| 2 | `upload.oversize_then_identity` | G2 | D | 400 / 413 |
| 3 | `multipart.large_form_field` | G3 | D | 413 / 400 |
| 4 | `method.PUT./api/materials` | G4 | A | 405 / 405 |
| 5 | `method.PUT./api/materials/11111111-1111-4111-8111-111111111111` | G4 | A | 405 / 405 |
| 6 | `method.PUT./api/materials/11111111-1111-4111-8111-111111111111/file` | G4 | A | 405 / 405 |
| 7 | `method.PUT./api/auth/login` | G4 | A | 405 / 405 |
| 8 | `method.PUT./login` | G4 | A | 405 / 405 |
| 9 | `method.PATCH./api/materials` | G4 | A | 405 / 405 |
| 10 | `method.PATCH./api/materials/11111111-1111-4111-8111-111111111111` | G4 | A | 405 / 405 |
| 11 | `method.PATCH./api/materials/11111111-1111-4111-8111-111111111111/file` | G4 | A | 405 / 405 |
| 12 | `method.PATCH./api/auth/login` | G4 | A | 405 / 405 |
| 13 | `method.PATCH./login` | G4 | A | 405 / 405 |
| 14 | `method.DELETE./api/materials` | G4 | A | 405 / 405 |
| 15 | `method.DELETE./api/materials/11111111-1111-4111-8111-111111111111` | G4 | A | 405 / 405 |
| 16 | `method.DELETE./api/materials/11111111-1111-4111-8111-111111111111/file` | G4 | A | 405 / 405 |
| 17 | `method.DELETE./api/auth/login` | G4 | A | 405 / 405 |
| 18 | `method.DELETE./login` | G4 | A | 405 / 405 |
| 19 | `method.OPTIONS./api/materials` | G4 | D | 200 / 405 |
| 20 | `method.OPTIONS./api/materials/11111111-1111-4111-8111-111111111111` | G4 | D | 200 / 405 |
| 21 | `method.OPTIONS./api/materials/11111111-1111-4111-8111-111111111111/file` | G4 | D | 200 / 405 |
| 22 | `method.OPTIONS./api/auth/login` | G4 | D | 200 / 405 |
| 23 | `method.OPTIONS./login` | G4 | D | 200 / 405 |
| 24 | `validator.headers./api/materials/33333333-3333-4333-8333-333333333333/file` | G5 | D | 200 / 200 |
| 25 | `validator.If-None-Match./api/materials/33333333-3333-4333-8333-333333333333/file` | G5 | D | 304 / 200 |
| 26 | `validator.If-Modified-Since./api/materials/33333333-3333-4333-8333-333333333333/file` | G5 | D | 304 / 200 |
| 27 | `validator.headers./static/logo.png` | G5 | D | 200 / 200 |
| 28 | `validator.If-None-Match./static/logo.png` | G5 | D | 304 / 200 |
| 29 | `validator.If-Modified-Since./static/logo.png` | G5 | D | 304 / 200 |

## 10. 用户确认五项边界后的完整110项验收

用户明确选择第9节五个问题的建议前者，授权仅用于 `migrate-backend-to-go-mysql` Phase5基础认证/材料HTTP验收，不适用于RAG接口。以下为已确认决定，不再是待确认建议：

| 边界 | Flask 保留行为 | Go 验收行为 | 理由与保留约束 |
|---|---|---|---|
| `?&` | 200，本班原字段列表 | 400，`Client scope parameters are not accepted` JSON | 允许严格拒绝非空原始query；Flask仍核对完整本班列表，不能接受外班内容。 |
| 超限＋身份字段 | 400 identity JSON | 413 HTML | 保留流式超限早停；单独超限/身份错误及学生权限仍按原契约，不产生写入。 |
| 大表单字段 | 413 HTML | 400 identity JSON | 允许读到身份字段后提前拒绝，不复制框架表单限额；不是允许大输入写入。 |
| OPTIONS / Allow | OPTIONS200，Allow包含OPTIONS | OPTIONS405，Allow仅含Go实际注册的方法 | 同源材料业务不新增自动OPTIONS能力；PUT/PATCH/DELETE仍必须405，不能伪造Allow或新增CORS。 |
| 验证器 / 条件GET | ETag/Last-Modified及匹配请求304 | 无验证器、完整200；附件`private, no-store` | 不迁移非必要缓存验证能力；仍核对完整原始附件/静态字节、附件文件名、nosniff和权限。 |

允许规则位于 [accepted_differences.py](../../tests/http_contract/accepted_differences.py)，由3个具体输入用例、5条具体路由×4个具体方法、2条具体资源路径×3种验证器检查组成，**精确覆盖29个用例ID，无通配符、无RAG路径**。每端先通过各自明确的状态/媒体/JSON/Allow/字节断言，比较器才允许记录 `accepted`；任一断言失败不能被规则豁免。此前修复的三项不在注册表中，必须按正常相等比较通过。

每个接受项在新报告中记录 `accepted_rule`，HTTP状态、媒体类型、JSON、有关响应头和请求路径继续保存在HTTP观测中；只按既有规则隐藏Token、忽略框架HTML错误页逐字节差异。没有把400/413、200/304或JSON/HTML改写成相同结果。条件GET的Go200除了与初次响应相同，还要求附件与夹具原字节一致、解码文件名正确、no-store/nosniff安全头不变。所有普通权限、JWT、撤销、上传与班级隔离检查保持。

实际命令：

```powershell
.\.venv\Scripts\python.exe -m unittest discover -s tests/http_contract -p test_harness.py -v
.\.venv\Scripts\python.exe tests/http_contract/run.py
openspec validate migrate-backend-to-go-mysql --strict
```

规则工具单测 **6项通过**，包括精确规则清单、不包含已修复错误或RAG、错误JSON字段/媒体类型不能获豁免。完整共享测试结果：

| Backend | Passed | Failed | Skipped | Total |
|---|---:|---:|---:|---:|
| Flask / SQLite | 110 | 0 | 0 | 110 |
| Go / MySQL | 110 | 0 | 0 | 110 |

**80项一致、30项已接受差异、0项pending、0项incompatible。** 30项由本轮29项及先前明确接受的下载Cache-Control差异1项组成，不是30个独立缺陷。110项仍包括106个HTTP用例和4个启动配置拒绝检查；没有减少用例或把未执行算通过。三个修复用例全部为 `match`。

完整证据：`runtime/campusclaw-phase5-contract-de3a9dfe/report.json`，SHA-256 `bff3831aa1cac620b9d434dbcde897e1d7f95131b3dd663b2af4a531128737d3`。报告 `complete=true`、`full_suite=true`、`not_executed_count=0`、`accepted=true`，runner退出0。历史102/110项及局部报告均未覆盖。

本轮使用新建隔离MySQL8.4.4数据库、独立SQLite和上传目录，Flask明确 `MATERIAL_BACKEND=sqlite`。实际HTTP端口为Flask63867、Go63868；结束后停止自建HTTP服务和测试MySQL容器，保留 `campusclaw-phase5-contract-de3a9dfe` 容器/网络、对应 `-data` 卷及runtime证据。没有使用正式8080为测试目标，也没有探测/修改Demo、挂载其三个卷、修改课程报告或正式入口。

本轮只调整共享测试的明确边界与验收文档/task；没有修改Flask或Go业务来迎合差异。**6.1全部既定验收项通过，勾选6.1，总进度24/30。** 本验收限定基础材料HTTP，不等于RAG迁移、正式Nginx验收或切换演练；6.2–6.4仍未勾选，未自动推进。没有新的未确认差异。
