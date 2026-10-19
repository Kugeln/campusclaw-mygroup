# Phase 5 本轮验证记录

日期：2026-10-18，Asia/Shanghai。仅执行5.1、5.2及5.5准备；5.3未授权、5.4待真实同伴、5.5未完成。开始前完整复读proposal/design/tasks、四份spec及Phase0–4验证。分支`feature/week5-phase1-go-rag`，HEAD `51a8749f6550c279efebe63bb67eda724878f5a1`；已有未提交内容保留，实施前hash见phase5-baseline.json。

## 环境与安全边界

只读重核独立MySQL8.4.4 `campusclaw-align-storage-17d5f822`、Qdrant1.13.4 `campusclaw-align-stream-6e1982af-qdrant`，均为既有独立测试tmpfs实例、无持久卷，内部网络`campusclaw-align-storage-17d5f822-net`。每例新建随机`align_w5_<16hex>`数据库；联测新建collection，不操作历史collection。课堂三个容器保持停止，没有启动/重建Compose、正式迁移、卷操作或收费模型调用。

Go1.27.1主机单元、Go1.26.0隔离测试服务、Edge155.0.4283.45真实headless浏览器、Nginx1.28.0代理。仅代理连接bridge并发布随机127.0.0.1端口，Go/MySQL/Qdrant保持internal网络。实际走生产路由、Bearer、仓储、检索及StreamingModel；Embedding/模型为本地HTTP受控网关。合成账号不是同伴。

本轮**不自动停止或清理**测试资源：新增Go服务/代理、库、collection、材料和日志均保留。服务夹具有24小时安全期限，之后服务自然退出；容器、库/collection及证据不删除。tmpfs停机可能丢失，不能当正式备份。凭据只在忽略的runtime配置和子进程环境，不写报告。

## 5.1 完整自动化：通过

旧综合执行器`run_phase4.ps1 -Stage phase6`在读取旧配置前转发新`run_align_acceptance.ps1`，不再连接旧固定库或运行旧反向断言。新入口组合全量存储、Phase2/3、单元/SSE/OpenAPI、当前API及最终另一个新夹具的DoD验收。本轮按以下分步命令实际执行同一矩阵，未以旧结果替代：

| 实际命令/证据 | 结果 |
|---|---|
| `run_align_storage.ps1 -Stage All`；runtime/phase5-storage.log | 15个顶层测试通过，80.314秒；新库/v1→v2→v3/v2→v3、重复、41个DDL中断点、漂移拒绝、旧数据/事务/删除；无SKIP |
| 同runner `-Stage Phase2`；runtime/phase5-phase2.log | 9个顶层通过，10.086秒；source CRUD/原子性、词法路由、空绑定/subject/800字、默认角色/停用续聊、6条/500字/24字、Week4保护；无SKIP |
| 同runner `-Stage Phase3`；runtime/phase5-phase3.log | 9个顶层通过，12.693秒；真实MySQL/Qdrant、四组合、HTTP/SSE、清洗、DB一致性、注入/断连/超时/恢复及旧历史hash；无SKIP |
| `go test ./cmd/... ./internal/... -count=1`；runtime/phase5-unit-final.log | 全部源码包适用单元通过，包含Swagger契约、RRF/阈值及Bearer，无跳过；无测试包明确标注 |
| `node --test tests/go_rag/sse.test.cjs`；runtime/phase5-sse.log | 原解析器3/3通过 |
| `go -C scripts/openapi-convert test ./... -count=1 -v`；runtime/phase5-converter.log | 通过，strict请求/schema/Unicode/meta语义 |
| `scripts/generate_openapi.ps1 -Check`；runtime/phase5-openapi-check.log | exit0，固定swag1.16.6→kin-openapi0.133.0→OAS3.0.3；产物无差异 |
| `python tests/go_rag/align_storage_recovery.py`；runtime/phase5-recovery.log | 本轮新库备份/独立恢复通过，runtime/.../recovery-9174d88a/保留dump/hash/恢复记录 |
| `apitest/run_all.py`；reports/apitest/20261018T021738Z-9e44c742/result.json、index.html | **27/27检查通过、422条断言**：26个当前URL/方法与动态契约集合一致，另1项本轮Go故障证据补充；接口数量是本次观察值，不是固定门槛 |

新API执行器只允许随机loopback代理及本轮ready.json的新库，SQL只读SELECT。source POST/单条GET/DELETE增加独立脚本，PUT source/toggle、混合输入拒绝、删除当前关系保持历史验证；旧固定技能、空prompt400、无据零调用、meta后503/无消息等反向断言已替换。Week4固定文案/零调用保留。SSE首片实际约0.063秒、受控尾片约9.797秒结束；首片回调直接查DB、并发409无新消息对、编辑不改接受快照，之后下一轮应用新配置。

四组合在本轮真实服务日志中分别为：有据有技能/有据无技能均模型1、Embedding1；无据有技能/无据无技能均模型1、Embedding0。正常零命中、subject归零另有用例生成。meta前检索故障503/模型0、已接受消息对failed；meta后连接失败200且meta→error、无done。清洗后delta拼接=DB，failed/interrupted保留可靠前缀，done丢失查历史completed。

Week4 keyword/vector/hybrid、RRF、.35/.349阈值、payload+class+ready回表、依赖503不降级、Ask无命中`资料中未找到相关内容`/sources=[]/对话0次均通过。材料上传、索引故障仍保留源、下载、重建后旧引用快照、401/403/404及伪身份头清除亦有本轮API/浏览器证据。

## 5.2 官方八项DoD

在5.1全部通过后，使用`run_align_browser.ps1 -Phase5 -BrowserOnly`创建另一个全新库/collection，通过真实Edge→Nginx→Go完成最终DoD，**8/8通过、10组浏览器检查通过**。最终证据`runtime/align-browser-d08b5cd8/`，数据库`align_w5_afa9de09163b11cc`、collection `align_browser_18df7e6ee5935887`；Go/代理`campusclaw-align-ui-d08b5cd8-app/proxy`保留。环境、时间/版本及SHA256见phase5-check-result.json。每个DoD的实际inputs/outputs/UTC/environment/evidence在最终browser.json的dod.D1–D8中；请求元数据只记方法/路径/Bearer存在性，无Token。

| DoD | 实际输入与输出 |
|---|---|
| D1 管理 | 页面读CRLF文件→source创建/GET/编辑/toggle→双技能材料绑定/空prompt→刷新一致→删除自动解绑；学生写403 |
| D2 绑定 | 同题、同会话依次仅绑定材料A、仅B、空集：候选各只含A、B、[]；空集Embedding0，模型1，不补未绑定同班材料 |
| D3 归属 | 本人会话、同班他人/外班GET和发送404；覆盖agent_id400，历史agent_id保持；停用拒新404但原会话继续 |
| D4 技能 | 固定含`x+2=5，最终答案x=3`的材料，同题启用/停用/启用但解绑：受控网关根据实际System边界分别返回提问引导/允许结论/允许结论，meta技能与快照对应。**只证明机制和可观察差异，不证明真实模型教学效果** |
| D5 无据 | 新会话正常零命中与空绑定均模型1、done成功、引用[]，无数字编号或旧固定文案；subject归零由自动化补证 |
| D6 来源 | meta两个完整候选，done仅原number=2；历史正文与页面一致，标题/材料ID/1-based切片及授权下载对应，不按子集重编号 |
| D7 增量 | hold_open时页面已显示候选和技能、模型HTTP计数未增；放行后首delta可见而尾片未完成；Finish后done；error保草稿/候选不标实际引用，取消/断连/done丢失历史确认通过 |
| D8 Swagger | 真正Authorize及Execute：匿名GET401，授权GET200、source POST201、toggle PUT200、DELETE204；学生SSE面板早候选/增量/最终2 |

沿用现有Nginx配置（仅替换新测试upstream），nginx -T记录buffer/cache off、120秒read timeout、Bearer透传及伪身份头清除。讲义9条分页、教师/学生/讲义/Swagger390px无横向溢出、材料上传下载和401清身份通过。原生产页面、品牌、Logo、导航、配色、共享检索/SSE解析器及部署配置本轮未改。

## 失败记录与限制

- 首次`go test ./...`实际失败：扫描旧runtime目录时Access denied；没有改目录权限或删数据。改为完整业务源码范围`./cmd/... ./internal/...`通过，独立转换module另跑；不把首次扫描失败说成全绿。
- 首次浏览器零命中用例复用了有历史会话，扩展检索命中历史词；改用全新会话后通过。原失败runtime/align-browser-7934c9c8保留。
- API动态路由比较最初未保留`/swagger/`尾斜杠，报告明确失败；修正后同一本轮隔离环境完整重跑27/27通过。失败报告reports/apitest/20261018T021624Z-8de3483e保留。
- 中间浏览器runtime/align-browser-0467a291用于自动化/API补证；最终DoD在5.1之后另建夹具，并明确含答案材料，不将中间结果代替最终验收。
- 未运行旧固定库integration标签或课堂Compose重建脚本；这些旧入口不当作当前证据。对应业务保护由新随机库alignstorage、当前API与真实代理/browser实测覆盖。没有真实付费模型或真实同伴记录。

## 修改与交付准备

修改既有测试/执行器：phase4_browser_integration_test.go增加受控输入观测/故障、含答案材料与保留模式；check_align_browser.py补八DoD结构化记录和跨owner/class/异绑定/技能差异；run_align_browser.ps1补Phase5/BrowserOnly及不清理；run_phase4.ps1综合入口转发。apitest/support.py、run_all.py、README及6份旧方法脚本适配新契约；新增技能POST/GET/DELETE三份脚本、新run_align_acceptance.ps1。没有修改业务Go实现、页面、schema/catalog、部署或主module依赖。

本change更新当前proposal/design/tasks状态，新增本记录、phase5-baseline/check-result及[验收准备稿](acceptance-handoff-draft.md)。F01–F13、Phase0–5索引、Week4/旧Week5保护、正式迁移前提/备份/恢复风险及剩余条件在准备稿中；5.5仍不勾选。旧change17份文件、32/32与Phase0–4历史记录保持原hash。

## 剩余条件与停止点

- **5.3未授权**：需另行明确供应商/模型、调用次数与费用上限，再人工审读固定材料启停/解绑、连续追问和无据角色回答；受控结果不能替代。
- **5.4待人工**：需真实参与者本人四项交叉验证及确认、时间/版本/脱敏请求响应；本轮自动账号不算同伴。
- **5.5仅准备**：以上必需证据齐备、最终风险/脱敏/历史核对后才可正式完成，当前不能宣称29/29或完整课程验收通过。

5.1/5.2在实际验证通过后勾选，进度**26/29**，5.3/5.4/5.5保持未勾选。`openspec validate align-week5-with-course-requirements --strict`通过；strict只证明OpenSpec结构，不代替业务、人工或迁移验收。最终核对见phase5-check-result.json。本轮完成后停止，不自动进入收费模型、同伴验证、迁移或部署。
