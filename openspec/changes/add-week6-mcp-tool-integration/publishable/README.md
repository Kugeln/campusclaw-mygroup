# 可提交证据与本地原始记录

这里三个 JSON 是原始证据的脱敏副本。原 JSON 不改写、不提交；副本中的
`publication_note.original_sha256` 标识其本地原件。只替换本机仓库根目录或
个人附件位置，时间、退出码、测试输出、原任务、批准范围和未执行项不变。

| 历史记录中使用的本 change 相对路径 | Git 中可阅读的副本 |
|---|---|
| `phase5-static-checks.json` | [Phase 5 静态检查](phase5-static-checks.json) |
| `phase6-static-checks.json` | [Phase 6 静态检查](phase6-static-checks.json) |
| `phase6-user-decisions.json` | [用户验收调整](phase6-user-decisions.json) |

历史报告和覆盖表的原证据定位保留原意；阅读上述三个名字时按本表查阅副本。
其余未纳入提交清单的 preservation/baseline、Docker 原始清单、过程日志、
runtime/、reports/、截图和 SQL 备份是本地留存证据，不保证在 Git 中存在，
不属于应用或测试源码依赖。测试重新执行时生成新的隔离资源和运行证据，
不得以缺少历史证据为由连接课堂数据库或伪造原运行记录。

Week 6 的 42/42 仍是用户批准的项目调整标准：6.5 使用受控 HTTP 模型、真实
MCP 和自动技术复核；6.7 使用独立账号自动模拟交叉验证。原真实模型调用、
用户人工审阅和真实同伴参与未执行，本次脱敏不改变这些事实。
