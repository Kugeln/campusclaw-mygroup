# Phase 2 实施与验收（2026-10-14）

当前分支feature/week5-phase1-go-rag，未提交/推送/merge。复用现有campusclaw-week5-rag-ee706bec_isolated网络、week5_rag_test MySQL和Go测试镜像；不重新创建或重建服务、不迁移数据库、不修改schema/catal​​og。原受保护文件及其他change哈希仍全部一致。

## 已实施

- Go GET/POST /api/agents、GET/PUT /api/agents/{id}、GET /api/skills、PUT /api/skills/{id}。复用Bearer principal和teacher guard；class_id只来自服务端。学生能读取启用助手和技能元信息，不能新建/修改配置；跨班资源404。教师保存提示词/绑定及更新时间并递增revision，同班名称唯一，UUID材料/技能绑定在事务中验证。
- 一个内置“解题引导”技能，完整指令要求识别已知/目标、一次提示一个思路或步骤、追问理解、禁止最终答案/数值/完整最终解法。教师只启停，source及任意脚本输入拒绝；没有MCP、脚本或Agent框架。
- tutor.Capture在同一只读repeatable-read事务读取助手和技能元信息；PrepareTurn先捕获及选择，再调用现有共享检索，只有合法证据才加载完整指令；无命中固定文本、空实际技能、无模型接口调用。返回Prepared包含配置快照、实际技能ID/name/version/hash及系统输入，可供Phase3保存和调用。其本身不创建会话/消息或调用模型。
- bootstrap在明确RAG_ENABLED=true的初始化中为每班补齐技能元信息，默认关闭；保留旧无归属占位行与已有启停设置。既有库可显式执行 `go run ./cmd/admin -mode tutor-skills`，严格schema检查后只补缺失技能数据；GET不会偷偷写库。HTTP启动不迁移或初始化技能。

新配置API在Go的RAG_ENABLED=true模式接线；关闭RAG时保持原入口。旧/api/ask仍是第四课单次问答，不自动注入教师提示词/技能；学生真正通过新会话使用配置将在Phase3接入，当前不添加对话接口。

## 实际修改文件

新增：internal/tutor/tutor.go、tutor_test.go；internal/repository/mysql/tutor.go；internal/http/tutor.go、tutor_integration_test.go；tests/go_rag/run_phase2.ps1。

修改：internal/http/handler.go；cmd/server/main.go、cmd/bootstrap/main.go、cmd/admin/main.go；本change tasks.md及本记录。未修改Phase1 rag核心、检索/问答处理、schema、Compose、课堂Demo或其他change。

## 测试结果

`powershell -NoProfile -File tests/go_rag/run_phase2.ps1` 退出0：2个技能/快照单元测试＋1个真实隔离MySQL/HTTP集成测试通过；HTTP集成1.84秒。报告 `runtime/campusclaw-week5-rag-ee706bec/phase2-7c3fd915.json`，controlled_tests_passed=true、schema_upgrade_executed=false、real_model_verified=false；脱敏日志phase2-7c3fd915.log。报告仅在Git忽略的runtime中。

验证包括：匿名401、teacher/student写权限、伪造role仍403、A/B读取/修改404及列表不泄露、启停状态和revision、同名400、外班技能/材料绑定404、客户端class_id400、禁止source/非boolean400、拒绝请求DB不变、提示词修改/下一次捕获使用新版本、旧快照不变、重新打开DB连接仍读到持久配置、教师可读关闭助手而学生404。技能测试验证选择先于检索、启用系统输入包含教师prompt和完整技能、关闭无完整技能、无证据保留selected决策但actual为空和固定文本。

`go test ./internal/tutor ./internal/http ./internal/repository/mysql ./internal/rag ./internal/auth ./cmd/server ./cmd/bootstrap ./cmd/admin` 全部通过，部分已有包使用缓存；未重跑Phase1迁移或真实RAG闭环，旧相关单元测试未回归。`git diff --check`通过，仅已有LF/CRLF提示。

## 任务状态与未验证部分

2.1、2.2完成并勾选。

2.3实现了Phase2可独立交付的原子配置快照和实际技能审计数据接口，但完整任务还要求真实同会话历史消息持久化不变；该接口属于未授权Phase3，尚未实施/验收。因此本轮保持2.3未勾选，不用纯快照测试冒称历史消息端到端通过。

2.4静态指令/启停输入及同题准备输入对照通过；当前进程没有DIALOGUE_BASE_URL/API_KEY/MODEL，未读取真实密钥文件、未调用真实模型。没有实测真实回答“不给最终答案”的教学效果，也没有实际网关调用中prompt/技能组合记录，保持未勾选。Phase3接入模型时需延续同一系统约束并独立做受控和真实教学复验。

不把指令文字包含“不给最终答案”等同于真实LLM保证。本轮没有Phase3–6业务代码，没有提交/部署/归档；停止等待审核。后续可先审查配置API和快照接口，再按新的授权接会话审计及模型复验。
