## Why

2026-10-16 的官方课件复核确认，已完成的 Week 5 本地规约与课程在技能文件/路由、材料空绑定、无依据生成和 SSE 首帧等方面存在行为差异。原 `add-week5-ai-tutor-and-streaming-chat` 的 32/32 是其既定范围的历史完成记录；本变更以独立增量对齐课程，保留该记录和已验证的运行能力。

## What Changes

- **BREAKING（仅 Week5）**：F03 空材料绑定从全班改为无材料；F01/F02 从内置开关改为教师文本技能 CRUD、多绑定、确定性评分及学科过滤；F07 零命中仍生成；F10 meta 恒为 ok 且先带本轮证据。旧 Week5 客户端/测试须显式适配。
- F04 允许空提示词并回退默认角色；F05 停用只阻止新会话；F06 上下文末尾6条含本轮问题、每条500字；F08 首问前24字标题；F11 模型资料每条800字。
- F09 仅补会话删除时的消息清理语义与存储测试，不新增公开 DELETE API。F12 处理器注解生成 OpenAPI，保留现有 Swagger/SSE 调试页面。
- F13 保留官方八项DoD与原始四项真实同伴要求的来源记录；按用户明确选择，本项目5.4改为独立教师/学生账号自动化交叉验证P1～P4，技术断言、浏览器/数据库证据及失败阻止条件不降低。自动化、受控网关、真实模型、用户审核与历史结果分开，不伪造同伴参与。
- 显式增量 schema 升级、旧助手/技能/绑定兼容、完整 catalog 校验、幂等/中断恢复及回滚风险说明；不重建数据库、材料、Qdrant collection 或消息历史。
- 保留 Go/MySQL/Qdrant、Bearer、owner+class 隔离、共享三模式检索、Week4 `/api/ask` 零命中固定文案及零对话调用，复用 SSE 解析、逐段先落库再推送、done/error、取消和中断恢复。

## Capabilities

### New Capabilities

主 `openspec/specs` 当前为空，以下以 ADDED 完整定义后续有效要求，沿用旧 change 的 capability 路径而不虚构已归档主规格。与旧 delta 冲突的条款及其替代关系见 design；旧文件不改写。

- `tutor-configuration`: 教师技能文件与 CRUD、当前绑定、词法路由、空角色回退、材料范围。
- `student-streaming-conversations`: 会话生命周期、历史窗口、无依据生成、meta 来源与持久化协议。
- `week5-api-documentation-and-acceptance`: 注解文档、官方DoD、回归矩阵及本项目独立账号交叉验证；明确与官方真实同伴方式的差异。
- `week5-schema-evolution`: 显式增量升级、旧数据映射、catalog 完整性与安全恢复。

### Modified Capabilities

无已有主规格可作 MODIFIED；不修改其他 change。将来归档/同步时必须按本变更替代关系解决冲突，不能让旧 delta 再次覆盖新行为。

## Impact

后续预计涉及 `internal/tutor/`、`internal/conversation/`、HTTP handlers/教师页面、MySQL tutor/conversation/schema/catalog 与 bootstrap 接线、注解文档生成、`apitest/` 和隔离验收脚本。数据库升级需独立维护窗口；旧 v2 严格 catalog 二进制不能直接用于升级后的库。生成工具属于构建依赖，不增加业务框架或线上服务。

按用户本轮明确授权的本项目验收方案修订，5.4采用独立账号自动化验证；现有28项全部通过，原始请求/响应、真实浏览器及数据库证据已只读复核。5.3真实模型及用户审核、5.5最终报告与风险核对均齐备，本change按调整后的项目标准完成29/29，见[最终报告](acceptance-final-report.md)及[本轮复核](project-acceptance-adjustment-check.json)。官方原始真实同伴要求仍未执行；29/29不代表原始官方要求全部满足，也不代表已正式迁移或部署。真实账本保持对话11/12、Embedding8/8、7978 tokens，本轮无新增调用。

官方依据：已在本会话成功读取的[课程入口](https://devops.hello1023.com/课件/第5课-课件-提示词AI技能与流式会话/index.html)、本课重点、会话模型、技能模型、路由、SSE、回答链路、实践与思考章节；逐项来源与强制/可选边界见 design.md。


### 历史时点：5.3授权及首次预检（2026-10-18，以下不是当前状态）

User authorized the course gateway only, at most 12 dialogue and 8 Embedding calls, serial and without retries. The authenticated model-list GET returned 200, but configured course-chat/course-embedding aliases were not listed and their mapping/format/dimension could not be confirmed. Stopped before any generation or database/Qdrant operations; actual dialogue=0, Embedding=0. See [5.3 verification](phase5-real-model-verification.md). This authorization supersedes earlier pending-authorization status; 5.3 remains unchecked pending configuration and user review. 5.4/5.5 remain unchecked; total 26/29.

最新5.3执行状态：用户已核对官方PDF配置并恢复限额真实调用。最小验证HTTP200/2048维；随后首轮有据回答未包含合法数字引用，正确error/failed并停止。累计对话2/12、Embedding3/8，无重试。完整回答及技术失败见5.3专项记录；仍26/29，5.3待处理及人工审读，不进入5.4/5.5。
