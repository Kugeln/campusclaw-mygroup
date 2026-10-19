> **Phase 6 最终状态（2026-10-19）：42/42，按用户批准的项目调整标准完成。** 6.5/T71采用受控HTTP模型、真实MCP和自动技术复核；6.7/T68采用独立账号模拟交叉验证4/4。未调用真实付费模型、未执行用户人工审阅、未邀请真实同伴，不代表课程原始参与要求已完成。72项技术断言、strict及保护检查通过；下方旧状态及原始要求保留历史时点。详见[最终报告](phase6-verification.md)、[逐项覆盖](phase6-coverage.md)和[用户调整](phase6-user-decisions.md)。已停止，不提交、推送或归档。

> **当前 Phase 4 状态（2026-10-19）：4.1–4.7 已验收，累计30/42；Phase 5–6的12项未实施。** 技能引用、确定性MCP执行、安全输出、meta及历史快照、终态审计和第五课回归见[Phase 4报告](phase4-verification.md)与phase4-verification.json。下方旧阶段措辞保留历史时点；本轮未修改页面、默认Compose或调用真实模型。

> **当前 Phase 3 状态（2026-10-18）：3.1–3.6 已验收，累计 23/42；Phase 4–6 的 19 项未实施。** 八个教师管理操作、真实 MCP 发现、隔离 MySQL/HTTP 和旧技能/RAG/SSE 回归证据见 [Phase 3 报告](phase3-verification.md)与 phase3-verification.json。下方旧阶段措辞保留原时点；本次未接技能工具编排、SSE 或前端。

> **当前Phase 2状态（2026-10-18）：2.1–2.7验收完成，累计17/42，Phase3–6未实施。** 最新实现/测试/保护边界见[Phase 2报告](phase2-verification.md)和phase2-verification.json。下方规划及Phase0/1记录中的未实施或旧进度措辞保留其原时点；跨阶段Requirement的HTTP/UI/SSE部分并未因此完成。调用/审计崩溃窗口和名称复用等既有风险继续披露。

> **当前状态更新（2026-10-18）：Phase 0.1–0.4静态复核与设计冻结完成，4/42；其余38项未完成。** 本次用户指令已明确接受指定设计与A–F工程选择，Q1–Q5及缓存解释的审核门槛已关闭，来源矛盾与已知风险继续披露。下方对齐审核及“待用户审核/0/42”等措辞保留历史时点；当前以[本次基线](phase0-baseline.md)、[设计冻结](design.md)和verification-result.json.phase0_review为准。没有业务实施，下一阶段仍等待明确授权。

# 第六课官方要求对齐审核与双向追踪

本版保留原72个Requirement/72个主Scenario编号；本轮逐条审阅全部72项并同步Test定义。这里的40个O条款是对已读官方强制/兼容/示范/建议/研讨内容的项目审阅索引，**不是宣称老师给出40条强制要求**。最终执行任务42项，业务测试全部未执行，文档待用户审核。

## 一、真实来源访问与完整性

2026-10-18先用web工具仍失败；之后只读直接HTTP请求成功。L与C-index及导航全部9个子页均200，已读取完整main正文、示例代码/表格/流程图文字。导航/分页/正文链接检查没有发现未读取的额外第六课HTML章节。CSS/JS为导航/样式资产，不当独立课程章节。任务清单链接的官方OpenAPI已解析MCP子集，Swagger/原型仅取HTML，**未运行浏览器或业务试调**。未访问不存在的示范源码下载路径，不声称读过project-src/week06-harness实际仓库代码。

早前不可访问及初版pending结论是历史；本轮成功读取后已被本报告取代。没有伪造旧时点的访问。下面hash为HTTP解码后.NET字符串重新UTF8编码的SHA256，不是原始压缩网络包哈希；未把整页拷贝成项目源码，工具会话保留读取结果。

| 来源 | 原始URL | 状态/正文长度 | 读取范围 | UTF8内容SHA256 |
|---|---|---|---|---|
| L | [L](https://devops.hello1023.com/课程任务清单-第6课.html) | 200 / 5945字符 | 六节23个li；每条均读 | f53026a92543e103be6255a0730449b33a5447771e00105330492ba2b21e454e |
| C-index | [C-index](https://devops.hello1023.com/课件/第6课-课件-MCP与工具接入/index.html) | 200 / 4356字符 | §1承接；§2三结论及9个导航链接 | d9e783d8ec39bf42af379e89a0d7fbb6ca547b380f48582cc6ce4985c23b94e1 |
| C-preview | [C-preview](https://devops.hello1023.com/课件/第6课-课件-MCP与工具接入/pages/preview.html) | 200 / 10944字符 | §1旧工具栏；§2HTTP子集/三方法；§3报文和Schema示例 | d5c17e63ab0ebb2a5f0f75efc72e9db12908693a2000980fe3ab6b26e5947901 |
| C-scope | [C-scope](https://devops.hello1023.com/课件/第6课-课件-MCP与工具接入/pages/scope.html) | 200 / 8729字符 | §1七交付及流程图；§2角色；§3八DoD | d1db599a018acb77552aaf07e93b3a33fb7f943d7c21c5d64ddd87bdbf03e929 |
| C-model | [C-model](https://devops.hello1023.com/课件/第6课-课件-MCP与工具接入/pages/mcp-model.html) | 200 / 9766字符 | §1三表/图；§2字段；§3凭据；§4写权限与删除FK503 | b3161763493befd383ac836d8979bdfab3d7ac2961b72fabeba81a85562ff4ed |
| C-binding | [C-binding](https://devops.hello1023.com/课件/第6课-课件-MCP与工具接入/pages/binding.html) | 200 / 7805字符 | §1语法/分隔/例子/图；§2四校验；§3断开改名删除重连 | 1db7ba60294c63bc34945afb5f66b854c4dced216746cc523cba77ff1c703341 |
| C-bridge | [C-bridge](https://devops.hello1023.com/课件/第6课-课件-MCP与工具接入/pages/bridge.html) | 200 / 10478字符 | §1连接五步及图；§2确定性时序/图；§3审计例外 | e4107e9617b81e0008465022abedf4260dfe78372263b82245a398d2c9c59bcc |
| C-security | [C-security](https://devops.hello1023.com/课件/第6课-课件-MCP与工具接入/pages/security.html) | 200 / 7187字符 | §1user来源前缀/图；§2句式/1200/整丢；§3能力/参数/令牌边界 | e2ff8320f17e3a63094c4cbba6829637dc6bee844f850395d9fafda8c4232133 |
| C-audit | [C-audit](https://devops.hello1023.com/课件/第6课-课件-MCP与工具接入/pages/audit.html) | 200 / 7933字符 | §1字段/图/50与100；§2SSE示例/省略；§3八接口/注解 | 25996a0569357f1e0f2f7021e7b22bd61c95ef2ec2d7e4f45a16d30457247229 |
| C-apply | [C-apply](https://devops.hello1023.com/课件/第6课-课件-MCP与工具接入/pages/apply.html) | 200 / 7613字符 | §1全部14步骤；§2全部故障/预期行为；§3四项真实同伴 | 8ed9bd58e2ee8cef81b4d4344193c6abee5d79b4bf8b8cb6c9a227ad16d6c3d0 |
| C-think | [C-think](https://devops.hello1023.com/课件/第6课-课件-MCP与工具接入/pages/think.html) | 200 / 14495字符 | §1HTTP子集；§2校验时点；§3确定性；§4同数据位；§5注入/后续边界；各含观察/含义/替代/图 | bf0deeb860e0cd500785f8abc30b501b6fdf8b940236876db5af31e58b1f5854 |
| OA | [官方OpenAPI](https://devops.hello1023.com/week06/openapi.json) | 200，JSON解析成功 | 全部MCP操作/相关schema；未全审旧课程API | 历史未记录；本次补核hash及完整八操作合同见phase0-openapi-evidence.json |
| SW/PROTO | [Swagger](https://devops.hello1023.com/week06/swagger/) / [原型](https://devops.hello1023.com/week06/) | 200 / Swagger516字符 | 入口HTML；未登录/Execute/业务验收 | 不作业务证据 |

## 二、两官方来源独立覆盖

### L：任务清单23条逐项覆盖

条目编号按六个section内li顺序编写，保留各来源而不是把重复内容抹去。L5标题本身是“接口测试（建议）”，所以不能把HTML/runner说成老师强制。

| L条目 | 内容摘要 | O索引 | Requirement / Task / Test |
|---|---|---|---|
| L1.1 | 同仓mcp-server及Compose，非新仓 | O01 | W6-F01, W6-F07, W6-F13, W6-F67 / Tasks 1.1, 1.4, 1.6, 6.1 / W6-T01, W6-T07, W6-T13, W6-T67 |
| L1.2 | /mcp、/health、环境Bearer及密钥边界 | O02, O03 | W6-F02, W6-F03, W6-F04, W6-F05, W6-F08, W6-F09, W6-F10, W6-F12, W6-F14, W6-F26, W6-F53 / Tasks 1.1, 1.2, 1.3, 1.4, 1.5, 2.4, 2.5, 4.5 / W6-T02, W6-T03, W6-T04, W6-T05, W6-T08, W6-T09, W6-T10, W6-T12, W6-T14, W6-T26, W6-T53 |
| L1.3 | 三方法、401、可读RPC错误 | O02, O03 | W6-F02, W6-F03, W6-F04, W6-F05, W6-F08, W6-F09, W6-F10, W6-F12, W6-F14, W6-F26, W6-F53 / Tasks 1.1, 1.2, 1.3, 1.4, 1.5, 2.4, 2.5, 4.5 / W6-T02, W6-T03, W6-T04, W6-T05, W6-T08, W6-T09, W6-T10, W6-T12, W6-T14, W6-T26, W6-T53 |
| L1.4 | 至少2工具、名称规则、示范可替换 | O01, O04, O39 | W6-F01, W6-F04, W6-F05, W6-F06, W6-F07, W6-F08, W6-F13, W6-F23, W6-F39, W6-F67 / Tasks 1.1, 1.3, 1.4, 1.6, 3.3, 4.2, 6.1 / W6-T01, W6-T04, W6-T05, W6-T06, W6-T07, W6-T08, W6-T13, W6-T23, W6-T39, W6-T67 |
| L1.5 | 真实同网Base URL连接非mock | O01 | W6-F01, W6-F07, W6-F13, W6-F67 / Tasks 1.1, 1.4, 1.6, 6.1 / W6-T01, W6-T07, W6-T13, W6-T67 |
| L2.1 | 教师登记、服务端凭据不回传 | O03, O05 | W6-F09, W6-F10, W6-F14, W6-F15, W6-F17, W6-F18, W6-F26, W6-F28, W6-F53, W6-F63, W6-F66 / Tasks 1.1, 1.5, 2.1, 2.4, 3.1, 4.5, 5.1, 5.3 / W6-T09, W6-T10, W6-T14, W6-T15, W6-T17, W6-T18, W6-T26, W6-T28, W6-T53, W6-T63, W6-T66 |
| L2.2 | 连接握手发现缓存、断开清空 | O08, O10 | W6-F04, W6-F19, W6-F20, W6-F21, W6-F22, W6-F23, W6-F38, W6-F41, W6-F50 / Tasks 1.3, 3.2, 3.3, 4.2, 4.3, 4.5 / W6-T04, W6-T19, W6-T20, W6-T21, W6-T22, W6-T23, W6-T38, W6-T41, W6-T50 |
| L2.3 | 失败可读且主路径不阻断 | O09 | W6-F12, W6-F19, W6-F47, W6-F53, W6-F54 / Tasks 2.5, 3.2, 4.5 / W6-T12, W6-T19, W6-T47, W6-T53, W6-T54 |
| L2.4 | Base URL指自建服务并可见发现工具 | O08 | W6-F04, W6-F19, W6-F21, W6-F22, W6-F23 / Tasks 1.3, 3.2, 3.3 / W6-T04, W6-T19, W6-T21, W6-T22, W6-T23 |
| L3.1 | knowledge_search及tool@server | O15, O17 | W6-F34, W6-F36, W6-F37, W6-F38, W6-F39 / Tasks 4.1, 4.2 / W6-T34, W6-T36, W6-T37, W6-T38, W6-T39 |
| L3.2 | 本班/已连/已发现保存校验 | O17 | W6-F34, W6-F36, W6-F37, W6-F38, W6-F39 / Tasks 4.1, 4.2 / W6-T34, W6-T36, W6-T37, W6-T38, W6-T39 |
| L3.3 | 带令牌call、结果进user数据 | O18, O26 | W6-F40, W6-F43, W6-F45, W6-F47, W6-F48, W6-F56, W6-F57, W6-F59 / Tasks 4.1, 4.3, 4.4, 4.5, 4.6 / W6-T40, W6-T43, W6-T45, W6-T47, W6-T48, W6-T56, W6-T57, W6-T59 |
| L3.4 | meta tool_calls及denied | O21, O22, O28 | W6-F20, W6-F24, W6-F31, W6-F41, W6-F42, W6-F47, W6-F49, W6-F50, W6-F51, W6-F53, W6-F54, W6-F55, W6-F64, W6-F66 / Tasks 2.7, 3.1, 3.2, 4.3, 4.5, 4.6, 5.2, 5.3 / W6-T20, W6-T24, W6-T31, W6-T41, W6-T42, W6-T47, W6-T49, W6-T50, W6-T51, W6-T53, W6-T54, W6-T55, W6-T64, W6-T66 |
| L3.5 | 沿用身份/班级/跨班404 | O06, O37 | W6-F16, W6-F23, W6-F25, W6-F37, W6-F40, W6-F43, W6-F46, W6-F59, W6-F60, W6-F61, W6-F62, W6-F65, W6-F68, W6-F69 / Tasks 2.7, 3.3, 3.4, 4.1, 4.2, 4.3, 4.5, 4.6, 4.7, 5.2, 6.2/6.7, 6.3 / W6-T16, W6-T23, W6-T25, W6-T37, W6-T40, W6-T43, W6-T46, W6-T59, W6-T60, W6-T61, W6-T62, W6-T65, W6-T68, W6-T69 |
| L4.1 | 本班可追踪审计 | O27, O29 | W6-F31, W6-F32, W6-F62 / Tasks 2.7 / W6-T31, W6-T32, W6-T62 |
| L4.2 | 注解OpenAPI/Swagger | O32 | W6-F66 / Tasks 5.3 / W6-T66 |
| L5.1 | 建议：apitest角色与管理 | O34 | W6-F67, W6-F68 / Tasks 6.1, 6.2/6.7 / W6-T67, W6-T68 |
| L5.2 | 建议：连接→技能→会话→ok审计 | O34 | W6-F67, W6-F68 / Tasks 6.1, 6.2/6.7 / W6-T67, W6-T68 |
| L5.3 | 建议：HTML不进Git | O35 | W6-F70 / Tasks 6.4 / W6-T70 |
| L6.1 | 独立服务health/list/call验据 | O02, O03, O04 | W6-F02, W6-F03, W6-F04, W6-F05, W6-F06, W6-F07, W6-F08, W6-F09, W6-F10, W6-F12, W6-F14, W6-F23, W6-F26, W6-F39, W6-F53 / Tasks 1.1, 1.2, 1.3, 1.4, 1.5, 2.4, 2.5, 3.3, 4.2, 4.5 / W6-T02, W6-T03, W6-T04, W6-T05, W6-T06, W6-T07, W6-T08, W6-T09, W6-T10, W6-T12, W6-T14, W6-T23, W6-T26, W6-T39, W6-T53 |
| L6.2 | 主机端到端发现绑定触发审计 | O33 | W6-F01, W6-F09, W6-F15, W6-F19, W6-F20, W6-F24, W6-F26, W6-F31, W6-F36, W6-F49, W6-F50, W6-F51, W6-F62, W6-F66, W6-F67 / Tasks 1.1, 1.5, 2.4, 2.7, 3.1, 3.2, 4.2, 4.5, 5.3, 6.1 / W6-T01, W6-T09, W6-T15, W6-T19, W6-T20, W6-T24, W6-T26, W6-T31, W6-T36, W6-T49, W6-T50, W6-T51, W6-T62, W6-T66, W6-T67 |
| L6.3 | 断开后明确拒绝 | O10, O28 | W6-F20, W6-F24, W6-F31, W6-F38, W6-F41, W6-F42, W6-F50, W6-F51, W6-F53, W6-F54 / Tasks 2.7, 3.1, 3.2, 4.2, 4.3, 4.5 / W6-T20, W6-T24, W6-T31, W6-T38, W6-T41, W6-T42, W6-T50, W6-T51, W6-T53, W6-T54 |
| L6.4 | 两侧令牌一致且无前端/Git/镜像泄漏 | O03 | W6-F09, W6-F10, W6-F14, W6-F26, W6-F53 / Tasks 1.1, 1.5, 2.4, 4.5 / W6-T09, W6-T10, W6-T14, W6-T26, W6-T53 |

### C：全部页面、章节、示例及流程图覆盖

上方来源表逐页列出所有章节；以下保留课程主线的独立索引，与L重复时双来源并存。

| 页面 | 章节覆盖 | O索引 | 类型/差异 |
|---|---|---|---|
| C-index | §1/§2、三结论/9导航 | O01/O17/O20/O24/O37 | 强制概览，不能以首页代替子页 |
| C-preview | §1–§3及知识检索vs远端图/三方法报文/Schema | O02/O04/O19/O37/O39 | 协议结构必须，工具语义/参数示例分开 |
| C-scope | §1七交付/流程、§2角色、§3八DoD | O01/O05–O10/O17/O21/O27/O31–O33 | 8DoD主测试另列acceptance-matrix |
| C-model | §1–§4全部表/字段/图/令牌/删除外键 | O05/O07/O12–O14/O27/O30 | 物理删除有审计503必须测试；存储64非引用32 |
| C-binding | §1–§3语法/分隔/例子/图/保存/后变更表 | O10/O12/O15–O17/O28 | 改名失配denied无审计，旧稳定ID方案撤回 |
| C-bridge | §1五步/图、§2时序/图及参数、§3审计分支 | O08–O10/O18–O21/O27–O28 | 远端失败502，DB503；单位固定参数为示范 |
| C-security | §1数据位/前缀/图、§2句式/1200/整丢、§3能力/令牌 | O03/O20/O24–O26 | 典型句式非完整源码正则；整条丢弃不能裁剪 |
| C-audit | §1字段/图/50与100、§2SSE示例、省略、§3八接口/注解 | O11/O13/O21–O23/O27/O29/O32 | 字段/省略、接口method/path/body/response/error均核对 |
| C-apply | §1全部14步骤、§2全部故障/预期、§3真实同伴四项 | O01/O03/O08–O10/O28/O31/O36/O40 | 真实同伴强制；令牌不入库一句与字段页冲突需披露 |
| C-think | §1–§5各观察/含义/替代/图 | O18–O20/O26/O38 | 研讨不是额外实现function calling/OAuth/第8课能力的要求 |

## 三、官方/示范/项目/待确认分类与冲突

- 官方强制/兼容：8操作、四道保存校验、引用32字和工具禁连字符、多分隔符、meta.ref/status/text与无调用省略、1200/句式整丢、审计50/100及可归属例外、真实同伴四项等。
- 课程示范：工具formula_hint/unit_convert允许更换；query/value/from/to和固定换算参数、HTTP子集而非完整MCP SDK、环境名MCP_AUTH_TOKEN/8090/mcp-demo均不得冒充唯一强制部署名。项目选用两示范主名，参数适配明确与固定参数示例不同。
- 官方建议、项目继续要求：L5的apitest与run_all.py HTML；额外负测/模型预算/人工内容质量/品牌保护延续项目范围。
- 项目补强：AES-GCM、SSRF、CAS、次数/超时/原始字节上限、严格Schema、额外缓存运行检查、无额外接口、当前v3→v4显式升级。保留既有需求但标项目属性。
- 待确认：官方未给精确保留天数、完整安全正则、全部Schema边界或limit非法处理；本项目明示选择，不能用“已读取”假装原文提供了这些数值。

| 冲突/差异 | 证据 | 本版处理及待审阅点 |
|---|---|---|
| 存库vs不入库 | C-model§2–3/L2.1明确服务端令牌存表；C-apply§1后段笼统说令牌不入库 | 采用详细字段与边界：主机登记令牌服务端加密存储，模型/服务部署密钥仅环境；冲突保留供用户审核，不改写来源 |
| 403vs孤立404措辞 | C-scope§2、C-audit§3及OA全部非教师403；C-bridge§1括注不严谨 | 匿名401、学生403、编号外班/不存在404；按一致表格/接口执行 |
| 删除保历史的表述 | OA说删除服务器/缓存且历史保留；C-model§4明确有审计外键拦503 | 拆两fixture：无审计204，有审计503且不变；不以软删除伪兼容 |
| 改名/删除无新审计 | C-binding§3/C-bridge§3/C-apply§2 | 撤回“所有尝试都有审计”和稳定ID旧绑定；查不到旧名只meta denied |
| connected与失败缓存 | C-bridge§1说失败不更新缓存；scope要求失败未连接、断开清缓存 | 不发布部分失败新清单，失败置false并清理失效旧缓存；此一致性解释是项目处理，待审核 |
| 64 vs 32 | C-model存储server/tool64，C-binding引用各32 | 两对象分别验证；注册长名可存在但不可写超长引用，UI提示 |
| PUT令牌schema不完整 | OA-MCP003 description允许auth_token但schema只列name/base_url；C-model§3说明空保留 | 项目注解明确补optional auth_token，name/base_url兼容；不是称原schema已列此属性 |
| 审计简短schema省字段 | OA-MCP008例子不列detail/会话/时间；C-audit§1明确列出 | 返回字段并集，保留tool_ref/detail等必要事实 |
| 标准MCP与教学HTTP子集 | C-think§1主动说明非完整传输 | 项目实现真正独立网络HTTP子集；撤回完整Streamable HTTP/SDK/session必需前提 |
| 当前第五课JWT/旧字段 | OA旧接口与本仓库撤销/version、UUID、skill_name等不同 | 本轮不重做第五课；尤其既有认证存储500 vs官方503保留差异，不能称整份OA兼容 |

## 四、O官方条款 → Requirement → Task → Test

所有O均可从上方L/C/OA原文页节回查；表中“研讨/示范/建议”不提升为老师强制。PROJECT是无直接老师条款的用户/工程需求。

| O编号 | 原文要求摘要 | 来源页节 | 分类 | Requirement | Task | Test |
|---|---|---|---|---|---|---|
| O01 | 同仓独立mcp-server、Compose网络可达、至少两个真实工具 | L1.1/L1.4/L1.5；C-scope§1、C-apply§1.1 | 强制 | W6-F01, W6-F07, W6-F13, W6-F67 | 1.1, 1.4, 1.6, 6.1 | W6-T01, W6-T07, W6-T13, W6-T67 |
| O02 | POST /mcp、/health、initialize/list/call及可读JSON-RPC错误 | L1.2/L1.3；C-preview§2–3、C-bridge§1 | 强制 | W6-F02, W6-F03, W6-F04, W6-F05, W6-F08, W6-F10, W6-F12 | 1.1, 1.2, 1.3, 1.4, 2.5 | W6-T02, W6-T03, W6-T04, W6-T05, W6-T08, W6-T10, W6-T12 |
| O03 | 环境Bearer、错误401、密钥不回前端/Git/镜像/日志 | L1.2–3/L2.1/L6.4；C-security§3、C-model§3 | 强制 | W6-F09, W6-F10, W6-F14, W6-F26, W6-F53 | 1.1, 1.5, 2.4, 4.5 | W6-T09, W6-T10, W6-T14, W6-T26, W6-T53 |
| O04 | tools/list包含名称/描述/inputSchema且call返回text | L1.4；C-preview§3、C-model§2 | 强制 | W6-F04, W6-F05, W6-F06, W6-F07, W6-F08, W6-F23, W6-F39 | 1.3, 1.4, 3.3, 4.2 | W6-T04, W6-T05, W6-T06, W6-T07, W6-T08, W6-T23, W6-T39 |
| O05 | name/base_url/auth_token输入和has_auth/tool_count/connected输出 | L2.1；C-scope§3；OA-MCP001–003 | 强制 | W6-F15, W6-F17, W6-F18, W6-F26, W6-F28, W6-F63, W6-F66 | 2.1, 2.4, 3.1, 5.1, 5.3 | W6-T15, W6-T17, W6-T18, W6-T26, W6-T28, W6-T63, W6-T66 |
| O06 | 教师专属，学生403，当前认证班级，跨班/不存在404 | L3.5；C-scope§2；C-audit§3 | 强制 | W6-F16, W6-F23, W6-F25, W6-F37, W6-F62, W6-F68 | 2.7, 3.3, 3.4, 4.2, 6.2/6.7 | W6-T16, W6-T23, W6-T25, W6-T37, W6-T62, W6-T68 |
| O07 | 同班服务器唯一，重复400 | L2.1/L6.1；C-scope§3、C-model§2 | 强制 | W6-F15, W6-F28 | 2.1, 3.1 | W6-T15, W6-T28 |
| O08 | connect握手+发现、事务先删后插刷新、connected状态 | L2.2/L2.4；C-bridge§1、C-apply§1.4 | 强制 | W6-F04, W6-F19, W6-F21, W6-F22, W6-F23 | 1.3, 3.2, 3.3 | W6-T04, W6-T19, W6-T21, W6-T22, W6-T23 |
| O09 | 远端连接失败502、未连接；DB503；不阻断原问答 | L2.3；C-scope§3、C-bridge§1/3；OA-MCP005 | 强制 | W6-F12, W6-F19, W6-F47, W6-F53, W6-F54 | 2.5, 3.2, 4.5 | W6-T12, W6-T19, W6-T47, W6-T53, W6-T54 |
| O10 | disconnect清缓存；绑定拒绝，后续调用denied | L2.2/L6.3；C-binding§3、C-bridge§3 | 强制 | W6-F20, W6-F38, W6-F41, W6-F50 | 3.2, 4.2, 4.3, 4.5 | W6-T20, W6-T38, W6-T41, W6-T50 |
| O11 | 官方八个管理接口为公共基线，不含详情GET或discover | C-audit§3；OA-MCP001–008 | 兼容强制 | W6-F15, W6-F16, W6-F17, W6-F18, W6-F19, W6-F20, W6-F21, W6-F23, W6-F24, W6-F25, W6-F62, W6-F66 | 2.7, 3.1, 3.2, 3.3, 3.4, 5.3 | W6-T15, W6-T16, W6-T17, W6-T18, W6-T19, W6-T20, W6-T21, W6-T23, W6-T24, W6-T25, W6-T62, W6-T66 |
| O12 | 允许修改/改名，空令牌保留，配置变更失效缓存 | C-model§3–4、C-binding§3；OA-MCP003 | 强制 | W6-F18, W6-F22, W6-F38, W6-F42, W6-F44, W6-F51 | 3.1, 3.3, 4.2, 4.3, 4.5 | W6-T18, W6-T22, W6-T38, W6-T42, W6-T44, W6-T51 |
| O13 | 三表字段/长度：server名64/Base URL512、tool名64、tool_ref128、detail512、三状态/关联 | C-model§1–2；C-audit§1 | 课程数据模型基线 | W6-F28, W6-F31, W6-F32, W6-F33, W6-F62 | 2.1, 2.7 | W6-T28, W6-T31, W6-T32, W6-T33, W6-T62 |
| O14 | 缓存级联，历史审计不删，有审计服务器删除503 | C-model§4；OA-MCP004的删除失败503 | 明确约束 | W6-F24, W6-F28, W6-F33, W6-F42 | 2.1, 2.7, 3.1, 4.3 | W6-T24, W6-T28, W6-T33, W6-T42 |
| O15 | 引用两段均小写字母开头≤32；工具禁连字符，服务器可连字符 | C-binding§1；L1.4/L3.1 | 强制 | W6-F34 | 4.1 | W6-T34 |
| O16 | 工具栏支持逗号、顿号、分号、竖线多项 | C-binding§1 | 强制 | W6-F34, W6-F36 | 4.1, 4.2 | W6-T34, W6-T36 |
| O17 | 保存前格式/本班存在/已连接/工具已发现四道校验 | L3.1–2；C-binding§2 | 强制 | W6-F34, W6-F36, W6-F37, W6-F38, W6-F39 | 4.1, 4.2 | W6-T34, W6-T36, W6-T37, W6-T38, W6-T39 |
| O18 | 已选启用绑定技能驱动逐个调用，跳过knowledge_search | L3.3；C-bridge§2、C-think§3 | 强制 | W6-F40, W6-F43, W6-F45, W6-F47, W6-F48, W6-F56 | 4.1, 4.3, 4.4, 4.5 | W6-T40, W6-T43, W6-T45, W6-T47, W6-T48, W6-T56 |
| O19 | formula_hint的query、unit_convert的value/from/to，固定换算参数仅展示连通 | C-preview§3、C-bridge§2、C-security§3 | 示范实现 | W6-F06, W6-F07, W6-F48 | 1.4, 4.4 | W6-T06, W6-T07, W6-T48 |
| O20 | 代码先调用再生成，不用模型自主选择或function calling | C-index§2、C-security§3、C-think§3 | 强制 | W6-F45, W6-F47, W6-F48, W6-F56, W6-F57 | 4.3, 4.4, 4.5, 4.6 | W6-T45, W6-T47, W6-T48, W6-T56, W6-T57 |
| O21 | 所有工具完成在meta及首delta之前 | L3.4；C-bridge§2、C-audit§2 | 强制 | W6-F47, W6-F49 | 4.5 | W6-T47, W6-T49 |
| O22 | meta.tool_calls项为ref/status/text，三状态 | L3.4；C-audit§2 | 兼容强制 | W6-F49, W6-F50, W6-F51, W6-F53, W6-F54, W6-F55, W6-F64, W6-F66 | 4.5, 4.6, 5.2, 5.3 | W6-T49, W6-T50, W6-T51, W6-T53, W6-T54, W6-T55, W6-T64, W6-T66 |
| O23 | 无MCP调用不发tool_calls字段 | C-audit§2 | 兼容强制 | W6-F40, W6-F43, W6-F49, W6-F60, W6-F64, W6-F66 | 4.1, 4.3, 4.5, 5.2, 5.3 | W6-T40, W6-T43, W6-T49, W6-T60, W6-T64, W6-T66 |
| O24 | 危险句式命中整条丢弃换固定说明 | C-index§2、C-security§2、C-apply§1.10 | 强制 | W6-F55, W6-F57, W6-F58 | 4.6 | W6-T55, W6-T57, W6-T58 |
| O25 | 超过1200字整条丢弃，不用裁剪残留 | C-security§2 | 强制 | W6-F55, W6-F58 | 4.6 | W6-T55, W6-T58 |
| O26 | 工具返回作为user数据并标来源，不进system、不当知识库编号 | L3.3；C-security§1、C-think§4 | 强制 | W6-F57, W6-F59 | 4.6 | W6-T57, W6-T59 |
| O27 | 审计工具引用/服务器/状态/用户/会话/时间/detail，成功摘要前200 | L4.1；C-bridge§3、C-audit§1 | 强制 | W6-F31, W6-F32, W6-F62 | 2.7 | W6-T31, W6-T32, W6-T62 |
| O28 | 断开denied有审计；改名/删除查无旧名denied无审计；执行失败error有审计 | C-scope§3、C-binding§3、C-bridge§3、C-apply§2 | 强制 | W6-F20, W6-F24, W6-F31, W6-F41, W6-F42, W6-F50, W6-F51, W6-F53, W6-F54 | 2.7, 3.1, 3.2, 4.3, 4.5 | W6-T20, W6-T24, W6-T31, W6-T41, W6-T42, W6-T50, W6-T51, W6-T53, W6-T54 |
| O29 | 审计本班倒序默认50最多100，只读 | C-audit§1；OA-MCP008 | 强制 | W6-F62 | 2.7 | W6-T62 |
| O30 | 审计只增不改、不随服务删除回删；无公开保留天数要求 | C-model§1/4、C-audit§1 | 明确约束 | W6-F24, W6-F32, W6-F33 | 2.7, 3.1 | W6-T24, W6-T32, W6-T33 |
| O31 | 教师MCP页签、列表/状态/工具数/登记/最近调用；学生工具反馈 | C-apply§1.13–14、C-scope§2 | 强制 | W6-F63, W6-F64, W6-F65 | 5.1, 5.2 | W6-T63, W6-T64, W6-T65 |
| O32 | 代码注解生成OpenAPI3.0.3，Swagger试调connect/tools/技能保存 | L4.2；C-audit§3；OA | 强制 | W6-F66 | 5.3 | W6-T66 |
| O33 | scope八项DoD逐项检查及独立服务/主机/断开/密钥自查 | L6.1–4；C-scope§3 | 验收强制 | W6-F01, W6-F09, W6-F15, W6-F19, W6-F20, W6-F24, W6-F26, W6-F31, W6-F36, W6-F49, W6-F50, W6-F51, W6-F62, W6-F66, W6-F67 | 1.1, 1.5, 2.4, 2.7, 3.1, 3.2, 4.2, 4.5, 5.3, 6.1 | W6-T01, W6-T09, W6-T15, W6-T19, W6-T20, W6-T24, W6-T26, W6-T31, W6-T36, W6-T49, W6-T50, W6-T51, W6-T62, W6-T66, W6-T67 |
| O34 | apitest管理角色与端到端成功审计测试 | L5.1–2 | 官方建议/项目要求 | W6-F67, W6-F68 | 6.1, 6.2/6.7 | W6-T67, W6-T68 |
| O35 | run_all.py生成HTML且不提交Git | L5.3 | 官方建议/项目要求 | W6-F70 | 6.4 | W6-T70 |
| O36 | 真实同伴四项：隔离、密钥不回传、断开审计、本班审计 | C-apply§3 | 人工验收强制 | W6-F68 | 6.2/6.7 | W6-T68 |
| O37 | 沿用第3–5课认证/班级/角色/检索/流式，增量改同仓 | L3.5及结尾；C-index§1、C-preview§1、C-apply§1 | 强制兼容 | W6-F16, W6-F25, W6-F40, W6-F43, W6-F46, W6-F59, W6-F60, W6-F61, W6-F65, W6-F69 | 3.4, 4.1, 4.3, 4.5, 4.6, 4.7, 5.2, 6.3 | W6-T16, W6-T25, W6-T40, W6-T43, W6-T46, W6-T59, W6-T60, W6-T61, W6-T65, W6-T69 |
| O38 | 五道思考：HTTP子集、保存/运行校验、确定性、数据位置、后续安全扩展 | C-think§1–5 | 研讨/示范取舍 | W6-F02, W6-F27, W6-F35, W6-F44, W6-F45, W6-F56, W6-F57, W6-F71, W6-F72 | 1.2, 2.6, 4.1, 4.3, 4.4, 4.6, 6.5, 6.6 | W6-T02, W6-T27, W6-T35, W6-T44, W6-T45, W6-T56, W6-T57, W6-T71, W6-T72 |
| O39 | 至少两工具强制，formula_hint/unit_convert命名与语义可按班级更换 | L1.4、C-preview§3 | 可替换示范 | W6-F06, W6-F07 | 1.4 | W6-T06, W6-T07 |
| O40 | 14层实现步骤与常见故障表，端到端证据和排查 | C-apply§1–2 | 实践路线/验证 | W6-F13, W6-F28, W6-F29, W6-F30, W6-F47, W6-F50, W6-F51, W6-F63, W6-F64, W6-F66, W6-F67, W6-F70, W6-F72 | 1.6, 2.1, 2.2, 2.3, 4.5, 5.1, 5.2, 5.3, 6.1, 6.4, 6.6 | W6-T13, W6-T28, W6-T29, W6-T30, W6-T47, W6-T50, W6-T51, W6-T63, W6-T64, W6-T66, W6-T67, W6-T70, W6-T72 |

## 五、逐条72项反向审核：Requirement/Test → Task → 来源

每项审阅结论是**规划文档对齐**，不是已实施/已验收。保留项也逐条补足归属；原编号不删不复用，旧内容hash记录于verification-result。实现模块在capability列给出职责，与design一致。

| Requirement | 内容/验收标准 | 来源O | 分类 | Task | Test | Spec/计划模块 | 本轮结论 | 实施/测试状态 |
|---|---|---|---|---|---|---|---|---|
| W6-F01 | 提供仓库内 mcp-server/ 独立进程，Go通过网络调用真实工具，不在主程序伪造结果；验收：启动可发现工具，停止后Go观察到不可用而非返回模拟成功 | O01, O33 | 官方强制/兼容基线（附项目实现选择） | 1.1 | W6-T01 | [W6-F01 独立网络服务](specs/week6-mcp-service/spec.md) | 保留；补足官方归属与项目扩展分类 | 未实施/未执行 |
| W6-F02 | 以独立HTTP JSON-RPC实现initialize、tools/list、tools/call，握手版本由项目声明，不把课程子集宣称为完整标准Streamable HTTP实现；验收：三方法真实可用，返回可读RPC错误；不要求课程未规定的GET SSE/批量/session才能通过 | O02, O38 | 官方强制/兼容基线（附项目实现选择） | 1.2 | W6-T02 | [W6-F02 课程HTTP协议边界](specs/week6-mcp-service/spec.md) | 修订 | 未实施/未执行 |
| W6-F03 | 提供POST /mcp与UTF8 JSON-RPC 2.0、匹配id的result/error，base_url自动补/mcp；学生会话SSE与MCP HTTP分开；验收：正确端点可用，result/error互斥，错误可读；错id不可算成功 | O02 | 官方强制/兼容基线（附项目实现选择） | 1.2 | W6-T03 | [W6-F03 传输与信封](specs/week6-mcp-service/spec.md) | 修订 | 未实施/未执行 |
| W6-F04 | 通过tools/list返回工具名称、说明和inputSchema，连接后以事务完整覆盖缓存，不追加幽灵项；项目限100项；验收：两工具真实可见、行数不翻倍，失踪工具消失；非法列表不发布部分缓存 | O02, O04, O08 | 官方强制/兼容基线（附项目实现选择） | 1.3 | W6-T04 | [W6-F04 工具发现](specs/week6-mcp-service/spec.md) | 修订 | 未实施/未执行 |
| W6-F05 | 为每个工具发布并执行严格JSON Schema，不接受额外属性、类型错误、过长文本或非有限数值；验收：Schema与执行一致；JSON-RPC -32602，无计算副作用 | O02, O04 | 官方强制/兼容基线（附项目实现选择） | 1.3 | W6-T05 | [W6-F05 输入Schema](specs/week6-mcp-service/spec.md) | 保留；补足官方归属与项目扩展分类 | 未实施/未执行 |
| W6-F06 | 实现formula_hint(query:string)并通过MCP content返回公式/步骤文本，不直接代算最终数值；具体题型为项目示范；验收：真实返回可用提示文本，输入非法被拒；不强制返回嵌套JSON | O04, O19, O39 | 课程示范+项目选择 | 1.4 | W6-T06 | [W6-F06 公式提示工具](specs/week6-mcp-service/spec.md) | 修订 | 未实施/未执行 |
| W6-F07 | 以unit_convert作为项目最终主名称，参数value/from/to，真实提供同维度换算文本；unit_converter仅为上一版草案名称，不自动注册别名；验收：发现unit_convert且换算100 cm；不存在必选unit_converter要求，非法维度失败可读 | O01, O04, O19, O39 | 课程示范+项目选择 | 1.4 | W6-T07 | [W6-F07 单位换算工具命名](specs/week6-mcp-service/spec.md) | 修订 | 未实施/未执行 |
| W6-F08 | tools/call结果接受MCP content的text数组和isError，成功文本先安全处理；不得要求所有远端text本身也是JSON；验收：正常文本能用于回答，isError为error且不把失败载荷当成功数据 | O02, O04 | 官方强制/兼容基线（附项目实现选择） | 1.4 | W6-T08 | [W6-F08 执行返回格式](specs/week6-mcp-service/spec.md) | 修订 | 未实施/未执行 |
| W6-F09 | 独立服务对协议入口校验专用Bearer凭据，缺失/错误为401，不能使用学生JWT代替服务凭据；验收：仅正确服务Token可发现或调用；响应无Token明文 | O03, O33 | 官方强制/兼容基线（附项目实现选择） | 1.5 | W6-T09 | [W6-F09 服务Bearer鉴权](specs/week6-mcp-service/spec.md) | 保留；补足官方归属与项目扩展分类 | 未实施/未执行 |
| W6-F10 | 提供最小无敏感信息健康检查和明确启动命令，缺少服务密钥启动失败；不以health成功替代协议连接成功；验收：缺配置非零退出；健康仅暴露状态，连接必须实际握手 | O02, O03 | 官方强制/兼容基线（附项目实现选择） | 1.1 | W6-T10 | [W6-F10 健康与启动](specs/week6-mcp-service/spec.md) | 保留；补足官方归属与项目扩展分类 | 未实施/未执行 |
| W6-F11 | 单工具执行及网络请求受可取消截止时间约束，不自动重试tools/call；验收：在限时内取消并返回timeout；实际执行次数至多一次 | PROJECT | 项目补强（非老师新增要求） | 1.5 | W6-T11 | [W6-F11 工具执行限时](specs/week6-mcp-service/spec.md) | 保留；补足官方归属与项目扩展分类 | 未实施/未执行 |
| W6-F12 | 网络断开、非200、非法JSON、响应超限或协议错误不能伪装正常结果；验收：客户端产生稳定脱敏错误，不向模型注入失败载荷 | O02, O09 | 官方强制/兼容基线（附项目实现选择） | 2.5 | W6-T12 | [W6-F12 服务异常隔离](specs/week6-mcp-service/spec.md) | 保留；补足官方归属与项目扩展分类 | 未实施/未执行 |
| W6-F13 | 以独立Compose服务接入Go私网，密钥由环境或secret注入，无默认公开宿主端口，不依赖Flask；验收：服务健康且网络调用真实，无现有卷或Demo变动 | O01, O40 | 官方强制/兼容基线（附项目实现选择） | 1.6 | W6-T13 | [W6-F13 Compose私网集成](specs/week6-mcp-service/spec.md) | 保留；补足官方归属与项目扩展分类 | 未实施/未执行 |
| W6-F14 | 验证Origin及Content-Type，限制请求大小，禁止任意脚本、文件读取和命令执行；验收：分别403、413或参数拒绝，无命令/文件副作用 | O03 | 项目补强（非老师新增要求） | 1.5 | W6-T14 | [W6-F14 入站安全](specs/week6-mcp-service/spec.md) | 保留；补足官方归属与项目扩展分类 | 未实施/未执行 |
| W6-F15 | 教师以name/base_url/auth_token登记本班服务器，同班名称唯一且初始connected=false；响应has_auth与tool_count，不含凭据；验收：分别201/400/201；响应含base_url/has_auth/connected/tool_count，不含auth_token或token | O05, O07, O11, O33 | 官方强制/兼容基线（附项目实现选择） | 3.1 | W6-T15 | [W6-F15 登记服务器](specs/week6-mcp-management/spec.md) | 修订 | 未实施/未执行 |
| W6-F16 | 服务器列表仅显示Principal当前班级，不接受客户端class_id覆盖；验收：仅各自班级；覆盖字段400，无外班数据 | O06, O11, O37 | 官方强制/兼容基线（附项目实现选择） | 3.4 | W6-T16 | [W6-F16 列表隔离](specs/week6-mcp-management/spec.md) | 保留；补足官方归属与项目扩展分类 | 未实施/未执行 |
| W6-F17 | 通过本班服务器列表提供配置及connected/tool_count/has_auth，不增加单条GET详情作为课程必需接口；验收：列表只含本班，按编号资源404一致；无需GET /servers/{id}额外路由 | O05, O11 | 官方强制/兼容基线（附项目实现选择） | 3.1 | W6-T17 | [W6-F17 列表读取与状态](specs/week6-mcp-management/spec.md) | 修订 | 未实施/未执行 |
| W6-F18 | PUT以name/base_url及可选auth_token修改本班服务器，空auth_token保留旧值；修改清空连接与工具缓存，改名不改技能原文；验收：响应200且connected=false/tool_count=0；空令牌保留；旧名引用denied且无新审计行 | O05, O11, O12 | 官方强制/兼容基线（附项目实现选择） | 3.1 | W6-T18 | [W6-F18 修改与改名](specs/week6-mcp-management/spec.md) | 修订 | 未实施/未执行 |
| W6-F19 | connect实际完成initialize与tools/list后返回connected/tools，远端不可达、鉴权或握手失败返回502并connected=false；数据库失败503；验收：正常200含connected=true及工具；远端失败502，DB失败503，无部分新缓存或假成功 | O08, O09, O11, O33 | 官方强制/兼容基线（附项目实现选择） | 3.2 | W6-T19 | [W6-F19 连接握手](specs/week6-mcp-management/spec.md) | 修订 | 未实施/未执行 |
| W6-F20 | disconnect幂等置connected=false并清空工具缓存，保留技能原文和已有审计；验收：返回connected=false、tools=[]、零远端call；meta denied且增加一条本服务器denied审计 | O10, O11, O28, O33 | 官方强制/兼容基线（附项目实现选择） | 3.2 | W6-T20 | [W6-F20 断开服务器](specs/week6-mcp-management/spec.md) | 修订 | 未实施/未执行 |
| W6-F21 | 重新发现复用既有connect接口，不新增discover接口；每次都握手并完整覆盖缓存；验收：通过同一官方接口刷新，缓存不翻倍；不要求POST /discover或单独的409状态 | O08, O11 | 官方强制/兼容基线（附项目实现选择） | 3.3 | W6-T21 | [W6-F21 通过重连重新发现](specs/week6-mcp-management/spec.md) | 修订 | 未实施/未执行 |
| W6-F22 | 重连事务内先删后插，断开/修改时清空；项目revision/CAS防止旧响应覆盖新配置；验收：只有当前配置对应列表可用，无部分列表、幽灵工具或旧响应复活 | O08, O12 | 官方强制/兼容基线（附项目实现选择） | 3.3 | W6-T22 | [W6-F22 缓存原子刷新](specs/week6-mcp-management/spec.md) | 修订 | 未实施/未执行 |
| W6-F23 | 仅教师通过官方tools接口查询本班服务器最近成功发现的清单，断开后返回空列表；验收：分别200/403/404；断开后tools=[]且无凭据 | O04, O06, O08, O11 | 官方强制/兼容基线（附项目实现选择） | 3.3 | W6-T23 | [W6-F23 工具列表](specs/week6-mcp-management/spec.md) | 修订 | 未实施/未执行 |
| W6-F24 | DELETE无审计服务器时物理删除并级联缓存；已有mcp_tool_calls引用则返回503且整体不变，重复成功删除404；技能原文不改；验收：无审计204且缓存清空；有审计503且服务/缓存/审计不变；旧引用meta denied无新增审计 | O11, O14, O28, O30, O33 | 官方强制/兼容基线（附项目实现选择） | 3.1 | W6-T24 | [W6-F24 删除与审计外键](specs/week6-mcp-management/spec.md) | 修订 | 未实施/未执行 |
| W6-F25 | 所有管理操作复用Bearer当前身份和teacher guard，查询/更新包含class_id；匿名及学生不可访问；验收：匿名401、学生403、外班资源404，拒绝不改DB或发起网络调用 | O06, O11, O37 | 官方强制/兼容基线（附项目实现选择） | 3.4 | W6-T25 | [W6-F25 教师角色与班级校验](specs/week6-mcp-management/spec.md) | 保留；补足官方归属与项目扩展分类 | 未实施/未执行 |
| W6-F26 | API以auth_token接收服务令牌且只返回has_auth；原文不进响应、前端持久存储、Git、镜像或日志；认证加密为项目补强；验收：仅has_auth布尔返回；密码框写后清空；服务端可用于Bearer，外部无密钥明文 | O03, O05, O33 | 官方强制/兼容基线（附项目实现选择） | 2.4 | W6-T26 | [W6-F26 凭据保密](specs/week6-mcp-management/spec.md) | 修订 | 未实施/未执行 |
| W6-F27 | 服务器登记和每次连接/call校验管理员允许的scheme/host/port及实际解析IP，禁重定向和云元数据/回环/未允许内网；验收：被拒绝且目标零访问；仅显式允许的私网mcp-server可访问 | O38 | 项目补强（非老师新增要求） | 2.6 | W6-T27 | [W6-F27 出站地址限制](specs/week6-mcp-management/spec.md) | 保留；补足官方归属与项目扩展分类 | 未实施/未执行 |
| W6-F28 | 三张MCP表保留base_url/connected/tool_name/schema_json/tool_ref/detail等字段分工，班级唯一、缓存server唯一、审计三状态及外键约束生效；验收：约束拒绝违规，connected仅0/1；审计保留且关联服务删除503，不用软删除替代 | O05, O07, O13, O14, O40 | 官方强制/兼容基线（附项目实现选择） | 2.1 | W6-T28 | [W6-F28 三表存储约束](specs/week6-mcp-management/spec.md) | 修订 | 未实施/未执行 |
| W6-F29 | 新schema通过显式离线v4升级及完整catalog/账本/就绪校验，新旧路径、重试及中断可恢复，HTTP不执行DDL；验收：合法路径就绪；重复不复活数据；漂移/不完整拒绝，无静默修复 | O40 | 项目补强（非老师新增要求） | 2.2 | W6-T29 | [W6-F29 显式幂等迁移](specs/week6-mcp-management/spec.md) | 保留；补足官方归属与项目扩展分类 | 未实施/未执行 |
| W6-F30 | 迁移保留用户、材料、切片、助手、技能source/绑定、历史消息引用和占位数据，不自动扩大授权；验收：旧内容逐项一致；没有给旧技能补MCP绑定或改变空材料语义 | O40 | 项目补强（非老师新增要求） | 2.3 | W6-T30 | [W6-F30 旧数据保护](specs/week6-mcp-management/spec.md) | 保留；补足官方归属与项目扩展分类 | 未实施/未执行 |
| W6-F31 | 找到本班服务器时对ok、未连接denied、远端失败error各追加一行，含tool_ref/server/user/conversation/time/detail；改名或删除导致找不到旧名时仅返回denied而无审计行；验收：各存在服务分支各加一条终态行；找不到旧名零新增，detail脱敏且≤512字，成功摘要≤200字 | O13, O27, O28, O33 | 官方强制/兼容基线（附项目实现选择） | 2.7 | W6-T31 | [W6-F31 审计归属与例外](specs/week6-mcp-management/spec.md) | 修订 | 未实施/未执行 |
| W6-F32 | mcp_tool_calls只追加终态ok/denied/error，不写pending后改行；审计写失败不得伪报完整成功或重放远端动作；验收：meta前503/failed且不重放；披露崩溃可能无审计，不伪造全路径可追踪保证 | O13, O27, O30 | 官方强制/兼容基线（附项目实现选择） | 2.7 | W6-T32 | [W6-F32 终态追加及持久化失败](specs/week6-mcp-management/spec.md) | 修订 | 未实施/未执行 |
| W6-F33 | 不随服务/工具/技能/会话清理级联删除调用审计；有审计服务删除受限；课程未规定保存天数，不虚构要求；验收：服务删除503；历史仍在，审计无公开删除API；期限作为项目运维事项而非缺课程来源 | O13, O14, O30 | 官方强制/兼容基线（附项目实现选择） | 2.7 | W6-T33 | [W6-F33 历史审计保护](specs/week6-mcp-management/spec.md) | 修订 | 未实施/未执行 |
| W6-F34 | 工具字段按逗号/顿号/分号/竖线分项，支持,，、;；\|；tool匹配[a-z][a-z0-9_]{0,31}，server匹配[a-z][a-z0-9_-]{0,31}，保留knowledge_search及空；验收：合法分项保存且顺序不变；工具连字符/大写/33字符/裸工具名拒绝，服务器合法连字符允许 | O15, O16, O17 | 官方强制/兼容基线（附项目实现选择） | 4.1 | W6-T34 | [W6-F34 引用语法与分隔符](specs/week6-skill-tools/spec.md) | 修订 | 未实施/未执行 |
| W6-F35 | 一技能最多3个不同MCP引用，重复引用拒绝；内置knowledge_search不消耗MCP调用额度；验收：三项通过，四项/重复400；保存和运行上限一致 | O38 | 项目补强（非老师新增要求） | 4.1 | W6-T35 | [W6-F35 引用去重上限](specs/week6-skill-tools/spec.md) | 保留；补足官方归属与项目扩展分类 | 未实施/未执行 |
| W6-F36 | 保存技能前逐条校验格式、本班服务器存在、connected及工具已发现，全部通过后原子保存source/hash/version，不隐式持久化稳定ID绑定改变改名行为；验收：整份拒绝无部分写入；外班与不存在错误一致，旧技能保持 | O16, O17, O33 | 官方强制/兼容基线（附项目实现选择） | 4.2 | W6-T36 | [W6-F36 保存时四道校验](specs/week6-skill-tools/spec.md) | 修订 | 未实施/未执行 |
| W6-F37 | 按当前Principal班级与引用服务器名称解析，不允许请求覆盖class或借外班同名服务绑定；验收：两者同样拒绝且响应一致，A/B数据不变；不以全局名称查找 | O06, O17 | 官方强制/兼容基线（附项目实现选择） | 4.2 | W6-T37 | [W6-F37 绑定班级校验](specs/week6-skill-tools/spec.md) | 修订 | 未实施/未执行 |
| W6-F38 | 新建/修改技能的MCP引用须指向本班connected服务器，断开/删除/改名不自动改技能文本；验收：原技能原文保持；重新保存不合法引用拒绝，停用专用toggle仍可用 | O10, O12, O17 | 官方强制/兼容基线（附项目实现选择） | 4.2 | W6-T38 | [W6-F38 连接有效性校验](specs/week6-skill-tools/spec.md) | 修订 | 未实施/未执行 |
| W6-F39 | 保存前工具须在该服务器当前缓存清单；inputSchema用于展示及项目确定性参数适配，不把工具自述当指令；验收：未知工具拒绝；合法已发现工具不因未返回嵌套JSON被拒，Schema不赋予调用权限 | O04, O17 | 官方强制/兼容基线（附项目实现选择） | 4.2 | W6-T39 | [W6-F39 发现有效性校验](specs/week6-skill-tools/spec.md) | 修订 | 未实施/未执行 |
| W6-F40 | 仅当前问题命中的已绑定且启用技能的工具可调用，保留最高词分/小ID路由；验收：启用且命中才调用；停用/解绑/零分零工具调用 | O18, O23, O37 | 官方强制/兼容基线（附项目实现选择） | 4.3 | W6-T40 | [W6-F40 启用与路由](specs/week6-skill-tools/spec.md) | 保留；补足官方归属与项目扩展分类 | 未实施/未执行 |
| W6-F41 | 断开后技能照常路由，调用denied且写审计；同名同服务重新连接且工具还在可恢复；验收：前者零RPC/一条denied审计；后者正常调用，既有技能原文不改 | O10, O28 | 官方强制/兼容基线（附项目实现选择） | 4.3 | W6-T41 | [W6-F41 断开绑定行为](specs/week6-skill-tools/spec.md) | 修订 | 未实施/未执行 |
| W6-F42 | 服务器改名或成功删除后技能引用保持旧名，按旧名查不到则meta denied而不写调用审计；验收：前两者denied且新增审计0；有审计删除503后原引用仍归原服务器，不能算已删 | O12, O14, O28 | 官方强制/兼容基线（附项目实现选择） | 4.3 | W6-T42 | [W6-F42 改名删除绑定行为](specs/week6-skill-tools/spec.md) | 修订 | 未实施/未执行 |
| W6-F43 | 空工具、knowledge_search、旧source来源/版本/原文往返、enabled-only toggle及已有材料绑定继续可用；验收：无MCP时无额外网络请求；原功能、旧数据与技能哈希规则保持 | O18, O23, O37 | 官方强制/兼容基线（附项目实现选择） | 4.1 | W6-T43 | [W6-F43 旧技能兼容](specs/week6-skill-tools/spec.md) | 保留；补足官方归属与项目扩展分类 | 未实施/未执行 |
| W6-F44 | 每次调用按当前班级及技能引用查服务器和连接状态；项目额外复核当前工具缓存/revision及预算，不使用旧稳定ID跨改名恢复；验收：新配置立即生效；旧名失配无审计，存在服务拒绝有审计；旧结果不能冒充当前成功 | O12, O38 | 官方强制/兼容基线（附项目实现选择） | 4.3 | W6-T44 | [W6-F44 运行授权复核](specs/week6-skill-tools/spec.md) | 修订 | 未实施/未执行 |
| W6-F45 | 模型、学生文本、上传材料和工具描述不得新增白名单、切换服务或触发未绑定工具；验收：仅已授权计划可执行；攻击目标零请求且权限不改变 | O18, O20, O38 | 官方强制/兼容基线（附项目实现选择） | 4.3 | W6-T45 | [W6-F45 禁止自由工具调用](specs/week6-skill-tools/spec.md) | 保留；补足官方归属与项目扩展分类 | 未实施/未执行 |
| W6-F46 | 将MCP接入现有会话消息SSE入口，保留POST /api/ask非流式输入输出和零证据零对话行为；验收：仅会话可产生工具记录；旧ask无MCP且不创建消息 | O37 | 官方强制/兼容基线（附项目实现选择） | 4.7 | W6-T46 | [W6-F46 沿用真实问答入口](specs/week6-tool-conversations/spec.md) | 保留；补足官方归属与项目扩展分类 | 未实施/未执行 |
| W6-F47 | 认证/班级与原技能路由之后按白名单逐个调用，工具终态备齐后才发送meta，再生成回答；knowledge_search仍由检索负责；验收：工具返回早于meta、meta早于模型首字，按声明顺序且无模型自主选择，无跨网络长事务 | O09, O18, O20, O21, O40 | 官方强制/兼容基线（附项目实现选择） | 4.5 | W6-T47 | [W6-F47 meta前确定性编排](specs/week6-tool-conversations/spec.md) | 修订 | 未实施/未执行 |
| W6-F48 | formula_hint传当前问题原文为query；unit_convert由项目明确确定性解析生成value/from/to，课程固定换算参数仅作示范，不引入模型规划；验收：合法解析固定100 cm；歧义denied零RPC；无绑定不调用，原生function calling未启用 | O18, O19, O20 | 课程示范+项目选择 | 4.4 | W6-T48 | [W6-F48 确定性参数与示范差异](specs/week6-tool-conversations/spec.md) | 修订 | 未实施/未执行 |
| W6-F49 | 有MCP调用尝试时meta.tool_calls每项含ref/status/text，status为ok/denied/error；没有命中技能或无MCP项时省略字段，不发空数组；验收：前三者都有ref/status/text；后三者断言属性不存在，meta.status及既有第五课字段不变 | O21, O22, O23, O33 | 官方强制/兼容基线（附项目实现选择） | 4.5 | W6-T49 | [W6-F49 官方工具元数据](specs/week6-tool-conversations/spec.md) | 修订 | 未实施/未执行 |
| W6-F50 | 存在但未连接的服务器调用为denied，返回安全text并追加一行denied审计，原问答继续；验收：RPC0、meta ref/status/text齐全、审计新增1；模型仍按原角色/RAG回答 | O10, O22, O28, O33, O40 | 官方强制/兼容基线（附项目实现选择） | 4.5 | W6-T50 | [W6-F50 断开服务降级](specs/week6-tool-conversations/spec.md) | 修订 | 未实施/未执行 |
| W6-F51 | 按旧名找不到服务器或仅外班存在时返回denied安全text，不新增调用审计，不泄露外班；验收：RPC0、审计新增0、meta明确denied；不把有审计删除503误算已删除 | O12, O22, O28, O33, O40 | 官方强制/兼容基线（附项目实现选择） | 4.5 | W6-T51 | [W6-F51 改名删除失配降级](specs/week6-tool-conversations/spec.md) | 修订 | 未实施/未执行 |
| W6-F52 | 项目加固：存在服务的旧引用工具不在当前缓存时拒绝远端调用，denied且写本服务器审计，原问答继续；验收：RPC0，meta denied，审计新增1且detail为安全原因 | PROJECT | 项目补强（非老师新增要求） | 4.3 | W6-T52 | [W6-F52 工具未发现降级](specs/week6-tool-conversations/spec.md) | 修订 | 未实施/未执行 |
| W6-F53 | MCP 401/403归error/auth_failed，不回传服务凭据或把上游401当学生JWT失效；验收：学生身份仍有效，记录error；UI不退出学生登录，问答继续 | O03, O09, O22, O28 | 官方强制/兼容基线（附项目实现选择） | 4.5 | W6-T53 | [W6-F53 鉴权失败降级](specs/week6-tool-conversations/spec.md) | 保留；补足官方归属与项目扩展分类 | 未实施/未执行 |
| W6-F54 | 超时归error/timeout，取消该请求不重试，在总问答预算内继续；验收：记录超时且余下合法工具及模型可继续；整轮不被无条件失败 | O09, O22, O28 | 官方强制/兼容基线（附项目实现选择） | 4.5 | W6-T54 | [W6-F54 工具超时降级](specs/week6-tool-conversations/spec.md) | 保留；补足官方归属与项目扩展分类 | 未实施/未执行 |
| W6-F55 | RPC畸形/错误id/无文本/不支持content为error；普通文本合法，危险句式及超1200字整条丢弃后仅用固定说明；验收：普通文本通过；危险或超长正文任何片段均不进入meta.text/审计detail/模型，仅固定说明，问答仍继续 | O22, O24, O25 | 官方强制/兼容基线（附项目实现选择） | 4.6 | W6-T55 | [W6-F55 非法返回降级](specs/week6-tool-conversations/spec.md) | 修订 | 未实施/未执行 |
| W6-F56 | 按技能文本声明顺序串行去重计划，最多3次，单工具≤3秒、工具阶段总≤10秒；一个失败不自动停止其他合法工具；验收：顺序稳定、总预算可控；末项可因预算不足denied/budget_exceeded，第四项保存时拒绝 | O18, O20, O38 | 官方强制/兼容基线（附项目实现选择） | 4.4 | W6-T56 | [W6-F56 多工具顺序](specs/week6-tool-conversations/spec.md) | 保留；补足官方归属与项目扩展分类 | 未实施/未执行 |
| W6-F57 | 安全工具结果置于用户消息的数据位并标明工具引用，不进入system、不覆盖教师或教学边界；拒绝的原文不进入任何模型输入；验收：命中危险句式整条丢弃；通过文本只在user且带工具来源，不能触发新工具或外发 | O20, O24, O26, O38 | 官方强制/兼容基线（附项目实现选择） | 4.6 | W6-T57 | [W6-F57 外部数据边界](specs/week6-tool-conversations/spec.md) | 修订 | 未实施/未执行 |
| W6-F58 | 进入上下文前扫描完整工具正文：含忽略前述、ignore previous instructions、你现在是或system:等危险句式，以及超过1200 Unicode字的正文均整条丢弃换固定说明，不删句/截断后使用；先于审计与SSE脱敏；验收：1199/1200字正常安全正文可用；1201或命中句式全丢弃且无前缀残留，detail≤512/成功摘要≤200，秘密不泄露 | O24, O25 | 官方强制/兼容基线（附项目实现选择） | 4.6 | W6-T58 | [W6-F58 危险句式与1200字边界](specs/week6-tool-conversations/spec.md) | 修订 | 未实施/未执行 |
| W6-F59 | Week5无知识库依据仍生成，工具仅独立外部依据，不授予数字引用资格；资料引用只能来自合法检索证据；验收：空证据最终citations=[]且无伪造数字引用；有证据保留原校验，不把工具写成材料引用 | O26, O37 | 官方强制/兼容基线（附项目实现选择） | 4.6 | W6-T59 | [W6-F59 零证据与引用](specs/week6-tool-conversations/spec.md) | 保留；补足官方归属与项目扩展分类 | 未实施/未执行 |
| W6-F60 | 在现有消息配置快照中保存安全ref/status/text并允许本人历史读取，旧消息缺字段兼容且刷新不重放工具；SSE无调用仍省略；验收：安全状态可回看，RPC数不增加，旧快照不改；内部空列表不能使SSE输出空tool_calls | O23, O37 | 项目补强（非老师新增要求） | 4.5 | W6-T60 | [W6-F60 历史持久与展示](specs/week6-tool-conversations/spec.md) | 修订 | 未实施/未执行 |
| W6-F61 | 保留active_turn并发409、delta先落库、failed/interrupted前缀、取消传播及崩溃恢复，工具副作用不因重试自动重放；验收：无额外重复工具执行或孤儿消息；失败不发成功done，锁可释放 | O37 | 项目补强（非老师新增要求） | 4.7 | W6-T61 | [W6-F61 失败恢复与并发](specs/week6-tool-conversations/spec.md) | 保留；补足官方归属与项目扩展分类 | 未实施/未执行 |
| W6-F62 | GET /api/mcp/tool-calls只读返回本班按时间倒序记录，默认最近50、最多100，含tool_ref/server_name/tool_name/status/detail/user_id/conversation_id/created_at；验收：默认50、上限100不泄漏外班；limit=101按项目夹至100，学生403、只读无写入、detail安全 | O06, O11, O13, O27, O29, O33 | 官方强制/兼容基线（附项目实现选择） | 2.7 | W6-T62 | [W6-F62 审计查询50与100](specs/week6-tool-conversations/spec.md) | 修订 | 未实施/未执行 |
| W6-F63 | 最小扩展现有教师tutor页面提供MCP页签、服务器列表/登记/修改/删除/连接发现/断开/工具数/最近调用，复用8个官方操作；验收：UI反映真实状态和失败，has_auth反馈但无令牌回填，不依赖额外详情/discover接口 | O05, O31, O40 | 官方强制/兼容基线（附项目实现选择） | 5.1 | W6-T63 | [W6-F63 教师MCP页签](specs/week6-mcp-acceptance/spec.md) | 修订 | 未实施/未执行 |
| W6-F64 | 学生正常流式问答显示脱敏工具成功/拒绝/失败反馈，无管理入口或服务器凭据；验收：工具反馈与meta一致，delta仍增量，不能打开管理操作 | O22, O23, O31, O40 | 官方强制/兼容基线（附项目实现选择） | 5.2 | W6-T64 | [W6-F64 学生最小反馈](specs/week6-mcp-acceptance/spec.md) | 保留；补足官方归属与项目扩展分类 | 未实施/未执行 |
| W6-F65 | 保留CampusClaw品牌、Logo、导航、知识库、材料页、历史及原SSE解析/异常交互；验收：原页面功能及身份处理保持，无破坏布局或无done假成功 | O31, O37 | 官方强制/兼容基线（附项目实现选择） | 5.2 | W6-T65 | [W6-F65 视觉交互兼容](specs/week6-mcp-acceptance/spec.md) | 保留；补足官方归属与项目扩展分类 | 未实施/未执行 |
| W6-F66 | 沿用注解生成链，覆盖官方8个MCP URL/方法及base_url/auth_token输入、has_auth输出、tool_ref/detail、ref/status/text和无调用省略；Swagger可试调；验收：无漏项/漂移或手写第二份契约；无额外详情/discover必需操作，不重写第五课JWT或材料UUID | O05, O11, O22, O23, O32, O33, O40 | 官方强制/兼容基线（附项目实现选择） | 5.3 | W6-T66 | [W6-F66 OpenAPI官方兼容](specs/week6-mcp-acceptance/spec.md) | 修订 | 未实施/未执行 |
| W6-F67 | 以隔离MySQL/Qdrant和独立MCP真实网络调用覆盖正常/错误鉴权、Schema、缓存、权限、迁移、审计与全部拒绝/故障；验收：正常全通过，注入失败非零；不能用模拟Go返回替代两个真实工具 | O01, O33, O34, O40 | 官方建议、项目要求 | 6.1 | W6-T67 | [W6-F67 自动化真实工具矩阵](specs/week6-mcp-acceptance/spec.md) | 保留；补足官方归属与项目扩展分类 | 未实施/未执行 |
| W6-F68 | 自动验证A/B双向和同班owner边界，并由真实同伴用本人账号完成服务器/工具隔离、令牌不回传、断开denied审计及本班审计四项，留请求响应证据；验收：自动权限全部通过且真实同伴四项证据齐备；自动账号不可冒充同伴，缺人不得勾选人工任务 | O06, O34, O36 | 官方强制/兼容基线（附项目实现选择） | 6.2/6.7 | W6-T68 | [W6-F68 双向隔离与真实同伴](specs/week6-mcp-acceptance/spec.md) | 修订 | 未实施/未执行 |
| W6-F69 | 回归Bearer/撤销、材料/索引、三种检索、旧Ask零命中、技能启停/解绑/路由、6条窗口、引用、SSE及恢复；验收：原保护矩阵全部通过；不沿用旧无据零调用断言到Week5 | O37 | 官方强制/兼容基线（附项目实现选择） | 6.3 | W6-T69 | [W6-F69 第五课回归](specs/week6-mcp-acceptance/spec.md) | 保留；补足官方归属与项目扩展分类 | 未实施/未执行 |
| W6-F70 | 项目继续交付脱敏JSON/HTML与完整测试元信息；课程任务清单第五节将apitest/run_all.py及HTML列为建议，不能称老师强制；验收：报告真实区分执行状态和模型性质、HTML不入Git；官方建议与项目必交付分开 | O35, O40 | 官方建议、项目要求 | 6.4 | W6-T70 | [W6-F70 报告与官方建议分类](specs/week6-mcp-acceptance/spec.md) | 修订 | 未实施/未执行 |
| W6-F71 | 真实模型验证需新的明确上限/费用授权，默认禁用、禁止自动重试；真实教学安全需人工审读，不把受控模型算真实；验收：无授权零收费请求；账本逐次扣减，人工审核工具依据/公式/换算与注入边界，未执行明确列出 | O38 | 项目补强（非老师新增要求） | 6.5 | W6-T71 | [W6-F71 真实模型预算人工验收](specs/week6-mcp-acceptance/spec.md) | 保留；补足官方归属与项目扩展分类 | 未实施/未执行 |
| W6-F72 | 保留现有change及本轮文档边界，对已读取任务清单/概览/9子页/官方OpenAPI建立双向映射，明确来源冲突与项目扩展；完成后等待审核不自动实施；验收：来源核对记录真实可定位，未宣称浏览器已调试或业务已验收；待决冲突保留，实施需新指令 | O38, O40 | 官方强制/兼容基线（附项目实现选择） | 6.6 | W6-T72 | [W6-F72 来源审核与后续门槛](specs/week6-mcp-acceptance/spec.md) | 修订 | 未实施/未执行 |

## 六、Task → Requirement/Test 反向索引

主实施任务全部可在tasks.md按编号定位。阶段门槛/综合验收任务会覆盖额外子集，不能以一次单测替代全阶段。

| Task | Requirement | Test |
|---|---|---|
| 1.1 | W6-F01, W6-F10 | W6-T01, W6-T10 |
| 1.2 | W6-F02, W6-F03 | W6-T02, W6-T03 |
| 1.3 | W6-F04, W6-F05 | W6-T04, W6-T05 |
| 1.4 | W6-F06, W6-F07, W6-F08 | W6-T06, W6-T07, W6-T08 |
| 1.5 | W6-F09, W6-F11, W6-F14 | W6-T09, W6-T11, W6-T14 |
| 1.6 | W6-F13 | W6-T13 |
| 2.1 | W6-F28 | W6-T28 |
| 2.2 | W6-F29 | W6-T29 |
| 2.3 | W6-F30 | W6-T30 |
| 2.4 | W6-F26 | W6-T26 |
| 2.5 | W6-F12 | W6-T12 |
| 2.6 | W6-F27 | W6-T27 |
| 2.7 | W6-F31, W6-F32, W6-F33, W6-F62 | W6-T31, W6-T32, W6-T33, W6-T62 |
| 3.1 | W6-F15, W6-F17, W6-F18, W6-F24 | W6-T15, W6-T17, W6-T18, W6-T24 |
| 3.2 | W6-F19, W6-F20 | W6-T19, W6-T20 |
| 3.3 | W6-F21, W6-F22, W6-F23 | W6-T21, W6-T22, W6-T23 |
| 3.4 | W6-F16, W6-F25 | W6-T16, W6-T25 |
| 4.1 | W6-F34, W6-F35, W6-F43 | W6-T34, W6-T35, W6-T43 |
| 4.2 | W6-F36, W6-F37, W6-F38, W6-F39 | W6-T36, W6-T37, W6-T38, W6-T39 |
| 4.3 | W6-F40, W6-F41, W6-F42, W6-F44, W6-F45, W6-F52 | W6-T40, W6-T41, W6-T42, W6-T44, W6-T45, W6-T52 |
| 4.4 | W6-F48, W6-F56 | W6-T48, W6-T56 |
| 4.5 | W6-F47, W6-F49, W6-F50, W6-F51, W6-F53, W6-F54, W6-F60 | W6-T47, W6-T49, W6-T50, W6-T51, W6-T53, W6-T54, W6-T60 |
| 4.6 | W6-F55, W6-F57, W6-F58, W6-F59 | W6-T55, W6-T57, W6-T58, W6-T59 |
| 4.7 | W6-F46, W6-F61 | W6-T46, W6-T61 |
| 5.1 | W6-F63 | W6-T63 |
| 5.2 | W6-F64, W6-F65 | W6-T64, W6-T65 |
| 5.3 | W6-F66 | W6-T66 |
| 6.1 | W6-F67 | W6-T67 |
| 6.2 | W6-F68 | W6-T68 |
| 6.3 | W6-F69 | W6-T69 |
| 6.4 | W6-F70 | W6-T70 |
| 6.5 | W6-F71 | W6-T71 |
| 6.6 | W6-F72 | W6-T72 |
| 6.7 | W6-F68 | W6-T68 |

| 0.1–0.4 | 全部来源/兼容决策、72项基线与工程范围冻结 | T72及来源/保护校验 |
| 3.5/3.6 | 8操作注解和MCP管理阶段集成 | T15–T33/T66 |
| 5.4/5.5 | 同仓Compose与浏览器代理门槛 | T13/T26/T47/T63–T66 |
| 6.3/6.4/6.6 | 综合回归/报告/最终审核 | T01–T72对应阶段子集 |

T短编号统一前缀W6-。72项均有主Task/Test且O40条款均有落点；官方建议/示范/研讨与项目自选分类独立，不能把72或40当官方强制条目总数。无遗漏声明限于上述已读公开页面范围，不包括未发布的课堂口头补充。


## Phase 5 实施证据（2026-10-19）

5.1→F63/T63；5.2→F64,F65/T64,T65；5.3→F66/T66；5.4→T13,T26；5.5→T47,T63–T66。反向均指向原任务及主场景，未新增或删除合同。实际实现、18组主浏览器+3组恢复、279 Go叶用例、3 SSE及独立Compose证据见[Phase5报告](phase5-verification.md)和[机器证据](phase5-verification.json)。测试夹具与真实服务路径在报告中分开说明。

## Phase 6 最终实际证据

原72行规格与追踪保持；[最终覆盖表](phase6-coverage.md)逐行保留Requirement → Task → Scenario并关联实际套件证据，JSON可反查Task/测试。23条清单、概览加9子页、8DoD及官方8操作复核通过。T68/T71原始参与门槛未执行，按用户批准项目调整完成；不得据此改写原官方要求。
