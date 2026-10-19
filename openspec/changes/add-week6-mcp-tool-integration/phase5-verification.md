# Phase 5 实施与验收（2026-10-19，Asia/Shanghai）

结论：5.1–5.5 实现与隔离测试通过，累计 **35/42**。最终 strict **通过（exit 0）**。Phase 6 七项保持未完成，未开始最终课程验收、真实模型或人工同伴验收。

机器证据：[phase5-verification.json](phase5-verification.json)，初始工作区：[phase5-preservation-baseline.json](phase5-preservation-baseline.json)，[静态检查](phase5-static-checks.json)，[Docker 收尾](phase5-docker-closeout.json)，最终保护检查见 phase5-final-checks.json。

## 1. 逐项状态

| Task / Test | 结果与实际证据 |
|---|---|
| 5.1 / T63 | 现有 tutor 教师页签；本班列表、新建、编辑/改名、空 Token 保留、连接/断开、缓存工具、删除确认、默认50/最大100审计均通过；真实502和已有审计删除503，失败保留卡片。 |
| 5.2 / T64–T65 | meta 与历史 configuration_snapshot 共用纯文本 ref/status/text 展示；ok/denied/error，无调用及旧缺省/null/空值不显示区域；切换/刷新/取消/断线/done丢失无重放。原 SSE 解析器保留，UTF8分片、心跳、EOF/错误顺序通过。 |
| 5.3 / T66 | 八操作注解语义及安全示例，经固定 swag 1.16.6 → kin-openapi → 3.0.3 生成，Check与官方合同 Go 测试通过；真实 Swagger Authorize/Execute connect、tools、技能保存及实时工具面板通过。 |
| 5.4 / T13,T26 | 默认 Compose 新增可选 mcp profile、同仓构建、健康检查、无宿主 MCP 端口、精确内部网络和环境模板；新随机项目真实构建 server/initializer/MCP、fresh初始化v4、登录/登记/连接/发现两工具通过。README包含升级、回执、备份、恢复与失败停止步骤。 |
| 5.5 / T47,T63–T66 | 实际 Edge → 原 deploy/nginx.conf → Go/MySQL/Qdrant → 独立 MCP；模型门关闭时先看到工具/meta且正文为空，随后释放模型；工具终态≤meta<模型Open≤首delta≤done。四档响应式/截图及文档检查通过。 |

## 2. 本轮文件清单

实现、部署和测试共18个文件（其余原有 dirty 文件保留）：

- `.env.example`
- `docker-compose.yml`
- `README.md`
- `internal/http/pages.go`
- `internal/http/mcp.go`
- `internal/http/web/templates/tutor.html`
- `internal/http/web/static/tutor.css`
- `internal/http/web/static/tutor.js`
- `internal/http/web/static/mcp.js`
- `internal/http/web/swagger/index.html`
- `internal/http/web/swagger/init.js`
- `internal/http/web/swagger/openapi.json`
- `internal/repository/mysql/week6_phase5_browser_integration_test.go`
- `tests/go_mcp/compose.phase5.yml`
- `tests/go_mcp/run_phase5.py`
- `tests/go_mcp/check_phase5_browser.py`
- `tests/go_mcp/run_phase5_recovery.py`
- `tests/go_mcp/check_phase5_recovery.py`

本 change 文档：新增 phase5-preservation-baseline.json、phase5-static-checks.json、phase5-docker-closeout.json、phase5-verification.md/json、phase5-final-checks.json；更新 tasks.md、verification-result.json、requirements-traceability.md、review-summary.md、risks-and-open-questions.md。未修改 proposal/design 或五份72项合同。

## 3–4. 教师界面与真实网络结果

MCP页签复用原 tutor 和 CampusAPI，保留教师助手/技能/材料入口；不新增公共接口。服务名遵守后端小写字母开头规则（管理名称64字符、技能引用名称32字符边界明确提示）。密码控件不回填，成功或失败提交后清空，取消/退出/权限清除时清空。创建允许空凭据，与官方合同一致。编辑空字符串保留由真实 MCP Bearer 连接成功证明。

主运行：`runtime/week6-phase5-20261019T003358Z-d20c8ac6/`。browser-result.json 保留实际请求 method/path/status、是否 Bearer、是否 Cookie，不记录请求正文或认证头。真实远端拒绝502、无审计删除204、有审计删除503保留卡片、disconnect清缓存且后续denied审计均已验证；没有强制删除或审计清理。

## 5. 学生 SSE 与历史

每条仅展示安全 ref/status/text，工具结果与候选/实际材料引用分区；不提升为指令、不使用 innerHTML、不伪造引用。历史使用已有安全快照，未改后端历史结构或工具执行。真实 MCP 正常、断开、停止三种情况分别显示 ok/denied/error，工具失败仍完成受控模型回答，浏览器 JWT 保留。

补充运行：`runtime/week6-phase5-recovery-20261019T003826Z-2ffab57e/`。lost_done 从历史确认 completed；cancel 与真实断线恢复 interrupted 和已保存前缀、工具快照；3次问题仅3次工具阶段，刷新与代理均无重放。

## 6. 浏览器、响应式及可核验截图

主运行 browser-result.json：18组通过；补充恢复3组通过。1440/1024/768/390px 均无页面横向溢出，长服务名/URL/审计 detail/工具正文可阅读，dialog不越视口，移动会话折叠保留。Edge版本与纳秒时序见机器证据。首轮 gated 场景约140ms即在模型门关闭时看到工具/meta，正文仍空。原 Nginx 文件逐字节未改，保留无 buffering/cache/retry 和120秒 read/send预算；未以短场景冒称执行了120秒等待边界测试，原预算由Phase4证据及配置复核支持。

主运行截图：teacher-connected-1440.png、student-tools-1440.png、teacher-dialog-{1440,1024,768,390}.png、student-wrap-{1440,1024,768,390}.png、knowledge-mobile.png、swagger-after-execute.png；补充 recovered-tool-history.png。已实际查看桌面工具结果、390px弹窗截图，保留原品牌蓝、#f4f6fa、Logo资产、导航和字体，无框架替换。

边界说明：400/403/404/503、坏JSON、网络及15秒超时中的部分UI分支使用明确的浏览器响应夹具；实际后端权限/跨班404、真实502/503及正常网络路径单独验证。长URL与200字以上恶意审计文本属于布局/转义夹具，没有放宽出站白名单或写入伪造后端审计。主工具及关键会话链路均为独立真实 MCP 服务，不以夹具替代。

## 7. Swagger/OpenAPI

八管理操作路径、方法、请求、响应与错误码保持既有合同；has_auth只表达是否有凭据，更新断开并清缓存，改名不重写旧技能引用，connect只initialize/list不调用工具，审计终态只追加。补充安全成功示例与字段说明；AES-GCM密钥或真实凭据不入示例。Check退出0，默认MCP关闭及仅开profile不打开应用功能的两项只读Compose配置检查通过；根目录没有Go文件的swag提示为既有非致命提示。TestMCPOpenAPIContract在279项回归中通过。Swagger实际执行时使用本人隔离教师JWT；截图前撤销Authorize并删除含curl认证头的响应面板，JWT不落盘。

## 8. 权限与安全

匿名401、学生403、A/B班列表及审计隔离、跨班PUT/DELETE/connect/disconnect/tools全部404；权限拒绝前后本班服务/审计内容相同。没有Cookie或跨域请求。错误显示固定安全说明，不渲染原始远端/数据库错误。HTML工具结果和恶意审计文本仅作纯文本，未产生img或执行事件。生成的服务Token在运行时检查/日志脱敏；文本证据JWT特征扫描零发现。现有JWT撤销、班级隔离、AES-GCM、出站策略和危险/超长整条弃用语义未改。

## 9. 第五课兼容

Go cmd/internal默认回归279个叶用例通过（含官方MCP文档合同）；原SSE解析器3项通过。真实浏览器验证助手绑定、技能source保存/编辑/开关、原材料管理、知识库9卡分页、keyword/vector/hybrid三模式真实Qdrant检索、RAG实际引用、历史和移动会话折叠。既有第五课代码与旧OpenSpec只按授权修改tutor展示/必要资源注册，RAG/技能/会话业务、认证、网关、schema/迁移未改；未重复执行Phase4完整昂贵故障矩阵或Phase6总验收。

## 10. 数量与失败历史

最终：Go叶用例279、SSE解析3、浏览器验收组21、默认Compose部署验收组1，合计 **304次叶用例/验收组执行通过，0失败，Phase5必需项0未执行/跳过**。这不是304个互斥Requirement；夹具进程setup不另计功能用例。vet、OpenAPI Check和strict另列检查，不混入用例数。

保留5次早期失败/中断运行，各有result.json及清理日志：

| Runtime后缀 | 原因及处理 |
|---|---|
| 002034Z-6ef713fd | Compose up退出0，但随后就绪/地址断言失败；原记录不能唯一定位断言。补就绪等待、独立前端网络与精确定位，不假设容器启动失败。 |
| 002223Z-a8bced7e | Playwright回调第二参数覆盖code默认值，产生Request无法JSON序列化；修正测试签名。 |
| 002416Z-579d9116 | 现代浏览器HTML pattern未转义连字符导致原生校验失效；修正新表单pattern并保留服务端最终校验。 |
| 002610Z-ef1c114e | 知识库断言错误期待原中文文件名，实际按既有存储规则规范化为01.md/02.md；修正测试，不改第五课材料业务。 |
| 003034Z-9bc6b58d | Swagger状态选择器误取Code表头；改取tbody实际状态，再实际执行通过。 |

全部失败证据保留，后续独立运行通过；不覆盖失败，不将失败混计成最终通过。

## 11. Docker隔离与清理

全部随机项目、精确新子网、独立MySQL/tmpfs与Qdrant、无外部卷；代理仅随机loopback端口。默认Compose的独立fresh项目使用本项目专属新卷，结束只清理本轮项目。每轮原有213容器生命周期及网络/卷清单一致；最终全部本轮容器、网络、卷为空。唯一标签构建镜像保留供复查，未覆盖原有镜像标签或操作既有Docker资源。

## 12. OpenSpec

最终 strict **通过（exit 0）**。72 Requirement、72主Scenario、官方8管理操作、42任务保持；Phase0–4共30项、Phase5共5项完成，Phase6共7项未完成。

## 13. Git与保护

初始分支 feature/week5-phase1-go-rag，HEAD 51a8749f6550c279efebe63bb67eda724878f5a1，保留初始完整dirty清单及逐文件SHA256。最终逐文件核对见 phase5-final-checks.json；仅上述18个实现/部署/测试文件及本change必要文档发生本轮变化。无stage/reset/clean/commit/push/merge/rebase/archive；未连接现有Demo数据库，没有正式迁移、付费模型调用或修改旧OpenSpec。

## 14–15. 进度、风险与下一阶段

当前 **35/42**。服务器名称复用与审计归属边界仍是已知风险；没有第四张稳定ID绑定表。管理64字符和技能引用32字符差异在表单明确提示。默认MCP CIDR部署前须检查不重叠并同步精确白名单。旧认证数据库500与MCP业务503区别保留，无本课外重构。审计保留原外键和历史，无清理功能或虚构天数。

最终strict及保护检查均通过，技术前置条件已满足，仍须用户下一次明确授权才能进入Phase6。真实模型预算/人工审阅、真实同伴参与及最终课程交付证据留待Phase6，不冒充本轮已完成。完成本报告后停止，不自动迁移、部署现有目标、提交或归档。
