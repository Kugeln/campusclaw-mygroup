# Week 5 + Week 6 最终可执行提交方案

**READY TO COMMIT**（仅限下面两个精确白名单；本轮未暂存、提交、推送或归档）。

## 阻塞处理与事实保护

- 三个原始JSON逐字节保留，不直接改写历史日志或用户批准记录。新增publishable/下三个脱敏副本及README；各副本记录原路径与SHA256，仅替换本机根目录或个人附件位置。G2改收副本，三个原件移入本地排除范围。phase6-user-decisions.md的机器记录链接指向副本，历史覆盖表中的原名由publication README映射。
- tests/IMAGE_PREPARATION.md写明固定基础镜像、从源码构建测试基底、Go代理/APK依赖、隔离准备前提、离线导出校验与构建缓存限制；两个测试README接入说明。没有改动任何Go、SQL、前端、生产Compose或Dockerfile实现。
- 根.dockerignore增加本地证据、秘密、缓存、数据库导出等规则，保留.env.example、Go源码、前端/Swagger嵌入、SQL/catalog、配置及MCP源码。Docker实际导出filtered context成功，所有internal Go/SQL/JSON/前端字节与源文件一致，23组embed输入完整。

## 构建与必要检查

- 镜像构建六项成功：backend server、initializer、MCP server、MCP probe、从当前go.mod/go.sum构建的Week5测试基底、基于该新基底的MCP测试镜像。后者只在runtime临时Dockerfile副本替换FROM为本轮独立标签，其余指令与已验收Dockerfile相同。
- 首次默认proxy.golang.org下载被拒，失败记录保留；仅一次受限重试使用已验证goproxy.cn，最终全通过。不以失败证明源码错误，也不声称离线构建成功。实际命令、日志定位与结果摘要见同名JSON的build_verification/historical_failed_builds。
- 213个既有容器Status/StartedAt/FinishedAt不变；既有镜像标签、网络、卷不变；未启动新容器，未创建网络/卷。仅保留本轮独立验证镜像和忽略的runtime日志/上下文导出，未prune。
- git diff --check、三份OpenSpec strict、固定生成链OpenAPI Check均退出0。最终白名单未发现已知真实凭据、JWT/私钥/provider密钥特征或本机绝对路径。启发式扫描不等同于所有秘密格式不存在的证明。
- 原Week6 72 Requirement、72主Scenario、8管理操作、42/42不变。6.5受控模型/自动技术复核、6.7模拟交叉4/4仍保留；原真实模型、用户人工审阅、真实同伴参与未执行，不代表老师批准替代。

## 哈希与文件范围

- 已有原始历史JSON、原52个排除候选全部保持字节不变。源码/SQL/前端/生产配置没有变化；现有文件变化仅是根.dockerignore、两个测试README、用户决定说明的发布链接、两份审计记录更新。详细before/after SHA256见同名JSON。
- 原功能247：增加.dockerignore和tests/IMAGE_PREPARATION.md，变成249。
- 原文档46：移出3原件、加入3副本，净0；增加publishable/README.md、最终方案MD/JSON、两个精确pathspec文件共5，变成51。
- 原默认排除52全部仍排除，另加3历史原件，共55。两份旧审计报告适合保留：不含秘密原文，新增当前方案指针，247/46/52历史记录未冒充现状。
- 两组互斥，无默认排除自动晋升；当前候选总355=249+51+55。预计已跟踪修改20、未跟踪335、暂存0。

## 两个完整路径清单

机器清单中groups[].files与下列UTF-8无BOM文件逐行一致，文件名含花括号也按字面处理：

- [提交1：249个文件](week5-week6-commit1.paths)
- [提交2：51个文件](week5-week6-commit2.paths)
- [最终机器清单及55个排除路径](week5-week6-commit-plan-20261019.json)

### 提交1

`feat(campusclaw): deliver Week 5 tutor and Week 6 MCP integration`

### 提交2

`docs(acceptance): record Week 5/6 verification and project exceptions`

联合提交保留跨周类型/迁移/初始化依赖，不强拆不完整Week5中间状态。G2依赖G1及既有tracked文件；历史过程证据、runtime/report路径是本地定位，不是源码依赖或保证已发布的链接。原名与三个公开副本的对应关系见publishable/README.md。

## 下一轮获授权后才执行（本轮没有执行）

先复核branch/HEAD仍与JSON一致、暂存区为空，并按sha256_manifest确认原文件未变化。若有新修改停止重新审计，不扩大白名单。然后在仓库根目录使用：

```powershell
git --literal-pathspecs add --pathspec-from-file=docs/audits/week5-week6-commit1.paths
git diff --cached --check
git diff --cached --name-only
# 对照最终JSON确认仅G1的249个路径，再执行：
git commit -m "feat(campusclaw): deliver Week 5 tutor and Week 6 MCP integration"
git --literal-pathspecs add --pathspec-from-file=docs/audits/week5-week6-commit2.paths
git diff --cached --check
git diff --cached --name-only
# 对照最终JSON确认仅G2的51个路径，再执行：
git commit -m "docs(acceptance): record Week 5/6 verification and project exceptions"
```

不执行git add .；不把残留未提交文件误作提交失败；本方案不授权push、正式部署/迁移或archive。仅后续Git操作仍需明确授权，没有发现新的技术阻塞。既有课程参与方式差异、名称复用和审计崩溃窗口风险继续保留。

## ????/???????

??????????????????????????

???6???

- `.dockerignore`
- `docs/audits/week5-week6-precommit-20261019.json`
- `docs/audits/week5-week6-precommit-20261019.md`
- `openspec/changes/add-week6-mcp-tool-integration/phase6-user-decisions.md`
- `tests/go_mcp/README.md`
- `tests/go_rag/README.md`

???9???

- `docs/audits/week5-week6-commit-plan-20261019.json`
- `docs/audits/week5-week6-commit-plan-20261019.md`
- `docs/audits/week5-week6-commit1.paths`
- `docs/audits/week5-week6-commit2.paths`
- `openspec/changes/add-week6-mcp-tool-integration/publishable/README.md`
- `openspec/changes/add-week6-mcp-tool-integration/publishable/phase5-static-checks.json`
- `openspec/changes/add-week6-mcp-tool-integration/publishable/phase6-static-checks.json`
- `openspec/changes/add-week6-mcp-tool-integration/publishable/phase6-user-decisions.json`
- `tests/IMAGE_PREPARATION.md`

## Spec EOF-only commit recovery (2026-10-19)

The first staged diff check exposed one extra CRLF at EOF in each of the five Week 6 specs. User authorized removing exactly that final CRLF per file. Body bytes, CRLF style, 72 Requirements, 72 primary Scenarios, eight operations and 42 tasks remain unchanged. Historical SHA256 values in baseline/source-audit/coverage records remain historical; current candidate values are recorded in the final plan JSON under spec_eof_commit_recovery and sha256_manifest. The two path allowlists and 249/51/55 scope are unchanged. No whitespace check is disabled.
