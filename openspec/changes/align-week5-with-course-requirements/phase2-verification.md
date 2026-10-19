# Phase 2 验收记录

日期：2026-10-17，Asia/Shanghai。change：align-week5-with-course-requirements，schema：spec-driven。本轮仅授权2.1–2.5，未执行Phase3–5。开始前复读proposal/design/tasks、四份spec、Phase0/Phase1验收及迁移预检模板。

分支保持`feature/week5-phase1-go-rag`，HEAD未变；已有未提交内容保留。phase2-baseline.json记录实施前283份文件hash；最终范围、旧历史及测试状态见phase2-check-result.json。原Week5的17份文件和32/32任务保持不变，Phase0/Phase1历史记录及四份spec不改写。

## 隔离环境

只读核验`campusclaw-align-storage-17d5f822`标签、HostConfig.Tmpfs中的`/var/lib/mysql: rw`、独立internal网络及归属；该实例为Phase1新建的MySQL8.4.4，非课堂库。Docker本次Mounts列表为空，但HostConfig.Tmpfs仍明确存在，不能把空列表当作持久卷证据。所有本轮数据库用例在此实例CREATE全新随机`align_w5_<16hex>`库，不读写之前测试库或课堂库。测试进程只连接该新内网，没有启动/重建课堂Compose，没有操作任何Qdrant collection。

HTTP使用实际路由、Bearer校验和MySQL用户/仓储；向量、Embedding及对话上游为明确的进程内受控替身，StreamingModel输入/协议测试仅访问httptest本地HTTP服务。未调用收费模型。测试日志/脱敏JSON在`runtime/campusclaw-align-storage-17d5f822/`，连接配置及密码仍被Git忽略；没有commit/push/部署/归档。

## 任务结果

| 任务 | 结果 | 实际验证 |
|---|---|---|
| 2.1 source API | 通过，已勾选 | GET列表/单条、POST、PUT source、DELETE接入现有Bearer/教师guard；enabled-only仍可用；原文CRLF/正文#往返，解析字段与revision/source_version同步。匿名401、学生写403、外班/不存在404一致；重名、null/类型错误/未知/混合字段、重复JSON字段、非法UTF-8及格式错误400。真实MySQL触发器拒绝source或toggle更新时503且全部字段/关系/版本hash不变。 |
| 2.2 确定性路由 | 通过，已勾选 | 当前问题与扩展检索query分离；只在当前绑定且enabled集合按`,，、;；\|`分词/小写/子串评分，重复词按出现次数计分；最高正分、同分小ID、零分不选。所有分隔符、乱序候选、未绑定高分、停用和零分均有纯测试；真实库验证编辑/停用/解绑下轮生效，正在生成快照不变；仅选中说明进入System，名称含解题引导追加服务端教学边界，不调用模型路由或执行工具。 |
| 2.3 材料与证据 | 通过，已勾选 | nil/空绑定均短路，最后一份材料解绑后也不访问检索/Embedding/向量。非空绑定按principal班级及显式集合调用共享检索；取top4之后subject标题子串过滤、去空、连续编号，不补第5条或未绑定资料，外班/未绑定证据拒绝。捕获实际StreamingModel请求验证799/800/801个补充平面Unicode字符边界，正文最多800字，仅进User；原证据全文保持。依赖故障与空结果区分。 |
| 2.4 角色/会话资格 | 通过，已勾选 | 空白prompt保存为空并在有据准备时使用Phase0原样默认角色；prompt2000/2001、名称64/65及重名边界验证。新建只允许本班enabled；disabled直取学生404、教师可管理；已有会话经过owner+class校验后可继续并保持agent_id。无启用助手400、外班/同班他人/转班404，覆盖agent字段400且无数据库变更。 |
| 2.5 消息窗口/标题 | 通过，已勾选 | History改为消息序列，按写入顺序取末尾6条有效user/已完成assistant，包含正常完成的no_evidence历史；空正文、非法角色/UTF-8（纯测试）、failed/interrupted/generating草稿排除。每条历史截500字但DB原文不截；当前已接受user由active_turn服务端标记，仅组装一次，普通历史中的重复提问不按文本错误去重。真实库以受控Start→History顺序验证含当前user；StreamingModel真实请求验证非System总计≤6、每条历史≤500。首问trim后前24个rune，24/25补充平面字符边界、后问标题不变、updated_at/列表排序通过。 |

2.4默认角色精确使用Phase0冻结措辞，没有把默认角色变成所有轮都只能引导的强制技能。教学约束测试只证明服务器输入机制，不能代替真实模型教学效果或同伴验收。

## Phase 2 与 Phase 3 的分界

- 本轮只改`ConversationHandler`的一处Prepare调用，使路由接收当前q、检索接收组合query。恢复该调用后，整个handler可重构为phase2-baseline中的原文件hash（兼容LF/CRLF）；Start/Model.Open预读/响应头/meta/delta/done/error的顺序和内容保持。
- 有据生成使用新角色、当前所选source、800字证据和新消息窗口。空绑定/正常零命中/subject归零仍走原`rag.NoEvidence`旁路；没有开启Week5无据模型生成，实际技能执行审计也不把未调用模型记成执行。
- `History`在Start后识别已保存的当前user；模型输入窗口支持已保存/尚未保存两种服务端状态，当前user不重复追加。实际HTTP仍在旧位置读取历史和预读模型，尚未把初始user/占位事务前移。该事务编排和证据快照准备接线明确由3.1完成，本轮受控顺序测试不冒称生产接受顺序已改变。
- meta仍是旧空候选来源，模型Open/首片仍在meta前；meta恒ok/完整候选早到、无据编号清洗、流后错误边界的调整均留给3.1–3.5。没有修改sse.js、Nginx、Swagger资产、生成工具或教师/学生页面。

## 实际测试命令与结果

| 命令 | 结果 |
|---|---|
| `powershell -NoProfile -File tests/go_rag/run_align_storage.ps1 -Stage Phase2API` | 通过，真实HTTP/Bearer/MySQL CRUD、权限和失败原子性 |
| 同runner `-Stage Phase2Routing` | 通过，当前配置/路由/快照 |
| 同runner `-Stage Phase2Evidence` | 通过，绑定短路/范围/故障 |
| 同runner `-Stage Phase2Roles` | 通过，角色/新建与续聊/身份及配置边界 |
| 同runner `-Stage Phase2History` | 通过，接受轮窗口、保留全文、标题/排序 |
| 同runner `-Stage Phase2` | 全部通过，8.617秒；包括最终扩展的Week4保护及配置拒绝场景 |
| 同runner `-Stage Store`、`-Stage Delete` | Phase1相关仓储和内部删除保护复验通过，分别5.550秒、2.278秒；未重跑未改动的41步schema中断矩阵 |
| `go test ./internal/tutor ./internal/auth ./internal/http ./internal/rag ./internal/conversation ./internal/repository/mysql ./cmd/server ./cmd/admin ./cmd/bootstrap -skip '^TestDocumentationRoutesAndContract$' -count=1` | 通过；该排除项单独执行并如实记为失败，不算通过 |
| `node --test tests/go_rag/sse.test.cjs` | 3/3通过，解析/增量/错误保护 |
| `go test ./internal/http -run '^TestDocumentationRoutesAndContract$' -count=1` | **退出1，未通过**；报新增GET单条技能/POST技能未在旧文档注册。旧检查的正则还不覆盖DELETE；实际新增三项操作及source请求schema都须4.3补齐 |
| `openspec validate align-week5-with-course-requirements --strict` | 通过；只证明OpenSpec结构有效，不代替上述业务或文档验收 |

主机单元Go1.27.1，隔离容器Go1.26.0/MySQL8.4.4。每轮runner JSON包含阶段、时间、退出码及日志位置，最新证据和文件SHA256见phase2-check-result.json。

### Week4保护

共享`internal/rag/`、MySQL RAG实现和HTTP Search/Ask文件均未修改。纯测试复验RRF、分块、阈值、向量payload二次检查、网关协议、Ask引用及零命中零调用。隔离库额外执行真实MySQL Keyword/Hydrate + 受控向量/Embedding/Dialogue的HTTP验证：

- keyword/vector/hybrid三模式正常，本班材料UUID/1-based切片不变，keyword零Embedding。
- .35接受、.349拒绝；外班/错误payload/未ready回表结果拒绝；hybrid依赖故障503，不降级为keyword结果。
- Week4 nil材料范围仍是全班；即使Tutor解绑全部材料，Week4仍能命中本班资料，Tutor则零次访问共享检索依赖。
- `/api/ask`保持question/mode与answer/sources，无命中固定文案、sources空、对话调用为0；有据引用正常，不写会话消息。

没有调用真实Qdrant或真实收费模型，不能把受控向量验证称为本轮真实Qdrant集成复验；该跨模块阶段门槛留待3.6。Bearer/教师角色及当前用户数据库校验均实际经过HTTP中间件。

## 修改文件和新增测试

修改11份已有文件：

- `internal/http/tutor.go`、`handler.go`：source CRUD及严格输入；`conversation.go`：当前问题与检索query分离。
- `internal/tutor/tutor.go`、`tutor_test.go`：多技能路由、角色与证据边界及替换旧反向断言。
- `internal/conversation/types.go`、`stream.go`、`stream_test.go`：消息窗口、服务端Current标记、单次当前user组装及证据输入截断。
- `internal/repository/mysql/tutor.go`、`conversation.go`：当前候选、disabled续聊资格、消息级History及24字标题。
- `tests/go_rag/run_align_storage.ps1`：新增Phase2分阶段执行入口，继续仅使用新库。

新增11份测试：

- `internal/tutor/routing_test.go`、`evidence_test.go`、`roles_test.go`。
- `internal/conversation/evidence_input_test.go`、`window_test.go`。
- `internal/repository/mysql/phase2_http_integration_test.go`、`phase2_routing_integration_test.go`、`phase2_evidence_integration_test.go`、`phase2_roles_integration_test.go`、`phase2_history_integration_test.go`、`phase2_week4_integration_test.go`（统一alignstorage标签，每例新库）。

本change只更新proposal/design/tasks的阶段状态，新增本记录、phase2-baseline.json、phase2-check-result.json；历史记录与四份spec均不修改。

## 进入 Phase 3 前的风险与阻塞

无未解决的Phase3需求设计冲突。3.1必须使用新的消息级History与Current标记完成Start后读取及证据快照准备；不能重新取6轮、重复追加问题或放宽owner/class。3.2才取消Week5无据旁路，须保留Week4零调用；3.3再调整meta首帧与HTTP/SSE错误分界并更新原反向断言。

Swagger契约检查当前明确失败，属于4.3计划内尚未完成工作，本轮没有绕过或声称全套测试全绿；新接口文档/source schema、动态路由枚举与DELETE覆盖必须完成后才能宣称DA1通过。教师UI多选/空prompt入口仍待4.1；真实模型/同伴/正式迁移/部署均未授权。Phase3执行授权尚未获得，这是继续实施的流程门槛。

2.1–2.5全部在对应实现和测试通过后勾选，新进度14/29，剩余15项保持未勾。Phase2结束立即停止。
