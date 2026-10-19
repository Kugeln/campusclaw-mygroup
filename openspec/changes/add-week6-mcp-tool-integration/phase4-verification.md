# Phase 4 实施与最终验收（2026-10-18至19，Asia/Shanghai）

结论：**4.1–4.7全部完成，累计30/42**；Phase 5–6共12项未勾选，已停止等待下一次授权。本次继续原change，保留72 Requirement/72主Scenario、官方8管理操作和42任务，未改UI、默认部署、旧41DDL或v4结构。

[机器证据](phase4-verification.json)包含源文件及全部原始运行证据SHA256；[实施前保护基线](phase4-preservation-baseline.json)保留分支、HEAD、完整dirty工作区清单和逐文件摘要。

## 逐任务与追踪

| Task | 实现及测试证据 | 结论 |
|---|---|---|
| 4.1 / T34,T35,T43 | ParseSkillSource调用纯ParseReferences；六分隔符、两段32字符正则、工具禁连字符/服务允许、空片段/重复/最多3个MCP；旧空工具/knowledge_search、UTF8/长度/原文/toggle回归 | 完成 |
| 4.2 / T36–T39 | 保存及启用时事务内锁定本班同名connected服务及缓存、检查schema；混合引用全拒绝、跨班/不存在同400、失败source/hash/version/revision不变；无效绑定仍能专用toggle停用 | 完成 |
| 4.3 / T40–T45,T52 | 不改Capture→Route最高词分/小ID规则；仅当前选中启用绑定技能，从同一source快照提取计划；逐调用重读当前class/name/cache/schema，返回后revision复核；断开/移除工具denied有审计，旧名失配无审计；同名替换不归错审计 | 完成 |
| 4.4 / T48,T56 | formula_hint.query为当前问题，不用历史扩展query；unit_convert确定性数值/单位解析，歧义/多项/异维/未知适配拒绝；最多3项声明顺序串行，RPC≤3秒、阶段10秒、共享原轮次≤120秒，无call重试 | 完成 |
| 4.5 / T47,T49–T54,T60 | Start后、Prepare持久化/meta前执行并追加终态审计；可选meta.tool_calls恰ref/status/text；无匹配/停用/解绑/仅knowledge_search省略；安全快照存原configuration_snapshot，读取和恢复不重放 | 完成 |
| 4.6 / T55,T57–T59 | 完整合并text先UTF8/危险模式/1200字校验，再脱敏；1199/1200通过、1201/多块超限/危险句开中尾/大小写/伪数字引用全丢弃；已知服务/主密钥/JWT签名/模型密钥及Header/JSON凭据脱敏，脱敏膨胀也整体拒绝；只作为user外部数据 | 完成 |
| 4.7 / T46,T61 | 真实MCP两工具/SQL/HTTP/受控模型完整链；原Ask零据零模型、技能/RAG/SSE回归；409、工具取消/超时、审计INSERT失败503前meta、配置竞争、delta断连前缀、done丢失、Recover不重放 | 完成 |

## 冻结合同与实际接入

新mcp.Runner仅接受当前选中技能的已验证文本引用，不能读取学生或模型输出中的工具名来扩大计划。Capture/Route与Start的快照语义保持：教师在已接受轮次中修改技能只影响下一轮；MCP服务器配置则每次调用实时校验，晚到结果不能冒充新配置成功。不引入稳定ID技能表，不改旧source/hash，也不重写改名/删除后的旧引用。

保存/更新都校验MCP引用，启用toggle也校验；仅停用toggle可用于无效旧绑定。调用网络不持有数据库事务。审计最终锁定同一服务器ID/revision、与disconnect保持一致；审计与删除统一先锁主记录，修复实际回归捕获的名称索引/主键反序死锁。原删除竞态断言未放宽，已扩至20轮。

参数不经模型：formula传当前问题原文（沿用既有Question规范化），unit支持明确单项“数值 单位 转/换算为/to/= 单位”，合法示例1 m转cm→100 cm；其他工具虽可被语法/发现识别，缺少项目确定性适配时denied，不能让模型补参数。schema按适配所需子集闭合检查，未知验证关键词拒绝，不宣称完整JSON Schema实现。

Call阶段沿用原120秒父context；新增10秒子context和既有3秒RPC；三次超时约9秒后模型只剩原轮次约111秒，不重新开始120秒。超出阶段/轮次deadline时不延长审计期限，写不成返回503、failed且不发meta；明确客户端取消可做最多1秒的终态恢复写，随后沿用原Finish恢复机制，不延长生成预算。原Nginx120秒配置、heartbeat、Append先于delta、completed先于done保持；本轮未执行Phase 5的浏览器/代理端到端验收。

## 安全结果、模型与SSE

完整文本先扫描再脱敏，危险/过长正文不保留任何前缀片段；替换“工具返回内容未通过安全检查，已忽略。”，记error/安全原因。数字[1]/[99]等工具伪引用也整条拒绝。成功审计仅保存安全文本前200字；失败/拒绝为固定安全原因，不记参数或敏感原载荷。上游401/403转工具error，学生JWT仍有效，不把远端HTTP状态传作学生认证失败。

只有ok外部结果进入当前user JSON的external_tool_results，包含ref/text及“[工具 ref 返回的数据]”来源标签；与question/evidence分开。system只增加服务端静态不可信数据提示，不拼接工具正文；结果不创建number/material_id/chunk_id。保留32000字总输入、6条历史/500字、证据800字限制及原引用校验；空证据仍生成，数字引用过滤继续只看真实RAG证据。

实际meta（摘自真实Phase 1服务，非模拟清单）：

```json
{"status":"ok","citations":[],"skill_name":"数学工具","message_id":2,"tool_calls":[{"ref":"formula_hint@math-demo","status":"ok","text":"去括号可用分配律 a(b+c)=ab+ac。先检查括号前的系数是否乘到每一项，再检查负号引起的符号变化。"},{"ref":"unit_convert@math-demo","status":"ok","text":"1 m = 100 cm"}]}
```

无MCP尝试时tool_calls属性不存在；denied/error也各有ref/status/text，meta.status仍ok。安全状态写入消息现有configuration_snapshot.tool_calls；旧消息缺字段兼容。刷新、Get、Recover不调用工具；delta写失败保留已落库前缀并interrupted，done传输丢失仍可补读completed及安全快照。

## 原始证据与测试统计

最终运行：[result.json](../../../runtime/week6-phase4-20261018T233731Z-6247b3b7/result.json)、[integration.stdout](../../../runtime/week6-phase4-20261018T233731Z-6247b3b7/integration.stdout)、[compatibility.stdout](../../../runtime/week6-phase4-20261018T233731Z-6247b3b7/compatibility.stdout)、[week5-storage-regression.stdout](../../../runtime/week6-phase4-20261018T233731Z-6247b3b7/week5-storage-regression.stdout)。[真实两工具/模型输入/历史](../../../runtime/week6-phase4-20261018T233731Z-6247b3b7/real-tool-chain.json)；[工具→审计→meta→模型输出时序](../../../runtime/week6-phase4-20261018T233731Z-6247b3b7/tool-meta-order.json)。故障及竞态的安全HTTP响应另存同目录TestMCPConversation*.json，不保存请求凭据。

| 最终组 | 叶级通过 | 失败 | 跳过 |
|---|---:|---:|---:|
| MCP管理/工具会话/客户端/安全集成 | 168 | 0 | 0 |
| 默认Go兼容回归 | 279 | 0 | 0 |
| 原第五课真实MySQL技能/RAG/SSE回归 | 57 | 0 | 0 |

共504次叶级执行，81项重复，去重 **423项通过、0失败、0本阶段必需未执行**；Go vet退出0。按package+叶级test去重，不把父汇总或package无test files计入。最终运行全源码SHA256开始/结束及当前文件一致。

核心正常验收使用独立Phase 1 Docker服务真实formula_hint/unit_convert；受控MCP仅用于非法输出/Schema、鉴权失败、超时、取消及竞态。模型为本地受控HTTP流，验证输入角色/边界和事件顺序；**真实付费模型未执行**，留Phase 6明确授权。原Ask/RAG与SSE故障回归采用原有测试，向量/embedding/model受控，无现有Qdrant或Demo连接。

五轮记录均保留：

1. 20261018T142109Z-da5ee111：136通过、23失败；旧fixture误把技能revision判成内部MCP字段，且子测试文件名含路径分隔符；修正夹具，未删除失败证据。
2. 20261018T142536Z-c2362186：166/279/57全部通过；随后最终复核补边界处理。
3. 20261018T143005Z-6c0dfd60：跨夜宿主时钟跳变，Python出现负值TimeoutExpired；命令未完整返回，不能算通过，历史计数不补造；临时资源清理完成。runner后来补充超时部分输出留存。
4. 20261018T233345Z-f5e67fa5：Phase 4用例通过，旧审计/删除竞态1项失败；修正实际锁顺序后重验。
5. 20261018T233731Z-6247b3b7：最终全部通过、源码未在测试期间改变、资源清理完成。

## 文件清单与保护

修改13个既有代码/生成文件：

- cmd/server/main.go：运行编排和已知密钥脱敏接线。
- internal/tutor/skill_source.go、tutor.go：工具引用、准备校验及安全快照。
- internal/repository/mysql/tutor.go、mcp.go、conversation.go：原子保存/启用、终态复核/锁顺序、安全配置快照持久化。
- internal/mcp/client.go、security.go：区分上游鉴权失败，公共管理错误合同不变。
- internal/http/conversation.go：meta前执行及可选tool_calls，原轮次预算上限。
- internal/conversation/stream.go：user外部数据与静态边界说明。
- internal/http/openapi_types.go、web/swagger/openapi.json：必要后端可选字段类型与原流程生成产物；未修改Swagger UI。
- internal/http/mcp_integration_test.go：测试fixture允许独立p4项目、合法技能revision/子测试路径处理，竞态20轮，原断言保持。

新增6项：internal/mcp/tools.go、tools_test.go；internal/repository/mysql/mcp_runtime.go；internal/http/mcp_conversation_integration_test.go；tests/go_mcp/compose.phase4.yml、run_phase4.py。

本change更新tasks.md、design.md、requirements-traceability.md、review-summary.md、risks-and-open-questions.md、acceptance-matrix.md、verification-result.json；新增phase4-preservation-baseline.json、phase4-verification.md/json、phase4-docker-closeout.json、phase4-final-checks.json。审核旧文档只补当前状态，72合同正文不动。

基线分支feature/week5-phase1-go-rag，HEAD 51a8749f6550c279efebe63bb67eda724878f5a1；保留全部原dirty文件，未暂存/提交/push/merge/rebase/archive。除本轮授权19项及本change审核记录外无变化；旧OpenSpec、旧DDL/catalog/v4结构、独立mcp-server、页面/JS、默认Compose/.env.example/网关、原认证/RAG代码均保持摘要一致。会话/技能必要改动属于本轮授权增量，不宣称这些文件完全没改。

[Docker收尾](phase4-docker-closeout.json)核对五个唯一项目均无容器/网络残留、213个既有容器生命周期未变。独立非重叠10.241子网、内部网络、无宿主端口/外部卷、MySQL tmpfs、源码只读；仅清理本轮项目，保留唯一标签测试镜像/构建缓存，不改既有镜像。无正式库迁移、现有数据读写或真实模型调用。

## 最终门槛及遗留风险

[最终检查](phase4-final-checks.json)：strict、OpenAPI生成Check、72/72/8/42计数、源文件及保护检查全部通过。

危险模式只覆盖冻结示例，属于尽力防护；不保证识别所有注入。名称复用仍可能让后续轮次的旧文本指向后来同名服务，本次只修复进行中调用的错误审计归属，不宣称消除风险。远端执行成功、终态INSERT前崩溃仍可能缺审计；revision拒绝旧结果不能撤回已开始的远端动作，不自动重放，也不声称exactly-once。原认证存储500与MCP业务503的差异保留。审计写失败不虚报成功，不新增清理或保留天数。

Phase 5前置条件满足，但**本轮立即停止**；UI、默认部署、完整Swagger页面/Authorize及Nginx浏览器验收均等待下一次明确授权。
