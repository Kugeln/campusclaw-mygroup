# Phase 3 实施与最终验收（2026-10-18，Asia/Shanghai）

结论：3.1–3.6 六项完成，累计 **23/42**；Phase 4–6 的 19 项保持未完成。已停止，等待下一次明确授权。复用现有 change、Phase 2 仓储/加密/客户端和第五课身份机制；本轮未新增技能工具解析、模型调用、SSE 编排或页面功能。

完整基线：[phase3-preservation-baseline.json](phase3-preservation-baseline.json)；逐文件 SHA256、运行记录和统计：[phase3-verification.json](phase3-verification.json)。

## 逐任务证据

| 任务 | 实现与验收证据 | 状态 |
|---|---|---|
| 3.1 / T15,T17,T18,T24 | MCPHandler+原 MCP repository；登记/同班重名400/跨班同名；安全视图、改名空Token保留并重新连接、更新重名回滚；删除204/重删404/缓存级联/审计503且缓存审计保持 | 完成 |
| 3.2 / T19,T20 | Connect 经 Phase 2 client 执行 initialize→tools/list，网络在事务外；实际 Phase 1 服务两工具。鉴权、停服、协议、Schema、3秒超时502；发布触发器故障503；重复及8路并发disconnect200 | 完成 |
| 3.3 / T21–T23 | 重连完整替换；2→1工具移除旧项，GET不联网；update/disconnect/delete/新connect使晚到租约无效；无幽灵缓存或复活 | 完成 |
| 3.4 / T16,T25 | 八路 Authenticate→TeacherRequired；真实MySQL身份回查；全部匿名401、A/B学生403、外班与不存在404相同；同班另一教师可管；class_id覆盖400、拒绝网络/写副作用为零；撤销/当前班级变更即时生效；审计50/100/101夹限、非法400 | 完成 |
| 3.5 | 八路 handler 注解/DTO/错误示例；现有swag→kin-openapi→3.0.3生成与Check；TestMCPOpenAPIContract对保存OA逐字段、required、错误状态和方法路径核对；原动态路由枚举测试保留并通过 | 完成 |
| 3.6 | 真实隔离MySQL v4、Phase 1独立MCP、当前Go handler的真实TCP HTTP；凭据密文/脱敏、故障、SSRF、竞态、旧第五课真实MySQL技能/RAG/SSE回归；213旧容器生命周期保持，临时资源清理 | 完成 |

T18/T20/T24等跨阶段Scenario中的“再提问、meta denied、SSE审计”仍待Phase 4；T26前端密码框清空待Phase 5。T28–T33迁移/保留/终态INSERT细节复用Phase 2证据（原DDL/catalog/hash及这些算法未改），本阶段新增验证HTTP映射和审计查询/删除保护；没有把未授权部分算作端到端通过。

## 官方八操作对照

| 操作 | 成功与特有错误 |
|---|---|
| GET /api/mcp/servers | 200 servers；id/name/base_url/has_auth/connected/tool_count；仅本班 |
| POST /api/mcp/servers | name/base_url/auth_token三项必需，空Token允许；201 false/0；重名400 |
| PUT /api/mcp/servers/{id} | name/base_url必需，auth_token可选且空串保留；200 false/0；改名不写技能 |
| DELETE /api/mcp/servers/{id} | 无审计204并级联缓存；有审计503且不删；重删404 |
| POST /api/mcp/servers/{id}/connect | 200 connected=true及tools；远端/鉴权/协议/Schema/超时/SSRF502，DB503 |
| POST /api/mcp/servers/{id}/disconnect | 200 connected=false；清缓存、保留审计、无RPC，幂等 |
| GET /api/mcp/servers/{id}/tools | 200 tools（name/description，项目附加inputSchema）；断开[]；不联网 |
| GET /api/mcp/tool-calls | 200 calls；tool_ref/detail/user/conversation/created_at；本班时间及id倒序；默认50、最大100 |

全部操作匿名401、学生403；按ID外班/不存在404，业务DB503，错误体为{error:说明}。**附件示意409与冻结D3/实际tasks冲突，按冻结合同同名400；不增加409、详情GET或discover。** PUT空Token表示保留，没有clear_token。官方工具schema为可选附加字段，本实现也返回实际缓存inputSchema。

旧认证存储异常500继续保留，已在真实HTTP负测与注解中披露，未为第六课改写第五课认证。加密解封失败503；被更新/断开/其他连接取代的connect返回安全502，目标已删除返回404，均不得发布旧缓存。

## 实现与工程调整

新增MCPStore接口和共享mcp.Server/AuditCall/错误类型，原mysql类型以别名兼容；避免旧alignstorage集成测试通过http包形成循环依赖。未新增数据访问层或第二套HTTP客户端。Disconnect改为单次事务锁行、清缓存和失效租约，修复原两事务并发stale缺口；Update在DNS前先核本班资源，杜绝外班ID触发出站解析。

启动仅在MCP_ENABLED=true时装配，要求环境/Secret的MCP_MASTER_KEY、MCP_KEY_ID、MCP_ALLOWED_TARGETS有效，v4 catalog和receipt只读校验通过且已有可信身份。无DDL、无JWT密钥回退、无默认Compose改动；这些环境模板和UI仍留Phase 5。

## 实际运行与统计

最终隔离证据：[result.json](../../../runtime/week6-phase3-20261018T135556Z-b05263e7/result.json)、[integration.stdout](../../../runtime/week6-phase3-20261018T135556Z-b05263e7/integration.stdout)、[compatibility.stdout](../../../runtime/week6-phase3-20261018T135556Z-b05263e7/compatibility.stdout)、[week5-storage-regression.stdout](../../../runtime/week6-phase3-20261018T135556Z-b05263e7/week5-storage-regression.stdout)。各TestMCPHTTP*.json保存安全响应、用户编号、路径和状态，不保存请求Token/JWT。

| 最终测试组 | 通过叶级用例 | 失败 | 跳过/本阶段必需未执行 |
|---|---:|---:|---:|
| 真实管理/客户端/安全集成 | 80 | 0 | 0 |
| 默认Go兼容回归 | 223 | 0 | 0 |
| 第五课真实MySQL技能/RAG/SSE回归 | 57 | 0 | 0 |

合计360次叶级执行；加密/出站25项同时出现于前两组，去重后 **335项通过、0失败、0本阶段必需未执行**。Go vet退出0。package没有test files不算测试跳过。最终Docker运行后仅补充错误示例注解/合同断言；又执行生成、Check和10个叶级文档合同用例，全部通过，不重复累加到335。

第五课回归使用原alignstorage测试，独立私网MySQL别名campusclaw-align-storage-随机值，覆盖技能CRUD/隔离、角色/绑定、原Ask零据保护、SSE早meta、失败清理、并发轮次和恢复。embedding/model/vector由这些原用例的受控fixture提供；本轮未启动Qdrant、浏览器或真实付费模型，不宣称完成Phase 6全栈回归。

保留全部三轮记录：首轮20261018T134902Z-0be2fdef有2项测试失败（详情GET误期望405、子测试HTTP fixture提前关闭），72通过；已修正测试而未为迎合断言增加接口。第二轮20261018T135213Z-346113e2通过；最终20261018T135556Z-b05263e7增加解耦与真实第五课存储回归后通过。初次生成前的文档枚举失败、swag别名/RawMessage识别问题已通过类型命名和注解纠正；最初受限网络拉依赖失败后获准重用原生成链，未替换生成器。

测试执行Phase 1服务的真实initialize/list；正常发现的清单不是mock。受控远端仅用于非法Schema、协议、redirect、超时、取消、停服和竞态。SSRF拒绝metadata/回环/未允许端口/URL组件；DNS登记通过后重绑定metadata，实际目标访问0；GET缓存不联网、连接不执行tools/call，原客户端无自动重试。

## 文件、资源与保护

本轮代码/测试/生成产物共10项：

- cmd/server/main.go（装配）
- internal/http/mcp.go、mcp_contract_test.go、mcp_integration_test.go（新增）
- internal/http/web/swagger/openapi.json（原流程生成）
- internal/mcp/client.go（仅schema/required文档tag）、management.go（新增共享视图）
- internal/repository/mysql/mcp.go（连接装配、原子断开、前置范围检查、兼容类型别名）
- tests/go_mcp/compose.phase3.yml、run_phase3.py（新增独立runner）

本change文档：tasks.md、design.md、review-summary.md、requirements-traceability.md、risks-and-open-questions.md、acceptance-matrix.md、verification-result.json，以及新增phase3-preservation-baseline.json、phase3-verification.md/json、phase3-docker-closeout.json、phase3-final-checks.json。审核旧文档只补当前状态，无重写72合同。

分支feature/week5-phase1-go-rag，HEAD 51a8749f6550c279efebe63bb67eda724878f5a1保持。初始工作区已有大量未提交文件；424文件起点摘要中仅4个本轮授权现有代码/产物发生变化，其余受保护内容字节一致。第五课change（29/29、8/8及32/32）、主应用认证/RAG/技能/会话/SSE代码、旧41DDL/v4SQL/catalog、独立mcp-server、默认Compose/.env.example/网关均未修改；索引保持未暂存。未连接既有业务数据库/向量/模型，未commit/push/merge/rebase/archive。

[Docker收尾证据](phase3-docker-closeout.json)：三个唯一Compose项目均无残留容器/网络；213个既有容器的ID/状态/启动/结束时间与每次基线一致。专用未占用10.241子网、内部网络、无宿主端口、tmpfs MySQL、无外部卷；源目录只读挂载。只操作本轮创建的project；保留本轮唯一标签测试镜像和构建缓存，未替换既有镜像。

## 阶段门槛与遗留风险

最终[检查原始日志与保护核对](phase3-final-checks.json)均通过。strict校验通过；72 Requirement、72主Scenario、8操作、42任务保持。名称复用可能让旧文本引用解析到后来同名服务器、远端调用后终态INSERT前崩溃可能缺审计，这两项仍未彻底解决；不引入稳定ID第四表或审计清理来扩大范围。Phase 4实施前置满足，但仅在下一次明确授权后开展；1200字整条丢弃、不可信结果进入user数据位、meta.tool_calls等尚未实施。
