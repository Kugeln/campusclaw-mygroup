# Phase 1 验收记录

日期：2026-10-17，Asia/Shanghai。使用change：align-week5-with-course-requirements；schema：spec-driven。本轮授权仅1.1–1.6，Phase2–5不执行。已完整复读proposal/design/tasks、四份spec及Phase0记录。

## 环境与边界

分支`feature/week5-phase1-go-rag`，HEAD `51a8749f6550c279efebe63bb67eda724878f5a1`，保留用户原有未提交/未跟踪业务变更，没有checkout/reset/commit/push/归档/部署。原change的17份文件已记录于phase1-history-baseline.json，最终复核见phase1-check-result.json，原32/32不改写。

初始Docker引擎不可用；暂停数据库动作后先完成1.1。用户选择恢复引擎后，重新只读核对资源及主机路由。Docker默认地址池耗尽的首次网络创建失败，未清理旧网络；核对后使用不冲突的新子网10.254.242.0/24。

新实例`campusclaw-align-storage-17d5f822`，MySQL8.4.4，独立internal网络、仅tmpfs数据目录、无旧卷或目录挂载。Go测试进程只通过该新内网连接新容器；每个场景都CREATE新的`align_w5_<16hex>`数据库，不复用任何既有测试/课堂库。未连接、启动、重建或写入旧MySQL/Qdrant；未执行课堂Compose。所有新测试库保留在该实例，tmpfs不保证停机后持久；备份和日志单独保存在runtime。

隔离证据和测试JSON/日志：`runtime/campusclaw-align-storage-17d5f822/`。本地连接密码仅在已被Git忽略的runtime配置中，报告不包含密码、Token、Key。容器测试使用已有Go1.26.0镜像，主机单元使用Go1.27.1；未安装生成工具或调用真实收费模型。

## 逐项验收

| 任务 | 实现与实际证据 |
|---|---|
| 1.1 通过 | ParseSkillSource纯函数，保留source及正文原始字节；UTF-8、前导注释/空行、首分隔空行、双冒号、可选资料/工具、启用与五项元数据校验。表驱动覆盖8000/8001、32/33、120/121、32/33、2000/2001、CRLF、正文#、缺字段/重复/未知/工具/空正文/非法UTF-8，含补充平面Unicode字符。 |
| 1.2 通过 | v2 DDL保持独立，新增v3 DDL和完整阶段catalog；继承完整v1元数据，加冻结的每步增量重建每一个完整catalog，不是只验新增列。v3保留LONGTEXT名称、32字前缀班内唯一、scope复合索引/FK、两关联表、独立升级账本、ngram验证。初始/部分升级不ready；HTTP入口无DDL。 |
| 1.3 通过 | typed脱敏冲突报告、只读rag-preflight、合法旧source合成及来源/独立版本hash、一事务回填关系与完成标记。空材料不扩绑，NULL-class原样保留，旧JSON退出真相源；重复升级不覆盖教师编辑/解绑/删除。 |
| 1.4 通过 | source技能CRUD、enabled-only事务更新source与字段/revision、当前复合绑定仓储、repeatable-read原子快照；重复名称/越权拒绝，真实MySQL关系插入拒绝整体回滚。并发助手绑定/角色与技能source编辑均无混合快照，删除技能清关系且不改旧消息审计。 |
| 1.5 通过 | Store.Delete内部方法，owner+class锁定、Busy、先消息后会话同事务清理；越权/不存在统一NotFound。中途注入及真实MySQL父行删除拒绝都整体回滚，不影响其他会话；无公开DELETE路由。 |
| 1.6 通过 | 实际mysqldump备份、独立新库恢复、两份合成文件hash核对、受控DDL中断及前向续跑、旧catalog拒绝v3、新增记录损失边界均有实证。正式迁移预检模板见phase1-migration-preflight.md；没有升级课堂库。 |

## 数据库实际验证矩阵

| 场景 | 结果与边界 |
|---|---|
| 新库→v1→v2→v3 | 通过；先显式PrepareSchema再离线PrepareRAGSchema，完整v3就绪 |
| 已有v1→v2→v3 | 通过；无班级旧占位ID/长名称不变，不补猜归属或source |
| 已有v2→v3 | 通过；旧用户、Token版本、材料/切片、技能/助手/会话/消息ID及历史正文/引用/审计hash一致 |
| 重复升级 | 通过；catalog不变、无重复种子/关系，旧JSON不复活解绑，source编辑和删除保持 |
| DDL中断 | 41/41提交后中断点通过；模拟升级器在DDL成功后、ledger写入前退出，非主机断电；每例部分状态拒绝ready，重试收敛 |
| 回填中断/SQL失败 | 通过；首个source写入后注入失败，以及材料关系真实SQL拒绝，source/已插技能关系/完成标记全部回滚；重试可完成 |
| catalog漂移 | 通过；列类型、必需索引命名、复合FK删除规则、版本/步骤/完成标记异常拒绝ready；结构漂移升级拒绝且不修复 |
| 旧数据冲突 | 10类均在v3 DDL前阻止：未知技能、超限名、重名、错误JSON类型、外班/悬挂技能、重复绑定、未知助手归属、外班材料、活动turn；旧结构和数据hash不变 |
| 事务与隔离 | 通过；同班/跨班同名、学生写Forbidden、外班NotFound；完整保存越权资源无部分绑定/revision变化；直接DB拒绝关系写入、父会话删除均全回滚 |
| 备份恢复 | 通过；真实v2 dump恢复至独立新库，完整旧catalog与旧字段hash及两个文件hash一致；旧v2 catalog拒绝升级后的源库 |
| 恢复损失 | 通过；升级后新增class记录存在于v3源库、不在旧备份恢复库。它证明实际损失边界，不能声称回退保留升级后的新写入 |

旧字段hash核对覆盖users.session_version、sessions、材料/知识条目/切片、技能原name/key/enabled/instruction_version/revision/audit、助手原配置JSON、完整会话/消息。最终夹具额外填入非空selected_skill_id/skill_id、prompt版本、历史skill_hash及evidence_snapshot，再复验CRUD/删除/备份保持；具体hash与独立库名称见最终日志和机器汇总，不用先前空审计夹具的结果替代。

## 实现衔接与未实施范围

技能原文字段、source_origin/source_version/source_hash与旧instruction_version独立，历史审计不重写。旧v2 schema_rag_catalog.json作为冻结兼容证据保留；schema_upgrade_steps.json以v1完整catalog和41步已批准DDL哈希/元数据增量定义所有完整阶段。MySQL添加覆盖索引时会自动替换部分隐式FK支持索引，已将这些精确变化纳入catalog，不是忽略差异。首次开发捕获因只允许增加元数据的假设失败，该失败日志保留；修正后从另一个全新空库捕获成功，再在其他新库验证。

v3初始化账本与v1初始化收据分离。bootstrap不再自动ProvisionSkills，以免重跑初始化复活教师已删除技能；显式admin tutor-skills仍可按操作者意图播种，迁移本身不调用。部分升级使用显式admin rag-schema续跑，bootstrap/HTTP的验证失败不隐式修复。

SaveCurrentAgent为支持多绑定的存储入口；现有HTTP使用的SaveAgent仍保留旧验证，空prompt、确定性路由、停用续聊、六消息窗口、无据生成、meta前移、UI及注解工具链均留给Phase2–4，未提前实现。Snapshot携带当前候选及原子关系，只保留旧单技能编排衔接，不以它宣称新词法路由已完成。

## 命令与结果

- `go test ./internal/tutor ./internal/auth ./internal/http ./internal/rag ./internal/conversation ./internal/repository/mysql ./cmd/server ./cmd/admin ./cmd/bootstrap -count=1`：通过；两个入口包无测试，其余包均执行。日志unit-final.log。
- `node --test tests/go_rag/sse.test.cjs`：3/3通过，UTF-8/CRLF/心跳、多行data、真实增量时序、error/EOF保护。
- `powershell -NoProfile -File tests/go_rag/run_align_storage.ps1 -Stage All`：最终共用ledger/preflight逻辑回归通过，108.315秒；41中断点及Schema/Legacy/Store/Delete均通过。之后仅增强旧消息审计夹具，针对受影响的Legacy/Store/Delete及恢复脚本重新执行，详见最终机器记录。
- `python tests/go_rag/align_storage_recovery.py`：实际备份/恢复通过；每次创建独立运行目录，前次证据不覆盖。
- strict validate、最终任务数量、受保护文件hash与修改文件清单：见phase1-check-result.json。

原Bearer、角色、RAG纯核心/客户端、Week4 Ask无据固定文案与零对话调用、SSE解析均通过适用单元保护；未重跑需要旧Qdrant的真实检索集成，不把本轮MySQL/受控测试称为全业务或真实模型教学验收。正式库数据冲突仍须独立预检，收费/同伴/部署授权未发生。Phase1结束后立即停止，不进入Phase2。

## 修改文件与新增测试清单

- 技能核心：新增`internal/tutor/skill_source.go`、`legacy_skill.go`；修改`tutor.go`扩展持久字段与快照候选，不改路由/生成算法。
- 数据结构与升级：新增`internal/repository/mysql/schema_tutor_v3.sql`、`schema_upgrade_steps.json`、`tutor_schema.go`、`tutor_migration.go`；修改`rag_schema.go`、`catalog.go`。
- 仓储与删除：修改`internal/repository/mysql/tutor.go`、`internal/conversation/types.go`；新增`internal/repository/mysql/conversation_delete.go`。
- 离线入口：修改`cmd/admin/main.go`（只读预检/冲突报告/v3就绪版本）、`cmd/bootstrap/main.go`（不自动复活已删种子）。
- 新增纯核心测试：`internal/tutor/skill_source_test.go`，三组表驱动测试RoundTrip/Limits/RejectsMalformed。
- 新增真实MySQL测试：`internal/repository/mysql/tutor_schema_integration_test.go`、`tutor_migration_integration_test.go`、`tutor_store_integration_test.go`、`conversation_delete_integration_test.go`、`tutor_recovery_integration_test.go`；统一`alignstorage`标签，普通测试不隐式连接数据库。
- 新增隔离执行器：`tests/go_rag/prepare_align_storage.ps1`、`run_align_storage.ps1`、`align_storage_recovery.py`。
- 本change规划/证据：更新proposal/design/tasks；新增本记录、phase1-migration-preflight.md、phase1-history-baseline.json、phase1-check-result.json。Phase0记录和四份spec未改，原change所有文件未改。

本轮任务1.1–1.6均验收后勾选，新进度9/29；没有勾选Phase2–5任务。
