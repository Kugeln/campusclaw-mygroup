# Phase 4 验收记录

日期：2026-10-18，Asia/Shanghai。change：`align-week5-with-course-requirements`，schema：spec-driven。开始前完整复读proposal/design/tasks、四份spec及Phase0–3验收；本轮仅授权4.1–4.4。分支保持`feature/week5-phase1-go-rag`，HEAD保持`51a8749f6550c279efebe63bb67eda724878f5a1`，保留已有未提交内容。Phase5五项未执行，未commit/push/归档/部署。

## 环境与范围

只读重核独立`campusclaw-align-storage-17d5f822`（MySQL8.4.4、tmpfs）和`campusclaw-align-stream-6e1982af-qdrant`（Qdrant1.13.4、tmpfs）及internal测试网络。每次浏览器运行新建随机`align_w5_<16hex>`库、`align_browser_<时间值>` collection及独立uploads目录，不复用既有库或collection。库初始化/完整v3 catalog验证只发生在这些新库。未操作课堂库、Docker数据卷或既有Qdrant数据。

新建Go测试服务与Nginx1.28.0代理，只有代理连接bridge并发布随机127.0.0.1端口，业务测试服务、MySQL和Qdrant保持internal网络。代理配置由原`deploy/nginx.conf`仅替换新测试upstream并添加前台启动指令，原文件不改。测试结束Go服务退出，经过名称/标签校验后停止本轮代理，不停止或删除已有资源。

Embedding、流模型全部为fixture中的本地HTTP替身；实际走生产Go路由、Bearer、仓储、VectorClient/Retrieval/StreamingModel及原SSE解析器。真实浏览器为Edge155.0.4283.45 headless（Playwright）；主机Go1.27.1，容器Go1.26.0。没有真实收费模型、人工同伴或正式库迁移。

最终完整浏览器证据：`runtime/align-browser-9c9349f8/`，含`ready.json`、`browser.json`九组检查、`stats.json`、`fixture.log`、`nginx-effective.txt`、`result.json`及teacher/student/knowledge/Swagger截图。先前失败/通过运行目录均保留，最终结果与资源/源码hash汇总见[phase4-check-result.json](phase4-check-result.json)，实施前基线见[phase4-baseline.json](phase4-baseline.json)。凭据不写入报告、截图或源码；连接文件和运行产物保持Git忽略。

## 4.1 教师管理页面：通过

在既有tutor模板/脚本补充source编辑器、本地UTF-8文件读取、GET查看/编辑、创建和删除；删除确认明确解除当前绑定但不改历史。未编辑的文件/GET原文保留CRLF，避免textarea显示归一化时误改原文；非法UTF-8、超长文本和服务端解析错误均反馈。

技能/材料使用本班多选checkbox，零选择明确表示不检索材料；空prompt不再required，提示使用默认助教角色。保留CampusAPI、Bearer、原品牌/Logo/导航/配色。旧enabled-only开关仍直接走兼容API并刷新原文与元数据。

真实浏览器验证：读取含注释/CRLF文件→保存→GET字节一致→刷新查看→编辑→兼容开关→创建第二技能→助手双技能/双材料及空prompt保存→刷新仍存在→删除技能自动解绑；无效source错误提示。另从页面创建零绑定空角色助手。学生无教师面板，技能/助手写入实际403，外班助手/材料404。

教师停用后新建会话404，既有会话经页面继续成功；教师解绑最后材料后同一既有会话下一轮模型1次、Embedding0次、空候选，显式重绑后恢复。教师390px无横向溢出并有截图。

## 4.2 学生流式页面：通过

meta即展示候选来源和本轮技能；delta立即追加；done保留候选列表并单独展示最终实际引用，保留原编号。失败草稿的来源明确为候选，实际引用未确认，不冒称已使用。无证据显示空候选/空实际引用；历史接口省略空evidence_snapshot时也保持明确空来源，旧历史缺候选快照则注明未记录，不捏造完整候选。

实际门控：Go在meta Flush后进入Model.Open，但受控HTTP模型连接等待放行；浏览器已看到两候选和技能、正文为空，模型HTTP调用数未增长。放行连接后首个delta“先分析𠮷条件，”立即可见、生成尚未完成；放行尾片后done仅实际number=2，正文与数据库一致。历史来源查看/授权下载成功，下载名与真实引用标题一致，沿用既有安全文件名处理。

真实error/模型连接503、取消、TCP断连、done写丢失均已通过：failed/interrupted保留可靠前缀及候选，done丢失后历史确认completed；无据输出数字引用被后端清洗，页面无虚构引用。浏览器直接用原CampusStream测试逐字节UTF-8、CRLF、心跳和EOF缺done拒绝；另有原Node解析用例3/3通过。浏览器协议分片测试与真实网络增量门控分别记录，不声称TCP每次恰好按指定字节切分。

Swagger实时面板同步提供候选/正文/最终引用及技能，失败保留草稿并查询历史；原sse.js、CampusAPI、401清身份及材料权限保持。学生页及Swagger长候选/事件文本均在390px无横向溢出。

## 4.3 注解OpenAPI：通过，遗留失败已修复

固定链为swag v1.16.6 → kin-openapi v0.133.0 → OpenAPI3.0.3。工具能力依据沿用[swag对应版本说明](https://github.com/swaggo/swag/blob/v1.16.6/README.md)及[ToV3源码](https://github.com/getkin/kin-openapi/blob/v0.133.0/openapi2conv/openapi2_conv.go)。构建工具在独立module锁定go.mod/go.sum，不增加线上框架或主module依赖。

处理器注解及其引用的Go请求/响应类型是唯一维护源。POST/PUT共用业务实现的助手/技能端点使用薄包装处理器，分别注解正确请求、201/200及角色错误，原验证/事务不变。转换器只有通用格式转换和注解扩展处理，没有路由/业务schema清单；删除旧`generate_week5_openapi.py`，保留原Swagger vendor资产和Authorize/Execute。

覆盖source CRUD及enabled-only oneOf（false合法、禁止混合/未知/null）、智能体完整绑定数组/空prompt、owner+class会话、Week4 search/ask、材料/认证和SSE四事件。meta.status仅ok，空数组合法，done实际子集保留编号；meta前JSON与meta后200/error分界明确。另准确保留原有meta前输入预算校验502，以及材料存储HTML500/身份JSON500的不同媒体类型；未更改业务错误行为。

实际生成命令：

```powershell
powershell -NoProfile -File scripts/generate_openapi.ps1
powershell -NoProfile -File scripts/generate_openapi.ps1 -Check
```

其内部固定命令：

```powershell
go run github.com/swaggo/swag/cmd/swag@v1.16.6 init -g cmd/server/main.go -d . --parseInternal --exclude runtime,.venv,scripts/openapi-convert --outputTypes json -o runtime/week5-openapi-v2
go -C scripts/openapi-convert run . -in ../../runtime/week5-openapi-v2/swagger.json -out ../../internal/http/web/swagger/openapi.json
```

exclude仅避免扫描下载缓存中故意无效的Go测试数据及独立转换module，不改变工具版本或契约。官方proxy本轮连接超时，改用模块镜像并保留Go模块校验；构建缓存仅在runtime。离线`go run @version`元数据查询未通过，后续CI需可用模块源或事先准备可用工具环境，不把离线尝试算通过。

最终生成产物SHA256：`3770acab9bd235cf81abe2f5ae9239b365eb03112ded2576a0ce4959d5651d1d`。重新生成/Check退出0且前后hash一致。对runtime临时副本故意改title，Check实际退出1并定位artifact drift，正式产物未改。

`TestDocumentationRoutesAndContract`已由Phase2–3失败转为通过；原失败记录不改写。新检查通过Go AST遍历实际Go注册表达式，包含DELETE，不使用固定23或26操作作为门槛；请求字段与严格处理器字段对照，schema/角色/错误/SSE另有语义检查。当前观察到26个操作，仅为结果计数。没有公开会话DELETE。

## 4.4 浏览器、Nginx与Swagger：通过

最终fixture和浏览器均exit0。实际`nginx -T`在upstream仍运行时成功，证据包含buffer/cache off、next_upstream off、120秒read timeout、Bearer透传及伪造身份头清除；真实页面请求故意带X-User-Id/X-Role/X-Class-Id，服务端观测均已清除。

Swagger真实Execute依次验证匿名GET401、Authorize后技能GET200、source POST201、enabled=false PUT200、DELETE204；不是只看静态JSON/页面加载。学生授权的SSE面板也验证候选早到、首delta早于完成、最终number=2。JWT只在页面内存/sessionStorage中使用，不写证据或截图中的curl；截图时操作折叠，Authorize弹窗已关闭。

材料上传经真实页面完成并可见；来源查看/下载与404边界正常。Week4真实MySQL/Qdrant keyword/vector/hybrid均返回本班资料；Ask零命中仍固定文案、sources=[]、对话0次。RRF/阈值/故障与Bearer纯测试通过。讲义9条一页、下一页/上一页及390px无溢出实测通过。原Logo、主样式、导航、materials/knowledge页面源码和SSE解析器hash保持，未做无关页面重构。

## 命令与结果

| 命令 | 实际结果 |
|---|---|
| `powershell -NoProfile -File tests/go_rag/run_align_browser.ps1` | 最终九组检查通过，fixture exit0、browser exit0；真实Edge→独立Nginx→Go→新MySQL/Qdrant |
| `powershell -NoProfile -File scripts/generate_openapi.ps1 -Check` | exit0，重复生成hash一致 |
| 转换器Check临时漂移副本 | 预期拒绝，实际exit1，不计为业务失败 |
| `go test ./internal/http ./internal/auth ./internal/rag ./internal/tutor ./internal/conversation -count=1` | 全通过，没有跳过Swagger检查 |
| `go test ./internal/http -run 'TestDocumentationRoutesAndContract\|TestAlignedOpenAPISchemasAndSSE\|TestDocumentedStrictFieldsMatchHandlers' -count=1 -v` | 最终三个契约检查全部通过 |
| `go -C scripts/openapi-convert test ./... -count=1 -v` | 通过，语义校验enabled=false、混合/null拒绝、空数组/空角色和2000/2001 Unicode边界、meta仅ok |
| `run_align_storage.ps1 -Stage Phase2API`、`-Stage Phase2Roles` | 通过，新随机库验证薄包装路由的CRUD、权限、原子性、配置/续聊回归；没有执行Phase5整体验收 |
| `node --test tests/go_rag/sse.test.cjs` | 3/3通过，原解析器不改 |
| `openspec validate align-week5-with-course-requirements --strict` | 通过，不代替Phase5整体验收/人工证据 |

早期失败均保留：文件名断言最初误以为上传名保留中文，后改为核对原安全文件名处理后的真实标题；空证据历史展示漏洞已修复；首次nginx -T在测试app退出后因DNS失效失败，已将取证移至运行时并最终成功；转换器扩展分支开发错误已修正并完成新产物/语义/浏览器复验。最终以上通过结果对应当前交付文件，不覆盖旧历史为新结果。

## 修改文件与新增测试

修改17份已有文件：`cmd/server/main.go`；`internal/http/auth.go`、`login.go`、`upload.go`、`material_reads.go`、`rag.go`、`conversation.go`（注解）；`tutor.go`（注解和薄包装）、`handler.go`（四个包装路由接线）；`documentation.go`、`documentation_test.go`；`web/templates/tutor.html`、`web/static/tutor.js`、`tutor.css`；`web/swagger/index.html`、`init.js`、`openapi.json`。删除旧手写契约源`./scripts/generate_week5_openapi.py`。

新增9份实现/工具/测试文件：

- `internal/http/openapi_types.go`：Go请求/响应及事件类型。
- `internal/repository/mysql/phase4_browser_integration_test.go`：只新建库/collection的真实服务夹具、模型门控、Flush/断连/done丢失观测。
- `scripts/generate_openapi.ps1`；`scripts/openapi-convert/go.mod`、`go.sum`、`main.go`、`main_test.go`。
- `tests/go_rag/run_align_browser.ps1`、`check_align_browser.py`：独立代理及真实浏览器全流程。

本change只更新proposal/design/tasks当前状态，新增本记录及phase4-baseline/check-result。四份spec、Phase0–3记录、原Week5 change的17份文件和32/32均保持；主go.mod/go.sum、部署/Compose、schema/catalog及后端检索算法不改。

额外范围核对：移除新增注解及其格式空行后，7份后端入口/认证/会话/材料/RAG文件可恢复实施前hash；移除4个薄包装及注解后，原Tutor实现hash保持；恢复4个分派名称后，原handler注册文件hash保持。没有借注解生成修改原业务验证或流式编排。

## Phase 5 前的风险与停止点

Phase4没有未解决的实现或浏览器/Swagger阻塞。Phase5尚未授权，五项保持未勾：旧固定库/旧反向断言的整体验收执行器仍待5.1适配；完整课程DoD、真实模型教学效果和真实同伴证据尚未交付。5.3需独立费用/调用上限授权，5.4需真实参与者；不能用本轮受控模型/合成账号替代。正式迁移目标、停写窗口、备份恢复及部署仍是独立授权边界。

本轮没有重跑完整DDL中断矩阵、旧Compose演练或Phase5全接口验收，不声称这些已通过。测试资源/tmpfs仅供复核，不保证停机后留存。完成4.1–4.4后立即停止，总进度24/29，等待下一阶段授权。
