## Context

动机见 proposal。当前 Go 入口为 `cmd/server/main.go`；认证、三模式检索与 Week4 Ask 共享已有模块，Week5 在 `internal/tutor/`、`internal/conversation/`、HTTP handler 和 MySQL 仓储增量编排。规划时基线证据：`tutor.go:60` 拒绝空 prompt/多个技能，`:162` 无证据旁路；MySQL `tutor.go:215` 续聊也要求启用；`conversation.go:120/208` 取6对历史/64字标题；HTTP `conversation.go:157/213` 先预读模型再发空引用 meta；`stream.go:30` 不截证据；文档来自 Python 手工定义。上述冲突已由Phase1–4按本change替代，原历史保持。

本change按用户本轮明确调整后的项目验收方案完成29/29。5.3真实课程网关回答已获用户审核；5.4独立账号P1～P4自动化验证28/28及浏览器/数据库证据已只读复核；5.5报告与迁移风险齐备。见[最终报告](acceptance-final-report.md)及[调整复核](project-acceptance-adjustment-check.json)。没有真实同伴参与，不宣称原始官方同伴要求已满足；原change32/32及历史证据保持。

### 官方来源登记（本会话 2026-10-16 成功读取）

以下均来自官方 HTML；浏览工具初次失败后，只读 HTTP 请求取得200。这里归纳而非复制课件，不把讨论中的替代方案列为要求。

| 来源 | 章节与约束 | 覆盖 |
|---|---|---|
| [S1 本课重点](https://devops.hello1023.com/课件/第5课-课件-提示词AI技能与流式会话/pages/scope.html) | §1–3 六项工作、角色权限、八项DoD；零到多个绑定、来源首帧到达 | F01/02/03/07/10/12/13 |
| [S2 会话模型](https://devops.hello1023.com/课件/第5课-课件-提示词AI技能与流式会话/pages/session.html) | §1–3 停用后续聊、24字标题、删会话清消息、6条含本轮/500字、owner+class | F05/06/08/09 |
| [S3 技能模型](https://devops.hello1023.com/课件/第5课-课件-提示词AI技能与流式会话/pages/skill-model.html) | §1–5 空prompt回退、空材料不检索、文本字段/上限/解析、CRUD、当前关系及删除解绑 | F01/03/04 |
| [S4 路由组装](https://devops.hello1023.com/课件/第5课-课件-提示词AI技能与流式会话/pages/route.html) | §1–3 子串得分、分隔符/小写、同分小ID、零分不选、subject标题过滤、用户资料800字 | F02/11 |
| [S5 SSE](https://devops.hello1023.com/课件/第5课-课件-提示词AI技能与流式会话/pages/stream.html) | §2–3 四事件、增量、先meta、落库后done、Flush及代理配置 | F10/13 |
| [S6 回答链路](https://devops.hello1023.com/课件/第5课-课件-提示词AI技能与流式会话/pages/answer.html) | §1–3 无据仍生成、不虚构引用、meta恒ok、部分输出保存 | F07/10/11 |
| [S7 实践验收](https://devops.hello1023.com/课件/第5课-课件-提示词AI技能与流式会话/pages/apply.html) | §1 注解生成文档、分层自验；§3 四项同伴记录 | F12/13 |
| [S8 思考分析](https://devops.hello1023.com/课件/第5课-课件-提示词AI技能与流式会话/pages/think.html) | §3–5 确定性、技能与资料独立、上传数据边界；其替代方案非必做 | F02/07及非目标 |
| [S9 Week4衔接](https://devops.hello1023.com/课件/第5课-课件-提示词AI技能与流式会话/pages/preview.html) | §8 Ask无证据固定文案/零调用，与Week5并存 | Week4保护 |

官方 S5 有一处旧图示与 S1/S6/S8 的无据生成规定矛盾；采用后者反复明确的正文与DoD。S6对首片失败有503措辞，同时规定meta在模型前、流开始后仅error；本设计以首事件是否已提交划分HTTP和事件，绝不在meta之后改HTTP状态。

## Goals / Non-Goals

**Goals:** 在现有模块边界内补官方外部行为；数据库关系支持安全删除和当前配置；修复最小编排范围，并使迁移、文档和验收可复核。

**Non-Goals:** 不新增公开会话DELETE、不改JWT生命周期/材料UUID/1-based切片、不建设多Agent或技能执行框架、不重写共享检索或SSE解析器、不重建Qdrant、不自动部署/归档旧change。WebSocket、语义路由、多副本租约、技能市场和额外输出安全平台均为C类工程扩展，不是本change验收条件。

## Decisions

### D1. 后继规格与授权边界

旧 `tutor-configuration` 的固定技能/证据门控/空prompt400，由新TC1–TC5替代；旧 `student-streaming-conversations` 的零命中旁路/完整轮次历史/空meta，由SC2–SC5替代；旧文档契约由DA1–DA3补充。同名旧要求未冲突的Bearer、私有性、逐段持久化与恢复继续受保护。本文引用的TC/SC/DA/SE编号是新spec中需求标题前缀。

不以planning done或strict通过作为实施完成。真实模型仅在明确限额授权内执行，现有11/12对话、8/8 Embedding及7978 tokens账本保持锁定；本轮不调用。测试只在独立库/collection及代理，正式迁移、清理、部署仍需独立授权。5.4的项目验收方式按D7下述用户选择调整，不追认真实同伴参与。

### D2. 材料范围采用显式集合，不改Week4接口

在Week5 Tutor入口对零个绑定直接返回合法空证据，不调用检索/Embedding/Qdrant；非空集合原样约束共享检索，保留class过滤和回表。Week4内部nil仍是全班。替代方案是改变共享nil语义，但会破坏Ask/Search，因此不采用。

HTTP助手完整更新继续要求`material_ids`/`skill_ids`数组，空数组合法，null或省略400，避免PATCH式隐含放宽；Go内部nil归一为空。数据库JSON null仅作为迁移输入归一为空；SQL NULL的旧无班级占位行保留且不可用，不推断班级。教师UI增加本班材料/技能多选，空选明确显示无绑定材料。修改后下一轮使用新集合；正在生成的快照和历史不改。

旧助手空数组迁移为零关系，不能通过为它复制全班材料来悄悄保留旧扩大语义。升级报告单列受影响助手，维护窗口前由教师准备需要的明确绑定。

### D3. 技能文件、API和关系

扩展现有`skills`，增加可解析source、trigger、subject、tools、description及编辑版本；保留ID、class、enabled、revision和旧审计标识。文件输入是UTF-8文本，保留原文；教师可粘贴或从本地文本读取后提交`{source:string}`，不增加文件存储服务。

解析按首个有效元数据字段之后的首个空白行分隔正文：前导空白和元数据`#`注释忽略，正文中的`#`保持原样。字段为名称/适用条件/资料/工具/启用，键值支持中文或ASCII冒号；启用仅接受是/否，省略工具视为空列表，非空工具列表只允许knowledge_search。按Unicode字符计算：source≤8000、name1–32、trigger1–120、subject≤32、description1–2000；未知/重复字段、缺名称/trigger/启用、缺正文返回教师可读400。边界8000/8001等须验证。额外冒号保留在值中。

GET `/api/skills`继续可供已登录用户读本班；新增GET单条、POST、DELETE；PUT技能改为source完整更新，返回解析元数据及原文；教师写，学生403、外班/不存在404、同名400。旧`PUT {enabled}`仅保留为明确的兼容分支：从存量source解析后更新启用行、重新解析并保存source与enabled一致，不允许source和enabled混合提交。教师UI主要用source编辑；旧客户端开关仍能工作。skill_name用于历史，不随重命名或删除重写。

采用两张scope复合关联表存当前技能/材料ID，主键为(agent_id,resource_id)，复合外键共同带class_id。技能删除级联清理绑定，材料关系不扩张可见范围。JSON旧字段保留为迁移/回退证据，切换后不双写、不作运行时真相源；迁移账本标记回填完成，重复升级不得从过期JSON复活解绑。替代保留JSON并手动全量清理虽可等价，但删除竞态和悬挂绑定更复杂，因此选择关联表。

### D4. 确定性路由与快照

只读当前助手本班绑定且enabled技能；针对当前问题q而非“上一问+q”评分（组合查询仅供检索）。trigger按`,，、;；|`切分，trim、空词丢弃，两侧转小写，逐关键词子串命中加1；重复关键词按文件中的出现次数计分，避免暗中引入去重算法。最高分且>0胜，同分小ID；只注入所选说明，名称包含解题引导则服务端追加教学硬约束。subject非空时在绑定材料top4之后按标题子串过滤，不为补满而额外召回；再去空文并连续编号。

保持每轮原子配置/关系/技能快照和revision/hash。教师更新只影响接受快照后的下一轮，删除技能不破坏生成中快照。无证据仍可命中技能并实际用于生成；不能沿用“必须有资料才加载技能”。工具名是元数据，不向模型注入工具。

### D5. 角色、会话和历史

空/全空白prompt保存为空，生成时用固定默认助教角色；超2000仍400。新会话校验本班enabled；已有会话只校验本人+班级+绑定助手存在及归属，不因disabled拒绝。Prepare也必须区分新建资格与续聊资格，不能只去SQL条件后仍被第二层enabled检查拦住。请求中的身份/配置覆盖字段继续400，不允许换agent。

首问标题24个rune，后续不改；updated_at更新和倒序保留。历史由按写入顺序的消息序列构造，先限定owner/class和role，跳过空正文及非完成assistant草稿；保留完成的无证据回答，不再仅取result_status=ok的完整消息对。写入本轮user后取末尾6条有效消息（包含它），逐条截500字；当前user位置替换为含q和证据的本轮用户消息，不再另追加一遍问题。旧assistant失败/中断前缀仍全量留库可查，只不作为成功回答喂入模型；这是保留现有可靠性保护的工程选择，不是新增课程硬要求。不能以合计预算6000代替逐条限制。

F09选择保留当前FK，新增仅内部存储删除方法：owner/class验证、锁会话、active_turn非空返回Busy、同事务先删所属消息再删会话，任一步失败回滚；不存在/越权统一NotFound。所有应用删除均走此方法，无公开路由、页面删除按钮或定时清理。直接SQL的FK拒绝仍作为防止孤儿的最后防线，无需只为F09改FK成CASCADE。

### D6. Week5生成与SSE最小编排调整

推荐顺序：身份/会话/当前配置和路由 → start事务保存user、generating占位及占用 → 绑定检索/subject过滤（零绑定短路） → 持久化本轮证据快照与待用技能/版本 → 组装角色边界和6条历史 → 发送meta并Flush → 打开现有模型流 → 清洗后逐段Append再delta → 引用校验及Finish → done。Start不持有跨网关事务；新增证据准备写入须受turn token、owner/class和active约束，失败不得发meta。配置审计区分选中/准备与上游已提交的实际技能，模型未调用不得伪记成功执行。

检索/准备失败在meta前返回JSON并终结占位为failed，保留已接受的user及失败审计；初始Start事务失败无消息对；失败user可被下一轮历史读取但不伪造assistant。它改变旧“检索失败无消息”断言，是对官方先记问题及当前问题入窗的有意适配，须更新Week5测试，不改变Week4。

meta固定`status:ok`，`citations`是本轮已过滤、连续编号、交给模型的证据列表，空集为[]，保留skill_name/message_id并可附加`skill:{id,name}`。须在任何上游模型请求/首片之前发出，不再预读首token；因此模型连接失败发生于meta后，发送error且HTTP200。只有meta前可确认的参数/身份/权限/DB/检索错误使用原HTTP错误码。S6局部“首片之前503”的歧义不能导致重复写header。

done保留`message_id,state,citations,skill_name`扩展；其citations是最终实际引用子集，保留meta原编号，允许稀疏number，不按最终列表位置重编号。UI先呈现meta候选来源，done后标明实际引用；失败草稿的候选不能伪称已引用。历史保存最终实际引用和完整证据快照，旧历史字段仍可读。

无证据仍按角色/命中技能调用模型，System不用“仅本轮材料才可答”，而明确无来源、不写编号、不编造出处、不用固定未找到文案；有证据仍优先引用并沿用越界校验。每条模型资料截800rune，历史的500上限不包含服务器新加的证据块。资料在User、角色/选中说明和固定边界在System；上传说明不能覆盖权限/教学边界。

无证据的`[数字]`输出用轻量跨片状态机处理，暂存可能的编号前缀，确认后删编号再落库/推送；正常文字逐段通过，不缓冲整段生成。EOF释放不是编号的残留；空有效回答或完整回答恰好为旧固定未找到文案不得标completed，发error并保留草稿。正常完成空citations合法；有证据的至少一条合法引用保护保留。生成时已输出前缀失败仍failed/interrupted留库，禁止为对齐课程而改成假成功done。

复用现有ReadableStream/TextDecoder解帧、心跳、ResponseController.Flush、Nginx关闭buffer/cache和120秒超时；HTTP/1.1响应显式keep-alive，不向HTTP/2连接注入连接专用头。仅修改新meta展示和新契约断言。

### D7. 注解文档与验收证据

处理器注解作为路径/参数/响应/security和SSE说明的唯一输入，复用嵌入OpenAPI与Swagger页面。Phase0只读核对工具官方能力并选定能保留OpenAPI3和SSE扩展的Go注解方案，不安装或创建试验代码；实际生成验证在Phase4进行。必要时薄格式转换由注解产物驱动，不再用第二份Python手写契约。不更改运行时框架；工具选择不得改变文档约定或扩大业务范围。CI生成差异与实际路由（含新增DELETE技能）对照，删除硬编码23操作假设。

单元/真实隔离DB+Qdrant/受控上游/浏览器+Nginx/真实模型/用户内容审核/独立账号自动化分别记录命令、时间、版本、环境、结果和脱敏证据位置。未运行或环境缺失不标通过；旧模型四轮报告继续作历史证据。新真实模型复验必须有费用限额和授权；不自动重试扩大费用。

**本项目5.4验收方式调整（用户明确选择，并非官方要求变更）：** 官方S7及Phase0仍如实记录真实同伴本人操作四项并确认。用户决定本项目改用独立教师T-A、同班学生S-A1/S-A2及外班学生S-B1账号，由Codex自动化执行；不把账号数冒称参与人数，不虚构同伴、签名或反馈。

P1必须证明本人可读取续聊、他人列表不含会话且读取/发送404、拒绝无DB副作用；P2必须证明异绑定材料/技能不串用、agent覆盖400且未改绑、停用拒新404或无可用助手400、旧会话用当前配置续聊且历史快照不改；P3必须证明跨班会话/助手/技能/材料及下载404、身份覆盖400、跨班混合绑定整体回滚；P4使用同题同材料、每种状态新会话，比较技能启用命中、停用但绑定、启用后解绑的实际System/meta/回答/引用与数据库。技术标准不变，现有28个关键子步骤必须全通过、0失败/0未执行；权限、部分写入、串用、SSE/引用/持久化异常或证据缺失均阻止完成，不因调整方法忽略阻塞。

交付必须保留实际请求/响应、浏览器及截图、DB前后状态、技能/证据快照、模型输入和SSE事件、时间/时区、HEAD/工作区摘要及隔离目标；受控模型仅证明流程，真实教学质量仍由独立5.3与用户审核证明。现有结果可只读复核，不需重复收费测试。5.5按调整后的全部项目必需证据、路径/脱敏/历史hash及迁移风险复核完成；最终结论须同时写明项目29/29与官方真实同伴方式未履行。Phase0及原始验证历史不得重写成官方允许替代。

## Risks / Trade-offs

- [MySQL DDL隐式提交、旧catalog不兼容] → 显式离线版本升级、逐步账本与校验，旧二进制不能直接回退到新库。
- [空绑定从全班变空集是行为破坏] → 升级报告列出受影响助手，教师明确重绑，禁止自动复制全班。
- [旧内置技能缺trigger/source] → 按Migration Plan第3步生成有来源标记的规范种子，默认trigger限定常见数学词；该词表为兼容选择，不是官方穷尽词表，教师可改。
- [meta前移改变模型失败HTTP时机] → 在受控网关逐阶段注入失败，前后分界明确；保留失败前缀和取消。
- [历史窗口含当前问题导致重复/少一条] → 捕获完整模型messages，断言总窗口≤6且当前user只有一次。
- [并发编辑/删除、迁移重试复活解绑] → 配置事务一致快照、复合FK、一次性回填账本和重复升级测试。
- [工具切换导致Swagger或SSE扩展丢失] → 注解产物结构对照现有页面、浏览器真实Execute和流式面板验证。
- [默认角色/模型教学不确定] → 固定有最终答案的材料，同题启停及无依据测试分层复验；不承诺任何模型永远服从。

## Migration Plan

本节区分实现演练与正式库操作：Phase1仅在新建隔离库验证，正式库迁移本轮不执行。目标从显式RAG schema v2升到v3，保留v1入口向新版本逐步升级能力。实现保留旧v2 catalog为兼容证据，以v1完整catalog与逐步DDL的冻结元数据增量生成每阶段完整catalog；bootstrap不自动播种以免复活已删除技能，显式admin播种单独保留。

1. Phase0先用只读预检定义升级报告：版本/catalog、class归属、技能名称长度/重复、JSON类型、绑定资源归属、悬挂ID、生成中turn；敏感值不入报告。未知归属、冲突或非法非空绑定阻止升级，不静默删改/猜班级。停止业务写入并备份校验完成才可正式执行。
2. 在版本化新DDL与准备函数中增量增加skills解析字段、`(id,class_id)`索引、班内名称唯一索引和两张复合关联表；保留原name宽度和class=NULL占位行，不为旧占位强行赋归属。已归属技能须通过32字/唯一检查；不以截断或自动改名消除冲突。
3. 旧固定skill_key行保持ID/enable/revision和旧审计数据，合成符合格式的source，教学正文来自旧受版本控制指令，仅移除“没有材料不能回答”的过时证据限制以兼容SC3；新source计算自己的版本/摘要，不伪用旧正文hash。trigger拟为`方程,去括号,分数,通分,面积,应用题,未知数,行程`，subject为空（不额外丢弃旧已绑资料），工具knowledge_search。记录这是系统迁移生成，不能声称教师原始上传；历史消息的旧版本hash不重写。其他未知scoped技能先预检阻止并列人工映射，legacy无班级行原样隔离。
4. 校验同班JSON绑定后按原ID填关系，空/JSON null映射零关系；SQL NULL仅在可证明合法的managed旧行经报告归一为空，否则阻止。填充与完成标记事务化，DDL账本逐步可恢复；重试只补未完成步骤，不覆盖教师后续编辑或从旧JSON重绑。
5. 对全新库、v1→v2→v3、v2→v3、重复执行、中断续跑和漂移场景，在隔离MySQL8.4验证最终v3完整catalog及ngram=2；同步`schema_rag_catalog.json`/catalog分派/initializer就绪标记。只有DDL、源文本/关系回填和完整校验都完成，才发布v3就绪标记；不能在仅建完表时提前ready。线上HTTP启动只校验，不运行DDL或隐式修复；部分升级不ready。
6. 保留用户/Token版本、材料UUID/文件/切片/向量、助手/技能/会话/消息ID、旧正文/引用/技能hash。升级前后数量和关键内容hash一致，只有新增关系、解析元数据与版本标记允许变化。无索引重建、无外部模型调用。
7. 正式执行是独立维护操作，需要用户确认目标环境、窗口、备份和回退计划。Phase1测试通过不等于授权升级课堂库。

回滚：旧v2二进制严格catalog无法直接接新库。优先前向修复；必要时在维护窗口暂不接入Week5流量，属于运维恢复预案，不要求新增功能开关。现有RAG_ENABLED会同时影响Week4，不能把关闭它声称为“仅停Week5且Week4正常”。若必须回退，在停写期间从已校验的升级前备份恢复到独立目标并切换，明确会丢失/需导出重放升级后的新增业务记录。不得自动DROP新表/列、覆盖源卷、删除Qdrant或通过跳过catalog“回退”；不得宣称MySQL多条DDL可事务回滚。维护窗口的入口调整须独立获得部署授权。

## F01–F13 Coverage

| 问题 | 新需求 | 任务 | 性质/边界 |
|---|---|---|---|
| F01 | TC2/TC3/SE1–3 | 1.1–1.4、2.1、4.1、5.1 | A；无脚本/技能市场 |
| F02 | TC3/TC4/TC5 | 1.3、2.2/2.3、5.1 | A；纯词法非语义 |
| F03 | TC1/TC3/SE2 | 1.3/1.4、2.3、4.1、5.1 | A；仅Week5空集 |
| F04 | TC1 | 2.4、4.1、5.1 | A |
| F05 | SC1 | 2.4、5.1/5.4 | A |
| F06 | SC2 | 2.5、3.1、5.1 | A；含当前问题 |
| F07 | SC3/SC5 | 3.1/3.2/3.4、5.1/5.3 | A；Ask不改 |
| F08 | SC1 | 2.5、5.1 | A |
| F09 | SC6 | 1.5、5.1 | A清理；不新增公开API |
| F10 | SC4/SC5 | 3.1/3.3/3.5/3.6、4.2、5.2 | A；持久化/解析保护 |
| F11 | TC5 | 2.3、5.1 | A；仅模型资料截断 |
| F12 | DA1 | 0.3、4.3/4.4、5.2 | A；工具可替换 |
| F13 | DA2/DA3 | 0.1/0.2、5.1–5.5 | 项目验收完成；官方真实同伴方式未履行，用户选择独立账号自动化 |

## Open Questions

Phase0审查未发现尚未解决的需求设计冲突。D3关联迁移、旧toggle兼容、D6失败持久化已冻结，后续不能临时改变spec。v3 LONGTEXT名称前缀唯一索引、升级账本与v1收据分离、心跳不得提前提交响应等实施约束见Phase0验证。

0.3冻结的swag v1.16.6→kin-openapi v0.133.0→OpenAPI3.0.3、默认角色与迁移trigger词表保持。真实模型限额复验及用户审核、4.3生成链已完成，证据按原阶段记录。真实同伴未参与，本项目5.4按用户选择改用独立账号自动化。正式迁移目标/窗口及部署仍未授权，不能用测试结果代替正式库预检或授权。


### 历史时点：5.3授权及首次预检（2026-10-18，以下不是当前状态）

User authorized the course gateway only, at most 12 dialogue and 8 Embedding calls, serial and without retries. The authenticated model-list GET returned 200, but configured course-chat/course-embedding aliases were not listed and their mapping/format/dimension could not be confirmed. Stopped before any generation or database/Qdrant operations; actual dialogue=0, Embedding=0. See [5.3 verification](phase5-real-model-verification.md). This authorization supersedes earlier pending-authorization status; 5.3 remains unchecked pending configuration and user review. 5.4/5.5 remain unchecked; total 26/29.

最新5.3执行状态：官方别名由用户核对PDF确认；本次连通性HTTP200/2048维后，在新建隔离库/collection执行首轮真实教学。有据回答缺数字引用，保留正文并failed/error，无done；按原引用契约停止，不放宽校验或修改提示词求通过。实际对话2/12、Embedding3/8，剩余额度不自动使用。5.3仍待处理/人工审读，5.4/5.5未授权，26/29。
