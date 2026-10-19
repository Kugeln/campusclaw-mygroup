# Phase 6 自动化验收（2026-10-14）

先完成Phase 5浏览器验收，再实施apitest。当前分支feature/week5-phase1-go-rag；未切换分支、提交、推送、部署或归档，未修改课堂Demo、正式数据或其他change。原受保护文件哈希差异0。

## 交付与通过结果

apitest/cases按URL/方法提供23个独立脚本；apitest/support.py负责隔离配置、fixture、HTTP/SSE、只读SELECT与断言，各接口脚本自行定义角色、内容、状态及数据库场景，可单独运行。apitest/run_all.py按ORDER执行并生成本地脱敏HTML和机器结果。apitest/README.md列出环境变量与一键运行方式，不含真实凭据。

`powershell -NoProfile -File tests/go_rag/run_phase4.ps1 -Stage phase6`最终退出0。实际执行Python3.12容器内`python /src/apitest/run_all.py`，23接口+1组本轮Go故障补证，24/24通过。

HTML：[本地报告](../../../../reports/apitest/20261014T031750Z-f70d54ce/index.html)，同目录result.json。报告及runtime临时文件均被Git忽略。实际服务为新测试Go进程→真实隔离MySQL/Qdrant，HTTP经过复用生产Nginx配置的独立测试代理；模型/Embedding为受控HTTP网关，无真实外部模型调用。

专项证据：

- A/B教师学生登录、当前身份、学生写入403、同班A1/A2及跨班会话404；材料/助手/技能/会话列表与当前MySQL owner/class集合一致。
- 材料上传、原文/附件、三模式检索、真实引用与DB文本对应；keyword零Embedding，hybrid故障503不降级。非法重建不删旧索引，合法重建新UUID与ready状态；索引失败仍201且原文可读。
- 单次ask保留question/mode和answer/sources；无命中固定文本及零对话调用，非法引用502；不新增会话消息。
- 教师prompt/技能保存及revision断言，学生与跨班写入拒绝无变更；生成中修改配置，旧消息保留旧prompt/技能，下一轮使用新配置，客户端覆盖400。
- 正常SSE meta/三段delta/done，首delta约0.091秒，结束约3.113秒；首段到达时上游尚未完成，DB已有generating前缀。并发409没有新增消息，同内容普通重发两turn、四消息；引用与done/历史一致。
- 无证据精确“资料中未找到相关内容”、空实际技能/引用且零模型调用；模型流中错误/非法引用error无done，failed落库；断连interrupted且保留前缀；流前依赖失败503，无孤儿消息。
- 同题启停的系统输入和受控输出有区别，只属于受控机制证明，不能代替真实教学效果。
- Swagger文档脚本检查实际资源、23路径及安全/SSE契约；引用本轮Phase 5真实浏览器Authorize/流式调试证据，不仅检查HTML标题。

本轮补证`runtime/campusclaw-week5-rag-ee706bec/phase3-a03c8ab8.json`及日志：真实MySQL/Qdrant、受控上游的Go综合测试5.393秒通过，另三个流解析/预算测试通过。包括真实MySQL初始事务提交拒绝、片段/终态保存故障、模型超时、取消、已提交generating恢复、completed后done丢失不回退、转班隔离、重建历史引用及重发。没有把Flask历史或OpenSpec验证算作Go业务验收。崩溃是构造已提交状态并恢复，不是主机断电；片段/终态故障为存储接口注入，初始事务故障在真实MySQL连接上拒绝提交。

## 失败与运行器验证

第一轮因Windows cryptography被挂载到Linux而阻塞，改为只挂载纯Python PyMySQL后执行；第二轮21/24通过，三项失败来自控制网关JSON未声明Content-Type，修正夹具后完整重跑24/24。失败报告保留，未算通过；没有修改业务接口以迁就测试。

`.venv/Scripts/python.exe apitest/run_all.py --inject-failure`退出1、生成失败HTML，不请求业务；无必需配置执行run_all.py也退出1并列缺失项。对应本地报告20261014T031513Z-6d7d3ace及20261014T031513Z-eb6eb56a。

`go test ./internal/http ./internal/auth ./internal/rag ./internal/tutor ./internal/conversation ./internal/repository/mysql ./cmd/server`全部通过（部分缓存）。`openspec validate add-week5-ai-tutor-and-streaming-chat --strict`退出0，变更有效；`git diff --check`通过，仅LF/CRLF提示；报告中测试密码和JWT扫描均为false，Git忽略HTML及runtime结果。

## 安全与范围

复用campusclaw-week5-rag-ee706bec隔离网络、MySQL、Qdrant；仅新增本轮测试进程、材料、助手、会话和专用collection。DB断言只运行SELECT，没有迁移、清空或重建数据库。测试网关控制仅位于tutorbrowser构建标签夹具的独立8090模型网关，不是业务测试后门，不进入默认Go服务或OpenAPI。

最终环境记录runtime/campusclaw-week5-rag-ee706bec/phase6-847463a1/result.json和fixture.log，Go夹具与Python退出0，schema_upgrade_executed=false。报告不保存Token、密码、API Key或完整模型请求；官方差异清楚列出，不承诺全量官方脚本原样兼容。

新增apitest/__init__.py、support.py、run_all.py、README.md及23个cases脚本。修改tests/go_rag/run_phase4.ps1增加Phase 6调用，以及internal/http/tutor_browser_integration_test.go增加仅测试构建启用的网关计数/故障控制、独立A2和真实上传索引。没有修改检索、问答、会话或认证业务实现；规约变动限定本change的任务、差异状态和本轮验收记录。

## 当前任务状态

6.1–6.4及6.6通过。6.5的Go单元、隔离数据库/向量服务及受控网关回归通过，但任务还要求真实模型教学复验；用户本轮明确不实施2.4，因此6.5整项保留未勾选。整体30/32，剩余2.4、6.5，停止等待用户安排真实模型联调。
