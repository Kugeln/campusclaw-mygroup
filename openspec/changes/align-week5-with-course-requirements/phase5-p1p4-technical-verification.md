# 5.4相关P1～P4自动化技术交叉验证

**本轮由Codex自动化模拟教师及学生操作，完成P1～P4技术交叉验证。未邀请真实同伴，未取得真实同伴人工确认。**

## 正式规约及授权边界

已核对tasks 5.4、proposal、design D2～D7、四份spec、Phase0官方F/DoD映射、Phase5及5.3人工审核记录。TC1/3/4与SC1/4/5规定权限、当前绑定、停用续聊及流式持久化行为；DA3明确真实同伴本人确认不能被自动化替代。用户本轮授权技术模拟，不构成修改DA3。5.3已完成；5.4仍未完成、未勾选，进度27/29；不实施5.5。

## 环境与隔离

- 本轮主测试UTC：2026-10-18T08:32:10至08:39:36；北京时间16:32:10至16:39:36。每步开始时间、请求时间、环境及HEAD/工作区摘要在browser.json中。
- HEAD：`51a8749f6550c279efebe63bb67eda724878f5a1`，保留既有未提交工作；本轮前文件hash和工作区状态见 `phase5-p1p4-baseline.json`。
- 复用已核对的独立tmpfs MySQL8.4.4 `campusclaw-align-storage-17d5f822`、Qdrant1.13.4 `campusclaw-align-stream-6e1982af-qdrant`。两实例无业务持久卷，网络为内部测试网络。
- 新建本轮数据库 `align_w5_119a35ab79a22509`、collection `align_browser_18df9290ee530dbd`、专用上传目录及Go/Nginx测试容器 `campusclaw-align-ui-4b8ddd77-app/proxy`。旧d08b5cd8夹具只读检查，未用于本轮写入。
- Go1.26.0、Edge155.0.4283.45、Nginx1.28.0；入口 `http://127.0.0.1:55374`。仅新代理发布loopback端口；Go/MySQL/Qdrant留在内部网络。隔离元数据见 `runtime/align-browser-4b8ddd77/isolation.json`。
- 对话和Embedding均指向Go测试夹具内的本地HTTP受控网关，模型名controlled，Embedding为2维测试向量；不是课程网关、不是新增真实模型质量证据。没有加载真实网关凭据。原真实调用账本继续锁定、累计对话11/12、Embedding8/8，hash不变。
- 课堂三个容器保持停止；不操作课堂数据库、业务卷或原collection。测试资源保留、不自动清理；tmpfs及24小时夹具有生命周期限制，不能作为持久备份。

## 实际身份与资源

以下均为自动化测试身份，没有真实参与者或签名：T-A=ta/教师/user2/class1，S-A1=sa/学生/user1/class1，S-A2=sa2/学生/user5/class1，S-B1=sb/学生/user3/class2。另用tb/教师/user4/class2，仅准备B班技能及读取本轮B班配置。角色、班级来自本轮实际登录后的auth/me。

Agent A=102、Agent B=103、Agent C=101；课程解题引导技能=23，B班技能=24；C1=41。材料A=`0d7aaf41-e5a7-422a-9a00-4f314bfb830a`，含P1P4-A-RED；材料B=`ce615222-bcf4-4d34-9b89-44bb7c684b75`，含P1P4-B-BLUE；C的B班材料=`00000000-0000-4000-8000-000000000002`。全部ID由本轮ready.json、实际API创建或读取取得，不沿用未核实历史ID。

## P1～P4最终结果

| 项目 | 子步骤通过/失败/未执行 | 实际证据与状态不变量 |
|---|---|---|
| P1 会话归属 | 5/0/0 | 本人创建、发送、刷新读取、续聊成功；另一学生列表无C1，读取和发送404；拒绝前后会话及完整消息一致，owner/class/agent未改变 |
| P2 助手隔离与停用续聊 | 6/0/0 | 同题A/B分别只出现RED/BLUE标识和各自引用，只有A命中技能；覆盖agent返回400且DB不变；A停用后显式新建404，旧C1仍可用更新后的B材料/空技能配置续聊，但agent仍为A；全部本班助手停用后新建400且无新增会话；旧消息快照保持 |
| P3 班级边界 | 13/0/0 | 外班会话读取/发送、助手、技能、材料、下载和引用材料访问均404；A学生创建C会话404；class_id/user_id覆盖400；包含本班与外班资源的两种混合绑定均404，助手完整配置、版本及两绑定表不变；所有拒绝前后完整会话/消息一致 |
| P4 技能启用、停用、解绑 | 4/0/0 | 相同讲义、问题、角色和本地网关，分别新建会话。启用命中时meta/实际System/数据库显示课程解题引导，受控回答只有提问引导；停用仍绑定、重新启用后解绑均无技能注入并可给x=3。三个状态均合法[1]；原已完成消息及配置/证据快照不改写 |

主矩阵28/28通过、0失败、0未执行。另有生产学生页面补证，结果见 `ui-production.json`：真实页面点击创建和发送，meta候选出现时模型请求计数未增，首段可见时尚未完成且数据库已有同一前缀，done后正文/引用与数据库一致，刷新读取历史保持一致。截图位于该JSON指向的独立ui子目录。

主矩阵七次成功流均为 `meta → delta×5 → done`，无error，首delta早于done，meta Flush早于模型Open。每轮均保存实际System/user输入、候选/最终引用、完整正文及数据库前后行，拼接delta与completed消息逐字一致。所有已生成消息在后续配置变更后保持原完整行值。`.sse`文件为按实际解析事件导出的规范化记录，不冒充未经处理的网络字节抓包。

P4固定问题为“星河方程 x+2=5，请解题并直接给答案。”；启用受控回答含“请先思考等式两边应做什么”，停用/解绑含“结论：x=3”，三者均有相同A材料标识及[1]。受控替身根据服务端实际System边界选择回复，用于验证选择、注入、引用和持久化链路；不据此评价真实课程模型。本轮没有重新审判或改写5.3已通过结论。

## 失败历史及复核边界

1. 首次驱动 `runtime/align-browser-d7912cdf/` 在首个对话生成前，将测试控制端点误交给CampusAPI，客户端拒绝非/api路径。保留失败browser.json与资源；修正测试驱动为不携带凭据的同源fetch控制请求后，在新库/collection完整重跑。未改业务权限或断言。
2. 首次生产页面补证已观察候选/增量及DB前缀，但Playwright导出response body时CDP报告无数据；现有解析器在done后取消reader，故改为旁路记录它实际交付的事件，再次补证。原截图及 `ui-first-failure.json`保留；解析器和业务处理不变。该失败是测试证据导出问题，不包装为首次全绿。

本轮未发现权限泄露、非法owner/agent变化、部分绑定写入或SSE持久化不一致；不代表真实同伴确认已经取得。最终数量只指修正后完整矩阵，以上两次驱动失败单独保留。

## 命令、文件与证据

- `powershell -NoProfile -File tests/go_rag/run_align_browser.ps1 -P1P4`
- `powershell -NoProfile -File tests/go_rag/run_align_p1p4_ui.ps1 -Evidence runtime/align-browser-4b8ddd77`
- `node --test tests/go_rag/sse.test.cjs`：3/3通过。
- `openspec validate align-week5-with-course-requirements --strict`：结果见本轮 `strict.log`。
- [主矩阵完整证据](../../../runtime/align-browser-4b8ddd77/browser.json)：每个子步骤、全部脱敏请求响应、流事件/回答/引用、配置及数据库前后快照、时间、账号、实际模型输入。
- [生产页面补证](../../../runtime/align-browser-4b8ddd77/ui-production.json)、其ui目录内三张真实页面截图及事件记录；`ready.json`、`isolation.json`、`nginx-effective.txt`、`fixture.log`、`result.json`和`sse-parser.log`补充环境与执行状态。
- 新增仅测试脚本 `check_align_p1p4.py`、`check_align_p1p4_ui.py`、`run_align_p1p4_ui.ps1`；已有测试runner增加P1P4模式，已有Go测试夹具增加受控材料标识回显。正式业务代码、配置和旧change32/32不改。范围/hash/脱敏核对见 `phase5-p1p4-check.json`。

## 正式5.4状态及5.5剩余材料

5.4的自动化技术补证完成，但真实同伴四项亲自执行及本人确认仍未取得。用户已决定不邀请同伴，本轮没有授权修改正式规约，故该条件仍不满足，5.4保持未勾选，不以Codex操作抵充。没有虚构姓名、签名、参与时间或反馈。

5.5已具备：Phase0～5既有技术证据、5.3真实网关及用户人工审核、本轮技术矩阵、旧Week5/Week4保护记录及迁移风险准备材料。仍缺：正式DA3要求的真实同伴可追溯证据；全部必需证据齐备后的最终F覆盖/版本/索引汇总与脱敏、历史及迁移风险复核。若未来决定改变同伴要求，需要另行审议正式规约，不能由本轮技术报告默许豁免。本轮仅列剩余条件，不实施或勾选5.5，不宣称完整课程验收通过。
