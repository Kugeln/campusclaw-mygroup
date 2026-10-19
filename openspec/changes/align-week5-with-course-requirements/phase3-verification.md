# Phase 3 验收记录

日期：2026-10-17，Asia/Shanghai。change：`align-week5-with-course-requirements`，schema：spec-driven。本轮仅授权3.1–3.6，开始前复读proposal/design/tasks、四份spec及Phase0–2验收。分支`feature/week5-phase1-go-rag`，HEAD保持`51a8749f6550c279efebe63bb67eda724878f5a1`；保留已有未提交内容。Phase4–5未实施，没有commit/push/归档/部署。

## 隔离环境与证据

继续使用Phase1新建的`campusclaw-align-storage-17d5f822`（MySQL8.4.4、`/var/lib/mysql` tmpfs、独立internal网络`campusclaw-align-storage-17d5f822-net`）。每例CREATE全新随机`align_w5_<16hex>`库，未使用此前测试库或课堂库。新增`campusclaw-align-stream-6e1982af-qdrant`，本地`qdrant/qdrant:v1.13.4`镜像、`/qdrant/storage` tmpfs、同一内部测试网络，无主机端口和持久卷；每轮联测只CREATE新的`align_w5_<时间随机值>` collection，不修改或删除任何已有collection。

Embedding和模型为进程内/httptest本地HTTP替身，无收费调用。实际使用生产VectorClient、Retrieval、MySQL仓储、Bearer中间件、HTTP会话处理器和StreamingModel；故障分别标明为函数注入、真实测试库SQL拒绝、本地HTTP503、真实TCP断连。主机Go1.27.1，容器测试Go1.26.0，SSE解析使用Node原有测试。

日志和脱敏结果位于`runtime/campusclaw-align-storage-17d5f822/`；新Qdrant连接位置为被Git忽略的`runtime/align-stream-connection.json`，没有密码。MySQL密码继续只在忽略的原连接文件中。基线见[phase3-baseline.json](phase3-baseline.json)，最终命令/文件hash/资源属性见[phase3-check-result.json](phase3-check-result.json)。所有开发期间失败日志保留，不能用早期失败或缓存当最终成功。

## 逐项验收

| 任务 | 状态 | 实现和实际验证 |
|---|---|---|
| 3.1 接受与准备 | 通过 | Capture/Route冻结配置后，Start短事务原子保存user、generating占位和占用；再读含当前user的历史、绑定检索、Prepare短写保存证据。所有准备及模型输入校验在meta前。Start不预填实际技能；ModelStarted仅在Open成功后保存实际技能审计。独立连接在Open期间可`FOR UPDATE NOWAIT`锁会话，证明无跨网关长事务。真实触发器拒绝assistant INSERT时消息对全回滚；History/检索/Prepare注入及真实UPDATE拒绝返回503，已接受轮次failed且释放占用、模型0次；owner/class/turn token保护、并发HTTP409且无额外消息对通过。准备期间教师编辑/解绑后仍用旧快照。 |
| 3.2 无据生成 | 通过 | 移除Week5固定回答旁路。空绑定零检索、正常零命中和subject归零都构造角色/技能输入并生成；无据System禁止数字引用和编造出处，有据仍用原证据约束。四组合均调用一次模型；真实Embedding503返回meta前503、无模型调用，与合法空结果区分。Week4 Ask未改。 |
| 3.3 早meta | 通过 | meta恒ok，携带完整候选证据/连续编号、候选技能名及message_id，空集明确`[]`。记录器在Model.Open入口断言meta已写入并Flush；真实HTTP上游门控首片时客户端可读meta。done只给实际引用子集，引用第2条时仍为number=2。模型连接/首片失败均为200 + error，无done；真实HTTP上游503也验证该边界。 |
| 3.4 清洗与校验 | 通过 | 新增轻量跨片状态机，确认`[数字]`后在Append/delta前删除，普通括号、EOF非编号残留保持；支持Unicode字符跨字节，正常文字立即通过。纯测试遍历每个分片宽度；数据库流测试验证delta拼接=存储正文。空/只有编号/整段旧固定文案不能completed。有据缺引用、0号、越界仍failed/error；合法稀疏引用保持。原始/有效输出均限1MiB，避免异常无限编号前缀积累。 |
| 3.5 可靠性 | 通过 | Append成功才delta，Finish成功才done；在writer写入时直接查DB验证先后关系。函数注入首个/后续Append、终态提交和审计失败保留可靠前缀。meta Flush失败不调用模型、转interrupted；delta传输失败保留已存未送前缀；done写失败后历史仍completed。受控取消、超时、真实HTTP断连取消上游通过。所有终结写失败时保持generating/占用，不假报failed，显式Recover后interrupted并释放；恢复保留正文和技能。HTTP1.1 keep-alive、Flush、心跳和不缓冲响应头保留；sse.js和Nginx均未修改。 |
| 3.6 阶段集成 | 通过 | 真实隔离MySQL + 新隔离Qdrant + 本地Embedding/StreamingModel验证四组合、绑定集合和跨班、ready回表、meta/增量/终态与DB、真实连接失败/断连/检索故障。旧会话50/51消息的正文、引用、配置、证据、技能审计SHA256保持。Phase2全套和Phase1 Store/Delete回归通过，Week4真实三模式及受控阈值/故障/RRF/Ask保护通过。 |

## 实际时序和错误分界

成功链路：`Start提交 → History/检索/Prepare → meta/Flush → Model.Open → ModelStarted → Append → delta/Flush → … → Finish(completed) → done/Flush`。没有预读首token再发meta。meta的skill_name表示本轮候选；数据库实际skill_id/name/version/hash只有Open成功后写入，连接失败不伪记技能成功执行。

| 场景 | HTTP/事件 | 数据库与调用 |
|---|---|---|
| 非本人/外班、覆盖配置、并发 | JSON404/400/409 | 无额外轮次，延续Phase2实际Bearer权限测试 |
| Start真实SQL拒绝 | JSON503，无SSE | 事务全回滚、无孤儿、模型0次 |
| 已接受后History/检索/Prepare失败 | JSON503，无meta | user和failed占位保留、占用释放、模型0次 |
| meta写入/Flush失败 | 不再改JSON状态 | interrupted；模型0次；传输失败不能承诺客户端收到error |
| meta后Open/首片/部分流失败 | HTTP200，meta后error，无done | failed、前缀保留并释放占用；连接失败不写实际技能 |
| 清洗后空/旧固定文案、有据非法引用 | HTTP200 + error，无done | failed，已可靠写入内容保留 |
| 取消/断连 | 已有HTTP200，无法保证后续事件到达 | interrupted、上游取消、可靠前缀保留 |
| 超时/Append/终态失败 | HTTP200 + error（可写时），无成功done | failed；所有终结写失败则由Recover收敛为interrupted |
| Finish成功但done丢失 | HTTP200但客户端无done | 查询历史仍completed，不回退、不重试模型 |

四组合实际调用数（每个输入仅一轮；另有故障用例单独计数）：

| 证据 | 技能 | 模型HTTP调用 | Embedding调用 | 实际SSE |
|---|---|---:|---:|---|
| 有 | 有 | 1 | 1 | meta → delta×4 → done |
| 有 | 无 | 1 | 1 | meta → delta×4 → done |
| 无（空绑定） | 有 | 1 | 0 | meta → delta → done |
| 无（空绑定） | 无 | 1 | 0 | meta → delta → done |

有据上游的`[1]`被拆成多片合法保留；无据同样片段被清洗，输出引用`[]`。正常零命中/subject归零另各验证模型1次。只引用第2条的受控流中meta提供两候选、done仅number=2，不重新编号。

## 命令与结果

| 命令 | 最终结果 |
|---|---|
| `powershell -NoProfile -File tests/go_rag/run_align_storage.ps1 -Stage Phase3` | 通过，7.895秒；真实隔离MySQL/Qdrant及受控HTTP模型，含完整故障与恢复矩阵 |
| 同runner `-Stage Phase2` | 通过，7.965秒；CRUD/路由/角色/窗口及Week4保护；标题夹具从旧旁路改为显式Append/Finish后复验通过 |
| 同runner `-Stage Store`、`-Stage Delete` | 通过，3.685秒/1.580秒；历史、绑定原子性、内部删除回滚及无公开DELETE |
| `go test ./internal/tutor ./internal/auth ./internal/http ./internal/rag ./internal/conversation ./internal/repository/mysql ./cmd/server ./cmd/admin ./cmd/bootstrap -skip '^TestDocumentationRoutesAndContract$' -count=1` | 通过；Swagger例外单独执行并记录失败 |
| `node --test tests/go_rag/sse.test.cjs` | 3/3通过，分片UTF-8/CRLF/心跳/多行data、增量和错误/EOF保护 |
| `go test ./internal/http -run '^TestDocumentationRoutesAndContract$' -count=1` | **失败，exit1**：新增GET单条/POST技能未文档化，旧枚举还漏DELETE；新增接口/source及新SSE契约仍待4.3，不修改检查或伪报通过 |
| `openspec validate align-week5-with-course-requirements --strict` | 通过；不代替业务、Swagger或人工验收 |

Week4：共享`internal/rag/`及MySQL RAG、Search/Ask处理器hash不变；纯测试覆盖RRF/阈值/协议。隔离真实Qdrant三模式均仅返回本班两份材料；Week5仅绑定一份只给该份，解绑后零Embedding。真实ready回表拒绝pending切片；Phase2受控向量另外验证.35接受/.349拒绝、错误payload和班级、依赖故障不降级。Week4 Ask正常零命中固定`资料中未找到相关内容`、空sources、对话0次。

旧`conversationintegration`/浏览器/Compose runner依赖旧固定库与旧反向断言，本轮不连接其已有库，不声称其通过；本阶段用新`alignstorage`用例验证当前生产路径。全量旧验收脚本契约适配/整体验收仍由5.1收口；浏览器/代理实测为4.2/4.4，真实模型和同伴分别为5.3/5.4。

## 修改文件与新增测试

修改9份已有文件：

- `internal/http/conversation.go`：接受、准备、meta前移、无据流、错误和持久化编排。
- `internal/conversation/types.go`：Prepare/ModelStarted存储接口；`internal/repository/mysql/conversation.go`：短事务占位、受turn token保护的准备/审计写入，取消无据自动完成。
- `internal/tutor/tutor.go`：基于已接受快照检索，无据角色/技能组装。
- `internal/tutor/tutor_test.go`、`evidence_test.go`、`internal/repository/mysql/phase2_evidence_integration_test.go`：替换旧Week5无据旁路断言；`phase2_history_integration_test.go`：标题用例显式完成轮次。
- `tests/go_rag/run_align_storage.ps1`：Phase3/Phase3Core入口、独立Qdrant核验及环境传递。

新增5份实现/测试/执行器：

- `internal/conversation/citations.go`；新增测试`citations_test.go`（清洗分片、普通括号、EOF、空/固定文案、有据校验）。
- `internal/repository/mysql/phase3_stream_integration_test.go`（8个顶层测试及其矩阵：准备、四组合、清洗/故障、token/恢复、快照、生命周期、引用拒绝、并发）。
- `internal/repository/mysql/phase3_services_integration_test.go`（真实隔离依赖、HTTP门控/连接故障/断连、四组合、旧历史hash及Week4）。
- `tests/go_rag/prepare_align_stream.ps1`（只新建独立Qdrant，无清理已有资源）。

本change只更新proposal/design/tasks阶段状态，新增本验收及phase3-baseline/check-result。四份spec、Phase0–2历史记录及原Week5 change全部保持；不改变schema/catalog、业务库或部署配置。

## Phase 4 前风险与停止点

Phase3无未解决的后端设计阻塞。Phase4尚需授权：教师配置页面及学生候选/最终来源展示还未适配，Swagger契约检查仍失败，注解生成和浏览器/Nginx真实验收仍未做，不能宣称全套测试或课程DoD全部通过。旧集成runner的固定库和反向断言还需在后续整体验收适配；本轮没有启动它们。真实模型教学效果和真实同伴证据未取得，不由受控模型结果代替。

生产故障时若终结数据库写全部不可用，只能保留可靠前缀并等待单写者启动Recover，不能承诺立即释放占用；该已有恢复边界已注入验证。新测试资源保留供复核，tmpfs数据不保证停机后留存；没有自动清理数据库、卷或collection。

3.1–3.6均在对应实现/测试通过后勾选，总进度20/29；Phase4–5的9项保持未勾。Phase3完成后立即停止，等待下一阶段确认。
