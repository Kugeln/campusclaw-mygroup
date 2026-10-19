# Current acceptance: 5.3 USER-reviewed and completed

The USER explicitly approved the prior evidence/no-skill answer and all eight remaining real-model answers, including mathematics, citations, skill differences, no-evidence generation and multi-turn context. Progress is now 27/29. Tasks 5.4 and 5.5 remain incomplete. Teaching-guidance experience has minor non-blocking room for improvement; no business change is made. See [human review confirmation](phase5-human-review-confirmation.md). No new model calls: cumulative dialogue 11/12, Embedding 8/8, original ledger locked and unchanged.

The following execution-time record is retained as historical evidence; its pending-review status has been superseded by the USER confirmation above.

# 5.3 剩余真实模型教学复验（待用户人工审读）

日期：2026-10-18。进度保持26/29，5.3、5.4、5.5均未勾选。此前有证据、无技能场景已获用户明确人工审核通过，作为5.3内部已通过场景记录于 `runtime/align-real-26bcce67/citation-retest-b8478118/user-review.json`。本轮八份新回答均待用户审读；执行过程中由代理检查停止条件，不代替用户教学质量审核。

## 环境、命令与预算

- 网关：`https://ai-gateway.devops.hello1023.com/v1`；`course-chat`、`course-embedding`，OpenAI格式、2048维。使用现有安全配置，未输出凭据或修改生产配置。
- 复用隔离MySQL `align_w5_9597acd9f56c179d`、隔离Qdrant collection `align_real_18df805af4916d91`及专用账号、材料、技能。使用已有讲义向量，四次Embedding仅用于查询；未操作课堂资源。
- 命令：`powershell -NoProfile -File tests/go_rag/run_align_real.ps1 -Evidence runtime/align-real-26bcce67 -RemainingTeaching`。仅运行 `TestAlignPhase5RemainingReal`，PASS，252.517秒。测试进程、资源及证据保留，未自动清理。
- 起始累计对话3/12、Embedding4/8；新增对话8、Embedding4；最终累计对话11/12、Embedding8/8。原账本前七项保持不变，无自动重试，结束后锁定账本。剩余一次对话未使用，不再调用。
- 新增12次HTTP请求均200。网关报告本轮6504 total tokens（prompt3484、completion3020，含Embedding），历史累计报告7833 total tokens；未提供实际货币费用，不据此推算。

## 实际结果

| 场景 | 实际技能 | 候选/最终引用 | delta数 | 非system上下文条数 | 技术结果 |
|---|---|---|---:|---:|---|
| 有证据、技能启用命中 | 课程解题引导 | [1]/[1] | 59 | 1 | completed |
| 空绑定、无技能 | 无 | []/[] | 97 | 1 | completed |
| 空绑定、有技能 | 课程解题引导 | []/[] | 22 | 3 | completed |
| 有证据、技能停用 | 无 | [1]/[1] | 19 | 1 | completed |
| 有证据、trigger未命中 | 无 | [1]/[1] | 84 | 1 | completed |
| 有证据、技能解绑 | 无 | [1]/[1] | 76 | 1 | completed |
| 同一会话第三轮 | 无 | []/[] | 54 | 5 | completed |
| 同一会话第四轮 | 课程解题引导 | []/[] | 18 | 6 | completed |

启用、停用及解绑场景使用同一原问题和讲义。启用命中时原回答询问“已经尝试解这个方程到哪一步”，并主动引用[1]，未输出x=4；停用和解绑时原回答直接给出x=4并引用[1]。未命中问题不含trigger“方程/解题”，实际未选技能，回答给出完整步骤及[1]。空绑定四轮没有Embedding调用，原始回答未出现伪造数字引用；一般知识回答明确说明无本轮材料依据。以上是可观察差异，是否充分符合教学效果仍由用户审核。

全部八轮实际观察 `meta → 多个delta → done`，无error；服务端记录meta Flush早于模型Open，首delta早于done。每片delta发送前已核查数据库前缀；原始模型文本、delta拼接、最终数据库正文逐字相等，全部消息completed。四轮有据回答均由模型主动生成[1]，最终来源对应既有讲义chunk，未补写引用或修改校验器。

四轮历史复用同一会话，实际非system上下文为1、3、5、6条，每轮当前问题仅出现一次；第三轮正确复述“蓝鲸”。第四轮窗口丢弃最早user消息，保留最近六条有效消息，完整实际system/user输入见result.json。此轮未额外注入伪造引用或模型故障，不能把成功流验证称为新的故障恢复测试；相关受控回归保留此前结果。

## 完整证据与修改范围

- 全部新问题、完整原始回答、绑定、实际技能、候选及最终来源、调用用量：[人工审读文件](../../../runtime/align-real-26bcce67/remaining-real-01a6ec28/answers-for-review.md)。
- 完整请求消息、SSE事件、数据库消息及配置快照：`runtime/align-real-26bcce67/remaining-real-01a6ec28/result.json`；每场景另有同名 `.sse`；汇总断言 `verification.json`。
- 原始网关响应：`runtime/align-real-26bcce67/upstream-08.txt`至`upstream-19.txt`；与请求、时间及Token用量对应关系见原累计 `budget.json`。本轮开始快照在 `remaining-real-01a6ec28/budget-before.json`。
- 新增受预算约束的隔离真实测试 `internal/repository/mysql/phase5_remaining_real_integration_test.go`；扩展 `tests/go_rag/run_align_real.ps1` 的显式剩余场景模式与预算守卫。未修改业务提示词、引用校验器、Week4或SSE实现。
- 严格校验及范围/hash检查见同目录 `strict.log` 和本change `phase5-remaining-real-check.json`。

## 未完成条件

八份新回答的人工教学内容审核未完成，5.3保持未勾选。用户需重点审读技能启用时是否充分遵守教师要求、无据回答是否适度说明依据、历史引导是否一致。本轮未发现触发立即停止的客观异常，但这不保证其他问题或后续真实回答总能遵守引用和教学约束。5.4真实同伴验证、5.5最终验收完成均未执行；未commit、push、归档或部署。
