# Phase 3 恢复实施与验收（2026-10-14）

后续补验：3.5、3.7已在phase3-supplement-verification.md通过专项验收，Phase 3当前全部完成。本文以下表格与计数保留为首次恢复时的历史记录，以补验记录及tasks.md为当前状态。

在 feature/week5-phase1-go-rag 保留中断前全部修改继续实施；未切换分支、提交、推送或归档，未进入 Phase 4–6。复用 campusclaw-week5-rag-ee706bec 的隔离网络、MySQL、Qdrant及缓存卷，未迁移数据库、重建服务或修改课堂 Demo。原受保护文件与其他 change 哈希核对无差异。

## 实施结果

学生接口包括 GET/POST /api/conversations、GET /api/conversations/{id}、POST /api/conversations/{id}/messages。复用 Bearer 身份，服务器校验 owner 与当前 class；教师不能读取私有会话，客户端不能覆盖配置、历史或班级。

配置先原子捕获并选择技能，再检索；合法证据确认后加载指令。保存教师配置、实际技能版本/hash及证据快照。仅使用最近六组完整成功历史，历史预算6000 Unicode字符；模型输入预算32000字符，历史按完整轮次裁剪。

上游以 stream:true 返回真实增量 SSE，解析 UTF-8、delta 和 [DONE]；下游使用 meta/delta/done/error，并发送心跳。消息对事务提交后才推送 meta，各片段保存后才推送 delta，completed 与引用提交后才推送 done。失败保存 failed，断连取消上游并保存 interrupted；启动恢复保留已提交前缀，不覆盖 completed。无证据保存精确文本“资料中未找到相关内容”，实际技能及引用为空，不调用模型。原 /api/ask 保持非流式。

## 逐项状态

| 任务 | 状态 | 证据与限制 |
| --- | --- | --- |
| 2.3 | 补验通过 | 第一轮生成期间更新提示词、关闭技能；旧消息快照不变，下一轮读取新配置；客户端覆盖400 |
| 2.4 | 未验证 | 受控请求组合通过；没有真实外部模型配置，无法证明实际教学效果 |
| 3.1 | 通过 | Bearer、学生/教师、A1/A2/B1隐私、转班拒绝、列表与重新打开数据库读取 |
| 3.2 | 通过 | 复用 tutor 顺序、历史预算测试、同次请求包含提示词/技能/历史/当前证据 |
| 3.3 | 通过 | 上游首段由门控提前释放，客户端收到后才允许网关完成；协议故障、超时与取消 |
| 3.4 | 通过 | meta/三段delta/done顺序；数据库先有前缀；UTF-8跨写入解析；初始提交拒绝无孤儿消息 |
| 3.5 | 部分通过，未勾选 | 数据库真实引用、非法编号拒绝、无命中零调用通过；索引重建后历史快照专项未执行 |
| 3.6 | 通过 | 模型失败、超时、断连、片段/终态写入失败；恢复及completed后done丢失不回退 |
| 3.7 | 部分通过，未勾选 | 并发409、无额外消息、结束/恢复释放占用通过；同内容重发新turn专项未执行 |

## 测试证据

`powershell -NoProfile -File tests/go_rag/run_phase3.ps1`：退出0，三个顶层流解析/预算单元测试及一个综合隔离MySQL/HTTP测试通过。综合测试约3.947秒。机器结果：`runtime/campusclaw-week5-rag-ee706bec/phase3-4bfbbdc0.json`；脱敏日志同目录 `phase3-4bfbbdc0.log`。controlled_tests_passed=true、schema_upgrade_executed=false、real_model_verified=false。运行记录仅在Git忽略的runtime内。

`go test ./internal/conversation ./internal/http ./internal/repository/mysql ./internal/tutor ./internal/rag ./internal/auth ./cmd/server`：全部通过（部分缓存），包含原认证、材料、检索/引用相关单元回归。

`git diff --check`：通过，仅行尾转换提示。严格规约验证结果以本轮最终命令为准。

本轮集成使用真实隔离MySQL及受控HTTP模型网关；检索复用Go hybrid服务，向量分支使用空结果测试适配器，未重新执行真实Qdrant端到端验收，不以此替代Phase 1证据。故障恢复通过构造已提交generating状态、运行恢复并重新打开数据库连接验证，并非机器断电实验。事务故障在真实MySQL连接上拒绝提交；片段及终态故障通过存储接口注入。未进行浏览器或Nginx验收。

## 本轮文件

新增：internal/conversation/types.go、stream.go、stream_test.go；internal/repository/mysql/conversation.go；internal/http/conversation.go、conversation_integration_test.go；tests/go_rag/run_phase3.ps1；本验收记录。

修改：internal/http/handler.go（会话路由）、cmd/server/main.go（注入与启动恢复）、internal/rag/core.go（共享既有引用校验）、本change/tasks.md。未修改schema、Docker配置、前端或其他change。

## 剩余限制

恢复策略依赖单实例写入与启动恢复，不能未经设计直接用于多实例部署。真实外部模型、索引重建历史专项、同内容重发专项仍待验收；Phase 4–6未实施。当前任务计数16/32完成：Phase 1八项、Phase 2三项、Phase 3五项；剩余16项。停止等待审核。
