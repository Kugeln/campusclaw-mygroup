# Phase 6 最终验收（2026-10-19）

结论：6.1–6.7 按用户批准的项目调整标准完成，累计42/42。原T71真实模型调用/用户人工审阅、原课程T68真实同伴参与均未执行，不能宣称原始参与要求全部达成。项目调整证据见 phase6-user-decisions.md/json；72份合同没有被重写。

1. **任务状态**：6.1八URL/方法与故障矩阵、6.2双班权限、6.3第五课/第四课回归、6.4真实JSON/HTML、6.5调整后的模型技术验证、6.6追踪/strict/保护、6.7调整后的模拟交叉全部完成。任务0–5保持35/35，Phase6为7/7。
2. **72项覆盖**：72 Requirement、72主Scenario、42任务、五份spec字节不变。23清单项、课件概览+9子页、8DoD、8官方操作可追踪。逐行实际证据见[覆盖表](phase6-coverage.md)与JSON。70项按原自动技术合同通过，T68/T71技术断言通过但参与方式为项目调整。来源复用已有读取/哈希，不声称新抓取或当前官方无漂移。
3. **真实MCP端到端**：独立Go进程/真实TCP、Docker私网独立MCP均有协议证据；最终全新默认Compose真实构建主应用并由initializer建立v4，上传材料、真实Qdrant索引、连接发现两个工具、保存技能、学生SSE。formula_hint及unit_convert均ok，真实换算为 `1 m = 100 cm`；工具先于meta，后续delta/done与真实知识切片引用、两条ok终态审计一致。见 fresh-e2e.json、fresh-functional.json、standalone-process.json。
4. **权限/安全/审计**：真实独立ta/ta2/sa/sa2（A班）及tb/sb（B班），分别会话/Bearer；八操作匿名401、学生403，双向外班资源404，无写入/真实RPC计数副作用；同班学生owner限制。对外只有has_auth，无Token/密文/nonce/主密钥；页面DOM与报告安全扫描通过。AES-GCM、出站固定解析/DNS重绑定/metadata/redirect、3秒单次/10秒阶段/120秒整轮、无自动call重试、危险/1201字整条丢弃和不可信工具数据、无伪知识引用等在完整矩阵通过。断开denied有归属审计，改名/删除旧名失配无新增无归属行；有审计删除503及回滚、无审计204。SQL八条终态行与SSE和class/ref/status一致，远端tools/call共4次（A/B各2），断开拒绝零新增RPC。
5. **v4及保护**：fresh、v1/v2/v3→v4、重复升级、逐步中断、漂移拒绝、锁与回执均通过；旧材料/切片/source/hash/会话/引用前后对照和独立恢复在TestMCPBackupAndLegacyProtection通过。最终全新部署另做真实mysqldump并恢复到同一新建MySQL的独立恢复库，审计内容、加密凭据字节摘要和v3/v4完成账本一致，见fresh-v4-restore.json。未连接或迁移既有Demo数据库。
6. **浏览器**：真实Edge/Nginx18组通过，另恢复3组通过；教师CRUD/连接/凭据清空/失败显示、学生ok/denied/error、无工具隐藏、历史不重放、纯文本注入、取消/断开/done丢失、1440/1024/768/390宽度、旧材料/检索页面均检查。截图来自实际页面；已自动查看student-tools-1440.png及teacher-dialog-390.png，工具区/引用分离、弹窗适配正常。部分UI故障由可控代理构造，并由真实后端故障矩阵补足，不冒称所有UI故障均来自生产服务。
7. **Swagger/OpenAPI**：现有swag及converter生成Check退出0（仓库根无Go文件的swag警告不影响生成）；八method/path/body/response/error逐项合同测试通过；浏览器实际Authorize/Execute connect、tools和技能保存，SSE面板通过。见phase6-static-checks.json及browser-result.json。
8. **第五课兼容**：alignstorage回归128个Go叶子执行通过；默认Go兼容279通过，含JWT撤销/身份/班级/技能路由/RAG/流式/上传/材料与Week4保护。go vet退出0；没有改第五课业务代码或旧OpenSpec。
9. **交叉验证**：**Codex 自动化模拟交叉验证：4/4 通过。** A双向隔离及拒绝零副作用；B has_auth有/无正确且响应/DOM/审计无凭据；C断开工具不执行、SSE denied且新增正确归属审计、不重放；D本班可见、外班不可见、状态一致、无敏感参数。角色/班级/UTC/脱敏HTTP/SQL/真实网络计数见cross-verification.json。**未邀请真实同伴，未执行真实人员交叉验证；此为用户批准的项目验收方式调整，不代表完成课程原始真实同伴参与要求。**
10. **最终统计**：Go叶子执行670通过（MCP201、Week5 128、默认279、独立服务62；套件存在重叠，不是670个唯一用例）；API35组通过（34 URL/方法+1 Go补充组）；浏览器21组、SSE解析3项、新部署E2E1组、备份恢复1组、HTML三分支全部通过。模拟4项包含在API验收，不重复累加。最终非预期失败0，调整后项目必需测试未执行0。故意失败与缺配置两分支各报告失败且CLI=1，属于预期负向验证，未被伪装为成功。原真实模型/用户人工/真实同伴三项未执行，单列为批准的参与方式例外。
11. **历史失败及修复**：七次Phase6运行全部保留：005713的旧unknown-version=4测试与现v4合法账本碰撞，最小测试夹具改成仅将v4标为未知v5（拒绝断言不变）；010434独立进程证据目标误设目录，改为文件；011051浏览器异步响应日志竞争，改实际expect_response核502/503；011350/011825隔离guard把tmpfs空选项误当缺失，改检查键存在并只允许tmpfs挂载，仍校验随机库/标签；011947 API自建agent与基础fixture重名，改独立名称。012340最终通过。原失败stdout/stderr/result未覆盖。Phase1首次Docker地址池耗尽及独立子网重试历史仍保留于Phase1报告，本轮未改写。
12. **Docker**：七轮14个拥有范围项目只读收尾核对，无本轮容器/网络/卷残留；213个既有容器Status/StartedAt/FinishedAt与初始一致；runner校对既有网络/卷清单一致。仅保留本轮独立镜像标签，无全局prune。见phase6-docker-closeout.json。所有昂贵复用均在result.json记录来源/哈希/理由，未把复用描述成重复执行。
13. **strict**：`openspec validate add-week6-mcp-tool-integration --strict` 任务勾选前后均退出0，输出 `Change 'add-week6-mcp-tool-integration' is valid`。计数及保护检查见phase6-final-checks.json。
14. **Git与文件**：分支feature/week5-phase1-go-rag、HEAD 51a8749f6550c279efebe63bb67eda724878f5a1未变；534个既有文件哈希复核，仅授权测试文件及本change审核文件变化。既有未提交文件保留，未暂存、commit、push、merge、rebase或archive。现有生产Compose/Nginx/.env.example/DDL及主应用业务逻辑本轮字节不变。测试源码新增/变更清单见下节，最终完整Git状态保存在final-checks JSON。
15. **任务进度**：42/42，其中6.5/6.7明确“按项目调整标准完成”。
16. **遗留风险/差异**：真实模型质量和真实人员反馈未验证；原课程真实同伴门槛尚未满足。名称复用、执行与终态审计间崩溃窗口、旧认证500/MCP503差异保持已冻结风险；无恰好一次承诺、无审计清理/虚构保留天数。官方历史来源记录复用边界见source-audit；技术成功不代表教师批准项目调整。
17. **后续条件**：具备按调整后项目标准进入单独授权的Git提交前审计条件；若要求原课程全部参与门槛，仍需另外补真实同伴/原T71方式。当前已停止，不执行提交、推送、发布、正式库迁移或OpenSpec归档。

## 本轮测试文件与证据

新增：internal/repository/mysql/week6_phase6_browser_integration_test.go；tests/go_mcp/compose.phase6.yml、run_phase6.py、model/main.go；apitest/week6_support.py；apitest/cases/api/mcp/下八个URL/方法脚本。

修改（均为测试/测试报告支持）：internal/repository/mysql/tutor_schema_integration_test.go（未知版本漂移夹具）；tests/go_mcp/check_phase5_browser.py（实际HTTP等待）；apitest/run_all.py（显式week6/真实性报告）；apitest/support.py（合法tmpfs隔离守卫与独立运行上下文）。未改生产功能。

本change新增phase6-preservation-baseline、user-decisions、source-audit、static-checks、progress、coverage、docker-closeout、final-checks、verification的JSON/Markdown；更新tasks、proposal、design、acceptance-matrix、review-summary、requirements-traceability、risks-and-open-questions、verification-result。五份spec未修改。

权威运行目录：runtime/week6-phase6-20261019T012340Z-a3c89340/。正常API：reports/apitest/20261019T012427Z-99120405/；预期注入失败：20261019T012547Z-85bf8675；预期缺配置：20261019T012547Z-2f8f34b9。JSON/HTML/截图保持本地且Git忽略。私有备份含加密列，仅保存于忽略的runtime，不发布其内容。
