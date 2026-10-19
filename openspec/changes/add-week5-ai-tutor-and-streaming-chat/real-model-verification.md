# 真实模型教学与端到端复验（2026-10-14）

本轮只处理2.4、6.5。feature/week5-phase1-go-rag保留全部未提交修改；未提交、推送、部署或归档，未启动/修改课堂Demo、正式数据库或持久卷。

## 配置与真实请求

当前进程、用户/机器环境及仓库本地.env无真实模型配置；只读核对历史campusclaw-retrieval-demo-app-1后复用其火山Ark配置，不更换供应商。API Key仅临时注入测试进程，未输出或保存到日志/报告/Git；本轮含临时环境的测试容器自动移除，旧Demo未变动。

Embedding：doubao-embedding-vision-251215，ark_multimodal、/api/v3/embeddings/multimodal、2048维。对话：doubao-seed-2-0-lite-260428，/api/v3/chat/completions、stream=true、max_tokens=800。使用现有Go客户端和默认模型思考行为，未替换客户端或修改教学业务代码。协议参考[官方向量化](https://docs.volcengine.com/docs/ark/vectorization?lang=en&redirect=1)和[官方文本生成](https://docs.volcengine.com/docs/ark/text-generation?lang=en)。

先执行一次真实Go Embedding小文本预检，HTTP200、实际2048维，0.81秒；记录runtime/week5-real-model-6e6f08ab/embedding-preflight.json。无自动重试。随后真实端到端7次Embedding、4次对话，全部HTTP200、无网络错误；合计8次Embedding、4次对话。限额为端到端最多8次Embedding/4次对话，网络/上游HTTP失败会锁止后续模型请求。未查询或推断账单金额，没有购买资源。

## 2.4 内容审查

新测试讲义包含2x+3=11、等式性质及完整x=4答案，使模型确实拥有最后结果，但由技能约束决定是否披露。教师保存提示词要求“学习提示：”开头、耐心分析、简短并引用材料。三轮启用、第四轮关闭，都由学生浏览器使用同一私有会话发起。

| 场景 | 首次可见/完成秒 | 实际回答与审查 |
| --- | --- | --- |
| 直接索要最终答案 | 10.015 / 10.140 | 梳理已知和目标，询问已尝试步骤，提示两边减3；不披露x=4 |
| 请求解题提示 | 7.969 / 8.375 | 提示利用等式性质先移去常数3，不直接完成全部解法 |
| 连续追问已得2x=8 | 6.985 / 7.891 | 提示考虑如何消去系数2，使x单独留在一侧；不披露最终数值 |
| 关闭技能，同题完整求解 | 完成8.016 | 给出减3→2x=8→两边除2→x=4，回答方式明显改变 |

已逐条阅读实际文本并查看浏览器截图，不仅依据状态码或关键词。四轮遵守教师前缀并有[1]引用；前三轮skill_name为解题引导，第四轮为空。历史消息248/250/252/254均completed、prompt_revision=2，前三轮的历史技能不因开关更新而改变。完整脱敏问题、回答、状态、耗时及来源保存在browser.json。

## 6.5 真实端到端证据

复用campusclaw-week5-rag-ee706bec MySQL/Qdrant与内部网络；只新增本轮资料、独立2048维collection、助手和会话。新测试Go进程连接现有bridge出站及隔离网络，Nginx测试代理仅loopback端口；未改变Docker全局配置、旧网络或课堂服务。浏览器使用已有Go内嵌页面和Bearer API，真实Embedding→Qdrant→本班回表→快照→Doubao增量流→MySQL持久化及引用展示。

完成会话四轮、引用标题/切片/原文展示、历史实际技能读取；B班读取A班会话与资料404，B班真实vector检索无A班命中。绑定本轮尚未索引材料的助手检索无合法ready证据，返回精确“资料中未找到相关内容”，对话计数不增加；该无证据范围是助手合法材料限制，不把历史引用或常识当新证据。空问题400。真实运行没有故意制造供应商失败或额外收费请求；模型超时/失败、断连、事务故障、崩溃及done丢失由受控Go故障回归单独证明。

机器记录：runtime/campusclaw-week5-rag-ee706bec/realmodel-034a21a3/的browser.json、real-stats.json、result.json和real-teaching.png；exit_code=0、real_model_verified=true、schema_upgrade_executed=false。Key/Header/请求体未被计数器写入。首次启动脚本参数顺序错误导致误把容器名当镜像，启动失败且无模型对话；修正并设置--pull=never后使用已有镜像成功，该失败未算通过。

## 回归、范围与风险

本轮`go test ./internal/config ./internal/http ./internal/rag ./internal/conversation ./internal/tutor ./internal/auth ./internal/repository/mysql ./cmd/server`全部通过（部分缓存）；Node SSE解析3项通过。另执行`powershell -NoProfile -File tests/go_rag/run_phase3.ps1`，三个流解析/预算测试及真实隔离MySQL/Qdrant会话故障综合测试通过（6.854秒），记录phase3-a1ae0f84.json；没有增加真实模型请求。受控23接口+Go故障24/24记录仍独立保留在Phase 6；本轮真实请求不假装覆盖供应商故障注入。

`openspec validate add-week5-ai-tutor-and-streaming-chat --strict`退出0；`git diff --check`通过，仅LF/CRLF提示；受保护原文件及其他change哈希差异0。对真实证据JSON/日志及本记录扫描实际密钥，匹配数0；runtime证据被Git忽略。已只读确认临时真实模型测试容器移除及原课堂Demo仍保持退出状态。

新增internal/rag/real_model_integration_test.go、internal/http/real_budget_test.go、tests/go_rag/check_real_tutor_browser.py。仅扩展tutorbrowser测试夹具和阶段运行脚本的真实模式/限额/临时网络接入，以及本change任务和本记录；没有修改认证、检索、提示词、技能、会话或SSE业务算法。

当前32/32完成。教学证据限定本模型版本、此固定有证据题及三种提问，不保证所有题目/模型版本均严格遵守引导；默认深度思考使本次首段约7–10秒，虽为真流式，课堂体验应据实呈现。没有自动进入课堂Demo部署或归档，停止等待安排。
