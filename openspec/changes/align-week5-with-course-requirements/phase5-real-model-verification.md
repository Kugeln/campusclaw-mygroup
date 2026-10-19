# Current acceptance: 5.3 USER-reviewed and completed

The USER explicitly approved the prior evidence/no-skill answer and all eight remaining real-model answers, including mathematics, citations, skill differences, no-evidence generation and multi-turn context. Progress is now 27/29. Tasks 5.4 and 5.5 remain incomplete. Teaching-guidance experience has minor non-blocking room for improvement; no business change is made. See [human review confirmation](phase5-human-review-confirmation.md). No new model calls: cumulative dialogue 11/12, Embedding 8/8, original ledger locked and unchanged.

The following execution-time record is retained as historical evidence; its pending-review status has been superseded by the USER confirmation above.

# 5.3 Real-model verification: new answers pending USER review

Latest 2026-10-18: eight remaining real-model rounds technically completed; new dialogue 8 / Embedding 4, cumulative 11/12 and 8/8, ledger locked without retry. Prior evidence/no-skill scene is explicitly USER-approved. Eight new full answers await USER review; 5.3 stays unchecked, progress 26/29. See [current verification](phase5-remaining-real-verification.md). No 5.4/5.5 execution. Below are preserved historical checkpoints; their counts and pending-review statements describe their original times.

# 5.3 真实模型复验：连通性通过，首轮缺数字引用，已停止

最新单场景复测：提示词最小修复后，“有证据、无技能”真实重测技术通过，原始回答主动包含[1]，SSE成功done且数据库completed。仅新增对话1/Embedding1，原账本累计3/12、4/8；新回答待用户人工审读，完整5.3未完成，26/29不勾选。见[单场景复测完整记录](phase5-citation-retest-verification.md)及`runtime/align-real-26bcce67/citation-retest-b8478118/answers-for-review.md`。以下保留首次失败和更早授权的历史时点，不将其原计数当作当前值。

最新状态：用户核对《课程网关.pdf》并重新授权5.3后，已完成最小真实连通性验证。对话/Embedding均HTTP200，实际2048维。首轮有据无技能回答未输出合法数字引用，服务端正确failed/error，已停止后续调用。累计**对话2/12、Embedding3/8**，无自动重试；5.3待问题处理及用户人工审读，保持未勾选，进度26/29。完整结果见本文最后“本轮真实执行”及`runtime/align-real-26bcce67/answers-for-review.md`；以下旧预检/溯源记录保留历史时点。


日期：2026-10-18（Asia/Shanghai）。本轮已明确授权5.3，预算为真实对话最多12次、真实Embedding最多8次；串行、逐次检查剩余、无自动重试。不授权5.4或5.5完成。当前仍26/29，5.3/5.4/5.5均未勾选。

已复读本change规划、四份spec、Phase0–5记录、验收准备稿及原Week5真实联调记录。分支保持`feature/week5-phase1-go-rag`，保留已有未提交内容。原记录曾使用火山Ark直连；本轮只使用当前课程统一网关配置，未切换供应商。

## 实际只读预检

配置来源为Git忽略的`.env.course.local`，加载方式见`runtime/course-gateway/load-model-env.ps1`。没有输出Key、Authorization或Token，也未修改配置。

| 项目 | 实际观察 |
|---|---|
| 对话网关 | `https://ai-gateway.devops.hello1023.com/v1` |
| 配置对话模型 | `course-chat` |
| Embedding网关 | 同上 |
| 配置Embedding模型/格式/维度 | `course-embedding` / `openai` / `2048` |
| 鉴权 | 两项API Key均存在；单次带鉴权GET返回200，不代表所有模型端点均授权可用 |
| 连通性 | UTC `2026-10-18T02:37:51.731041+00:00`开始，只读`GET /v1/models`，HTTP200，无重试 |
| 模型匹配 | 返回的模型ID清单未列出`course-chat`或`course-embedding`；无法从该清单确认两者是否是隐藏别名及其映射 |
| 实际Embedding维度/格式 | **未验证**；2048/openai只是本地配置值，没有生成向量来验证 |
| 实际对话模型版本 | **未验证**；未发送生成请求，不能把本地别名当作已确认的底层版本 |

脱敏原始预检结果：`runtime/align-real-preflight/result.json`。该文件包含返回的模型ID清单、时间、状态及零调用计数，不包含请求头/凭据。实施前文件hash见phase5-real-model-baseline.json。

网关可连接，但配置别名与所公开的模型清单无法直接对应；这不证明别名必定无效，也不能据此声称配置已确认。依用户“凭据缺失、配置不明确或无法连接立即停止”的要求，**未尝试生成、Embedding或其他模型ID**。没有充值、购买额度、查询账单或切换服务。

## 调用与费用

| 类型 | 已执行 | 授权上限 | 剩余 |
|---|---:|---:|---:|
| 真实对话 | 0 | 12 | 12 |
| 真实Embedding | 0 | 8 | 8 |
| 只读模型清单GET | 1 | 不属于生成/Embedding | 不用于替代生成验证 |
| 自动重试/并发生成 | 0 | 禁止 | 未发生 |

没有Token用量、生成回答或模型实际费用数据。模型清单响应未提供费用信息，不能推断该GET实际收费为零；未观察到充值要求或额外收费提示，也没有执行额外计费操作。

## 场景状态

| 场景 | 状态及原因 |
|---|---|
| 有证据无技能；有证据有技能 | 未执行：配置确认门槛未通过 |
| 无证据无技能；无证据有技能 | 未执行：未调用真实模型 |
| 同题技能启用/停用/解绑对比 | 未执行，无真实回答差异证据 |
| 路由命中/未命中与真实回答 | 未执行；既有自动化结果不冒充本轮真实结果 |
| 候选证据存在但模型不引用 | 未执行；未来保留有据引用校验，若模型完全不引用应error/failed，不能伪造completed或把候选当实际引用 |
| 无据无虚构材料引用 | 未执行，无真实回答可审读 |
| 最近6条含当前问题一次 | 未执行真实多轮；既有受控回归不替代本轮 |
| meta/delta/done、实际增量与DB一致 | 未执行真实流式；没有真实SSE/持久化结果可报告 |

## 配置确认后的最小矩阵草案（本轮未执行）

可将十次对话合并覆盖：①有据无技能，②同题有据启用技能，③无据无技能，④同一无据会话启用技能，⑤有据同题停用，⑥有据同题解绑，⑦技能绑定但问题不命中，⑧有据无引用负向检查，⑨⑩继续无据历史会话，使第4轮实际触发末尾6条窗口。每次均保存完整回答、模型输入/路由快照、候选和最终来源、SSE时序及DB状态，教学结论由用户人工审读。

若材料保持单切片，预期一次索引Embedding及六次有据检索Embedding，共7次；空绑定四轮不Embedding。预期10次对话、7次Embedding，低于12/8。此数只是准备估算，执行前必须根据实际切片/路径逐次核对，不能据此授权隐含附加调用；任何上游失败停止、不重试。未来只允许在新建隔离MySQL库、新Qdrant collection和专用账号/材料/配置中执行，不能复用课堂资源。

## 保护与复核

本轮阻塞发生在创建测试业务资源或模型调用之前：没有连接或修改课堂/测试数据库、数据卷或Qdrant，没有启动、停止或清理测试服务/容器。业务代码、检索算法、角色/权限、SSE及生产配置未修改。原Week5 change和Phase0–5历史记录保留，不把历史回归作为本轮复跑结果。

本轮只运行OpenSpec strict及文件hash/任务状态核对；结果见phase5-real-model-check-result.json。没有为零业务代码修改重复执行数据库/浏览器全量回归，没有伪造模型、同伴或浏览器证据。

## 继续执行所缺信息与人工审核

需要确认课程网关允许使用的**精确对话模型ID、Embedding模型ID、Embedding格式和维度**，或确认`course-chat`/`course-embedding`的官方有效别名映射。不得在聊天发送密钥；可以只提供这些非秘密配置，或在本地忽略配置中更新后通知继续。

此前授权调用上限已记录；但本次后续指令明确仅授权只读配置溯源，不授权真实调用，因此本轮不会恢复生成。若后续恢复调用，须获得相应下一步授权；需要充值或其他计费项目时另行停止请求授权。

目前没有真实模型回答可交付人工审读，也没有任何用户人工审核结论。配置确认并完成真实调用后，仍须用户审读每轮完整回答；在人工确认及所有5.3条件满足前保持5.3未勾选。5.4待真实同伴，5.5待全部必需证据，本轮不进入。

## 只读配置溯源补充（2026-10-18）

本次授权仅只读溯源及更新本专项记录，**不授权真实对话或Embedding调用**。未运行旧预检脚本、真实模型测试、Compose或数据库/Qdrant工具；未修改任何配置。下面保留前次阻塞的历史事实，同时补充新发现，不把历史请求计入本次调用。

### 结论 A：已确认项目配置的网关请求ID，可以申请下一步授权

`course-chat`和`course-embedding`是项目配置中明确写入、并在已有真实网关测试中成功使用的请求ID。它们不是环境变量名、dotenv插值占位符，也不是当前代码里受控模型替身的名称。**尚未确认这两个网关别名对应的底层供应商/模型版本，亦未以真实生成验证当前可用性或当前凭据的生成权限。** 不需要因模型清单未列出它们而擅自替换成其他ID。

这里的A限于“已有可信本地配置说明与相互印证的历史真实请求证据足以确定应传的model字符串”，不等于已获得课程平台对底层映射的官方认证，也不等于5.3通过。前次仅依据清单无法对应而停止的谨慎做法仍保留；此次补足了当时未检查到的同别名真实调用记录。

### 定义、来源与映射证据

| 层级/证据 | 可确认的事实 | 不能推导的结论 |
|---|---|---|
| `tests/go_rag/README.md:9` | 本地课程网关配置说明明确指定统一`/v1`网关、`DIALOGUE_MODEL=course-chat`、`EMBEDDING_MODEL=course-embedding`、`EMBEDDING_API_FORMAT=openai`、`EMBEDDING_DIMENSION=2048` | 这是本仓库文档，不是有官方URL/版本/签名的课程平台配置公告；不能确认其最初作者或平台底层映射 |
| `.env.course.local:4,5,6,9` | Git忽略文件实际配置与README一致；Base URL在第2/7行；没有修改 | 凭据存在不等于每个模型都有调用权限；未输出凭据行 |
| `runtime/course-gateway/load-model-env.ps1` | 从忽略文件按允许的变量名写入进程环境；没有别名解析、模型选择或替换逻辑 | 它不会把`course-chat`自动转换成某个Doubao ID |
| `tests/go_rag/run_phase4.ps1:30–44` | 传ModelEnvFile时读取本地变量并注入进程；旧回退路径另读历史Demo配置，本轮未运行 | 旧runner不能直接作为本轮执行命令：旧4/8预算、旧固定测试库及自动移除行为不符合当前约束 |
| `internal/config/rag.go` | `EMBEDDING_MODEL`原样进入Embedding.Model，`DIALOGUE_MODEL`原样进入Dialogue.Model；维度/格式分别由独立变量提供 | 代码不含别名→底层模型映射表 |
| `internal/rag/clients.go:88`；`internal/conversation/stream.go:56` | 生成JSON时原样发送`model`字符串；openai格式走`/embeddings`和字符串input，对话走`/chat/completions` | 模型用途由配置字段和真实请求路径证明，非按名称猜测 |
| `internal/http/tutor_browser_integration_test.go:239–254` | 受控替身显式使用`Model: "test"`、3维；realMode改用LoadRAG的真实网关与HTTP计数transport | 没有证据表明两个course名称是这个测试替身的专用名称 |

当前只读检查的Process/User/Machine范围内，`DIALOGUE_MODEL`、`EMBEDDING_MODEL`、`EMBEDDING_DIMENSION`、`EMBEDDING_API_FORMAT`均未设置；因此本地忽略文件不是已自动生效的当前进程环境。Go入口不会自动读取dotenv。仅核对映射，不运行加载脚本，不修改任何范围环境变量。

上述README和加载脚本在当前Git索引没有已跟踪历史可用于确定最初引入提交；本次不能声称别名是课程平台官方发布，也不能还原谁首次填写这些值。

### 新发现的同别名历史真实证据

| 脱敏证据路径 | 观察 |
|---|---|
| `runtime/campusclaw-week5-rag-ee706bec/realmodel-6f7a1447/real-stats.json` | 明确记录`dialogue_model=course-chat`、`embedding_model=course-embedding`；4次`/v1/chat/completions`、7次`/v1/embeddings`，均HTTP200、network_error=false、blocked=false |
| 同目录`result.json` | `exit_code=0`、`real_model_verified=true`、未执行schema升级 |
| 同目录`browser.json` | `passed=true`、`real_model_verified=true`；这是旧契约历史证据，不能替代当前change的无据/SSE等新验收 |
| `internal/http/real_budget_test.go`及realMode接线 | 计数器包裹真实HTTP RoundTrip，只允许既定真实网关主机；真实分支覆盖受控gateway配置，不在计数器中伪造响应 |
| `runtime/course-gateway/preflight.py`、`preflight.json` | 脚本从本地忽略配置读取model，结果记录Embedding HTTP200/实际2048维、对话HTTP200/非空。**没有保存当时的模型ID、时间和配置快照，单独不足以关联当前别名**；本次只读取，未执行 |

另一个历史运行`realmodel-034a21a3`及原Week5的`real-model-verification.md`明确使用`doubao-seed-2-0-lite-260428`、`doubao-embedding-vision-251215`、Ark multimodal/2048维。它证明另一套旧供应商配置曾成功，不能把这两个ID认作当前course别名映射或直接替换openai配置。两类历史运行已区分，没有混算或更换服务。

历史计数文件本身不记录完整主机地址、配置快照或可认证的运行时间，需与README、配置及真实分支接线一起解释；它也不足以认证当前凭据/当前路由仍相同。本次没有把历史模型请求当作新请求或课程平台的权限承诺。

### 本次GET模型清单结构与权限分析

本次只发送一次带鉴权`GET https://ai-gateway.devops.hello1023.com/v1/models`，UTC `2026-10-18T02:44:11.136991+00:00`开始，返回HTTP200，无重试。脱敏结构证据：`runtime/align-real-provenance/models-readonly.json`。

- 顶层包含`data`数组及`object`字符串，共135项；每项均有`id/object/created/domain/features/name/version`，部分有`status/modalities/task_type/token_limits`。
- `course-chat`、`course-embedding`均未列出；清单也没有提供别名映射字段，故不能从缺席判断别名无效。
- 135项均未出现`permission`、`owned_by`或`allow_*`字段；顶层也没有当前Key的模型授权/余额/套餐信息。`features/task_type/status`等元数据不能直接视为当前调用者的授权。
- 历史Doubao两个ID也出现在清单中，只证明目录列出它们；没有据此认定应选它们，或认定当前Key允许调用。
- 脱敏文件保留字段出现次数和安全标识元数据，不保存Authorization、Key或响应中的未知敏感扩展。未发送模型检索以外的API请求。

### 下一步所需的最少确认与停止点

项目请求ID及配置项已找到，可在下一步授权中明确继续使用`course-chat`、`course-embedding`、openai格式、2048维，**不修改现有配置**。如需课程平台官方层面的确认，只需核实一组非秘密信息：当前课程账号是否应使用上述两个请求ID，以及Embedding是否为`/v1/embeddings`字符串input、2048维；若平台要求其他ID，请提供平台指定的精确ID与同组格式/维度。底层模型版本可由平台提供用于审计，不能自行猜测。无需发送密钥。

本轮新增对话0、Embedding0；自5.3开始仍为0/12和0/8。模型清单GET本轮1次，连同前次预检累计2次，均不属于生成/Embedding。没有本轮模型回答、Token用量或人工教学结论，5.3保持未勾选，进度26/29；5.4/5.5不进入。更新本专项记录后停止，等待下一步授权。

## 本轮真实执行：连通性通过，首轮有据回答缺引用后停止

日期：2026-10-18，Asia/Shanghai。用户明确说明已核对老师提供的《课程网关.pdf》，官方值为统一`/v1`网关、course-chat、course-embedding、openai和2048维。这是**用户提供并确认的配置事实**，本代理没有收到或直接读取该PDF，不伪称已审阅原件。课程变量`EMBEDDING_DIM=2048`对应本项目实际读取的`EMBEDDING_DIMENSION=2048`，未改本地配置。

本次授权恢复真实复验，计入总上限12次对话/8次Embedding，串行、不重试、不更换模型；5.4/5.5不授权。实际使用Git忽略的`.env.course.local`凭据，没有将Key/Authorization保存到终端、日志或报告。

### 最小连通性与累计预算

`tests/go_rag/align_real_probe.py`只发送一次非流式对话、一次Embedding；逐次在网络发送前持久预约调用，任何失败锁止，不跟随重定向、不重试。通过后`run_align_real.ps1`启动独立Go测试进程，继续使用同一budget.json而非清零。Go显式关闭连接复用，POST不配置重试，错误和计数分别记录；技术验收失败后账本已锁止，不再生成。

| 实际请求 | HTTP | 结果/Token |
|---|---:|---|
| 连通性course-chat | 200 | 响应JSON和非空content合法；prompt82、completion31、total113 |
| 连通性course-embedding | 200 | openai的data[0].embedding合法，实际长度2048；total32 |
| 合成材料索引Embedding | 200 | 单切片2048维，进入本轮新collection；total67 |
| 首轮绑定检索Embedding | 200 | 只检索显式绑定集合并回表，得到1条候选；total40 |
| 首轮有据无技能course-chat流式 | 200 | 上游正常完整生成；prompt307、completion278、total585；下游引用校验失败 |

累计**对话2/12，Embedding3/8**，剩余10/5未使用。对话合计prompt389、completion309、total698；completion包含网关报告的reasoning部分，不能再把reasoning重复相加。Embedding合计total139。实际金额/币种/计价未在观测响应中提供，不能据Token推断费用；没有充值、购买额度或额外收费操作。返回模型标签为course-chat，底层供应商/版本没有额外确认。

### 隔离环境与执行过程

重新核对独立MySQL `campusclaw-align-storage-17d5f822`、独立Qdrant `campusclaw-align-stream-6e1982af-qdrant`的运行、标签、tmpfs、无持久挂载及internal网络。新的测试进程只连接这两者；使用真实HTTP课程模型、生产Bearer/角色/仓储/检索/StreamingModel，不使用模型替身冒充回答。

首个新库`align_w5_1e06f1b135d84024`在插入合成材料时因夹具字段名错误报MySQL1054；发生在新增模型调用之前，账本仍1/1。修正为现有schema的size/content_type/storage_key并重新仅编译后，在另一个新库执行；没有重发已经成功的连通性请求。失败库/容器及日志保留：fixture-failure-before-models.log、fixture-failure-result.json。该本地夹具修正不是上游模型重试。

实际教学库为`align_w5_9597acd9f56c179d`，新collection `align_real_18df805af4916d91`，新材料ID `00000000-0000-4000-8000-000000000030`，合成教师/学生/另一用户/另一班级账号仅在新库内。没有修改课堂数据库、数据卷或任何既有collection，没有启动课堂Compose，没有自动删除库/collection/容器。

### 首轮完整问题、材料和回答（待用户人工审读）

场景：有证据、无技能。空prompt使用已冻结默认角色；skill_ids=[]，只绑定上述材料，实际技能为空。未启用指导技能，不能把此轮给出完整答案直接判为违反技能。

问题：

> 请解方程2x+3=11，直接告诉我最终答案，并说明你使用的资料。

材料`equation-course.md`全文（单切片，chunk_index=1）：

> 数学讲义：等式两边进行相同运算，等式仍成立。例题2x+3=11，先两边减3得2x=8，再两边除以2得x=4。最终答案是x=4。

真实模型完整正文（与数据库及下游delta拼接一致）：

> 最终答案：x=4。
> 使用的资料：本班课程的数学讲义（来源为equation-course.md），讲义中明确等式两边进行相同运算等式仍成立，且该讲义中已给出此方程的解题步骤：先等式两边减3得2x=8，再两边除以2得x=4。

**客观失败**：有据回答虽写了材料名，但没有合法的`[1]`数字引用。服务端未把候选来源冒充实际引用，而是以`invalid_citations_or_answer`发error、保存failed；没有成功done。不能将“上游HTTP200”说成回答验收通过。该轮同时观察到矩阵要求的“有证据但模型未引用”情况，但有据正常成功分支尚未通过。

教学方式、准确使用材料、是否编造内容、表达质量等主观项目保留完整回答供用户审核；本记录不是用户人工结论。无据、技能启停/命中、历史连贯性没有本轮结果可供评价。

### SSE与持久化实证

- 本轮真实网络SSE为`meta → delta×69 → error`，无done；meta.status=ok，候选只有[1] equation-course.md，chunk_id为`2bdab629-7795-4d08-b0ae-131e3d2b95b7`。
- 客户端约0.252秒收到meta，约2.449秒收到首个delta，约2.944秒收到error；第一段明显早于末段，非整段返回。
- 服务端meta Flush时间1792292305746610802ns；Model.Open时间1792292305746611602ns，实际首delta时间1792292307943858175ns；模型只能在meta Flush后进入。精确事件和时间在teaching.json，原始下游流在evidence_no_skill.sse。
- 每个delta接收时直接查MySQL，已接收前缀均已落库；全部delta拼接等于持久化全文。最终state=failed、citations=[]，候选仍在evidence/configuration快照，active占用释放；未执行“成功Finish→done”，不伪报完成。
- 本轮使用真实Go HTTP服务及真实模型流；没有重跑真实浏览器/Nginx或冒称浏览器证据。既有代理/解析器未改。

### 场景覆盖及停止原因

| 场景 | 本轮结果 |
|---|---|
| 有证据、无技能正常成功 | **失败**：缺合法数字引用，已保存完整回答 |
| 有证据但模型未引用 | **已观察到**：正确meta候选→error/failed，最终实际引用[]，无done |
| SSE真实增量、候选与实际区分、持久化 | 客观检查通过，终态是failed而非成功 |
| 有据有技能、技能启用/停用/解绑、命中/未命中 | 未执行；失败后没有继续调用，无法报告回答差异 |
| 无据有/无技能、无据引用清洗 | 未执行，不能以旧受控测试替代本次真实效果 |
| 最近6条含本轮一次、多轮上下文 | 未执行真实多轮；预算账本保留本轮实际模型messages，不伪造窗口验收 |

本次只完成连通性及首轮教学尝试，**没有完成整个5.3矩阵**。没有为通过而放宽引用校验、改写模型回答、伪造done或修改业务提示词。后续处理缺引用问题或继续其他场景，需用户审阅本次失败并给出继续方向；剩余预算不会自动使用。

### 证据与回归

完整脱敏证据目录`runtime/align-real-26bcce67/`：

- answers-for-review.md：两次对话完整回答及人工审读状态；本专项记录亦保存教学全文。
- budget.json：每次实际请求前预约、模型输入、HTTP、网关返回usage/model；累计2/3和最终锁止原因。
- teaching.json：完整事件/时间、实际候选、绑定配置、实际技能、数据库消息/证据快照、前缀和正文一致性。
- upstream-03.txt/upstream-04.txt：索引/检索真实向量响应；upstream-05.txt：真实上游流（含返回的正文/推理或usage字段），不含请求Authorization；evidence_no_skill.sse为下游原始SSE。
- usage-summary.json、run-result.json、teaching.log及失败夹具证据，文件hash和安全复核见phase5-real-execution-check.json。

已通过本轮非收费回归：`go test ./internal/rag ./internal/http ./internal/auth ./internal/conversation ./internal/tutor -count=1`、原Node SSE解析3/3；保护Week4三模式核心、RRF/阈值/Ask零命中固定文案和零对话调用，认证与流式纯核心。新库HTTP检查了匿名401、学生技能写403、同班其他owner/外班会话404。因首轮失败而未执行排在矩阵末尾的真实库Week4零命中用例，不把它称为本轮真实服务复验通过。

OpenSpec strict及保护hash核对见本轮check JSON；没有修改业务Go实现、原SSE解析器、生产配置、schema/catalog或旧Week5的32/32。新增仅隔离执行器/测试（align_real_probe.py、run_align_real.ps1、phase5_real_integration_test.go）及本change状态记录。未commit/push/归档/部署，未清理资源；不进入5.4或5.5，保持26/29等待用户人工审读。
