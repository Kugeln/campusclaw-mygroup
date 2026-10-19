> **提交准备更新（2026-10-19）：** 三项阻塞已处理，当前清单以[最终方案](week5-week6-commit-plan-20261019.md)和[最终机器清单](week5-week6-commit-plan-20261019.json)为准：功能249、文档51、排除55。下方247/46/52及待处理状态保留原审计时点；原三份JSON保持本地原件，提交脱敏副本。未暂存或提交。

# Week 5 + Week 6 Git 提交前审计

日期：2026-10-19（Asia/Shanghai）。本轮仅检查与规划，并新增本报告和同名JSON。未执行git add/commit/push/merge/rebase/reset/clean/archive，未执行任何Docker命令，也未改业务、原规约、忽略规则或证据文件。

**结论：不适合直接提交整个工作区。** 功能白名单未发现真实密钥，轻量质量门槛通过；建议合并Week5/6功能提交，再提交经筛选和必要脱敏的验收摘要。当前方案等待用户审核，存在下面列出的交付前待处理项。

## 1. 当前 Git 与远端

- 分支：`feature/week5-phase1-go-rag`；HEAD：`51a8749f6550c279efebe63bb67eda724878f5a1`。
- 本次只读ls-remote成功，origin/main与HEAD一致；远端仅有main，当前feature分支无upstream且未发布。未fetch，不把本地缓存引用误称最新远端；未提交功能仍只在工作区。
- 初始已跟踪修改19个、删除0个；其中18个有内容diff，tests/test_recovery.py仅LF→CRLF。未跟踪324个，暂存0个。本报告MD/JSON新增后预计未跟踪326个。
- git diff --stat：18文件，403行新增、13行删除。git diff --cached --stat为空。初始git status短格式显示目录折叠，完整逐文件清单以本报告/JSON为准。
- origin为现有GitHub仓库，未改remote；最近HEAD历史为现有认证/材料Go迁移，未见已提交Week5/6功能。

## 2. 完整性、依赖与拆分结论

- 全部343个初始变更候选逐项读取、分类并记录SHA256；下面每行均有确定归属，无未分类项。新增源文件都在G1精确清单中，`.gitignore`没有排除必要源码；23组go:embed输入存在且在既有tracked或G1范围。
- 静态检查生产/测试Compose的字面build context与本地bind源均存在；Dockerfile.backend的COPY只取go.mod/go.sum/cmd/internal，MCP镜像取独立模块。变量形式的runtime证据挂载由runner新建，不是需要提交的源文件。没有运行docker compose config以免读取/输出真实环境。
- schema_rag.sql→schema_tutor_v3.sql的41条DDL与schema_upgrade_steps.json逐项SHA256一致；schema_mcp_v4.sql的4条与schema_mcp_steps.json一致。v4为可空ddl_hash加三MCP表；源码/账本catalog/initializer/receipt必须一并提交，不执行迁移。
- 不能安全地按完整文件拆成独立Week5、Week6功能提交：internal/tutor/skill_source.go:4与tutor.go:6直接导入internal/mcp；tutor.Snapshot含mcp.ToolCall；repository/rag_schema.go:12已把current改v4且Prepare/Validate转到MCP；catalog.go与bootstrap/main.go也接v4与回执。Week5新文件本身已有Week6内容，Git没有可直接恢复的已提交Week5基线。
- `.env.example`追加的Week5/Week6注释理论上可按hunk拆；cmd/server/main.go、bootstrap、catalog、页面embed及新文件却有类型/调用/迁移耦合。仅按hunk暂存不足以构造可构建Week5中间状态，需要重建历史版本并重新验证，超出本轮审计。推荐G1联合功能提交，保留A/B/C分类供review。
- 运行应用不依赖忽略的旧runtime备份/截图。测试有显式准备前提：tests/go_mcp/Dockerfile:1依赖本机名campusclaw-week5-rag-test:local，源码可由 `docker build -f tests/go_rag/Dockerfile -t campusclaw-week5-rag-test:local .` 构建（本轮未执行）；必须在后续补入可复核bootstrap文档/入口，不能宣称干净clone直接跑全套成功。
- Week5 runner读取runtime/align-storage-connection.json和align-stream-connection.json，它们含临时凭据，必须由已提交prepare_align_storage.ps1/prepare_align_stream.ps1在新资源上重新生成，不能提交现有连接文件。旧run.ps1默认preflight连接文件同理；realmodel路径的本机.env仅是显式可选测试配置，不是受控验收入口的秘密依赖。
- tests/go_mcp/run_phase6.py默认不传reuse，会新建证据目录、override、账号、临时库；--reuse-*是显式可选历史证据优化，不能要求外部clone有本机runtime。相应故障/浏览器/模型桩源代码已在G1。

## 3. 敏感内容与必须排除

- 扫描全部候选文本，并用本机环境文件中的已知真实凭据在既有240个tracked文件及候选中做值匹配，命中0；未打印值。私钥/JWT/provider密钥特征命中0。10个一般关键字候选经复核均为变量拼接、受控桩测试常量或Swagger UI OAuth解析实现，位置见JSON，未将占位符误报真实凭据。扫描有启发式边界，不是所有格式秘密不存在的数学证明。
- .env、.env.course.local/.env.doubao.local、runtime/临时env和连接文件、SQL备份、reports/JSON/HTML/截图、日志、缓存、编译产物、.venv、.agents/.codex/.aws及凭据目录必须排除。git check-ignore验证了实际规则，必要internal/mcp和SQL源不被忽略。
- docker-compose.retrieval.host.yml未被忽略，必须显式排除；它是旧Demo宿主映射，并非Go部署依赖，端口写法未绑定loopback。保留本机文件，不运行、不删除；后续可另行批准忽略规则修订。
- tests/test_recovery.py只有换行差异，建议本轮不提交且保留现状，未替用户还原。
- 根.dockerignore没有忽略runtime/、reports/、.codex/、.aws/等。Git忽略不等于Docker忽略；本机已存在生成的测试env/连接/备份，构建上下文可能携带这些文件到daemon。现有Dockerfile没有COPY这些目录，不能据此声称已进入最终镜像；但应在后续授权中补context排除再做包含runtime的构建。
- Swagger UI bundle约1.45MB是固定版本离线运行资产，已有LICENSE/NOTICE；随Swagger提交，不能误当无用编译输出删除。其他候选没有超过500KB文件。

### 绝对路径证据（仅列位置，不输出内容）

| 文件 | 行号 | 处理 |
|---|---|---|
| `docs/migration/phase5-contract-comparison.json` | 7 | 本地可选证据，默认不提交 |
| `openspec/changes/add-week6-mcp-tool-integration/phase0-baseline.md` | 72 | 本地可选证据，默认不提交 |
| `openspec/changes/add-week6-mcp-tool-integration/phase5-static-checks.json` | 14 | G2候选，脱敏后才可提交 |
| `openspec/changes/add-week6-mcp-tool-integration/phase6-static-checks.json` | 14 | G2候选，脱敏后才可提交 |
| `openspec/changes/add-week6-mcp-tool-integration/phase6-user-decisions.json` | 4 | G2候选，脱敏后才可提交 |
| `openspec/changes/align-week5-with-course-requirements/phase1-history-baseline.json` | 3, 7, 11, 15, 19, 23, 27, 31 | 本地可选证据，默认不提交 |
| `openspec/changes/align-week5-with-course-requirements/phase3-check-result.json` | 240, 241, 279, 280, 338, 339 | 本地可选证据，默认不提交 |
| `openspec/changes/align-week5-with-course-requirements/phase4-check-result.json` | 316, 322, 328, 359, 401, 402, 441, 442 | 本地可选证据，默认不提交 |

JSON中的绝对路径主要是本机日志与附件路径；其中用户决定JSON含个人目录，必须改为可移植的授权来源说明，且不得删改实际批准内容。当前原文件全部保留。G2若排除被引用证据，必须同时调整可发布摘要的引用说明；runtime/报告链接只作为本机证据定位，不宣称在Git可直接打开。

## 4. 轻量检查与验收一致性

- git diff --check / git diff --cached --check：均0；CRLF转换提示非空白错误。
- OpenSpec strict：add-week5-ai-tutor-and-streaming-chat、align-week5-with-course-requirements、add-week6-mcp-tool-integration全部通过。
- OpenAPI Check：成功；首次sandbox访问模块代理被拒，授权网络重试后固定工具生成和Check退出0。swag提示仓库根无Go文件不影响生成；没有覆盖生产openapi.json。
- Week6保留72 Requirement、72主Scenario、8官方MCP管理操作及42/42。phase6-user-decisions.md/json、tasks和最终报告均保留：6.5受控HTTP模型+真实MCP+自动技术复核；6.7独立账号自动化模拟4/4。未调用真实模型、未做用户人工审阅、未邀请真实同伴，原课程真实同伴与原T71方式未执行；不能写成老师批准替代。
- Week5两个change已完成32/32、29/29；保留其最终人工审阅/真实调用既有证据与项目交叉验证调整，不用Week6受控验证覆盖Week5历史。大规模测试沿用最终验收证据，本轮未重跑、未调用付费模型。

## 5. 推荐提交组（仅方案）

### G1：247 个文件

建议 message：`feat(campusclaw): deliver Week 5 tutor and Week 6 MCP integration`

联合提交当前已验收功能、测试、v1→v4迁移、共享配置和必要规约，确保中间提交可构建。

跨组依赖：A/B/C内部互相依赖，联合提交；既有go.mod/go.sum未变且仍是构建输入。

提交前待处理：确认采用联合功能提交，并显式排除本机文件；不强拆hunk。

精确路径清单：

- `.env.example`
- `Dockerfile.backend`
- `README.md`
- `apitest/README.md`
- `apitest/__init__.py`
- `apitest/cases/api/agents/GET.py`
- `apitest/cases/api/agents/POST.py`
- `apitest/cases/api/agents/{id}/GET.py`
- `apitest/cases/api/agents/{id}/PUT.py`
- `apitest/cases/api/ask/POST.py`
- `apitest/cases/api/auth/login/POST.py`
- `apitest/cases/api/auth/logout/POST.py`
- `apitest/cases/api/auth/me/GET.py`
- `apitest/cases/api/conversations/GET.py`
- `apitest/cases/api/conversations/POST.py`
- `apitest/cases/api/conversations/{id}/GET.py`
- `apitest/cases/api/conversations/{id}/messages/POST.py`
- `apitest/cases/api/materials/GET.py`
- `apitest/cases/api/materials/POST.py`
- `apitest/cases/api/materials/{id}/GET.py`
- `apitest/cases/api/materials/{id}/file/GET.py`
- `apitest/cases/api/materials/{id}/reindex/POST.py`
- `apitest/cases/api/mcp/servers/GET.py`
- `apitest/cases/api/mcp/servers/POST.py`
- `apitest/cases/api/mcp/servers/{id}/DELETE.py`
- `apitest/cases/api/mcp/servers/{id}/PUT.py`
- `apitest/cases/api/mcp/servers/{id}/connect/POST.py`
- `apitest/cases/api/mcp/servers/{id}/disconnect/POST.py`
- `apitest/cases/api/mcp/servers/{id}/tools/GET.py`
- `apitest/cases/api/mcp/tool-calls/GET.py`
- `apitest/cases/api/search/GET.py`
- `apitest/cases/api/skills/GET.py`
- `apitest/cases/api/skills/POST.py`
- `apitest/cases/api/skills/{id}/DELETE.py`
- `apitest/cases/api/skills/{id}/GET.py`
- `apitest/cases/api/skills/{id}/PUT.py`
- `apitest/cases/openapi.json/GET.py`
- `apitest/cases/swagger/GET.py`
- `apitest/cases/swagger/index.html/GET.py`
- `apitest/run_all.py`
- `apitest/support.py`
- `apitest/week6_support.py`
- `cmd/admin/main.go`
- `cmd/bootstrap/main.go`
- `cmd/server/main.go`
- `deploy/nginx.conf`
- `docker-compose.yml`
- `internal/config/rag.go`
- `internal/config/rag_test.go`
- `internal/conversation/citation_prompt_test.go`
- `internal/conversation/citations.go`
- `internal/conversation/citations_test.go`
- `internal/conversation/evidence_input_test.go`
- `internal/conversation/stream.go`
- `internal/conversation/stream_test.go`
- `internal/conversation/types.go`
- `internal/conversation/window_test.go`
- `internal/http/auth.go`
- `internal/http/conversation.go`
- `internal/http/conversation_integration_test.go`
- `internal/http/documentation.go`
- `internal/http/documentation_test.go`
- `internal/http/handler.go`
- `internal/http/login.go`
- `internal/http/material_reads.go`
- `internal/http/mcp.go`
- `internal/http/mcp_contract_test.go`
- `internal/http/mcp_conversation_integration_test.go`
- `internal/http/mcp_integration_test.go`
- `internal/http/openapi_types.go`
- `internal/http/pages.go`
- `internal/http/pages_test.go`
- `internal/http/rag.go`
- `internal/http/rag_integration_test.go`
- `internal/http/real_budget_test.go`
- `internal/http/tutor.go`
- `internal/http/tutor_browser_integration_test.go`
- `internal/http/tutor_integration_test.go`
- `internal/http/upload.go`
- `internal/http/web/static/knowledge.js`
- `internal/http/web/static/mcp.js`
- `internal/http/web/static/sse.js`
- `internal/http/web/static/tutor.css`
- `internal/http/web/static/tutor.js`
- `internal/http/web/swagger/index.html`
- `internal/http/web/swagger/init.js`
- `internal/http/web/swagger/openapi.json`
- `internal/http/web/swagger/vendor/LICENSE`
- `internal/http/web/swagger/vendor/NOTICE.md`
- `internal/http/web/swagger/vendor/swagger-ui-bundle.js`
- `internal/http/web/swagger/vendor/swagger-ui.css`
- `internal/http/web/templates/knowledge.html`
- `internal/http/web/templates/materials.html`
- `internal/http/web/templates/tutor.html`
- `internal/mcp/README.md`
- `internal/mcp/client.go`
- `internal/mcp/client_integration_test.go`
- `internal/mcp/management.go`
- `internal/mcp/security.go`
- `internal/mcp/security_test.go`
- `internal/mcp/tools.go`
- `internal/mcp/tools_test.go`
- `internal/rag/clients.go`
- `internal/rag/clients_test.go`
- `internal/rag/core.go`
- `internal/rag/core_test.go`
- `internal/rag/indexing.go`
- `internal/rag/real_model_integration_test.go`
- `internal/repository/mysql/catalog.go`
- `internal/repository/mysql/conversation.go`
- `internal/repository/mysql/conversation_delete.go`
- `internal/repository/mysql/conversation_delete_integration_test.go`
- `internal/repository/mysql/mcp.go`
- `internal/repository/mysql/mcp_integration_test.go`
- `internal/repository/mysql/mcp_runtime.go`
- `internal/repository/mysql/mcp_schema.go`
- `internal/repository/mysql/phase2_evidence_integration_test.go`
- `internal/repository/mysql/phase2_history_integration_test.go`
- `internal/repository/mysql/phase2_http_integration_test.go`
- `internal/repository/mysql/phase2_roles_integration_test.go`
- `internal/repository/mysql/phase2_routing_integration_test.go`
- `internal/repository/mysql/phase2_week4_integration_test.go`
- `internal/repository/mysql/phase3_services_integration_test.go`
- `internal/repository/mysql/phase3_stream_integration_test.go`
- `internal/repository/mysql/phase4_browser_integration_test.go`
- `internal/repository/mysql/phase5_citation_retest_integration_test.go`
- `internal/repository/mysql/phase5_real_integration_test.go`
- `internal/repository/mysql/phase5_remaining_real_integration_test.go`
- `internal/repository/mysql/rag.go`
- `internal/repository/mysql/rag_catalog_capture_test.go`
- `internal/repository/mysql/rag_schema.go`
- `internal/repository/mysql/rag_schema_integration_test.go`
- `internal/repository/mysql/schema_mcp_steps.json`
- `internal/repository/mysql/schema_mcp_v4.sql`
- `internal/repository/mysql/schema_rag.sql`
- `internal/repository/mysql/schema_rag_catalog.json`
- `internal/repository/mysql/schema_tutor_v3.sql`
- `internal/repository/mysql/schema_upgrade_steps.json`
- `internal/repository/mysql/tutor.go`
- `internal/repository/mysql/tutor_migration.go`
- `internal/repository/mysql/tutor_migration_integration_test.go`
- `internal/repository/mysql/tutor_recovery_integration_test.go`
- `internal/repository/mysql/tutor_schema.go`
- `internal/repository/mysql/tutor_schema_integration_test.go`
- `internal/repository/mysql/tutor_store_integration_test.go`
- `internal/repository/mysql/week6_phase5_browser_integration_test.go`
- `internal/repository/mysql/week6_phase6_browser_integration_test.go`
- `internal/transfer/mcp_receipt.go`
- `internal/transfer/mcp_receipt_integration_test.go`
- `internal/transfer/ready.go`
- `internal/tutor/evidence_test.go`
- `internal/tutor/legacy_skill.go`
- `internal/tutor/roles_test.go`
- `internal/tutor/routing_test.go`
- `internal/tutor/skill_source.go`
- `internal/tutor/skill_source_test.go`
- `internal/tutor/tutor.go`
- `internal/tutor/tutor_test.go`
- `mcp-server/.dockerignore`
- `mcp-server/Dockerfile`
- `mcp-server/README.md`
- `mcp-server/cmd/mcp-server/main.go`
- `mcp-server/cmd/probe/main.go`
- `mcp-server/compose.test.yml`
- `mcp-server/go.mod`
- `mcp-server/internal/checks/checks.go`
- `mcp-server/internal/server/server.go`
- `mcp-server/internal/server/server_test.go`
- `mcp-server/internal/server/tools.go`
- `mcp-server/run_phase1.py`
- `mcp-server/tests/process_test.go`
- `openspec/changes/add-week5-ai-tutor-and-streaming-chat/.openspec.yaml`
- `openspec/changes/add-week5-ai-tutor-and-streaming-chat/api-differences.md`
- `openspec/changes/add-week5-ai-tutor-and-streaming-chat/apitest-plan.md`
- `openspec/changes/add-week5-ai-tutor-and-streaming-chat/design.md`
- `openspec/changes/add-week5-ai-tutor-and-streaming-chat/proposal.md`
- `openspec/changes/add-week5-ai-tutor-and-streaming-chat/specs/student-streaming-conversations/spec.md`
- `openspec/changes/add-week5-ai-tutor-and-streaming-chat/specs/tutor-configuration/spec.md`
- `openspec/changes/add-week5-ai-tutor-and-streaming-chat/specs/week5-api-documentation-and-acceptance/spec.md`
- `openspec/changes/add-week5-ai-tutor-and-streaming-chat/tasks.md`
- `openspec/changes/add-week6-mcp-tool-integration/.openspec.yaml`
- `openspec/changes/add-week6-mcp-tool-integration/acceptance-matrix.md`
- `openspec/changes/add-week6-mcp-tool-integration/design.md`
- `openspec/changes/add-week6-mcp-tool-integration/phase6-user-decisions.md`
- `openspec/changes/add-week6-mcp-tool-integration/proposal.md`
- `openspec/changes/add-week6-mcp-tool-integration/requirements-traceability.md`
- `openspec/changes/add-week6-mcp-tool-integration/review-summary.md`
- `openspec/changes/add-week6-mcp-tool-integration/risks-and-open-questions.md`
- `openspec/changes/add-week6-mcp-tool-integration/specs/week6-mcp-acceptance/spec.md`
- `openspec/changes/add-week6-mcp-tool-integration/specs/week6-mcp-management/spec.md`
- `openspec/changes/add-week6-mcp-tool-integration/specs/week6-mcp-service/spec.md`
- `openspec/changes/add-week6-mcp-tool-integration/specs/week6-skill-tools/spec.md`
- `openspec/changes/add-week6-mcp-tool-integration/specs/week6-tool-conversations/spec.md`
- `openspec/changes/add-week6-mcp-tool-integration/tasks.md`
- `openspec/changes/align-week5-with-course-requirements/.openspec.yaml`
- `openspec/changes/align-week5-with-course-requirements/design.md`
- `openspec/changes/align-week5-with-course-requirements/proposal.md`
- `openspec/changes/align-week5-with-course-requirements/specs/student-streaming-conversations/spec.md`
- `openspec/changes/align-week5-with-course-requirements/specs/tutor-configuration/spec.md`
- `openspec/changes/align-week5-with-course-requirements/specs/week5-api-documentation-and-acceptance/spec.md`
- `openspec/changes/align-week5-with-course-requirements/specs/week5-schema-evolution/spec.md`
- `openspec/changes/align-week5-with-course-requirements/tasks.md`
- `scripts/generate_openapi.ps1`
- `scripts/openapi-convert/go.mod`
- `scripts/openapi-convert/go.sum`
- `scripts/openapi-convert/main.go`
- `scripts/openapi-convert/main_test.go`
- `tests/go_mcp/Dockerfile`
- `tests/go_mcp/README.md`
- `tests/go_mcp/check_phase5_browser.py`
- `tests/go_mcp/check_phase5_recovery.py`
- `tests/go_mcp/compose.phase3.yml`
- `tests/go_mcp/compose.phase4.yml`
- `tests/go_mcp/compose.phase5.yml`
- `tests/go_mcp/compose.phase6.yml`
- `tests/go_mcp/compose.test.yml`
- `tests/go_mcp/model/main.go`
- `tests/go_mcp/run_phase2.py`
- `tests/go_mcp/run_phase3.py`
- `tests/go_mcp/run_phase4.py`
- `tests/go_mcp/run_phase5.py`
- `tests/go_mcp/run_phase5_recovery.py`
- `tests/go_mcp/run_phase6.py`
- `tests/go_rag/Dockerfile`
- `tests/go_rag/README.md`
- `tests/go_rag/align_real_probe.py`
- `tests/go_rag/align_storage_recovery.py`
- `tests/go_rag/check_align_browser.py`
- `tests/go_rag/check_align_p1p4.py`
- `tests/go_rag/check_align_p1p4_ui.py`
- `tests/go_rag/check_knowledge_browser.py`
- `tests/go_rag/check_real_tutor_browser.py`
- `tests/go_rag/check_swagger_browser.py`
- `tests/go_rag/check_tutor_browser.py`
- `tests/go_rag/compose.yml`
- `tests/go_rag/prepare_align_storage.ps1`
- `tests/go_rag/prepare_align_stream.ps1`
- `tests/go_rag/run.ps1`
- `tests/go_rag/run_align_acceptance.ps1`
- `tests/go_rag/run_align_browser.ps1`
- `tests/go_rag/run_align_p1p4_ui.ps1`
- `tests/go_rag/run_align_real.ps1`
- `tests/go_rag/run_align_storage.ps1`
- `tests/go_rag/run_phase2.ps1`
- `tests/go_rag/run_phase3.ps1`
- `tests/go_rag/run_phase4.ps1`
- `tests/go_rag/sse.test.cjs`

### G2：46 个文件

建议 message：`docs(acceptance): record Week 5/6 verification and project exceptions`

保留必要结果摘要、真实来源边界、用户调整及本轮审计；原始日志/备份/截图不提交。

跨组依赖：G1功能/规约为本组结果说明的基础。

提交前待处理：`openspec/changes/add-week6-mcp-tool-integration/phase5-static-checks.json`; `openspec/changes/add-week6-mcp-tool-integration/phase6-static-checks.json`; `openspec/changes/add-week6-mcp-tool-integration/phase6-user-decisions.json` 中的本机绝对路径。

精确路径清单：

- `docs/audits/week5-week6-precommit-20261019.json`
- `docs/audits/week5-week6-precommit-20261019.md`
- `openspec/changes/add-week5-ai-tutor-and-streaming-chat/phase1-verification.md`
- `openspec/changes/add-week5-ai-tutor-and-streaming-chat/phase2-verification.md`
- `openspec/changes/add-week5-ai-tutor-and-streaming-chat/phase3-supplement-verification.md`
- `openspec/changes/add-week5-ai-tutor-and-streaming-chat/phase3-verification.md`
- `openspec/changes/add-week5-ai-tutor-and-streaming-chat/phase4-verification.md`
- `openspec/changes/add-week5-ai-tutor-and-streaming-chat/phase5-verification.md`
- `openspec/changes/add-week5-ai-tutor-and-streaming-chat/phase6-verification.md`
- `openspec/changes/add-week5-ai-tutor-and-streaming-chat/real-model-verification.md`
- `openspec/changes/add-week6-mcp-tool-integration/phase0-openapi-evidence.json`
- `openspec/changes/add-week6-mcp-tool-integration/phase1-verification.json`
- `openspec/changes/add-week6-mcp-tool-integration/phase1-verification.md`
- `openspec/changes/add-week6-mcp-tool-integration/phase2-verification.json`
- `openspec/changes/add-week6-mcp-tool-integration/phase2-verification.md`
- `openspec/changes/add-week6-mcp-tool-integration/phase3-verification.json`
- `openspec/changes/add-week6-mcp-tool-integration/phase3-verification.md`
- `openspec/changes/add-week6-mcp-tool-integration/phase4-verification.json`
- `openspec/changes/add-week6-mcp-tool-integration/phase4-verification.md`
- `openspec/changes/add-week6-mcp-tool-integration/phase5-static-checks.json`
- `openspec/changes/add-week6-mcp-tool-integration/phase5-verification.json`
- `openspec/changes/add-week6-mcp-tool-integration/phase5-verification.md`
- `openspec/changes/add-week6-mcp-tool-integration/phase6-coverage.json`
- `openspec/changes/add-week6-mcp-tool-integration/phase6-coverage.md`
- `openspec/changes/add-week6-mcp-tool-integration/phase6-source-audit.json`
- `openspec/changes/add-week6-mcp-tool-integration/phase6-static-checks.json`
- `openspec/changes/add-week6-mcp-tool-integration/phase6-user-decisions.json`
- `openspec/changes/add-week6-mcp-tool-integration/phase6-verification.json`
- `openspec/changes/add-week6-mcp-tool-integration/phase6-verification.md`
- `openspec/changes/add-week6-mcp-tool-integration/verification-result.json`
- `openspec/changes/align-week5-with-course-requirements/acceptance-final-report.md`
- `openspec/changes/align-week5-with-course-requirements/phase0-verification.md`
- `openspec/changes/align-week5-with-course-requirements/phase1-verification.md`
- `openspec/changes/align-week5-with-course-requirements/phase2-verification.md`
- `openspec/changes/align-week5-with-course-requirements/phase3-verification.md`
- `openspec/changes/align-week5-with-course-requirements/phase4-verification.md`
- `openspec/changes/align-week5-with-course-requirements/phase5-citation-prompt-verification.md`
- `openspec/changes/align-week5-with-course-requirements/phase5-citation-retest-verification.md`
- `openspec/changes/align-week5-with-course-requirements/phase5-human-review-check.json`
- `openspec/changes/align-week5-with-course-requirements/phase5-human-review-confirmation.md`
- `openspec/changes/align-week5-with-course-requirements/phase5-model-provenance-check.json`
- `openspec/changes/align-week5-with-course-requirements/phase5-p1p4-technical-verification.md`
- `openspec/changes/align-week5-with-course-requirements/phase5-real-model-verification.md`
- `openspec/changes/align-week5-with-course-requirements/phase5-remaining-real-verification.md`
- `openspec/changes/align-week5-with-course-requirements/phase5-verification.md`
- `openspec/changes/align-week5-with-course-requirements/project-acceptance-adjustment-check.json`

G1/G2均排除下表group为空的文件及所有Git忽略的本地敏感/运行目录。最终必须按审核后的精确白名单暂存，不能执行git add .。本报告新增两文件已计入G2，没有实际暂存。

## 6. 全部文件逐项分类

A=Week5必需；B=Week6必需；C=共享；D=可选验收证据；E=不应提交。G2中的hold文件只是条件候选，未脱敏不得暂存。local-optional仅表示默认本地保留，并非删除。

| 路径 | 类别 | 建议组 | 判断/处理 |
|---|---|---|---|
| `.env.example` | C | G1 | include；现有完整文件同时承载Week5与Week6；归联合功能提交，避免不完整中间状态 |
| `Dockerfile.backend` | C | G1 | include；现有完整文件同时承载Week5与Week6；归联合功能提交，避免不完整中间状态 |
| `README.md` | B | G1 | include；Week6 MCP服务/管理/客户端/迁移/测试/文档必需 |
| `apitest/README.md` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `apitest/__init__.py` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `apitest/cases/api/agents/GET.py` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `apitest/cases/api/agents/POST.py` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `apitest/cases/api/agents/{id}/GET.py` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `apitest/cases/api/agents/{id}/PUT.py` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `apitest/cases/api/ask/POST.py` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `apitest/cases/api/auth/login/POST.py` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `apitest/cases/api/auth/logout/POST.py` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `apitest/cases/api/auth/me/GET.py` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `apitest/cases/api/conversations/GET.py` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `apitest/cases/api/conversations/POST.py` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `apitest/cases/api/conversations/{id}/GET.py` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `apitest/cases/api/conversations/{id}/messages/POST.py` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `apitest/cases/api/materials/GET.py` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `apitest/cases/api/materials/POST.py` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `apitest/cases/api/materials/{id}/GET.py` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `apitest/cases/api/materials/{id}/file/GET.py` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `apitest/cases/api/materials/{id}/reindex/POST.py` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `apitest/cases/api/mcp/servers/GET.py` | B | G1 | include；Week6 MCP服务/管理/客户端/迁移/测试/文档必需 |
| `apitest/cases/api/mcp/servers/POST.py` | B | G1 | include；Week6 MCP服务/管理/客户端/迁移/测试/文档必需 |
| `apitest/cases/api/mcp/servers/{id}/DELETE.py` | B | G1 | include；Week6 MCP服务/管理/客户端/迁移/测试/文档必需 |
| `apitest/cases/api/mcp/servers/{id}/PUT.py` | B | G1 | include；Week6 MCP服务/管理/客户端/迁移/测试/文档必需 |
| `apitest/cases/api/mcp/servers/{id}/connect/POST.py` | B | G1 | include；Week6 MCP服务/管理/客户端/迁移/测试/文档必需 |
| `apitest/cases/api/mcp/servers/{id}/disconnect/POST.py` | B | G1 | include；Week6 MCP服务/管理/客户端/迁移/测试/文档必需 |
| `apitest/cases/api/mcp/servers/{id}/tools/GET.py` | B | G1 | include；Week6 MCP服务/管理/客户端/迁移/测试/文档必需 |
| `apitest/cases/api/mcp/tool-calls/GET.py` | B | G1 | include；Week6 MCP服务/管理/客户端/迁移/测试/文档必需 |
| `apitest/cases/api/search/GET.py` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `apitest/cases/api/skills/GET.py` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `apitest/cases/api/skills/POST.py` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `apitest/cases/api/skills/{id}/DELETE.py` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `apitest/cases/api/skills/{id}/GET.py` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `apitest/cases/api/skills/{id}/PUT.py` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `apitest/cases/openapi.json/GET.py` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `apitest/cases/swagger/GET.py` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `apitest/cases/swagger/index.html/GET.py` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `apitest/run_all.py` | C | G1 | include；现有完整文件同时承载Week5与Week6；归联合功能提交，避免不完整中间状态 |
| `apitest/support.py` | C | G1 | include；现有完整文件同时承载Week5与Week6；归联合功能提交，避免不完整中间状态 |
| `apitest/week6_support.py` | B | G1 | include；Week6 MCP服务/管理/客户端/迁移/测试/文档必需 |
| `cmd/admin/main.go` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `cmd/bootstrap/main.go` | C | G1 | include；现有完整文件同时承载Week5与Week6；归联合功能提交，避免不完整中间状态 |
| `cmd/server/main.go` | C | G1 | include；现有完整文件同时承载Week5与Week6；归联合功能提交，避免不完整中间状态 |
| `deploy/nginx.conf` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `docker-compose.retrieval.host.yml` | E | 排除 | exclude-preserve；本机旧Flask/Demo端口覆盖；不是默认Go部署依赖，端口未限定127.0.0.1 |
| `docker-compose.yml` | C | G1 | include；现有完整文件同时承载Week5与Week6；归联合功能提交，避免不完整中间状态 |
| `docs/migration/phase5-contract-comparison.json` | D | 排除 | local-optional；旧Go迁移基础认证/材料合同证据，不是本次Week5/6必要源文件；需处理本机路径 |
| `internal/config/rag.go` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `internal/config/rag_test.go` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `internal/conversation/citation_prompt_test.go` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `internal/conversation/citations.go` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `internal/conversation/citations_test.go` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `internal/conversation/evidence_input_test.go` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `internal/conversation/stream.go` | C | G1 | include；现有完整文件同时承载Week5与Week6；归联合功能提交，避免不完整中间状态 |
| `internal/conversation/stream_test.go` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `internal/conversation/types.go` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `internal/conversation/window_test.go` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `internal/http/auth.go` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `internal/http/conversation.go` | C | G1 | include；现有完整文件同时承载Week5与Week6；归联合功能提交，避免不完整中间状态 |
| `internal/http/conversation_integration_test.go` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `internal/http/documentation.go` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `internal/http/documentation_test.go` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `internal/http/handler.go` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `internal/http/login.go` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `internal/http/material_reads.go` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `internal/http/mcp.go` | B | G1 | include；Week6 MCP服务/管理/客户端/迁移/测试/文档必需 |
| `internal/http/mcp_contract_test.go` | B | G1 | include；Week6 MCP服务/管理/客户端/迁移/测试/文档必需 |
| `internal/http/mcp_conversation_integration_test.go` | B | G1 | include；Week6 MCP服务/管理/客户端/迁移/测试/文档必需 |
| `internal/http/mcp_integration_test.go` | B | G1 | include；Week6 MCP服务/管理/客户端/迁移/测试/文档必需 |
| `internal/http/openapi_types.go` | C | G1 | include；现有完整文件同时承载Week5与Week6；归联合功能提交，避免不完整中间状态 |
| `internal/http/pages.go` | C | G1 | include；现有完整文件同时承载Week5与Week6；归联合功能提交，避免不完整中间状态 |
| `internal/http/pages_test.go` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `internal/http/rag.go` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `internal/http/rag_integration_test.go` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `internal/http/real_budget_test.go` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `internal/http/tutor.go` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `internal/http/tutor_browser_integration_test.go` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `internal/http/tutor_integration_test.go` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `internal/http/upload.go` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `internal/http/web/static/knowledge.js` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `internal/http/web/static/mcp.js` | B | G1 | include；Week6 MCP服务/管理/客户端/迁移/测试/文档必需 |
| `internal/http/web/static/sse.js` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `internal/http/web/static/tutor.css` | C | G1 | include；现有完整文件同时承载Week5与Week6；归联合功能提交，避免不完整中间状态 |
| `internal/http/web/static/tutor.js` | C | G1 | include；现有完整文件同时承载Week5与Week6；归联合功能提交，避免不完整中间状态 |
| `internal/http/web/swagger/index.html` | C | G1 | include；现有完整文件同时承载Week5与Week6；归联合功能提交，避免不完整中间状态 |
| `internal/http/web/swagger/init.js` | C | G1 | include；现有完整文件同时承载Week5与Week6；归联合功能提交，避免不完整中间状态 |
| `internal/http/web/swagger/openapi.json` | C | G1 | include；现有完整文件同时承载Week5与Week6；归联合功能提交，避免不完整中间状态 |
| `internal/http/web/swagger/vendor/LICENSE` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `internal/http/web/swagger/vendor/NOTICE.md` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `internal/http/web/swagger/vendor/swagger-ui-bundle.js` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `internal/http/web/swagger/vendor/swagger-ui.css` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `internal/http/web/templates/knowledge.html` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `internal/http/web/templates/materials.html` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `internal/http/web/templates/tutor.html` | C | G1 | include；现有完整文件同时承载Week5与Week6；归联合功能提交，避免不完整中间状态 |
| `internal/mcp/README.md` | B | G1 | include；Week6 MCP服务/管理/客户端/迁移/测试/文档必需 |
| `internal/mcp/client.go` | B | G1 | include；Week6 MCP服务/管理/客户端/迁移/测试/文档必需 |
| `internal/mcp/client_integration_test.go` | B | G1 | include；Week6 MCP服务/管理/客户端/迁移/测试/文档必需 |
| `internal/mcp/management.go` | B | G1 | include；Week6 MCP服务/管理/客户端/迁移/测试/文档必需 |
| `internal/mcp/security.go` | B | G1 | include；Week6 MCP服务/管理/客户端/迁移/测试/文档必需 |
| `internal/mcp/security_test.go` | B | G1 | include；Week6 MCP服务/管理/客户端/迁移/测试/文档必需 |
| `internal/mcp/tools.go` | B | G1 | include；Week6 MCP服务/管理/客户端/迁移/测试/文档必需 |
| `internal/mcp/tools_test.go` | B | G1 | include；Week6 MCP服务/管理/客户端/迁移/测试/文档必需 |
| `internal/rag/clients.go` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `internal/rag/clients_test.go` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `internal/rag/core.go` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `internal/rag/core_test.go` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `internal/rag/indexing.go` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `internal/rag/real_model_integration_test.go` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `internal/repository/mysql/catalog.go` | C | G1 | include；现有完整文件同时承载Week5与Week6；归联合功能提交，避免不完整中间状态 |
| `internal/repository/mysql/conversation.go` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `internal/repository/mysql/conversation_delete.go` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `internal/repository/mysql/conversation_delete_integration_test.go` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `internal/repository/mysql/mcp.go` | B | G1 | include；Week6 MCP服务/管理/客户端/迁移/测试/文档必需 |
| `internal/repository/mysql/mcp_integration_test.go` | B | G1 | include；Week6 MCP服务/管理/客户端/迁移/测试/文档必需 |
| `internal/repository/mysql/mcp_runtime.go` | B | G1 | include；Week6 MCP服务/管理/客户端/迁移/测试/文档必需 |
| `internal/repository/mysql/mcp_schema.go` | B | G1 | include；Week6 MCP服务/管理/客户端/迁移/测试/文档必需 |
| `internal/repository/mysql/phase2_evidence_integration_test.go` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `internal/repository/mysql/phase2_history_integration_test.go` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `internal/repository/mysql/phase2_http_integration_test.go` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `internal/repository/mysql/phase2_roles_integration_test.go` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `internal/repository/mysql/phase2_routing_integration_test.go` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `internal/repository/mysql/phase2_week4_integration_test.go` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `internal/repository/mysql/phase3_services_integration_test.go` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `internal/repository/mysql/phase3_stream_integration_test.go` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `internal/repository/mysql/phase4_browser_integration_test.go` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `internal/repository/mysql/phase5_citation_retest_integration_test.go` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `internal/repository/mysql/phase5_real_integration_test.go` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `internal/repository/mysql/phase5_remaining_real_integration_test.go` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `internal/repository/mysql/rag.go` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `internal/repository/mysql/rag_catalog_capture_test.go` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `internal/repository/mysql/rag_schema.go` | C | G1 | include；现有完整文件同时承载Week5与Week6；归联合功能提交，避免不完整中间状态 |
| `internal/repository/mysql/rag_schema_integration_test.go` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `internal/repository/mysql/schema_mcp_steps.json` | B | G1 | include；Week6 MCP服务/管理/客户端/迁移/测试/文档必需 |
| `internal/repository/mysql/schema_mcp_v4.sql` | B | G1 | include；Week6 MCP服务/管理/客户端/迁移/测试/文档必需 |
| `internal/repository/mysql/schema_rag.sql` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `internal/repository/mysql/schema_rag_catalog.json` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `internal/repository/mysql/schema_tutor_v3.sql` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `internal/repository/mysql/schema_upgrade_steps.json` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `internal/repository/mysql/tutor.go` | C | G1 | include；现有完整文件同时承载Week5与Week6；归联合功能提交，避免不完整中间状态 |
| `internal/repository/mysql/tutor_migration.go` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `internal/repository/mysql/tutor_migration_integration_test.go` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `internal/repository/mysql/tutor_recovery_integration_test.go` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `internal/repository/mysql/tutor_schema.go` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `internal/repository/mysql/tutor_schema_integration_test.go` | C | G1 | include；现有完整文件同时承载Week5与Week6；归联合功能提交，避免不完整中间状态 |
| `internal/repository/mysql/tutor_store_integration_test.go` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `internal/repository/mysql/week6_phase5_browser_integration_test.go` | B | G1 | include；Week6 MCP服务/管理/客户端/迁移/测试/文档必需 |
| `internal/repository/mysql/week6_phase6_browser_integration_test.go` | B | G1 | include；Week6 MCP服务/管理/客户端/迁移/测试/文档必需 |
| `internal/transfer/mcp_receipt.go` | B | G1 | include；Week6 MCP服务/管理/客户端/迁移/测试/文档必需 |
| `internal/transfer/mcp_receipt_integration_test.go` | B | G1 | include；Week6 MCP服务/管理/客户端/迁移/测试/文档必需 |
| `internal/transfer/ready.go` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `internal/tutor/evidence_test.go` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `internal/tutor/legacy_skill.go` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `internal/tutor/roles_test.go` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `internal/tutor/routing_test.go` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `internal/tutor/skill_source.go` | C | G1 | include；现有完整文件同时承载Week5与Week6；归联合功能提交，避免不完整中间状态 |
| `internal/tutor/skill_source_test.go` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `internal/tutor/tutor.go` | C | G1 | include；现有完整文件同时承载Week5与Week6；归联合功能提交，避免不完整中间状态 |
| `internal/tutor/tutor_test.go` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `mcp-server/.dockerignore` | B | G1 | include；Week6 MCP服务/管理/客户端/迁移/测试/文档必需 |
| `mcp-server/Dockerfile` | B | G1 | include；Week6 MCP服务/管理/客户端/迁移/测试/文档必需 |
| `mcp-server/README.md` | B | G1 | include；Week6 MCP服务/管理/客户端/迁移/测试/文档必需 |
| `mcp-server/cmd/mcp-server/main.go` | B | G1 | include；Week6 MCP服务/管理/客户端/迁移/测试/文档必需 |
| `mcp-server/cmd/probe/main.go` | B | G1 | include；Week6 MCP服务/管理/客户端/迁移/测试/文档必需 |
| `mcp-server/compose.test.yml` | B | G1 | include；Week6 MCP服务/管理/客户端/迁移/测试/文档必需 |
| `mcp-server/go.mod` | B | G1 | include；Week6 MCP服务/管理/客户端/迁移/测试/文档必需 |
| `mcp-server/internal/checks/checks.go` | B | G1 | include；Week6 MCP服务/管理/客户端/迁移/测试/文档必需 |
| `mcp-server/internal/server/server.go` | B | G1 | include；Week6 MCP服务/管理/客户端/迁移/测试/文档必需 |
| `mcp-server/internal/server/server_test.go` | B | G1 | include；Week6 MCP服务/管理/客户端/迁移/测试/文档必需 |
| `mcp-server/internal/server/tools.go` | B | G1 | include；Week6 MCP服务/管理/客户端/迁移/测试/文档必需 |
| `mcp-server/run_phase1.py` | B | G1 | include；Week6 MCP服务/管理/客户端/迁移/测试/文档必需 |
| `mcp-server/tests/process_test.go` | B | G1 | include；Week6 MCP服务/管理/客户端/迁移/测试/文档必需 |
| `openspec/changes/add-week5-ai-tutor-and-streaming-chat/.openspec.yaml` | A | G1 | include；已完成课程的必要规划/规约；保留项目调整与原要求差异 |
| `openspec/changes/add-week5-ai-tutor-and-streaming-chat/api-differences.md` | A | G1 | include；已完成课程的必要规划/规约；保留项目调整与原要求差异 |
| `openspec/changes/add-week5-ai-tutor-and-streaming-chat/apitest-plan.md` | A | G1 | include；已完成课程的必要规划/规约；保留项目调整与原要求差异 |
| `openspec/changes/add-week5-ai-tutor-and-streaming-chat/design.md` | A | G1 | include；已完成课程的必要规划/规约；保留项目调整与原要求差异 |
| `openspec/changes/add-week5-ai-tutor-and-streaming-chat/phase1-verification.md` | D | G2 | include-summary；阶段验收证据，非编译/部署输入 |
| `openspec/changes/add-week5-ai-tutor-and-streaming-chat/phase2-verification.md` | D | G2 | include-summary；阶段验收证据，非编译/部署输入 |
| `openspec/changes/add-week5-ai-tutor-and-streaming-chat/phase3-supplement-verification.md` | D | G2 | include-summary；阶段验收证据，非编译/部署输入 |
| `openspec/changes/add-week5-ai-tutor-and-streaming-chat/phase3-verification.md` | D | G2 | include-summary；阶段验收证据，非编译/部署输入 |
| `openspec/changes/add-week5-ai-tutor-and-streaming-chat/phase4-verification.md` | D | G2 | include-summary；阶段验收证据，非编译/部署输入 |
| `openspec/changes/add-week5-ai-tutor-and-streaming-chat/phase5-verification.md` | D | G2 | include-summary；阶段验收证据，非编译/部署输入 |
| `openspec/changes/add-week5-ai-tutor-and-streaming-chat/phase6-verification.md` | D | G2 | include-summary；阶段验收证据，非编译/部署输入 |
| `openspec/changes/add-week5-ai-tutor-and-streaming-chat/proposal.md` | A | G1 | include；已完成课程的必要规划/规约；保留项目调整与原要求差异 |
| `openspec/changes/add-week5-ai-tutor-and-streaming-chat/real-model-verification.md` | D | G2 | include-summary；阶段验收证据，非编译/部署输入 |
| `openspec/changes/add-week5-ai-tutor-and-streaming-chat/specs/student-streaming-conversations/spec.md` | A | G1 | include；已完成课程的必要规划/规约；保留项目调整与原要求差异 |
| `openspec/changes/add-week5-ai-tutor-and-streaming-chat/specs/tutor-configuration/spec.md` | A | G1 | include；已完成课程的必要规划/规约；保留项目调整与原要求差异 |
| `openspec/changes/add-week5-ai-tutor-and-streaming-chat/specs/week5-api-documentation-and-acceptance/spec.md` | A | G1 | include；已完成课程的必要规划/规约；保留项目调整与原要求差异 |
| `openspec/changes/add-week5-ai-tutor-and-streaming-chat/tasks.md` | A | G1 | include；已完成课程的必要规划/规约；保留项目调整与原要求差异 |
| `openspec/changes/add-week6-mcp-tool-integration/.openspec.yaml` | B | G1 | include；已完成课程的必要规划/规约；保留项目调整与原要求差异 |
| `openspec/changes/add-week6-mcp-tool-integration/acceptance-matrix.md` | B | G1 | include；已完成课程的必要规划/规约；保留项目调整与原要求差异 |
| `openspec/changes/add-week6-mcp-tool-integration/design.md` | B | G1 | include；已完成课程的必要规划/规约；保留项目调整与原要求差异 |
| `openspec/changes/add-week6-mcp-tool-integration/phase0-baseline.md` | D | 排除 | local-optional；阶段验收证据，非编译/部署输入；基线/过程快照或重复中间记录建议本地保留；需处理本机路径 |
| `openspec/changes/add-week6-mcp-tool-integration/phase0-openapi-evidence.json` | D | G2 | include-summary；阶段验收证据，非编译/部署输入 |
| `openspec/changes/add-week6-mcp-tool-integration/phase0-session-baseline.json` | D | 排除 | local-optional；阶段验收证据，非编译/部署输入；基线/过程快照或重复中间记录建议本地保留 |
| `openspec/changes/add-week6-mcp-tool-integration/phase1-closeout-docker.json` | D | 排除 | local-optional；阶段验收证据，非编译/部署输入；基线/过程快照或重复中间记录建议本地保留 |
| `openspec/changes/add-week6-mcp-tool-integration/phase1-preservation-baseline.json` | D | 排除 | local-optional；阶段验收证据，非编译/部署输入；基线/过程快照或重复中间记录建议本地保留 |
| `openspec/changes/add-week6-mcp-tool-integration/phase1-verification.json` | D | G2 | include-summary；阶段验收证据，非编译/部署输入 |
| `openspec/changes/add-week6-mcp-tool-integration/phase1-verification.md` | D | G2 | include-summary；阶段验收证据，非编译/部署输入 |
| `openspec/changes/add-week6-mcp-tool-integration/phase2-docker-closeout.json` | D | 排除 | local-optional；阶段验收证据，非编译/部署输入；基线/过程快照或重复中间记录建议本地保留 |
| `openspec/changes/add-week6-mcp-tool-integration/phase2-preservation-baseline.json` | D | 排除 | local-optional；阶段验收证据，非编译/部署输入；基线/过程快照或重复中间记录建议本地保留 |
| `openspec/changes/add-week6-mcp-tool-integration/phase2-verification.json` | D | G2 | include-summary；阶段验收证据，非编译/部署输入 |
| `openspec/changes/add-week6-mcp-tool-integration/phase2-verification.md` | D | G2 | include-summary；阶段验收证据，非编译/部署输入 |
| `openspec/changes/add-week6-mcp-tool-integration/phase3-docker-closeout.json` | D | 排除 | local-optional；阶段验收证据，非编译/部署输入；基线/过程快照或重复中间记录建议本地保留 |
| `openspec/changes/add-week6-mcp-tool-integration/phase3-final-checks.json` | D | 排除 | local-optional；阶段验收证据，非编译/部署输入；基线/过程快照或重复中间记录建议本地保留 |
| `openspec/changes/add-week6-mcp-tool-integration/phase3-preservation-baseline.json` | D | 排除 | local-optional；阶段验收证据，非编译/部署输入；基线/过程快照或重复中间记录建议本地保留 |
| `openspec/changes/add-week6-mcp-tool-integration/phase3-verification.json` | D | G2 | include-summary；阶段验收证据，非编译/部署输入 |
| `openspec/changes/add-week6-mcp-tool-integration/phase3-verification.md` | D | G2 | include-summary；阶段验收证据，非编译/部署输入 |
| `openspec/changes/add-week6-mcp-tool-integration/phase4-docker-closeout.json` | D | 排除 | local-optional；阶段验收证据，非编译/部署输入；基线/过程快照或重复中间记录建议本地保留 |
| `openspec/changes/add-week6-mcp-tool-integration/phase4-final-checks.json` | D | 排除 | local-optional；阶段验收证据，非编译/部署输入；基线/过程快照或重复中间记录建议本地保留 |
| `openspec/changes/add-week6-mcp-tool-integration/phase4-preservation-baseline.json` | D | 排除 | local-optional；阶段验收证据，非编译/部署输入；基线/过程快照或重复中间记录建议本地保留 |
| `openspec/changes/add-week6-mcp-tool-integration/phase4-verification.json` | D | G2 | include-summary；阶段验收证据，非编译/部署输入 |
| `openspec/changes/add-week6-mcp-tool-integration/phase4-verification.md` | D | G2 | include-summary；阶段验收证据，非编译/部署输入 |
| `openspec/changes/add-week6-mcp-tool-integration/phase5-docker-closeout.json` | D | 排除 | local-optional；阶段验收证据，非编译/部署输入；基线/过程快照或重复中间记录建议本地保留 |
| `openspec/changes/add-week6-mcp-tool-integration/phase5-final-checks.json` | D | 排除 | local-optional；阶段验收证据，非编译/部署输入；基线/过程快照或重复中间记录建议本地保留 |
| `openspec/changes/add-week6-mcp-tool-integration/phase5-preservation-baseline.json` | D | 排除 | local-optional；阶段验收证据，非编译/部署输入；基线/过程快照或重复中间记录建议本地保留 |
| `openspec/changes/add-week6-mcp-tool-integration/phase5-static-checks.json` | D | G2 | hold-until-path-sanitized；阶段验收证据，非编译/部署输入；需处理本机路径 |
| `openspec/changes/add-week6-mcp-tool-integration/phase5-verification.json` | D | G2 | include-summary；阶段验收证据，非编译/部署输入 |
| `openspec/changes/add-week6-mcp-tool-integration/phase5-verification.md` | D | G2 | include-summary；阶段验收证据，非编译/部署输入 |
| `openspec/changes/add-week6-mcp-tool-integration/phase6-coverage.json` | D | G2 | include-summary；阶段验收证据，非编译/部署输入 |
| `openspec/changes/add-week6-mcp-tool-integration/phase6-coverage.md` | D | G2 | include-summary；阶段验收证据，非编译/部署输入 |
| `openspec/changes/add-week6-mcp-tool-integration/phase6-docker-closeout.json` | D | 排除 | local-optional；阶段验收证据，非编译/部署输入；基线/过程快照或重复中间记录建议本地保留 |
| `openspec/changes/add-week6-mcp-tool-integration/phase6-final-checks.json` | D | 排除 | local-optional；阶段验收证据，非编译/部署输入；基线/过程快照或重复中间记录建议本地保留 |
| `openspec/changes/add-week6-mcp-tool-integration/phase6-preservation-baseline.json` | D | 排除 | local-optional；阶段验收证据，非编译/部署输入；基线/过程快照或重复中间记录建议本地保留 |
| `openspec/changes/add-week6-mcp-tool-integration/phase6-progress.json` | D | 排除 | local-optional；阶段验收证据，非编译/部署输入；基线/过程快照或重复中间记录建议本地保留 |
| `openspec/changes/add-week6-mcp-tool-integration/phase6-source-audit.json` | D | G2 | include-summary；阶段验收证据，非编译/部署输入 |
| `openspec/changes/add-week6-mcp-tool-integration/phase6-static-checks.json` | D | G2 | hold-until-path-sanitized；阶段验收证据，非编译/部署输入；需处理本机路径 |
| `openspec/changes/add-week6-mcp-tool-integration/phase6-user-decisions.json` | D | G2 | hold-until-path-sanitized；阶段验收证据，非编译/部署输入；需处理本机路径 |
| `openspec/changes/add-week6-mcp-tool-integration/phase6-user-decisions.md` | B | G1 | include；已完成课程的必要规划/规约；保留项目调整与原要求差异 |
| `openspec/changes/add-week6-mcp-tool-integration/phase6-verification.json` | D | G2 | include-summary；阶段验收证据，非编译/部署输入 |
| `openspec/changes/add-week6-mcp-tool-integration/phase6-verification.md` | D | G2 | include-summary；阶段验收证据，非编译/部署输入 |
| `openspec/changes/add-week6-mcp-tool-integration/preservation-baseline.json` | D | 排除 | local-optional；阶段验收证据，非编译/部署输入；基线/过程快照或重复中间记录建议本地保留 |
| `openspec/changes/add-week6-mcp-tool-integration/proposal.md` | B | G1 | include；已完成课程的必要规划/规约；保留项目调整与原要求差异 |
| `openspec/changes/add-week6-mcp-tool-integration/requirements-traceability.md` | B | G1 | include；已完成课程的必要规划/规约；保留项目调整与原要求差异 |
| `openspec/changes/add-week6-mcp-tool-integration/review-summary.md` | B | G1 | include；已完成课程的必要规划/规约；保留项目调整与原要求差异 |
| `openspec/changes/add-week6-mcp-tool-integration/risks-and-open-questions.md` | B | G1 | include；已完成课程的必要规划/规约；保留项目调整与原要求差异 |
| `openspec/changes/add-week6-mcp-tool-integration/specs/week6-mcp-acceptance/spec.md` | B | G1 | include；已完成课程的必要规划/规约；保留项目调整与原要求差异 |
| `openspec/changes/add-week6-mcp-tool-integration/specs/week6-mcp-management/spec.md` | B | G1 | include；已完成课程的必要规划/规约；保留项目调整与原要求差异 |
| `openspec/changes/add-week6-mcp-tool-integration/specs/week6-mcp-service/spec.md` | B | G1 | include；已完成课程的必要规划/规约；保留项目调整与原要求差异 |
| `openspec/changes/add-week6-mcp-tool-integration/specs/week6-skill-tools/spec.md` | B | G1 | include；已完成课程的必要规划/规约；保留项目调整与原要求差异 |
| `openspec/changes/add-week6-mcp-tool-integration/specs/week6-tool-conversations/spec.md` | B | G1 | include；已完成课程的必要规划/规约；保留项目调整与原要求差异 |
| `openspec/changes/add-week6-mcp-tool-integration/tasks.md` | B | G1 | include；已完成课程的必要规划/规约；保留项目调整与原要求差异 |
| `openspec/changes/add-week6-mcp-tool-integration/verification-result.json` | D | G2 | include-summary；阶段验收证据，非编译/部署输入 |
| `openspec/changes/align-week5-with-course-requirements/.openspec.yaml` | A | G1 | include；已完成课程的必要规划/规约；保留项目调整与原要求差异 |
| `openspec/changes/align-week5-with-course-requirements/acceptance-final-report.md` | D | G2 | include-summary；阶段验收证据，非编译/部署输入 |
| `openspec/changes/align-week5-with-course-requirements/acceptance-handoff-draft.md` | D | 排除 | local-optional；阶段验收证据，非编译/部署输入；基线/过程快照或重复中间记录建议本地保留 |
| `openspec/changes/align-week5-with-course-requirements/design.md` | A | G1 | include；已完成课程的必要规划/规约；保留项目调整与原要求差异 |
| `openspec/changes/align-week5-with-course-requirements/phase0-baseline.json` | D | 排除 | local-optional；阶段验收证据，非编译/部署输入；基线/过程快照或重复中间记录建议本地保留 |
| `openspec/changes/align-week5-with-course-requirements/phase0-check-result.json` | D | 排除 | local-optional；阶段验收证据，非编译/部署输入；基线/过程快照或重复中间记录建议本地保留 |
| `openspec/changes/align-week5-with-course-requirements/phase0-source-review.json` | D | 排除 | local-optional；阶段验收证据，非编译/部署输入；基线/过程快照或重复中间记录建议本地保留 |
| `openspec/changes/align-week5-with-course-requirements/phase0-verification.md` | D | G2 | include-summary；阶段验收证据，非编译/部署输入 |
| `openspec/changes/align-week5-with-course-requirements/phase1-check-result.json` | D | 排除 | local-optional；阶段验收证据，非编译/部署输入；基线/过程快照或重复中间记录建议本地保留 |
| `openspec/changes/align-week5-with-course-requirements/phase1-history-baseline.json` | D | 排除 | local-optional；阶段验收证据，非编译/部署输入；基线/过程快照或重复中间记录建议本地保留；需处理本机路径 |
| `openspec/changes/align-week5-with-course-requirements/phase1-migration-preflight.md` | D | 排除 | local-optional；阶段验收证据，非编译/部署输入；基线/过程快照或重复中间记录建议本地保留 |
| `openspec/changes/align-week5-with-course-requirements/phase1-verification.md` | D | G2 | include-summary；阶段验收证据，非编译/部署输入 |
| `openspec/changes/align-week5-with-course-requirements/phase2-baseline.json` | D | 排除 | local-optional；阶段验收证据，非编译/部署输入；基线/过程快照或重复中间记录建议本地保留 |
| `openspec/changes/align-week5-with-course-requirements/phase2-check-result.json` | D | 排除 | local-optional；阶段验收证据，非编译/部署输入；基线/过程快照或重复中间记录建议本地保留 |
| `openspec/changes/align-week5-with-course-requirements/phase2-verification.md` | D | G2 | include-summary；阶段验收证据，非编译/部署输入 |
| `openspec/changes/align-week5-with-course-requirements/phase3-baseline.json` | D | 排除 | local-optional；阶段验收证据，非编译/部署输入；基线/过程快照或重复中间记录建议本地保留 |
| `openspec/changes/align-week5-with-course-requirements/phase3-check-result.json` | D | 排除 | local-optional；阶段验收证据，非编译/部署输入；基线/过程快照或重复中间记录建议本地保留；需处理本机路径 |
| `openspec/changes/align-week5-with-course-requirements/phase3-verification.md` | D | G2 | include-summary；阶段验收证据，非编译/部署输入 |
| `openspec/changes/align-week5-with-course-requirements/phase4-baseline.json` | D | 排除 | local-optional；阶段验收证据，非编译/部署输入；基线/过程快照或重复中间记录建议本地保留 |
| `openspec/changes/align-week5-with-course-requirements/phase4-check-result.json` | D | 排除 | local-optional；阶段验收证据，非编译/部署输入；基线/过程快照或重复中间记录建议本地保留；需处理本机路径 |
| `openspec/changes/align-week5-with-course-requirements/phase4-verification.md` | D | G2 | include-summary；阶段验收证据，非编译/部署输入 |
| `openspec/changes/align-week5-with-course-requirements/phase5-baseline.json` | D | 排除 | local-optional；阶段验收证据，非编译/部署输入；基线/过程快照或重复中间记录建议本地保留 |
| `openspec/changes/align-week5-with-course-requirements/phase5-check-result.json` | D | 排除 | local-optional；阶段验收证据，非编译/部署输入；基线/过程快照或重复中间记录建议本地保留 |
| `openspec/changes/align-week5-with-course-requirements/phase5-citation-prompt-baseline.json` | D | 排除 | local-optional；阶段验收证据，非编译/部署输入；基线/过程快照或重复中间记录建议本地保留 |
| `openspec/changes/align-week5-with-course-requirements/phase5-citation-prompt-check.json` | D | 排除 | local-optional；阶段验收证据，非编译/部署输入；基线/过程快照或重复中间记录建议本地保留 |
| `openspec/changes/align-week5-with-course-requirements/phase5-citation-prompt-verification.md` | D | G2 | include-summary；阶段验收证据，非编译/部署输入 |
| `openspec/changes/align-week5-with-course-requirements/phase5-citation-retest-baseline.json` | D | 排除 | local-optional；阶段验收证据，非编译/部署输入；基线/过程快照或重复中间记录建议本地保留 |
| `openspec/changes/align-week5-with-course-requirements/phase5-citation-retest-check.json` | D | 排除 | local-optional；阶段验收证据，非编译/部署输入；基线/过程快照或重复中间记录建议本地保留 |
| `openspec/changes/align-week5-with-course-requirements/phase5-citation-retest-verification.md` | D | G2 | include-summary；阶段验收证据，非编译/部署输入 |
| `openspec/changes/align-week5-with-course-requirements/phase5-human-review-check.json` | D | G2 | include-summary；阶段验收证据，非编译/部署输入 |
| `openspec/changes/align-week5-with-course-requirements/phase5-human-review-confirmation.md` | D | G2 | include-summary；阶段验收证据，非编译/部署输入 |
| `openspec/changes/align-week5-with-course-requirements/phase5-model-provenance-check.json` | D | G2 | include-summary；阶段验收证据，非编译/部署输入 |
| `openspec/changes/align-week5-with-course-requirements/phase5-p1p4-baseline.json` | D | 排除 | local-optional；阶段验收证据，非编译/部署输入；基线/过程快照或重复中间记录建议本地保留 |
| `openspec/changes/align-week5-with-course-requirements/phase5-p1p4-check.json` | D | 排除 | local-optional；阶段验收证据，非编译/部署输入；基线/过程快照或重复中间记录建议本地保留 |
| `openspec/changes/align-week5-with-course-requirements/phase5-p1p4-technical-verification.md` | D | G2 | include-summary；阶段验收证据，非编译/部署输入 |
| `openspec/changes/align-week5-with-course-requirements/phase5-real-execution-check.json` | D | 排除 | local-optional；阶段验收证据，非编译/部署输入；基线/过程快照或重复中间记录建议本地保留 |
| `openspec/changes/align-week5-with-course-requirements/phase5-real-model-baseline.json` | D | 排除 | local-optional；阶段验收证据，非编译/部署输入；基线/过程快照或重复中间记录建议本地保留 |
| `openspec/changes/align-week5-with-course-requirements/phase5-real-model-check-result.json` | D | 排除 | local-optional；阶段验收证据，非编译/部署输入；基线/过程快照或重复中间记录建议本地保留 |
| `openspec/changes/align-week5-with-course-requirements/phase5-real-model-verification.md` | D | G2 | include-summary；阶段验收证据，非编译/部署输入 |
| `openspec/changes/align-week5-with-course-requirements/phase5-remaining-real-baseline.json` | D | 排除 | local-optional；阶段验收证据，非编译/部署输入；基线/过程快照或重复中间记录建议本地保留 |
| `openspec/changes/align-week5-with-course-requirements/phase5-remaining-real-check.json` | D | 排除 | local-optional；阶段验收证据，非编译/部署输入；基线/过程快照或重复中间记录建议本地保留 |
| `openspec/changes/align-week5-with-course-requirements/phase5-remaining-real-verification.md` | D | G2 | include-summary；阶段验收证据，非编译/部署输入 |
| `openspec/changes/align-week5-with-course-requirements/phase5-report-baseline.json` | D | 排除 | local-optional；阶段验收证据，非编译/部署输入；基线/过程快照或重复中间记录建议本地保留 |
| `openspec/changes/align-week5-with-course-requirements/phase5-report-check.json` | D | 排除 | local-optional；阶段验收证据，非编译/部署输入；基线/过程快照或重复中间记录建议本地保留 |
| `openspec/changes/align-week5-with-course-requirements/phase5-verification.md` | D | G2 | include-summary；阶段验收证据，非编译/部署输入 |
| `openspec/changes/align-week5-with-course-requirements/project-acceptance-adjustment-baseline.json` | D | 排除 | local-optional；阶段验收证据，非编译/部署输入；基线/过程快照或重复中间记录建议本地保留 |
| `openspec/changes/align-week5-with-course-requirements/project-acceptance-adjustment-check.json` | D | G2 | include-summary；阶段验收证据，非编译/部署输入 |
| `openspec/changes/align-week5-with-course-requirements/proposal.md` | A | G1 | include；已完成课程的必要规划/规约；保留项目调整与原要求差异 |
| `openspec/changes/align-week5-with-course-requirements/specs/student-streaming-conversations/spec.md` | A | G1 | include；已完成课程的必要规划/规约；保留项目调整与原要求差异 |
| `openspec/changes/align-week5-with-course-requirements/specs/tutor-configuration/spec.md` | A | G1 | include；已完成课程的必要规划/规约；保留项目调整与原要求差异 |
| `openspec/changes/align-week5-with-course-requirements/specs/week5-api-documentation-and-acceptance/spec.md` | A | G1 | include；已完成课程的必要规划/规约；保留项目调整与原要求差异 |
| `openspec/changes/align-week5-with-course-requirements/specs/week5-schema-evolution/spec.md` | A | G1 | include；已完成课程的必要规划/规约；保留项目调整与原要求差异 |
| `openspec/changes/align-week5-with-course-requirements/tasks.md` | A | G1 | include；已完成课程的必要规划/规约；保留项目调整与原要求差异 |
| `scripts/generate_openapi.ps1` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `scripts/openapi-convert/go.mod` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `scripts/openapi-convert/go.sum` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `scripts/openapi-convert/main.go` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `scripts/openapi-convert/main_test.go` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `tests/go_mcp/Dockerfile` | B | G1 | include；Week6 MCP服务/管理/客户端/迁移/测试/文档必需 |
| `tests/go_mcp/README.md` | B | G1 | include；Week6 MCP服务/管理/客户端/迁移/测试/文档必需 |
| `tests/go_mcp/check_phase5_browser.py` | B | G1 | include；Week6 MCP服务/管理/客户端/迁移/测试/文档必需 |
| `tests/go_mcp/check_phase5_recovery.py` | B | G1 | include；Week6 MCP服务/管理/客户端/迁移/测试/文档必需 |
| `tests/go_mcp/compose.phase3.yml` | B | G1 | include；Week6 MCP服务/管理/客户端/迁移/测试/文档必需 |
| `tests/go_mcp/compose.phase4.yml` | B | G1 | include；Week6 MCP服务/管理/客户端/迁移/测试/文档必需 |
| `tests/go_mcp/compose.phase5.yml` | B | G1 | include；Week6 MCP服务/管理/客户端/迁移/测试/文档必需 |
| `tests/go_mcp/compose.phase6.yml` | B | G1 | include；Week6 MCP服务/管理/客户端/迁移/测试/文档必需 |
| `tests/go_mcp/compose.test.yml` | B | G1 | include；Week6 MCP服务/管理/客户端/迁移/测试/文档必需 |
| `tests/go_mcp/model/main.go` | B | G1 | include；Week6 MCP服务/管理/客户端/迁移/测试/文档必需 |
| `tests/go_mcp/run_phase2.py` | B | G1 | include；Week6 MCP服务/管理/客户端/迁移/测试/文档必需 |
| `tests/go_mcp/run_phase3.py` | B | G1 | include；Week6 MCP服务/管理/客户端/迁移/测试/文档必需 |
| `tests/go_mcp/run_phase4.py` | B | G1 | include；Week6 MCP服务/管理/客户端/迁移/测试/文档必需 |
| `tests/go_mcp/run_phase5.py` | B | G1 | include；Week6 MCP服务/管理/客户端/迁移/测试/文档必需 |
| `tests/go_mcp/run_phase5_recovery.py` | B | G1 | include；Week6 MCP服务/管理/客户端/迁移/测试/文档必需 |
| `tests/go_mcp/run_phase6.py` | B | G1 | include；Week6 MCP服务/管理/客户端/迁移/测试/文档必需 |
| `tests/go_rag/Dockerfile` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `tests/go_rag/README.md` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `tests/go_rag/align_real_probe.py` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `tests/go_rag/align_storage_recovery.py` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `tests/go_rag/check_align_browser.py` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `tests/go_rag/check_align_p1p4.py` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `tests/go_rag/check_align_p1p4_ui.py` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `tests/go_rag/check_knowledge_browser.py` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `tests/go_rag/check_real_tutor_browser.py` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `tests/go_rag/check_swagger_browser.py` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `tests/go_rag/check_tutor_browser.py` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `tests/go_rag/compose.yml` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `tests/go_rag/prepare_align_storage.ps1` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `tests/go_rag/prepare_align_stream.ps1` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `tests/go_rag/run.ps1` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `tests/go_rag/run_align_acceptance.ps1` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `tests/go_rag/run_align_browser.ps1` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `tests/go_rag/run_align_p1p4_ui.ps1` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `tests/go_rag/run_align_real.ps1` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `tests/go_rag/run_align_storage.ps1` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `tests/go_rag/run_phase2.ps1` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `tests/go_rag/run_phase3.ps1` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `tests/go_rag/run_phase4.ps1` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `tests/go_rag/sse.test.cjs` | A | G1 | include；Week5/Week4 Go RAG、助教、SSE、初始化、JWT兼容、UI或测试依赖 |
| `tests/test_recovery.py` | E | 排除 | exclude-preserve；仅LF→CRLF，规范化字节与HEAD相同，无Week5/6功能差异 |
| `docs/audits/week5-week6-precommit-20261019.md` | D | G2 | include-summary；本轮只读审计与精确提交规划；未经授权不暂存 |
| `docs/audits/week5-week6-precommit-20261019.json` | D | G2 | include-summary；本轮只读审计与精确提交规划；未经授权不暂存 |

## 7. 安全提交条件与用户决策

- 初始分类计数：{"C": 23, "B": 76, "A": 148, "E": 2, "D": 94}；另有本次2份D类报告。推荐G1=247，G2=46（含待脱敏文件），默认排除=52，合计345个（原343+报告2）。
- 当前全部工作区直接提交：不满足。按精确白名单提交的源码/迁移内容没有发现真实秘密，但完整交付仍需处理选定证据的个人路径、明确bootstrap与Docker上下文防护。审计不替代这些后续修订。
- 需用户审核：①是否接受G1联合Week5/6功能提交；②是否接受G2证据保留范围并另行授权脱敏/引用整理；③是否另行授权补测试bootstrap文档与根.dockerignore防护。没有执行这些修改，也不需要用户重复授权只读审计。
- 原课程差异、名称复用、终态审计与远端执行间崩溃窗口、旧认证500/MCP503差异继续保留。Git提交不会自动关闭这些风险。
- 分支/HEAD/暂存区与原文件保持，禁止事项未执行。完成审计后停止等待审核，不自动提交或归档。

## Spec EOF-only commit recovery (2026-10-19)

The first staged diff check exposed one extra CRLF at EOF in each of the five Week 6 specs. User authorized removing exactly that final CRLF per file. Body bytes, CRLF style, 72 Requirements, 72 primary Scenarios, eight operations and 42 tasks remain unchanged. Historical SHA256 values in baseline/source-audit/coverage records remain historical; current candidate values are recorded in the final plan JSON under spec_eof_commit_recovery and sha256_manifest. The two path allowlists and 249/51/55 scope are unchanged. No whitespace check is disabled.
