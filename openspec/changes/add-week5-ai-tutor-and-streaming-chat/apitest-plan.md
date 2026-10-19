# 接口测试目录与验收计划（仅规划）

本轮不创建 apitest/ 或任何脚本；以下路径均为实施阶段交付物。一个 URL+HTTP方法对应一个脚本，多个角色、异常和数据库场景放在同一脚本。动态路径用字面 `{id}` 目录，脚本运行时注入真实测试 ID。

| 规划脚本路径 | 必须核对的响应与持久状态 |
|---|---|
| apitest/cases/api/auth/login/POST.py | A/B教师学生登录成功、密码错误/不存在统一401；access_token/token_type/expires_in/redirect；没有Cookie，users/session_version不被登录重置 |
| apitest/cases/api/auth/me/GET.py | 正确当前身份；缺/错/过期/撤销Bearer和Cookie-only401；客户端伪造class/role不能授权；未认证不写库 |
| apitest/cases/api/auth/logout/POST.py | 204空体、两设备Token失效、新登录成功；DB session_version递增并持久，写入失败不报成功 |
| apitest/cases/api/materials/GET.py | 本班items、外班无标题/计数、非法scope400；测试库材料集合对照 |
| apitest/cases/api/materials/POST.py | 教师201、学生403、未认证401、非法/超限400/413；原文/文件同提交，索引失败保留原文，拒绝请求无行/文件新增 |
| apitest/cases/api/materials/{id}/GET.py | 本班plain_text正确，外班/不存在404；与数据库及文件hash核对 |
| apitest/cases/api/materials/{id}/file/GET.py | Bearer附件字节一致、外班404、无静态绕过；不得改变材料 |
| apitest/cases/api/materials/{id}/reindex/POST.py | 教师成功、学生403、外班404、非法策略400、处理中409；pending/ready/failed、清理确认、旧切片与文件状态 |
| apitest/cases/api/search/GET.py | keyword/vector/hybrid、mode/items形状、班级ready过滤；伪造外班命中丢弃、RRF/阈值、keyword零向量调用；外班独有200空items；故障503不降级，读操作无新增 |
| apitest/cases/api/ask/POST.py | question/mode、answer/sources、top4引用对应数据库；无命中精确固定文本且零LLM；非法引用502；单次问答不增加会话消息 |
| apitest/cases/api/agents/GET.py | 教师本班全部、学生仅enabled；agents字段，外班不可见；无写入 |
| apitest/cases/api/agents/POST.py | teacher201、student403、匿名401、长度/重复400、外班关联404；助手class、prompt、revision、skill/material绑定持久 |
| apitest/cases/api/agents/{id}/GET.py | 本班配置、学生仅可用；外班/禁用/不存在404；无状态改变 |
| apitest/cases/api/agents/{id}/PUT.py | 教师更新200、学生403、跨班404；revision增加、下一轮生效、进行中和历史快照不改，失败无DB变动 |
| apitest/cases/api/skills/GET.py | skills内置名称/说明/状态/版本、班级隔离，无新增或脚本执行 |
| apitest/cases/api/skills/{id}/PUT.py | 启用/关闭200、学生403、外班404、非boolean/source400；DB版本/状态正确，拒绝无变动；同题教学对照与下一轮生效 |
| apitest/cases/api/conversations/GET.py | 仅本人+班级、空列表、teacher403、匿名401；数据库owner集合对照 |
| apitest/cases/api/conversations/POST.py | agent_id→201字段齐全；无可用助手400、外班/禁用404、teacher403；owner/class来自principal，失败无新会话 |
| apitest/cases/api/conversations/{id}/GET.py | conversation/messages字段、顺序、内容、状态、引用、技能；A1对A2/B1均404，转班404，教师403；重启后不丢消息 |
| apitest/cases/api/conversations/{id}/messages/POST.py | Bearer POST SSE；正常≥3段且首段早于完成、meta/delta/done顺序及最终内容；历史/教师prompt/skill/本轮证据同一次模型输入；失败、断连、崩溃恢复、数据库故障、并发409；消息对、前缀、终态、版本/技能/引用DB断言；无命中固定文本、空引用、零LLM/完整技能加载 |
| apitest/cases/openapi.json/GET.py | OpenAPI版本、paths/method/schema/security/error/SSE示例与实际接口一致；匿名200、无秘密 |
| apitest/cases/swagger/index.html/GET.py | 匿名200及资源和JSON路径正确；浏览器Authorize/主要接口/SSE调试验收证据；不只断言HTML标题 |
| apitest/cases/swagger/GET.py | 页面别名200，与index使用同文档；不能绕过受保护API |

共 23 个接口脚本规划，不创建官方未采用的 POST search、技能CRUD或登录别名测试。若实施时增加接口，必须同样增加URL/方法脚本并更新清单，不能一个大通用脚本替代各接口交付。

## run_all.py 与隔离环境

`python apitest/run_all.py` 从仓库根一键执行。先校验 APITEST_BASE_URL、A/B 教师与至少 A1/A2/B1 凭据、APITEST_DB_*、网关控制地址与测试命名空间；全部来自本地环境或被忽略.env，公开README只列变量。提供测试环境专用初始化fixture/helper，按角色登录→材料→索引→配置→会话建立依赖，各脚本仍可独立运行并获取自己的fixture；runner汇总每脚本场景，不通过依赖链缓存掩盖失败。

MySQL/Qdrant用独立测试库/collection、附件目录，禁止用现有课堂数据；实际网关受控返回可延迟SSE与非法引用/故障，不开放业务测试后门。DB断言用只读测试连接，建样例和故障仅限隔离fixture。SQLite身份不进入Go验收。必要条件缺失必须非零退出并在报告列“阻塞/未执行”，不能全部skip后全绿。

HTML 输出 `/reports/apitest/<UTC时间>-<随机run-id>/index.html`，已有 `/reports/` Git忽略；保存用例名、接口、耗时、断言、失败原因和首片/终片相对时间。配套脱敏机器结果可放同目录。不把 Authorization、密码、API Key、原始网关请求完整转储进报告。异常不能自动上传或提交报告。验收用 git check-ignore 验证报告被忽略，git状态无真实配置。

## 必测流式/DB场景

1. 正常：有本班证据且技能启用；网关每200ms发送片段。第一delta到达时最终片尚未发送；逐帧前缀已存，最终completed后done，citations等于历史。客户端模拟UTF-8拆字、事件跨分片及多行。
2. 流前失败：无效身份/所有权、空白/超长、检索不可用、上游连接失败或初始消息事务失败；预期JSON错误、无模型越权调用、无孤立user或assistant消息。
3. 流中失败：至少首delta后模型超时/越界引用/空有效回答；error无done，failed前缀与实际技能留库，引用不得伪造。
4. 断连：首delta后关闭连接；上游取消，interrupted且已有前缀留存，刷新历史可读；完成已提交而done丢失则历史completed。
5. 崩溃与写入故障：isolated app中断/故障恢复，遗留generating→interrupted，completed不变；已收到片段都有提交证据，未成功终态不能done。
6. 并发：同会话generating期间第二次POST409，不产生第二对消息；完成后同内容重发是新turn，不承诺幂等重放。
7. 配置更新：第一轮生成中教师更新prompt/技能；第一轮快照保持，下一轮使用新版本，刷新旧历史不改变。
8. 无命中：普通无数据及A/B外班独有查询、历史曾有证据但当前无命中；完整固定文本、sources/citations空、actual skill空、LLM次数零。

技能流程需补充受控时序断言：先读取教师配置并确定selected_skill，再执行本班检索，取得合法证据且准备生成后才读取完整指令。命中轮实际技能与模型输入一致；无命中轮即使selected_skill非空，实际skill_name为空、完整加载和LLM调用均零。仅检查最终回答或enabled状态不足以证明顺序正确。

## 第 4 课回归与教学效果

复用 `tests/token_auth`、Go HTTP/auth/material/repository测试矩阵和 `tests/retrieval` 行为案例；Python真实服务测试不能被直接宣称为Go通过，需Go核心/HTTP/隔离MySQL+Qdrant对应证据。`go test ./...`不含build-tag集成，报告必须区分。

Phase 1独立前置验收在Phase 2实施之前完成：报告单列默认Go入口、各前置任务结果、真实隔离MySQL/Qdrant及网关证据。缺环境、只有Flask历史结果或只通过OpenSpec验证时Phase 1保持未完成，Phase 2不得开始；Phase 6回归不能事后替代该门槛。

接口测试以本地采用契约断言，同时在报告标注api-differences.md中的官方兼容风险。23个规划脚本不覆盖尚未采用的/api/login、POST search或source CRUD，因此不能称为“全量官方契约验收通过”；官方SSE核心字段兼容与本地扩展字段的严格比对风险也需分开记录。

全覆盖A/B teacher/student三模式、伪造scope、0.34/0.35阈值、ready回表、RRF k=60、keyword在向量故障时可用、上传原文先提交/索引失败、非法重建不删旧索引、单次ask引用校验、Token撤销与重启。真实模型至少选一份确定材料和同一道有最终答案的题，启用时人工确认不给最终答案且有思路/步骤/追问，关闭时允许直接结论；报告分别记录受控自动断言与真实模型教学复验，缺Key/外网时不冒称后者完成。
