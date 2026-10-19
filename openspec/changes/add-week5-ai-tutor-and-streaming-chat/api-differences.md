# 官方接口差异与拟采用契约

实施状态补充（2026-10-14）：下表“当前项目”列保留2026-10-13核查时的历史基线。默认Go现已实现GET search、POST ask、reindex、助手/技能及私有会话SSE；/openapi.json、/swagger/index.html、/swagger/已按实际契约提供并通过浏览器调试。登录仍/api/auth/login、检索仍GET q/mode、ask仍question/mode→answer/sources、技能仍仅enabled，UUID/1-based及SSE终态扩展均保留，未新增官方兼容别名或source CRUD。Swagger明确普通Try it out可能等待完整SSE，提供真实fetch流面板；不承诺老师全量官方脚本原样通过。

2026-10-13 已联网完整读取 [官方 OpenAPI JSON](https://devops.hello1023.com/week05/openapi.json)，版本 OpenAPI 3.0.3 / API 1.0.0；[Swagger 页面](https://devops.hello1023.com/week05/swagger/) HTTP 200，HTML 加载 `/week05/openapi.json` 和 Swagger UI 资源。本轮仅确认文档可取，未启动浏览器调试远端或当前项目；本地浏览器验收留在 Phase 5。本文件是对读取结果的字段/行为核对，不是完整官方接口实现承诺。

| 功能 | 官方参考接口 | 当前项目接口 | 差异 | 建议 |
|---|---|---|---|---|
| 登录 | POST /api/login，username/password → token,user；200/400/401/500/503 | POST /api/auth/login → access_token,token_type,expires_in,redirect；错误401/500 | 路径和响应不同；官方 SESSION_SECRET、jti、默认24h、携旧Token重登录撤销；现有 JWT_SIGNING_SECRET、ver、8h、重登录不撤销 | 保留已有生命周期及路径，在本地 Swagger 明确，不为了参考替换认证 |
| 当前身份 | GET /api/me → {user:{id,username,role,class_id,class_code}} | GET /api/auth/me → user_id,role,class_id,class_name | 结构/命名不同 | 复用 Go 当前用户身份查询，保留旧响应 |
| 退出 | POST /api/logout →200 {status:logged_out}，按 jti 撤销 | POST /api/auth/logout →204 空体，账号所有设备版本撤销 | 状态/范围不同 | 保留 204 和全部旧 Token 撤销；回归覆盖 |
| Bearer/错误 | bearerAuth type=http scheme=bearer bearerFormat=JWT；操作级 security；{error:string}；X-Request-Id | 显式 Authorization Bearer；错误同形；现有认证存储故障500 | 安全机制可复用；统一头未有 | 新 API 声明 bearerAuth，采用同形错误，可新增 request ID 追踪；旧错误状态不改 |
| 提示词管理 | GET/POST /api/agents，GET/PUT /api/agents/{id}；name,prompt,enabled,skill_ids,material_ids | 无，assistants 只有 id/name | 官方通过助手管理 prompt，没有独立 /api/prompts | 新增上述四方法，复用占位表；teacher 写，student 读启用助手。保留 UUID material_ids 扩展而非整数 |
| 技能列表/管理 | GET/POST /api/skills；PUT/DELETE /api/skills/{id}；source 文本解析 name/trigger/tools/subject/enabled/description | 无，skills 只有 id/name | 官方有文本文件CRUD，本课只需一个内置技能启停 | GET /api/skills + PUT /api/skills/{id} {enabled:boolean}；明确与官方 source 格式不同，不做 POST/DELETE或脚本执行 |
| 创建会话 | POST /api/conversations {agent_id} →201 id,agent_id,user_id,class_id,title,created_at,updated_at | 无 | 官方 teacher/student 均能建；本课学生专用 | 保持请求/响应；student only，teacher403；无启用助手400，非法/外班/禁用助手404 |
| 会话列表 | GET /api/conversations →200 {conversations:[]} | 无 | 未实现 | 本课新增，同形，只列 owner+class，不用客户端 scope |
| 历史 | GET /api/conversations/{id} →{conversation,messages:[id,role,content,citations,skill_name,created_at]} | 无 | 官方 citations items schema未细化、未表达失败状态 | 保留核心字段，加 state/result_status/prompt_revision/skill_version，定义严格引用schema；非本人/跨班404 |
| 流式消息 | POST /api/conversations/{id}/messages {content}；200 text/event-stream；meta/delta/done；流中error | 无，现有 /api/ask 完整 JSON | 上游 dialogue 没有 stream；官方忽略正文agent_id，本课拒绝；官方重复发送不冲突 | 沿用路径/核心事件；追加最终引用及状态；并发generating返回409，普通重发新轮次；不采信历史/提示词 |
| 检索 | POST /api/search {q,mode,limit,offset} →status,hits；q 2–500，limit默认10/max20、offset0 | Flask GET /api/search?q=&mode= →mode,items；默认top4，无翻页；Go无 | 方法/字段/分页/最短长度不同；官方整数 ID、0-based chunk_index、offset；现有 UUID、1-based、无offset | Go 补接原 GET 契约并共享检索；POST官方分页适配非本课必需，不自动新增。文档写清差异 |
| 单次问答 | POST /api/ask {q,history?} →answer,citations,status；无命中固定文本 | Flask POST /api/ask {question,mode?} →answer,sources；Go无 | 同路径不同字段，官方客户端history；官方内部错误500，当前引用/模型502 | Go 保持旧 question/mode/answer/sources/502；历史由新会话服务端加载，不混改旧API |
| 引用 | material_id/chunk_id整数，material_title,snippet,chunk_index从0,start_offset,end_offset,score_* | UUID，title,chunk_text,chunk_index从1,score；ask sources含number无score | 标识符/位置/分数字段不同 | 新会话citations承载现有 number/title/UUID/chunk_index/chunk_text，不伪造offset/页码；不换ID |
| 索引/材料 | 官方额外材料组织/管理接口 | Go材料上传/读取完整；Flask POST /api/materials/{id}/reindex | Go缺切片索引和重建 | 仅补接已有第4课行为，保留20MiB、原文事务、跨班404，不引入文件夹等管理功能 |
| Swagger | GET /openapi.json、GET /swagger/index.html，GET /swagger同页面 | 无 | 当前缺文档和调试 | 新增匿名页面/JSON、本地安全授权、SSE调试，明确本地契约差异 |

## 本轮复核与验收风险

2026-10-13 最后一轮再次读取官方 JSON，核对上述登录、search、ask、skills 和消息 SSE 操作。以下“可能失败”是按官方请求和本地规划差异推断的验收风险，未运行老师的验收脚本，不能推断其实际评分规则。

| 关注项 | 官方契约与本课要求 | 原样官方验收风险 | 当前范围与待决策边界 |
|---|---|---|---|
| POST /api/login | username/password → token,user；本课要求Bearer认证但未强制替换既有登录 | 当前仅/api/auth/login；官方路径请求可能404、token字段断言失败；Token生命周期也不同 | 保留旧认证。只有老师明确要求官方入口时，由用户决定是否追加薄别名/响应适配规划，不自动替换JWT机制 |
| POST /api/search | q,mode,limit,offset →status,hits；q去空白2–500，分页和分数字段；本课要求复用检索及班级范围 | 当前规划仅GET q/mode→mode,items；官方POST可能405、hits/paging断言失败；UUID/1-based与官方整数/0-based不一致 | 保留GET共享检索。不新增POST/分页；若老师要求原样契约，由用户决定独立适配范围，不重新实现检索、不伪造offset或整数ID |
| POST /api/ask | q和可选history →answer,citations,status；本课须保留既有单次问答 | 旧question/mode→answer,sources；官方仅q请求会400，citations/status断言失败；错误502与官方500/503不同 | 保留旧字段。若要求兼容，由用户决定新增请求/响应适配策略；同一路径两套形状须明确判别，不能静默改变旧客户端。客户端history不得绕过新私有会话上下文权限 |
| 技能管理 | 官方POST/PUT source技能文本，GET包含source/trigger/tools/subject，DELETE204；本课只要求一个引导技能及教师启停 | 当前PUT enabled，source请求400、CRUD路径/字段断言失败；功能合格不等于官方技能契约一致 | 保留固定技能toggle，不做市场/脚本执行。若老师要求source提交，由用户决定是否另加最小受限文本适配，不把任意技能生态带入本课 |
| SSE | 官方content 2–500，meta(status,citations)、delta(text)、done(message_id)，流中error且HTTP200；本课还要求失败/断连持久策略 | 事件核心同形；done新增字段、最终引用在done、并发409、teacher403、额外agent_id拒绝400均可能与严格断言不同；官方例子不能证明meta引用必须为空或必须完整 | 保留事件核心及真实流式。验证常规解析器兼容，但不承诺严格对象相等测试通过；若老师要求meta先给全部实际引用，应先决定引用时机方案，不能把检索候选伪称实际引用 |

官方 search schema 将q/mode/limit/offset列为required，但说明又给mode/limit/offset默认值；ask schema将history列为required，但说明称history可选。本地文档必须描述实际采用的契约，不据这些参考内部不一致新增客户端历史要求或改变旧检索行为；验收若按schema强制必填需另行确认。

任务清单优先：Bearer、教师写/学生禁止、技能引导、检索先于生成、无命中固定文本且零模型、私有会话、SSE、持久化、Swagger及按接口测试均在范围内。参考额外功能不纳入。尚待用户决定的仅是验收口径：是否要求老师针对上述官方路径/字段的脚本原样通过。当前继续按“本课行为达标、旧API保持、差异显式记录”规划，不擅自扩展兼容；若要求原样通过，应先修订规划而非在实施中临时重构。

## 新接口字段、参数和状态（本地采用契约）

所有新受保护 API 必需 Authorization: Bearer，path id 正整数，不可解析400。JSON 错误只含 error 字符串，不外包业务码。新存储/依赖故障503；现有 auth 存储500及 ask 502保持；生成前模型无效响应502、未就绪/超时503；流后错误为error事件。内部未归类故障500且无敏感信息。X-Request-Id 建议新接口响应提供并在文档声明，不作为权限凭据。

| 方法/路径 | 请求与成功响应 | 权限/错误 |
|---|---|---|
| GET /api/agents | 无body；200 {agents:[助手]} | teacher见本班全部，student仅启用；401/503 |
| POST /api/agents | 必需name,prompt,enabled,skill_ids整数数组,material_ids UUID数组；201助手 | teacher；400/401/403/404/503 |
| GET /api/agents/{id} | 200 助手{id,name,prompt,enabled,skill_ids,material_ids,revision} | 本班teacher或可用student；400/401/404/503 |
| PUT /api/agents/{id} | 与POST同必需字段；200助手，revision递增 | teacher；400/401/403/404/503 |
| GET /api/skills | 200 {skills:[{id,name,description,enabled,instruction_version}]} | 本班teacher/student；401/503 |
| PUT /api/skills/{id} | 必需enabled且为boolean；200同技能对象 | teacher；400/401/403/404/503；任意source/脚本字段400 |
| GET /api/conversations | 200 {conversations:[会话]}，空数组合法 | student；401/403/503 |
| POST /api/conversations | 必需agent_id整数；201会话 | student；400/401/403/404/503；客户端归属字段400 |
| GET /api/conversations/{id} | 200 {conversation,messages}；消息增加state、result_status、prompt_revision、skill_version | student本人+当前班；400/401/403/404/503 |
| POST /api/conversations/{id}/messages | 仅content，trim后2–500字符；200 SSE | student本人+当前班；400/401/403/404/409/500/502/503流前；流后error |
| GET /openapi.json | 200 OpenAPI 3.0.3 JSON | 匿名 |
| GET /swagger/index.html、/swagger/ | 200 HTML及静态UI | 匿名；请求接口仍需Bearer |

助手 material_ids 空数组表示全本班，有值是额外过滤，任何外班 UUID 404；不允许 material_ids 放宽 principal class。会话 title 初始“新的提问”，首条问题服务端裁剪后更新；timestamps RFC3339 UTC。会话和消息新 ID 为 BIGINT，前端不能依赖材料同为整数。

## SSE 补充与刻意差异

meta `{status:ok|no_evidence,citations:[],message_id,skill_name}`；delta `{text}`；done `{message_id,citations,skill_name,state:completed}`；error `{error:string}`。官方 meta 引用示例为空、done 只有 message_id，未限定失败存储；本课将最终校验引用置于 done并通过历史补读。引用结构是现有来源字段，不复制官方缺失数据。meta不能把检索候选都当实际引用。正常只一个done，失败不发送done，EOF而无done需读历史确认。HTTP成功200不等于生成completed。

官方附加作业、错题、技能CRUD、工具/MCP、文件夹、导出等功能不因出现在JSON自动进入本课。没有计划对整份官方测试套件作全量兼容承诺。以上差异为当前评审建议，要求完全对齐时先另行明确范围。
