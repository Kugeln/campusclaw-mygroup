# 5.3 单场景真实复测：技术通过，待用户审读

日期：2026-10-18（Asia/Shanghai）。仅获“有证据、无技能”一次重测授权；本轮新增上限对话1、Embedding1，无自动重试。**未执行完整5.3矩阵，5.3保持未勾选，进度26/29，不进入5.4/5.5。**

## 实际调用与预算

使用已确认课程网关`https://ai-gateway.devops.hello1023.com/v1`，对话`course-chat`、Embedding `course-embedding`，openai格式/2048维。本地忽略配置未改，凭据未进入报告或请求日志。

原账本`runtime/align-real-26bcce67/budget.json`从**对话2、Embedding3**开始；本次在该文件追加原请求序号6/7，未清零、未重发连通性验证。每次调用发送前持久预约；独立transport限制本轮每个模型端点最多一次，禁止重定向和重试。完成后重新blocked锁止，没有继续其他场景。

| 项目 | 本轮新增 | 累计 | 原总上限 |
|---|---:|---:|---:|
| 对话 | 1 | 3 | 12 |
| Embedding | 1 | 4 | 8 |

两次真实请求均HTTP200。Embedding为本轮查询向量，原始返回长度2048；没有重新索引材料。对话usage为prompt393、completion204（其中reasoning142包含在completion中）、total597；Embedding total40。网关没有提供本轮实际货币费用，不能据Token推算。返回的对话模型标签仍为course-chat，未声明底层供应商/版本。

## 对照环境与实际模型输入

复用此前**专用隔离测试库**`align_w5_9597acd9f56c179d`和独立tmpfs MySQL `campusclaw-align-storage-17d5f822`；复用独立tmpfs Qdrant `campusclaw-align-stream-6e1982af-qdrant`中的本次旧测试专用collection `align_real_18df805af4916d91`。先只读验证标签、挂载、internal网络、完整schema及旧材料/单ready切片，再创建新的学生会话42。没有改课堂库、卷或其他collection，没有DDL、重新索引、重建服务或清理资源。

助手仍ID10、名称A tutor、空prompt、enabled=true、skill_ids=[]，仅绑定材料`00000000-0000-4000-8000-000000000030`，revision仍8。使用同一新库内的专用测试学生身份及生产Bearer/owner+class处理器。此前失败会话保留，本轮新会话没有历史干扰。

问题和材料原文、候选chunk_id/切片索引全部复用：

> 请解方程2x+3=11，直接告诉我最终答案，并说明你使用的资料。

材料equation-course.md：

> 数学讲义：等式两边进行相同运算，等式仍成立。例题2x+3=11，先两边减3得2x=8，再两边除以2得x=4。最终答案是x=4。

发送真实对话**之前**检查system中存在`编号必须取自本轮evidence中的number字段；number=2对应[2]，不是chunk_index`；缺少则拒绝派发，实际账本序号7保存完整system/user消息。System使用受控测试通过的新有据引用规则，User仍为原question/evidence JSON，没有修改编号结构，也没有在学生问题中额外要求输出[1]。没有修改校验器、自动补引用或放宽规则。

## 修复前后完整回答

修复前（此前真实调用，已由用户审读数学/步骤/无技能边界，格式失败）：

> 最终答案：x=4。
> 使用的资料：本班课程的数学讲义（来源为equation-course.md），讲义中明确等式两边进行相同运算等式仍成立，且该讲义中已给出此方程的解题步骤：先等式两边减3得2x=8，再两边除以2得x=4。

修复后（本轮真实模型完整正文，**待用户审读**）：

> 最终答案是x=4，使用的资料是《equation-course.md》。根据该数学讲义内容，等式两边进行相同运算等式仍成立，对2x+3=11先两边减3得2x=8，再两边除以2得x=4[1]。

原始上游content主动包含合法`[1]`，finish_reason=stop且正常[DONE]；原始content拼接=下游delta拼接=数据库正文，未做补写。之前原始正文没有数字引用；本次引用校验成功。此比较只说明这一次修复后结果，不证明稳定成功率提升或其他场景也通过，也不把旧回答的人工认可当作新回答已审核。

## SSE、引用与数据库

| 项目 | 实际结果 |
|---|---|
| SSE | HTTP200，`meta → delta×62 → done`，无error |
| 时间（客户端） | meta约0.564秒、首delta约2.320秒、done约2.715秒，真实增量可见 |
| 服务端顺序 | meta Flush=1792309468300744913ns，Model.Open=1792309468300745813ns，首delta=1792309470057053435ns，done=1792309470441833779ns |
| meta候选 | status=ok，1条：number1、equation-course.md、chunk_index1 |
| done最终来源 | 只含原候选[1]，没有重新编号或虚构来源 |
| 实际chunk_id | `2bdab629-7795-4d08-b0ae-131e3d2b95b7`，与修复前候选一致 |
| 数据库 | 新会话42、assistant completed、实际技能为空、citations=[原number1来源]，active_turn释放 |
| 持久化 | 每次收到delta直接查库，前缀已落库；全文与raw及delta相同，Finish后done |

**本场景技术验收通过。** 本轮为真实Go HTTP→真实课程模型→独立MySQL/Qdrant检索/持久化，没有模型替身冒充；未运行或声称新的浏览器/Nginx验收。

## 脱敏证据

根目录`runtime/align-real-26bcce67/`保留修复前证据，新增：

- `citation-retest-b8478118/result.json`：完整新回答、事件/时间、配置、数据库消息、引用及技术断言。
- 同目录`budget-before.json`：本轮前2/3账本原样备份；`answers-for-review.md`：前后完整回答及人工审读状态。
- 原`budget.json`序号6/7：本轮真实Embedding/对话请求、完整模型输入、HTTP、usage及累计3/4；前5个请求记录保留。
- `upstream-06.txt`、`upstream-07.txt`：真实查询向量和上游流；同目录`response.sse`为下游流。
- `citation-retest-b8478118-fixture.log`、`citation-retest-b8478118-run.json`：Go真实测试通过、资源标识；容器自然退出且保留，不删除。

本轮仅增加独立单场景测试`phase5_citation_retest_integration_test.go`及现有run_align_real.ps1的显式SingleCitationRetest模式。既有整套矩阵仍受旧入口预算保护；本轮模式只匹配单个测试，不执行它。业务提示词、模型客户端、引用校验、协议、数据库schema及部署配置未修改。

最终范围、原历史/配置hash、原账本追加一致性及strict结果见phase5-citation-retest-check.json；实施前基线见phase5-citation-retest-baseline.json。真实证据不覆盖旧回答，原Week5的32/32保持。没有commit/push/部署/归档或资源清理。

本轮完成后立即停止，等待用户审读新回答并决定是否继续其他场景；5.3/5.4/5.5保持未勾选，26/29。
