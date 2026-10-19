# Phase 2 实施与最终验收（2026-10-18，Asia/Shanghai）

结论：2.1–2.7全部通过，进度17/42；Phase 3–6共25项未实施、未勾选。基于现有change及冻结合同增量实施，未新增管理HTTP API、UI、技能绑定/路由、SSE或模型接入。本报告与[机器证据](phase2-verification.json)为当前Phase 2状态，原Phase 0/1记录保留历史事实。

## 逐任务及证据

| Task | 状态 | 实现与对应测试证据 |
|---|---|---|
| 2.1 / T28 | 完成 | schema_mcp_v4.sql三表、唯一/复合class FK、connected/status CHECK；TestMCPStoreAndRealService/constraints、cache_cascade、audit_and_delete；参考catalog逐字段一致 |
| 2.2 / T29 | 完成 | 新增mcp_schema.go，保留旧41步，v4四DDL及独立hash/catalog/ledger；初始化receipt和HTTP只读就绪门槛；路径4项、13断点、GET_LOCK、12漂移/账本用例及receipt正常/中断恢复 |
| 2.3 / T30 | 完成 | TestMCPBackupAndLegacyProtection逐列NULL敏感摘要、独立dump→另一随机库恢复、材料文件备份复制/恢复hash；source/hash/version/enabled、用户撤销、绑定、消息和引用均保留 |
| 2.4 / T26 | 完成 | AES-256-GCM、随机nonce/key_id、班级/名称AAD绑定、空Token更新保留并在改名后重封；安全ServerView仅has_auth；TestVault及真实存储/改名后握手验证 |
| 2.5 / T12及协议子集 | 完成 | mcp.Client Base自动补/mcp、initialize/list/call、固定安全错误、ID/version/result校验、64KiB/UTF8、3秒取消且无重试；Phase 1真服务及受控HTTP故障测试 |
| 2.6 / T27 | 完成 | scheme/host/port精确目标、全部DNS答案校验、固定IP拨号、无代理/重定向，硬拒回环/metadata；专用10.241.1.0/24可达；DNS重绑定与redirect目标零访问 |
| 2.7 / T31–T33,T62 | 完成 | 终态INSERT、成功脱敏前200字、固定失败/拒绝≤512、50/100本班只读；存在服务denied有行/旧名删除失配无行，缓存失败回滚/CAS、审计FK删除受限、INSERT失败返回错误 |

跨阶段Scenario中的前端清空密码框、管理HTTP错误映射、meta前503/failed、SSE与模型安全过滤尚属Phase 3–5，不因本阶段底层组件通过而宣称已端到端验收。tools/list的缓存事务由底层组件实现，主应用管理路由尚未注册。

## 三表及账本实际合同

实际DDL：[schema_mcp_v4.sql](../../../internal/repository/mysql/schema_mcp_v4.sql)；由新隔离参考库采集的逐步结构证据：[schema_mcp_steps.json](../../../internal/repository/mysql/schema_mcp_steps.json)。捕获前逐条对SQL SHA256，已有非空artifact只允许比较，不以待迁移目标漂移结构重建“正确答案”。

| 表 | 实际字段和关键约束 |
|---|---|
| mcp_servers | signed BIGINT id/class_id；name VARCHAR64 ascii_bin，base_url VARCHAR512；BLOB token_cipher、VARBINARY12 token_nonce、VARCHAR64 key_id可空且三者一致；connected布尔CHECK0/1、connected_at、created_at/updated_at UTC字符串；revision/op_id/operation_started_at；UNIQUE(class_id,name)/(id,class_id)，class FK RESTRICT |
| mcp_tools | id/server_id/class_id BIGINT；tool_name VARCHAR64 ascii_bin、description TEXT、schema_json JSON；UNIQUE(server_id,tool_name)；(server_id,class_id)复合FK CASCADE |
| mcp_tool_calls | id/class_id/server_id/user_id BIGINT，conversation_id可空快照无FK；tool_ref VARCHAR128、tool/server快照VARCHAR64、status VARCHAR8 CHECK ok/denied/error、detail VARCHAR512、created_at VARCHAR32；server/class/user RESTRICT；(class_id,created_at,id)索引 |

无第四张绑定表、无明文Token列、无审计更新/删除方法、无保留天数或清理功能。调用审计是应用层只追加合同；不是声称数据库管理员不能直接执行任意SQL。

v4新增4步：先对现有schema_upgrades追加可空ddl_hash，再建3表。旧version=3完成行保留，ddl_hash=NULL；新version=4行独立记录step1–4、累计DDL hash与completed。原机制没有version=1/2行，本轮不伪造历史行；保留既有记录及旧41条语句/artifact。五类catalog仍检查表/列/索引/FK/CHECK，合法前缀和账本必须一致，只有单步DDL已提交而标记落后的精确状态可恢复。

发现无效编译hash时，现于推进旧链之前就拒绝；TestMCPDriftRefusal/DDL_hash从v1库断言错误hash不会先改旧结构。GET_LOCK涵盖旧链至v4，旧v3 helper保留原语义；HTTP启动不执行DDL。initializer先核原receipt和旧数据，再恢复合法部分状态，另写.campusclaw-schema-v4.json关联旧receipt hash；原receipt不覆盖，损坏文件保留拒绝。MCP_ENABLED=false不是DB回退；旧v3二进制无法直接读取严格v4 catalog。

## 最终测试与失败历史

最终目录：[week6-phase2-20261018T133028Z-5844f06a](../../../runtime/week6-phase2-20261018T133028Z-5844f06a/result.json)。runner退出0，全部记录命令退出0，failures为空。

| 最终检查 | 通过 | 失败 | 实际测试跳过 |
|---|---:|---:|---:|
| Phase 2集成/安全叶级测试 | 84 | 0 | 0 |
| 默认Go兼容性叶级测试 | 217 | 0 | 0 |
| 隔离参考catalog采集 | 1 | 0 | 0 |

前两轮共301次叶级测试执行，25项安全单测重复，去重276个测试标识；采集另计1，不虚增父级汇总事件。go vet ./cmd/... ./internal/...退出0。包级[no test files]不当作跳过必需用例。第五课完整tagged数据库、浏览器、Swagger及真实模型专项未重新执行；217指本次默认Go回归，不宣称第五课全量重新验收。本轮Phase 2必需检查未执行数0。

- [集成完整事件](../../../runtime/week6-phase2-20261018T133028Z-5844f06a/integration.stdout)
- [兼容性完整事件](../../../runtime/week6-phase2-20261018T133028Z-5844f06a/compatibility.stdout)
- [真实MCP客户端结果](../../../runtime/week6-phase2-20261018T133028Z-5844f06a/client-real-service.json)
- [独立恢复摘要](../../../runtime/week6-phase2-20261018T133028Z-5844f06a/preservation-restore.json)

真实MCP服务为Phase 1原Dockerfile/原源码，发现formula_hint、unit_convert；formula_hint(去括号/圆面积)返回公式提示，unit_convert(1,m,cm)返回“1 m = 100 cm”，专用Bearer正确可用、错误拒绝。故障测试为专用容器真实HTTP服务器，覆盖401/500、非法JSON/Content-Type/id/version/result/error、超限、null isError/text、非text、timeout/cancel、302及停服；调用计数严格1、redirect trap计数0，不以mock代替核心真服务联调。

历史7轮原始结果与hash全部保留，不覆盖失败：125842Z官方Go代理连接拒绝导致构建失败；130217Z复用缓存测试镜像继承entrypoint造成“go test go test”启动错误；130433Z测试地址IPv4-mapped表示不匹配精确CIDR且备份客户端认证失败；130802Z确认缺少caching_sha2_password客户端插件；131145Z全通过；131705Z在提前拒绝坏v4 hash的补强后全通过；最终133028Z补齐IPv6元数据硬拒后再次全通过。依赖改用本机不可变缓存镜像并显式清entrypoint、补连接器插件；没有降低MySQL认证或放宽SSRF断言。

## 数据、恢复及安全边界

最终旧数据迁移前后摘要相同：31d6a521796ab59ddf745913a4c3ffb52e59382bb4b6ebe5477e86748e550678。逐列覆盖用户/班级/session_version和session记录、材料UUID/storage_key、切片文本和UUID、技能source原始CRLF/version/hash/enabled、助手提示词/绑定、会话owner/class、消息序号/状态和引用/证据快照。v3→v4不更新旧数据；v1/v2经过既有v3链时仍保留其原回填语义，不新增MCP工具绑定或复活已删技能。

独立源库w6_69cf30e49e1b7085与恢复库w6_59549761030bd084同时存在时完成比对，完整catalog/账本/行摘要均为eb5f3654d6e24785c829fe14ce4475644ce5fbb92fb09fb8bafe3fc0e06e9d12；材料文件备份及恢复hash均ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad。保留合成测试库SQL备份、原文件/备份文件/恢复文件；不是删除表/卷后宣称恢复。

Qdrant没有启动或重索引。其point_id以ChunkID映射的现有代码及配置保持哈希，测试保留knowledge_chunks UUID/材料/班级/序号映射；不宣称执行过外部Qdrant快照恢复。

AES-GCM正常、nonce不同、密文篡改、错key/key_id/AAD及缺密钥测试通过；仓库视图不含Token，DB密文不含标记明文，改名空Token后仍能真实握手。密钥不与JWT来源混用、不退化明文。测试主密钥及服务凭据独立临时生成，报告不含真实密钥；LoadConfig供后续装配，不在Phase 2挂业务路由。

MCP客户端不透传Principal/JWT，固定IP连接时仍保留正确Host/TLS server name；请求每次重新解析，全部返回地址必须符合精确规则，硬拒metadata/回环及未允许地址，无重定向/代理。AWS元数据IPv6地址fd00:ec2::254亦硬拒，即使误配允许该地址也拒绝；已对照[AWS官方说明](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/configuring-instance-metadata-service.html)并在两组测试中验证。工具内容仍是不可信外部数据；完整注入过滤/1200整丢/user数据位及SSE序列将在Phase 4接入，不把JSON Schema元数据当模型权限。

审计只INSERT终态。存在但断开的服务记录denied，改名/已成功删除旧名查不到时返回recorded=false不虚构外键身份；有审计删除返回存储不可用（HTTP503映射由Phase 3提供），事务不改缓存/审计。远端执行成功与审计INSERT并非分布式原子；进程崩溃可能缺行，INSERT失败向调用者报错，不重放远端。名称复用风险仍按冻结设计披露，未声称解决。

## 资源与文件保护

所有连接只到新建专用MySQL8.4/MCP project、随机w6_*库，专网10.241.1.0/24无宿主端口；MySQL使用本轮tmpfs，无既有卷。真实恢复验证完成后才清理测试资源；此清理不作为回退证明。[只读收尾记录](phase2-docker-closeout.json)确认7轮容器/网络均无残留，213个既有容器生命周期未变。本轮独有镜像保留供审核；未操作Demo数据库/卷/Qdrant或真实模型。

Git分支feature/week5-phase1-go-rag，HEAD 51a8749f6550c279efebe63bb67eda724878f5a1。保护起点见phase2-preservation-baseline.json。授权修改的既有代码仅6份：cmd/bootstrap/main.go、cmd/server/main.go、repository/mysql/{catalog.go,rag_schema.go,tutor_schema.go}、transfer/ready.go；其中server只追加v4 receipt只读门禁，未改JWT/RAG/会话业务。旧41条SQL及hash文件、Phase1 mcp-server、旧OpenSpec、HTTP处理器、技能/路由/SSE/模型/前端、.env及默认Compose保持原哈希。ignored资源/秘密不包含在Git文件hash承诺内，未读取真实密钥，不清理用户文件。

新增代码：internal/mcp/{security.go,client.go,security_test.go,client_integration_test.go}，repository/mysql/{mcp.go,mcp_schema.go,mcp_integration_test.go,schema_mcp_v4.sql,schema_mcp_steps.json}，transfer/{mcp_receipt.go,mcp_receipt_integration_test.go}；新增测试fixture tests/go_mcp/{Dockerfile,compose.test.yml,run_phase2.py}及两份运行说明。change内更新tasks/当前审核说明与验证记录；完整文件和SHA256清单见机器证据。

## strict与停止

执行openspec validate add-week6-mcp-tool-integration --strict，结果另存机器记录；72项Requirement/72主Scenario/官方8操作/42任务保持，完成17项，Phase3–6的25项未勾选。无Phase 2阻塞，具备进入Phase3的前置条件；正式目标迁移/备份、管理API、前端和工具运行编排仍需新授权。完成后立即停止，不commit/push/archive，不进入Phase3。
