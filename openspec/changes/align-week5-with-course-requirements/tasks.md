# Implementation status and project acceptance method

The USER explicitly revised project task 5.4 to independent-account automated P1-P4 verification. Original official genuine-peer requirements remain recorded and have NOT been performed. Task 5.4 is complete after read-only review of 28 passed / 0 failed / 0 unexecuted substeps, real browser and database evidence; task 5.5 is complete after final report, risk, evidence-path, preservation and strict checks. Progress: 29/29 under the USER-adjusted project standard. No genuine peers or signatures are claimed. See [final report](acceptance-final-report.md) and [revision check](project-acceptance-adjustment-check.json).

Real course ledger stays locked: dialogue 11/12, Embedding 8/8, 7978 total tokens. No new model calls or business changes. Original Week5 32/32 and original verification history preserved.

## 0. Phase 0：基线与实施决策冻结（3项）

依赖：用户批准Phase0。退出：0.1–0.3交付本change下的复核记录，确认后续实现不改变外部约定；不勾选任何业务实施项。

- [x] 0.1 固化官方来源、F01–F13与八项DoD映射并核对旧32/32保护（DA2/DA3）。范围：只读重核design的S1–S9、旧change/工作区路径及已有验收报告，登记日期、来源、旧冲突替代关系。验收：13项无遗漏、F09无公开DELETE、F13人工待证与工程选择明确；测试：文档链接/需求/任务引用一致性和旧文件hash基线，不运行有副作用测试。
- [x] 0.2 固化Week4及流式回归保护矩阵（DA2、SC5）。范围：只读检查认证、search/ask、向量/索引、SSE解析/故障、Nginx、Swagger、Compose现有用例和命令，定义隔离环境和证据字段。验收：每项保护有具体命令/构建标签/断言/安全前提，已执行与未执行明确分开；测试：静态核对“Week4零命中零调用”“Week5仍生成”不会共享错误断言，不创建业务测试或启动环境。
- [x] 0.3 冻结迁移预检清单与构建工具方案（SE1–3、DA1）。范围：只读检查v2升级器/catalog/initializer及注解工具官方能力说明，选定工具与版本、OpenAPI3/SSE转换链；确认design关联表、旧种子source、toggle兼容、meta失败边界、内部删除方案。验收：在本change记录预期v3结构、冲突阻止条件、DDL中断/回退风险、可复现生成命令与可选措辞；测试：静态确认不引入运行时框架、不跳过catalog，不执行DDL/安装/代码试验，若方案会改变spec则先更新本change并复审。

## 1. Phase 1：增量存储、技能解析与兼容（6项）

依赖：Phase0退出及对应实施授权。内部顺序：1.1与1.2可独立；1.3依赖1.1/1.2；1.4依赖1.1–1.3；1.5依赖1.2；1.6依赖1.1–1.5。退出：隔离v3升级与解析/清理验证通过，无课堂数据操作。

- [x] 1.1 实现技能文本解析纯核心（F01、TC2）。范围：`internal/tutor/`解析source、注释/空行/字段、UTF-8、原文、五项元数据及错误。验收：必填/重复/未知/工具/启用和8000/32/120/32/2000字限制符合spec；测试：各上限及加1、CRLF、前导注释、正文#、空正文、冒号值和非法UTF-8表驱动测试，无外部依赖。
- [x] 1.2 实现版本化v3增量DDL与完整catalog验证（SE1/SE3）。范围：MySQL schema准备/版本分派、技能解析列、scope唯一索引、两关联表复合FK、initializer就绪版本与catalog；不得只把新DDL混入旧v2文件。验收：HTTP启动不执行DDL，完整结构和ngram可校验，部分升级不ready；测试：隔离MySQL新库、v1→v2→v3、v2→v3、重复执行、DDL步骤中断续跑、索引/FK/列/标记漂移拒绝。
- [x] 1.3 实现旧技能/绑定预检与一次性回填（F01/F02/F03、SE2）。范围：仓储迁移映射、合成source来源、保留ID/enabled/revision/旧审计；合法JSON到关系，空/null按design，未知归属/重名/非法绑定阻止。验收：无全班自动扩绑，class=NULL占位仍隔离；测试：升级前后ID/正文/引用hash与数量、冲突零静默修复、回填中断重试及教师解绑/编辑后重复升级不复活旧JSON。
- [x] 1.4 实现当前技能CRUD与绑定仓储（F01/F03、TC3）。范围：`internal/repository/mysql/tutor.go`等读取解析字段、source完整保存、enabled兼容分支、技能删除清关系、助手多关系写入和原子快照，停用不改变现有消息。验收：同班唯一/外班404/事务回滚/版本和source一致；测试：真实隔离MySQL CRUD、同名冲突、源文本往返、删除绑定、并发编辑/快照与旧历史不变。
- [x] 1.5 实现会话内部事务删除（F09、SC6）。范围：conversation存储接口及MySQL实现，锁本人+班级会话、活动轮Busy、先消息后会话同事务清理；不注册HTTP路由。验收：无孤儿、无跨会话影响、失败整体回滚；测试：隔离库成功、删除中失败、NotFound一致、生成中拒绝，路由静态断言无公开会话DELETE。
- [x] 1.6 完成升级恢复和旧数据兼容阶段门槛（SE1–3）。范围：隔离测试与新change验收记录，核对恢复脚本/运维步骤、备份校验及catalog，形成正式迁移预检模板；本任务不升级课堂库。验收：新/旧/重试矩阵全通过，恢复到独立目标可验证，明确旧二进制不兼容/新增记录损失；测试：受控中断与恢复演练，无自动DROP/卷/collection操作、无真实模型请求。

## 2. Phase 2：教师接口、词法路由与会话规则（5项）

依赖：Phase1退出及对应实施授权。2.1依赖1.4；2.2依赖2.1；2.3依赖2.2；2.4依赖1.4；2.5依赖2.4。退出：新配置、路由和历史核心全部有针对性测试，Week4保护用例通过；完整无据/SSE闭环留给Phase3。

- [x] 2.1 接入source技能读写API和旧toggle兼容（F01、TC2/TC3）。范围：HTTP Tutor/handler路由新增技能单条GET/POST/DELETE、PUT source，保留enabled-only，严格字段和教师guard。验收：响应含原文/元数据、source/开关混合400、学生403、外班404、重名400且DB不变；测试：HTTP+真实隔离MySQL各角色/匿名/错误/DB断言及旧开关客户端回归。
- [x] 2.2 实现多候选确定性路由及动态教学边界（F02、TC4）。范围：Tutor候选读取/纯评分/当前q、只加载所选source说明、名称包含解题引导的额外约束、快照hash。验收：最高分/小ID/零分、停用解绑下轮生效，无模型路由；测试：所有分隔符、大小写、重复/空词、乱序候选、未绑定高分、同题启停/解绑、生成中更新不改旧快照。
- [x] 2.3 修复材料空集、subject过滤和模型证据截断（F03/F11及F02、TC3/TC5）。范围：Week5 Tutor检索入口、证据准备/组装，空绑定短路，绑定top4后标题过滤/去空/连续编号、每条800rune。验收：不访问未绑定同班材料，最后一份解绑不扩张，class仍来自principal；测试：空/非空绑定、外班、过滤归零、编号、799/800/801字和中文，捕获模型资料与来源对应；Week4检索nil/三模式阈值与Ask不变。
- [x] 2.4 修复空角色与停用后续聊资格（F04/F05、TC1/SC1）。范围：Tutor校验和fallback、仓储新建/续聊读取及Prepare二次enabled检查，不放宽owner/class。验收：空prompt可用且默认角色有效；停用新建404或无可用400、旧会话续聊，额外agent字段不能换绑；测试：长度/重名/默认角色、新旧会话、同班他人/转班/跨班及学生写配置拒绝。
- [x] 2.5 修复消息级窗口与首问标题（F06/F08、SC1/SC2）。范围：conversation仓储/types的排序、6条含当前user、500rune、角色/空文/草稿处理、完成无据历史、24rune标题；完整原文留库。验收：组装窗口≤6而非6轮、当前user一次、后问仅更新时间；测试：超3轮、长assistant、非法/空记录、失败前缀、无据完成历史、中文标题24/25与列表排序，事务编排接线由3.1完成。

## 3. Phase 3：Week5无依据生成与SSE契约（6项）

依赖：Phase2退出及对应实施授权。3.1先行；3.2/3.3依赖3.1；3.4依赖3.2；3.5依赖3.3/3.4；3.6为阶段闭环。退出：受控流/真实隔离存储故障矩阵和Week4回归通过。

- [x] 3.1 调整接受轮次与准备阶段编排（F06/F07/F10、SC2/SC5）。范围：HTTP conversation和Store Start/证据准备/终结接线；先原子user+占位，再检索与持久化快照、取含当前问题历史，meta前错误标失败并释放占用。验收：无跨网关长事务、初始失败无孤儿、检索失败可追溯且不永久generating、实际技能未调用不伪记；测试：各准备写故障、历史不重复、配置更新、并发409及存储token范围。
- [x] 3.2 接入Week5无证据模型生成（F07、SC3、TC4/TC5）。范围：移除Tutor固定答案/完整技能证据门控，按有据/无据分支组装System/User和历史，沿用现有上游stream客户端。验收：空绑定/正常零命中/过滤归零均生成，角色/所选技能可参与，依赖故障503；测试：有据有技能/有据无技能/无据有技能/无据无技能四组合捕获输入与调用次数，明确旧Ask固定文案零调用仍通过。
- [x] 3.3 实现生成前meta候选与稳定引用协议（F10、SC4）。范围：HTTP先发meta再Model.Open/Next，status恒ok、完整候选/技能、done实际引用保留编号，取消no_evidence状态分支。验收：模型未开始时meta已Flush，有据/无据均可解析，meta后模型连接失败error/HTTP200；测试：门控上游、空数组、只引用第2条、meta前检索503、meta后连接/首片失败及已有解析器回归。
- [x] 3.4 实现无依据跨片编号清洗与完成校验（F07、SC3/SC5）。范围：Week5轻量流过滤，识别跨片数字引用后清洗再Append/delta，空有效回答/整段旧固定文案不假成功，有据校验不放宽。验收：正常文字仍逐段、delta拼接等于DB，[]引用无据可完成，非法引用有据仍失败；测试：逐字节/逐token拆分编号、普通括号/EOF残留、空输出、固定文案、越界/缺引用及持久化失败。
- [x] 3.5 保留流式前缀与失败中断状态（F10、SC5）。范围：复用现有Append/Finish/Recover、取消、heartbeat/Flush和done丢失确认，按新meta边界调整断言；HTTP1.1 keep-alive及现有不缓冲头。验收：每delta前已落库、completed先于done、failed/interrupted保留可靠前缀/技能，无错误后成功done；测试：部分模型失败/超时、初始/片段/终态DB失败、断连取消、崩溃恢复、并发、done丢失，区分注入与真实依赖。
- [x] 3.6 完成会话链路阶段集成门槛（F03/F07/F10、DA2）。范围：新/旧API、隔离MySQL/Qdrant及受控模型综合验证并记录新change；不调用收费模型。验收：Week5新规则及Week4 Bearer/材料/三模式/Ask全通过，共享检索算法不重复，历史旧引用/配置不重写；测试：跨班/跨用户/跨助手参数、绑定变更、真实ready回表、无据调用与Ask零调用、增量事件/DB状态一体断言。

## 4. Phase 4：教师与学生页面、注解文档（4项）

依赖：Phase3退出及对应实施授权；4.1/4.2/4.3可独立，4.4依赖前三项。退出：真实浏览器经隔离现有Nginx可完成管理、流式与Swagger调试。

- [x] 4.1 扩展现有教师页面（F01/F03/F04、TC1/TC3）。范围：tutor.html/tutor.js增加粘贴/本地读取source、CRUD、技能/材料多选、空材料提示，移除prompt必填，沿用CampusAPI。验收：教师两助手不同绑定、source往返/错误提示、停用/解绑反馈，学生无写入口且服务器403；测试：浏览器CRUD、空角色/空绑定/最后解绑、刷新持久及已有配置兼容，不做页面重构。
- [x] 4.2 展示首帧候选与最终实际引用（F10、SC4/SC5）。范围：现有学生UI及Swagger SSE面板，仅适配meta/done新语义，复用sse.js解析器。验收：meta即显示来源/技能、delta持续追加、done标实际来源、失败保留草稿候选标识、无据空来源，旧历史仍可读；测试：浏览器首帧门控、UTF-8跨块、EOF/error/取消、401清身份、done丢失补读和附件权限。
- [x] 4.3 从处理器注解生成文档（F12、DA1）。范围：按0.3固定工具加注解/生成命令与嵌入OpenAPI，覆盖新增技能接口与兼容toggle、新meta和错误分界；删除第二份手写契约源而保留Swagger资产。验收：重生成无差异、Bearer/schema/路径与实现一致、无公开会话DELETE；测试：生成漂移、路由动态枚举、错误/security/SSE检查，避免硬编码23操作，旧接口形状保持。
- [x] 4.4 完成浏览器/代理/Swagger阶段门槛（DA1/DA2）。范围：隔离Nginx复用现有配置、Go入口、真实浏览器与Swagger Execute。验收：meta早到/首delta早于完成，Authorize及技能/配置/会话可试调，buffer/cache关闭和120秒保留；测试：401/403/404、正常/失败SSE、材料上传/下载及旧页面回归，报告记录真实环境，不修改课堂Compose或全局Docker设置。

## 5. Phase 5：整体验收与证据交付（5项）

依赖：Phase4退出及对应实施授权。5.1先行；5.2依赖5.1；5.3/5.4依赖5.2且分别具备限额授权/独立教师和不同owner及班级学生的自动化证据；5.5待调整后的全部项目必需证据齐备。退出：八项DoD技术行为、回归与本项目DA3均有可复核结果，披露官方真实同伴方式未履行；不能将项目完成宣称为原始官方要求全部满足。技术失败、缺证据或阻塞不得忽略。

- [x] 5.1 运行F01–F12完整自动化及Week4回归（DA2）。范围：更新URL/方法apitest与必要单元/build-tag集成脚本，对新规范替换反向断言，保留旧Ask断言。验收：13项矩阵的自动化部分、角色/owner/class、schema/DB拒绝无改动、路由/历史/标题/删除、无据/引用全部有记录；测试：隔离MySQL/Qdrant与受控网关全套，缺环境非成功退出，不因旧32/32或缓存历史算本轮通过。
- [x] 5.2 完成官方八项DoD端到端矩阵（F13、DA2）。范围：默认Go与现有代理路径下的真实浏览器、Swagger、材料来源和SSE时序，记录提交/工作区摘要、命令/时间及脱敏产物。验收：八DoD逐条通过、跨用户跨班/绑定/技能开关差异可见、增量与DB一致；测试：两助手异绑定、强行换agent拒绝且未改绑、同题解绑、零命中仍答、首帧引用、Swagger Execute及旧Compose入口保护。
- [x] 5.3 在授权限额内完成真实模型教学复验（F07/F13、DA2）。范围：沿用已配置供应商和隔离数据，先取得明确调用上限/费用授权；验证有据引导启停/解绑和无据按角色作答。验收：人工审读不给最终答案、关闭可给结论、空依据无编号/非固定文案，调用数及供应商/模型版本独立记录；测试：固定含答案材料、连续追问和无据问题，禁止超额/自动重试；无授权保留未执行，不勾任务。
- [x] 5.4 完成本项目独立账号自动化四项交叉验证（F13、DA3；用户明确调整验收方式）。范围：独立教师、同班不同owner学生及外班学生身份，在隔离环境覆盖P1会话归属、P2智能体隔离/停用续聊、P3班级边界、P4技能启用/停用/解绑。验收：实际请求/响应、预期及实际HTTP、浏览器、SSE/引用、数据库前后状态、时间/版本/身份及资源ID齐备；28项全通过、0失败/0未执行，无权限泄露、改绑、部分写入、串用或持久化阻塞。测试：只读复核现有原始证据即可，不需重复模型调用；标明Codex自动化模拟、无真实同伴确认，保留官方原始方式与项目调整差异，不降低技术断言或伪造参与者。完成依据见project-acceptance-adjustment-check.json。
- [x] 5.5 交付最终验收与迁移风险报告（DA3、SE3，按用户调整后的项目标准）。范围：更新本change最终报告、任务状态与F覆盖，汇总正式迁移前提/降级/恢复演练和风险，原始验证历史不覆盖。验收：调整后的全部项目必需任务有证据，F01～F13、八DoD、5.3真实模型及用户审核、P1～P4自动化28项均保留；明确官方真实同伴方式未履行与项目方式调整，人工/真实/受控分开、秘密脱敏、旧change32/32保持。测试：strict validate、全部报告本地引用/依赖一致性、原始证据hash和Git工作区范围检查；调用账本11/12、8/8、7978 tokens不变，不能以strict代业务通过；不自动迁移/部署/归档/commit/push。

## 数量、依赖与证据原则

Phase0/1/2/3/4/5分别3/6/5/6/4/5项，总29项。阶段门槛是流程依赖，不是新增付费或正式环境操作授权。测试范围有副作用时仅在明确隔离且获准的测试环境实施；不清理/重建课堂资源。覆盖矩阵在design.md，官方来源S1–S9与四份spec共同构成评审依据。
