# CampusClaw Week 5课程对齐最终验收报告

**本项目调整后的验收标准已满足：29/29，5.4、5.5完成。官方原始要求的真实同伴操作与确认未执行，不宣称原始官方要求全部满足。**

本报告整理已经执行的测试与审核，不代表本轮重新执行这些测试。用户本轮明确授权将本项目5.4从真实同伴方式调整为独立账号自动化P1～P4交叉验证，正式修订proposal/design/DA3/tasks；不是追认曾有同伴参与。现有28项及浏览器、数据库证据只读复核通过，调整后的项目必需证据与5.5风险报告齐备。本轮只有规约/任务/验收文档修订、路径/hash及strict检查，没有调用模型、运行数据库迁移、修改业务代码或部署。

## 1. 背景、目标及判定依据

原change `add-week5-ai-tutor-and-streaming-chat`按原范围完成32/32；其历史记录保留。本change在现有Go/MySQL/Qdrant架构上对齐课程：技能文本CRUD、多绑定与确定性路由、材料空绑定不检索、默认角色、停用后续聊、消息级历史窗口、无证据生成、早meta与候选/最终引用、注解生成OpenAPI，以及增量schema兼容与验收证据。

依据为本change的[proposal](proposal.md)、[design](design.md)、[tasks](tasks.md)、四份spec及[Phase0官方来源映射](phase0-verification.md)。官方S1～S9来源、冻结决策及原文复核记录在[来源核对记录](phase0-source-review.json)；本轮没有重新联网审查课程内容。

四份有效规格：[教师配置TC1～TC5](specs/tutor-configuration/spec.md)、[学生会话SC1～SC6](specs/student-streaming-conversations/spec.md)、[文档及验收DA1～DA3](specs/week5-api-documentation-and-acceptance/spec.md)、[升级SE1～SE3](specs/week5-schema-evolution/spec.md)。CLI的planning artifacts complete只说明规划齐备；项目29/29还以调整后任务、原始证据和本轮复核为依据，不能单凭planning或strict判通过。

### 1.1 用户选择的项目验收方式调整与官方差异

| 内容 | 官方原始要求 | 本项目经用户明确选择后的要求 | 实际事实 |
|---|---|---|---|
| 5.4执行与确认方式 | 真实同伴本人操作四项并确认，可追溯参与者及请求/响应 | 独立教师、同班不同owner学生、外班学生账号，由Codex自动化执行并提供完整技术证据 | 无真实同伴、无人工签名/反馈；自动化28项通过 |
| 技术覆盖 | 会话归属、助手隔离含停用续聊、班级边界、技能停用/解绑 | 保留P1～P4全部行为、错误码、无副作用、SSE/引用/历史与持久化标准，0失败/0未执行且无阻塞 | 未因执行方式调整放宽技术断言 |
| 结论边界 | 原始官方同伴条件须真实参与才能声称满足 | 本项目5.4及依赖它的5.5按修订DA3完成 | 29/29是项目结论；官方同伴条件仍未履行 |

本调整来自用户对本课程项目验收方案的选择，不是老师或课程平台批准的事实。官方S7、Phase0来源映射和原始验证报告保持不改；本项目没有把自动化账号描述为真实同伴，没有虚构参与者、签名或人工反馈。真实课程模型教学质量及用户内容审核仍由5.3独立证明。数据库正式迁移、上线或归档不在本次完成范围。

## 2. 证据分层与索引

| 编号 | 记录及原始证据 | 证明范围 |
|---|---|---|
| E0 | [Phase0](phase0-verification.md)、[check](phase0-check-result.json) | 官方要求、工具/角色/迁移决策及保护矩阵；静态核对 |
| E1 | [Phase1](phase1-verification.md)、[check](phase1-check-result.json)、[正式迁移预检模板](phase1-migration-preflight.md) | 隔离真实MySQL、升级/回填/CRUD/内部删除事务、备份恢复 |
| E2 | [Phase2](phase2-verification.md)、[check](phase2-check-result.json) | 技能API、路由/材料、资格/历史边界及Week4保护 |
| E3 | [Phase3](phase3-verification.md)、[check](phase3-check-result.json) | 隔离真实MySQL/Qdrant、受控模型、SSE/故障/恢复 |
| E4 | [Phase4](phase4-verification.md)、[check](phase4-check-result.json)、[浏览器原始记录](../../../runtime/align-browser-9c9349f8/browser.json) | 教师/学生页面、真实Edge/Nginx、Swagger及契约修复 |
| E51 | [Phase5.1/5.2](phase5-verification.md)、[check](phase5-check-result.json)、[接口结果](../../../reports/apitest/20261018T021738Z-9e44c742/result.json) | 实际自动化27项接口检查/422断言、回归及恢复；不是同伴证据 |
| E52 | [八DoD浏览器原始记录](../../../runtime/align-browser-d08b5cd8/browser.json)、[代理配置](../../../runtime/align-browser-d08b5cd8/nginx-effective.txt) | 真实浏览器/代理/Go/隔离依赖，使用受控模型；dod.D1～D8 |
| E53 | [真实模型专项](phase5-real-model-verification.md)、[修复后复测](phase5-citation-retest-verification.md)、[后续八轮](phase5-remaining-real-verification.md)、[用户人工审核确认](phase5-human-review-confirmation.md) | 课程真实网关、实际回答/引用/流式与用户内容审核 |
| E54 | [P1～P4技术报告](phase5-p1p4-technical-verification.md)、[28步原始记录](../../../runtime/align-browser-4b8ddd77/browser.json)、[页面补证](../../../runtime/align-browser-4b8ddd77/ui-production.json) | Codex自动化模拟账号，真实浏览器/服务、受控模型；无真实同伴 |

历史记录中的旧进度、阶段未授权或Swagger失败说明保留其原时点，不按当前状态解释。尤其原P1～P4技术报告及phase5-report-check.json中“缺真实同伴、27/29”的判断符合调整前规约，原文与原始结果保留；本次只将已通过技术证据按用户新选择纳入项目验收。修订前六份当前文档全文及hash保存在[调整基线](project-acceptance-adjustment-baseline.json)。当前状态以修订tasks、5.3人工审核确认、本报告及本轮复核为准，未将早期失败改写为通过。

## 3. F01～F13实现与证据映射

| 要求 | 实现及验证结论 | 对应规格/任务与证据 |
|---|---|---|
| F01 技能文件与CRUD | UTF-8 source纯解析、字段/长度边界、原文往返、教师创建读取更新删除及旧toggle兼容；通过 | TC2/3、SE2；1.1/1.3/1.4/2.1/4.1；E1/E2/E4/E51 |
| F02 多绑定及确定性路由 | 当前绑定且启用技能按当前问题trigger子串评分，正分最高、同分小ID，零分不选；实际System、启停解绑及快照通过 | TC3/4/5；2.2/2.3；E2/E51/E53/E54 |
| F03 显式材料集合 | 空绑定不检索、Embedding0；非空仅授权集合，跨班绑定原子拒绝；通过 | TC1/3/5；1.4/2.3/4.1；E1/E2/E52.D2/E54.P2/P3 |
| F04 默认助教角色 | 空prompt合法，生成使用Phase0冻结角色；无据有/无技能真实回答获审核通过 | TC1；2.4；E0/E2/E52/E53 |
| F05 停用后续聊 | 停用仅禁新会话，已有会话继续并使用当前配置，owner/class及agent不放宽；通过 | SC1；2.4；E2/E52.D3/E54.P2 |
| F06 历史窗口 | 最近6条有效消息含当前user一次，每条历史正文最多500 Unicode字；原文完整留库；通过 | SC2；2.5/3.1；E2/E51/E53（四轮1/3/5/6条） |
| F07 无据仍生成 | 空绑定、正常零命中、subject过滤归零仍生成，引用[]、不退回Week4固定文案；受控故障与真实无据回答均有证据 | SC3/5；3.2/3.4；E3/E51/E53 |
| F08 标题 | 首问trim后24个Unicode字，后问不改标题；通过 | SC1；2.5；E2/E51 |
| F09 删除清消息 | 仅内部owner/class事务删除，Busy拒绝及中途失败回滚；没有公开会话DELETE或页面批量清理 | SC6；1.5；E1/E3/E51 |
| F10 SSE协议与可靠性 | meta先Flush、delta增量、Finish后done、meta候选与done最终子集、错误及DB一致性；通过 | SC4/5；3.1/3.3/3.5/4.2；E3/E4/E52.D7/E53/E54 |
| F11 有界证据 | top4后subject标题过滤、去空、连续编号；交模型每条最多800 Unicode字，最终子集保留原号；通过 | TC5；2.3/3.3；E2/E3/E51/E52.D6 |
| F12 注解OpenAPI | 唯一维护源为Go处理器注解及类型；固定工具链、重复生成检查、真实Authorize/Execute/SSE；通过 | DA1；4.3/4.4；E4/E51/E52.D8 |
| F13 分层验收与交叉验证 | 自动化、八DoD、真实模型及用户审核已有证据；P1～P4按用户修订后的项目DA3满足，**官方原始真实同伴方式未履行** | DA2/3；5.1～5.5；E51～E54；项目5.4/5.5完成，官方差异见§1.1 |

## 4. 八项官方DoD

以下是E52中实际执行的技术端到端结果，每项均有inputs、outputs、UTC、环境及证据字段；八项通过不自动等价于官方真实同伴要求满足。项目DA3另外依据E54独立账号自动化证据复核。

| DoD | 实际结果 | 证据位置 |
|---|---|---|
| D1 教师可管理助手与技能 | source文件/CRUD/toggle、多绑定、空角色及刷新保持；学生写403；通过 | E52 dod.D1、E4、E51 |
| D2 只用明确绑定资料 | 同题依次绑定A/B/空集，只出现对应来源或[]，空集Embedding0；通过 | E52 dod.D2、E54 P2 |
| D3 会话归属与固定助手 | 他人/外班404，覆盖agent400且未改绑，停用禁新但原会话继续；通过 | E52 dod.D3、E54 P1/P2/P3 |
| D4 引导及停用/解绑差异 | 受控System分支机制通过；真实课程回答及差异另经用户人工审核通过 | E52 dod.D4（受控）、E53（真实）、E54 P4（模拟） |
| D5 无据仍生成 | 正常零命中及空集实际生成，[]且无虚假编号；subject归零有自动化补证；通过 | E52 dod.D5、E51/E53 |
| D6 引用与材料对应 | meta两候选，done仅number=2且不重编号，授权附件对应；真实有据回答主动[1]；通过 | E52 dod.D6、E53 |
| D7 真正增量输出 | meta早于模型HTTP请求；首delta在尾片完成前可见；done、草稿及数据库一致；通过 | E52 dod.D7、E53、E54页面补证 |
| D8 Swagger可实际调试 | 匿名401，Authorize后GET200、source POST201、toggle PUT200、DELETE204，学生SSE调试成功；通过 | E52 dod.D8、E4 |

## 5. 权限、归属及助手绑定

Bearer身份来自服务端用户与班级记录；匿名401，学生写教师配置403，同班他人及外班会话读取/发送404。伪造身份头不能建立身份；Nginx透传Authorization并清除伪身份头。客户端user_id/class_id/agent覆盖字段400，拒绝不产生新消息或改绑。

助手可多技能、多材料或空集合。跨班混合资源保存404，事务回滚完整配置、版本及绑定表，不能部分成功。学生只能选择本班启用助手；显式新建停用助手404，班内无启用助手400。已有会话仍使用原agent的当前配置续聊，不能切换agent。E54保存每次拒绝前后完整会话、消息及绑定快照，而不是仅记录状态码。

## 6. 教师配置、技能与历史快照

教师页面支持文本粘贴、本地UTF-8读取、CRUD、旧开关及多选绑定；空材料明确表示不检索，空prompt采用冻结默认角色。保存后刷新配置仍存在。trigger只针对当前问题评分，不依赖模型路由；选中“解题引导”时追加服务端不给最终答案或完整解法的教学边界。

配置提交后下一轮生效；接受中的生成、历史skill_name、配置版本/hash及证据快照不随停用、解绑或删除重写。E2/E3覆盖并发及快照冻结；E54对启用、停用、重新启用后解绑分别新建会话，保持相同问题及材料，避免历史干扰，并核对旧完成消息完整行值不变。真实教学内容判定来自E53用户审核，不来自受控替身的分支输出。

## 7. RAG、引用与无证据生成

Week5先限定班级及明确材料集合；空集合直接返回空证据，不访问Embedding/Qdrant。非空集合共享原检索，在top4后按选中技能subject过滤标题，再去空、编号及截取模型正文。依赖故障返回准备失败，不伪装正常零命中。

有据System明确要求按evidence.number输出[n]，number不是chunk_index，材料名不能替代数字引用；没有修改引用校验器、自动补写引用或放宽缺失/越界拒绝。无据分支不强制引用，用跨片状态处理虚假数字编号，正常括号不误删；清洗后落库、发送。只有候选来源不意味着已引用，done和历史保存实际子集并保留原编号。

Week4保持keyword/vector/hybrid、RRF、.35接受/.349拒绝、class/payload与ready回表及故障不降级。Ask零命中仍为“资料中未找到相关内容”、sources=[]、对话模型调用0；不将此规则套到Week5。E51完整适用回归已执行，本轮只核对其证据，未重跑。

## 8. SSE、持久化及浏览器

原子保存user与assistant占位；检索、技能/证据快照及历史在meta前准备完毕，不持有跨模型调用的数据库事务。顺序为meta/Flush → 模型Open → Append → delta → Finish → done。meta.status恒ok、citations为完整候选或[]；done只列实际引用子集。

meta前参数/身份/权限/资源/并发错误返回相应HTTP JSON；初始事务失败回滚无孤儿，接受后准备失败终结占位。meta后模型连接、生成或引用失败保持HTTP200，发error且无成功done；断连不能保证客户端收到error。失败/取消保留可靠前缀及failed/interrupted，终结数据库全不可用时需Recover；done丢失可查历史completed，不自动重试模型。

E3/E51有连接故障、部分片段、超时、取消、DB失败、崩溃恢复及并发409注入。E4/E52真实浏览器验证UTF-8/EOF、候选/实际来源、草稿、附件和401清理；E54补充生产页面门控截图与首段数据库前缀，完成及刷新正文一致。保留原SSE解析器、Nginx禁buffer/cache、120秒读取超时；390px无横向溢出、讲义分页和原品牌导航的验收见E4/E52。

## 9. OpenAPI/Swagger与接口验收

工具链为swag v1.16.6 → kin-openapi v0.133.0 → OpenAPI3.0.3，Go处理器注解及其类型是唯一维护源；移除旧Python手工契约源。覆盖source CRUD/旧toggle oneOf、完整助手配置、会话、Week4 Search/Ask、材料与认证、SSE四事件及错误分界；没有公开会话DELETE。

生成命令：`powershell -NoProfile -File scripts/generate_openapi.ps1 -Check`。E4及E51已有实际成功日志，重复生成无差异；故意修改临时产物时检查能拒绝漂移。动态路由对照含DELETE，当前观察的26个操作不是固定通过门槛。Phase2～3的Swagger失败原记录保留，后续Phase4修复并通过，Phase5再次通过。真实Authorize/Execute已执行，不以静态页面可加载代替接口调试。

本轮未重新生成OpenAPI或重跑业务测试；已有[生成检查日志](../../../runtime/phase5-openapi-check.log)、[源码单元日志](../../../runtime/phase5-unit-final.log)和E51接口原始结果均已核对存在。

## 10. 真实课程模型与人工审核

真实网关为 `https://ai-gateway.devops.hello1023.com/v1`，请求模型ID为course-chat、course-embedding，Embedding OpenAI格式/2048维。配置依据用户核对教师PDF及项目溯源；模型别名不推断为已知底层供应商版本。鉴权、对话和Embedding连通性HTTP200，实际输出维度2048。没有在报告保存API Key或Bearer。

首轮有据无技能回答缺数字引用，正确error/failed并停止；数学内容不能抵消格式失败。经授权仅加强Week5有据System提示词，受控边界测试通过后单次真实复测主动生成[1]，用户审核通过；原失败回答保留。其后八轮覆盖有据有技能、无据有/无技能、停用、trigger未命中、解绑及四轮历史，均技术通过，并由用户明确确认全部内容审核通过。轻微教学追问体验改进不构成该次验收失败。

- [原首次失败完整回答](../../../runtime/align-real-26bcce67/answers-for-review.md)
- [修复后有据无技能完整回答](../../../runtime/align-real-26bcce67/citation-retest-b8478118/answers-for-review.md)
- [后续八份完整真实回答](../../../runtime/align-real-26bcce67/remaining-real-01a6ec28/answers-for-review.md)
- [后续实际输入、来源、SSE及数据库消息](../../../runtime/align-real-26bcce67/remaining-real-01a6ec28/result.json)
- [用户人工审核确认与5.3完成依据](phase5-human-review-confirmation.md)

用户审核是本轮会话中的明确确认，不虚构签名或将Codex评价当作用户结论。真实无据回答没有伪造数字引用；真实有据引用及delta拼接/DB一致，meta早于模型Open；历史实际窗口1/3/5/6条、当前问题一次。候选两个但最终仅引用第2条等更广的分支证据来自受控矩阵，不能冒称每项都由真实模型再次执行。

## 11. P1～P4自动化28项技术结果

**本轮由Codex自动化模拟教师及学生操作，完成P1～P4技术交叉验证。未邀请真实同伴，未取得真实同伴人工确认。**

真实浏览器及服务不等于真实同伴。主矩阵测试身份为ta（A教师）、sa（A学生1）、sa2（A学生2）、sb（B学生），另tb仅准备B班资源；均是隔离测试账号。实际身份与资源ID来自E54原始记录。受控模型根据实际服务端System返回可区分输出，仅验证配置、引用、流式及落库链路。

| 子步骤 | 验证内容及预期/实际结果 | 状态 |
|---|---|---|
| P1.own_create_send | 本人创建201、发送200/SSE完成 | 通过 |
| P1.refresh_read_continue | 刷新读取200、再次续聊完成 | 通过 |
| P1.other_list | 同班他人列表200但不含C1 | 通过 |
| P1.other_read | 他人直接读取404，DB不变 | 通过 |
| P1.other_send | 他人发送404，DB不变 | 通过 |
| P2.A_B_bindings | A/B仅各自材料标识/引用，技能不串用 | 通过 |
| P2.override_agent | 发送附带agent覆盖400，DB不变 | 通过 |
| P2.disabled_new | 停用A但仍有B启用，显式新建A为404 | 通过 |
| P2.disabled_continue_current | 原C1续聊200，用A当前更新配置，固定agent不变 | 通过 |
| P2.no_enabled_400 | 班内全部停用，新建400，无新增会话 | 通过 |
| P2.history_preserved | 停用/改配置后旧消息快照保持 | 通过 |
| P3.foreign_read | 外班读取C1为404，DB不变 | 通过 |
| P3.foreign_send | 外班发送C1为404，DB不变 | 通过 |
| P3.foreign_agent | 外班读取A助手404 | 通过 |
| P3.foreign_skill | 外班读取A技能404 | 通过 |
| P3.foreign_material | 外班读取A材料404 | 通过 |
| P3.foreign_download | 外班下载A材料404 | 通过 |
| P3.foreign_reference | 外班访问引用实际对应材料文件404 | 通过 |
| P3.foreign_create | A学生选择B班C创建会话404 | 通过 |
| P3.class_override | 请求体class_id覆盖400 | 通过 |
| P3.user_override | 请求体user_id覆盖400 | 通过 |
| P3.material_binding_atomic | 本班+外班材料混合绑定404，完整配置及关系不变 | 通过 |
| P3.skill_binding_atomic | 本班+外班技能混合绑定404，完整配置及关系不变 | 通过 |
| P3.no_side_effects | 所有拒绝后完整会话/消息/助手及绑定无变化 | 通过 |
| P4.enabled | 新会话、技能命中，实际System及快照一致，引导且合法[1] | 通过 |
| P4.disabled | 新会话、停用但仍绑定，无技能注入，可给结论且合法[1] | 通过 |
| P4.unbound | 新会话、启用后解绑，无技能注入，可给结论且合法[1] | 通过 |
| P4.history_preserved | 旧消息及启用场景审计/快照保持 | 通过 |

最终主矩阵28通过、0失败、0未执行；七次成功流均meta→delta×5→done。另生产页面补证通过，原SSE解析测试3/3通过。以上数量不包含真实同伴项目，真实同伴四项均未执行。

失败历史另保留：首次驱动误用CampusAPI请求非/api控制路径，在生成前被拒；修正测试接线后新建隔离资源完整重跑。首次页面补证的CDP响应体导出失败，保留截图与记录，改为旁路观察原解析器事件后补证成功。未修改权限期望、伪造响应或改业务代码。[首次驱动失败](../../../runtime/align-browser-d7912cdf/browser.json)、[页面导出失败记录](../../../runtime/align-browser-4b8ddd77/ui-first-failure.json)可复核。

## 12. 环境、调用预算及实际用量

| 层次 | 实际环境/范围 | 边界 |
|---|---|---|
| 存储/检索受控集成 | 独立MySQL8.4.4、Qdrant1.13.4、内部网络；新随机库及collection | 本地受控网关，不是收费模型 |
| 浏览器/代理 | 容器Go1.26.0、主机Go1.27.1、Edge155.0.4283.45、Nginx1.28.0 | 真实浏览器/生产路由，模型可能为受控替身，逐记录标注 |
| 5.2最终DoD | 数据库align_w5_afa9de09163b11cc、collection align_browser_18df7e6ee5935887 | E52，受控模型 |
| 5.3真实复验 | 数据库align_w5_9597acd9f56c179d、collection align_real_18df805af4916d91 | 课程真实网关；按授权串行、无自动重试 |
| P1～P4补证 | 数据库align_w5_119a35ab79a22509、collection align_browser_18df9290ee530dbd | E54，本地controlled模型/2维测试Embedding；无真实同伴 |

[原真实调用账本](../../../runtime/align-real-26bcce67/budget.json)累计对话11/12、Embedding8/8，19个请求条目，保持blocked。包括连通性、首次格式失败、修复后单次复测和后续八轮，未重置计数；本报告轮新增对话0、Embedding0。按19条请求各自的usage或response_usage字段完整汇总，网关累计报告**7978 total tokens**。旧专项记录的7833只汇总response_usage，漏计最初连通性对话113及Embedding32，共145；本报告纠正统计口径，原账本和历史记录不改。后续八轮及其查询Embedding报告6504（prompt3484、completion3020）。没有实际货币费用字段，不据Token推算价格，也不声称免费。未使用的1次对话额度不构成继续调用授权。

P1～P4本轮成功夹具含页面补证及一次导出失败，共记录本地模型9次、Embedding13次；另首次失败驱动夹具有本地准备调用，不能混入上述真实账本。原始统计位于E54页面补证controlled_gateway及失败目录。所有报告均无真实密钥、JWT或人工签名。

资源隔离检查、课堂容器保持停止及测试服务保留记录见[隔离元数据](../../../runtime/align-browser-4b8ddd77/isolation.json)、[结束状态](../../../runtime/align-browser-4b8ddd77/final-resource-status.json)。这些是执行时证据，不保证服务现在仍在线；tmpfs停机可能丢失数据，夹具有24小时生命周期。正式课堂库、业务卷与原Qdrant集合未因本change测试被迁移或重建。

## 13. 数据库升级、旧历史保护及正式迁移风险

E1及E51证明v1→v2→v3、v2→v3、重复升级、41个DDL提交后中断点续跑、catalog漂移拒绝、旧数据冲突预检、一次性回填及事务回滚。模拟DDL步骤中断不是声称经历主机断电；漂移检查没有静默修复。旧材料/切片/助手/技能/会话/消息ID和历史审计保留，解绑不能由旧JSON重新复活。

原Week5的17份历史文件及32/32保持，当前规范替代旧Week5无据零调用、空meta、固定技能、空prompt拒绝、6轮历史等冲突行为；不是将旧历史结果改成新契约成功。Week4保护证据独立保留。

正式迁移尚未执行。必须单独确认目标、权限、停写窗口与负责人，先做完整catalog/数据冲突/活动turn预检，取得一致数据库及材料备份，并在独立目标实际恢复核对。空材料不再等同全班，需教师明确重绑。MySQL多步DDL隐式提交，优先按账本前向恢复；旧v2严格catalog二进制不能直接连v3，回退可能损失升级后新增写入，须先制定导出/重放方案。不能删除新表/卷/collection或跳过catalog假回退；RAG_ENABLED也影响Week4，不能称仅禁Week5。部署、切换、迁移与清理均需独立授权。

## 14. 官方差异、已知限制及后续建议

1. **官方差异仍存在：真实同伴本人四项操作及确认未取得。** 本轮用户明确修订本项目DA3为独立账号自动化，故不再将此作为项目5.4/5.5阻塞；不得宣称它已经满足原始官方同伴条件，也没有虚构参与者、时间、签名或反馈。
2. **调整后项目5.4、5.5条件均已满足。** 独立账号28项、请求/响应、真实浏览器/数据库、5.3真实模型及用户审核、完整F/DoD/失败历史、迁移风险与证据核对齐备，无未解决项目技术阻塞。29/29仅为调整后的项目标准，不是原始官方要求全部满足、已部署或正式迁移就绪的声明。
3. 已通过的真实教学审核仅针对保留的实际回答；后续模型仍可能不遵循引用或教学边界。继续保留校验与失败策略，不靠自动补引用掩盖问题。
4. 非阻塞体验建议：教学追问可更自然、提示粒度更贴近学生已尝试步骤并减少机械重复；本轮未改提示词或业务代码。
5. strict只证明OpenSpec结构；文件存在/hash一致证明证据可定位且未改写，不证明历史服务当前在线、所有测试本轮重跑或正式库无数据冲突。
6. 如后续要求恢复原始官方同伴验收，仍须邀请真实参与者亲自完成四项并留下确认，不能复用自动化账号冒充；这不是本次项目29/29所证明的事实。任何后续真实模型、迁移、发布或资源清理仍须相应授权。

## 15. 本报告轮核对与结论

本轮已读取当前change相关规约、任务、验证及机器记录，并按用户授权修订项目方式。最终报告全部本地链接、主要原始日志、28步状态、七轮SSE/引用/DB及旧快照、身份独立、权限拒绝无副作用、生产页面证据、5.3账本及用户确认均只读核对。详细路径、SHA256、修订范围、Git工作区及strict退出码见[本轮复核](project-acceptance-adjustment-check.json)；修订前文档全文及保护hash见[调整基线](project-acceptance-adjustment-baseline.json)，strict输出见[本轮strict日志](project-acceptance-adjustment-strict.log)。原[报告核对](phase5-report-check.json)及[原报告基线](phase5-report-baseline.json)保留调整前时点，不覆盖。

**验收结论：按用户明确修订后的本项目标准，5.4独立账号P1～P4验证及5.5报告条件全部满足，任务29/29。5.3真实课程模型及用户审核保持有效；没有新增真实调用，账本仍为对话11/12、Embedding8/8、7978 tokens。官方原始真实同伴要求未履行，不能宣称原始官方要求全部满足。没有修改业务代码、迁移脚本或正式环境，没有commit、push、部署或归档；本轮完成后停止。**
