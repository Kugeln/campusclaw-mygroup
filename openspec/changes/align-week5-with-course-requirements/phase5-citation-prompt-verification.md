# 5.3 数字引用提示词最小修复（仅受控验证）

日期：2026-10-18（Asia/Shanghai）。本轮只授权代码最小修复及受控测试，**不授权真实模型调用**。change仍26/29；5.3未完成，5.4/5.5不执行。用户已审读首次真实回答，确认数学结论/步骤正确、无技能直接回答不违规，唯一确认的格式缺陷是缺少合法数字引用；本修复不能证明原提示词是唯一根因，也不能保证模型必然服从。

## 实际修改与前后差异

唯一业务文件为`internal/tutor/tutor.go`：新增包内常量evidenceCitations（第27行），仅在Prepare完成材料授权/top4/subject/空正文过滤后，`len(p.Evidence)>0`的else分支追加至System（第263–264行）。因此过滤归零也不会追加。DefaultRole、原safety和无据System原文保持。

修复前有据System为：原safety（包含“使用 [1] 等真实来源编号”）+教师角色，之后按原逻辑追加选中技能及教学边界。

修复后仅在上述有据System追加如下原样规则，之后仍按原逻辑处理技能：

> 使用本轮资料作答时，必须在相关结论或说明后标注对应的数字引用，例如‘等式两边进行相同运算仍保持相等[1]’。编号必须取自本轮evidence中的number字段；number=2对应[2]，不是chunk_index。仅写材料名称不能代替数字引用。只引用实际使用的证据，不得编造、重编号或为凑格式引用未使用的资料。

证据仍在User JSON中，字段`number/title/chunk_index/chunk_text`、连续编号、800字截断、历史窗口和当前问题组装均未修改；System未加入材料正文。没有修改shared ValidateSources或ValidateAnswer，没有后处理自动补写引用，没有修改Week4 Ask、无据生成、SSE处理器、事务或部署配置。

## 新增测试

新增`internal/conversation/citation_prompt_test.go`，`TestWeekFiveEvidenceCitationPromptAndIdentities`（第13行）。测试贯穿真实Tutor.Prepare→ModelMessages→ValidateAnswer纯核心，全部为内存输入，不访问模型或数据库：

- 有据System包含指定的强制引用、number→[n]映射、不能以标题替代、不能虚构/重编号等规则。
- 候选number为1/2，实际chunk_index故意为7/23；检查User JSON的四个字段、编号/切片/正文及当前问题保持。
- 标题`equation-course.md`出现而无[n]仍拒绝；[1]通过且映射chunk7，只引用[2]保持number2/chunk23。
- [7]/[23]误用切片号、[0]、[3]、[999]、[x]、全角［1］、合法与虚构混合[1][999]均拒绝。
- 无据System不包含追加的强制引用要求，仍禁止虚构数字引用；无据普通回答允许空来源，Prepared.Answer为空，保留后续生成入口。

## 本轮实际命令与结果

| 命令 | 结果/证据 |
|---|---|
| `go test ./internal/tutor ./internal/conversation ./internal/rag ./internal/http ./internal/auth -count=1` | 全通过，包含新增测试、Week4三模式/RRF/阈值/Ask零命中固定文案及零模型调用、权限和流协议；runtime/citation-prompt-unit.log |
| `node --test tests/go_rag/sse.test.cjs` | 3/3通过；runtime/citation-prompt-sse.log |
| `run_align_storage.ps1 -Stage Phase2` | 通过，10.335秒；9个顶层测试，真实独立MySQL新随机库+受控Embedding/向量/模型；CRUD/路由/空材料/subject/默认角色/窗口/Week4保护；runtime/citation-prompt-phase2.log |
| 同runner `-Stage Phase3Core` | 通过，8.275秒；8个顶层测试，真实独立MySQL新随机库+进程内/本地HTTP受控依赖，四组合均生成；meta/delta/done/error、缺失/越界引用失败、先落库后delta、Finish后done、SQL故障/取消/中断/恢复/丢done/并发及DB状态；runtime/citation-prompt-phase3core.log |
| `openspec validate align-week5-with-course-requirements --strict` | 通过；不代表真实模型复验或人工验收通过 |

只使用已确认的独立tmpfs MySQL `campusclaw-align-storage-17d5f822`及internal测试网络；每例CREATE全新随机库，未读取课堂库或复用历史业务库。Phase3Core明确排除真实Qdrant联测，**本轮没有访问任何Qdrant**。新测试容器/数据库和日志保留，没有清理资源。此前一次只读Docker格式化命令因PowerShell引号失败；随后用只读JSON检查确认Running/align-storage标签/tmpfs/无持久挂载，不把格式化失败当环境验收。

## 安全边界与剩余风险

本轮真实对话0、Embedding0；此前5.3累计2/12、3/8保持，旧budget.json、teaching.json与真实回答未改写。原Week5 change32/32、四份spec、Phase0–5历史记录保持hash。没有使用或加载真实网关凭据，没有执行realmodel标签或5.3真实执行器，没有commit/push/归档/部署。

明确映射和示例能消除表达歧义，但受控测试只证明提示词进入正确分支、输入及校验/协议不变，**不能保证course-chat真实回答遵循，也不能确认成功率提高**。仍可能缺引用、引用不存在编号或引用与事实不符；原拒绝逻辑继续兜底。真实引用语义、教学效果和完整5.3矩阵仍须单独授权后在剩余预算内复验，并由用户人工审读，不能将本轮通过当作5.3通过。

实施前基线见phase5-citation-prompt-baseline.json，最终hash/命令/范围核对见phase5-citation-prompt-check.json。本轮完成即停止，5.3/5.4/5.5保持未勾选，等待用户单独授权真实复验。
