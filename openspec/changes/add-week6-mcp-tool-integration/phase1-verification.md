# Phase 1 最终验收（2026-10-18，Asia/Shanghai）

结论：1.1–1.6全部满足本阶段验收，累计10/42；Phase 2–6的32项保持未完成。本次仅恢复收尾，未重新开发服务、未重跑已充分验证的测试，未实施主应用/数据库接入。冻结的72项Requirement和72个主Scenario保持原文。

## 最后一次运行与源码归属

最终运行目录：[20261018T105629Z-032749c0](../../../runtime/week6-phase1-20261018T105629Z-032749c0/result.json)。会话工具已返回进程退出0；result.json在runner finally写出，13条命令全部退出0、failures为空、passed=true且清理检查为true；不能把额度中断当成命令失败。运行的13个源码/配置文件SHA256与当前mcp-server全部一致。原始证据不覆盖、不回填，哈希清单与本次统计见[机器验收](phase1-verification.json)。

## 逐任务验收

| 任务 | 实际实现与证据 | 结论 |
|---|---|---|
| 1.1 / T01,T10 | 独立Go module及cmd/mcp-server入口；真实编译后独立子进程启动、缺凭据非零退出、health只有status、停进程后Go RPC请求失败；process_test.go及go-test.stdout | 通过 |
| 1.2 / T02,T03 | POST /mcp；initialize版本2025-03-26/capabilities/clientInfo；大整数及中文字符串id原样语义匹配，result/error互斥；坏JSON/UTF8、重复键、错误信封/id、未知方法、不兼容握手均有错误；两环境真实HTTP响应 | 通过 |
| 1.3 / T04,T05 | tools/list发布两工具及严格inputSchema；注册重复/超过100拒绝；缺字段、类型、额外字段、500/501 Unicode边界、非法数值负测；无缓存/DB实现 | 通过 |
| 1.4 / T06–T08 | 去括号/圆面积提示与通用下一步；长度/质量/时间确定性换算；未知单位/数值拒绝，异维度isError=true，未知工具协议参数错误；无unit_converter别名/模型/任意执行 | 通过 |
| 1.5 / T09,T11,T14 | 专用Bearer、学生JWT/缺错/重复Authorization拒绝；Origin缺失合法、显式允许通过、其他403；Content-Type415，64KiB/分块超限413；3秒超时、取消传播及调用计数=1、拒绝零执行 | 通过 |
| 1.6 / T13 | 独立Dockerfile、只读scratch/UID10001、运行期Token、内部Compose网络、无宿主端口/卷；独立Go probe握手/list/call及停服后RPC失败 | 通过 |

T03的主应用Base URL补/mcp、T04的发现缓存事务、T08的模型结果接入等跨阶段职责仍属Phase 2–4；本报告只验收本阶段独立服务职责，不把后续接入宣称完成。服务实现课程HTTP JSON-RPC子集，不宣称完整Streamable HTTP/session/GET SSE/SDK。

## 测试数量与复现记录

最终go test ./... -count=1 -json退出0：62个叶级测试通过、0失败、0必需测试未执行；65个pass事件包含父级汇总，不作为65个独立用例。三条package skip表示三个包没有test files，并非遗漏了三项必需测试。宿主52项协议检查已计入62，不重复相加。

Docker私网独立Go客户端52项协议检查通过，另1项停服RPC失败检查通过。合计按上述口径115项检查通过、0失败、0本阶段必需未执行；缺凭据启动、宿主停服及日志无泄漏也在父测试内断言通过，不另虚增用例数。go vet ./...退出0。

- [宿主请求与响应](../../../runtime/week6-phase1-20261018T105629Z-032749c0/host-network.json)
- [Go完整测试事件](../../../runtime/week6-phase1-20261018T105629Z-032749c0/go-test.stdout)
- [Docker请求与响应](../../../runtime/week6-phase1-20261018T105629Z-032749c0/docker-network.stdout)
- [Docker停服验证](../../../runtime/week6-phase1-20261018T105629Z-032749c0/docker-offline.stdout)
- [镜像配置](../../../runtime/week6-phase1-20261018T105629Z-032749c0/docker-image-config.stdout)及[构建历史](../../../runtime/week6-phase1-20261018T105629Z-032749c0/docker-image-history.stdout)

超时/取消通过测试注入受控executor，使用真实TCP，观测超时约3.01秒（3秒截止时间及响应调度开销）、取消传播且执行次数严格1。它们不是生产工具延迟接口，也不是Docker中的延迟故障测试。生产进程的关键协议与两工具分别由宿主子进程、Docker独立网络完成，未用mock替代。

安全检查范围：测试临时生成服务凭据，runner在持久化前扫描并脱敏，响应/服务日志无该值；镜像配置无Token环境项，构建历史不含凭据。真实密钥未读取或用于测试；不能将此表述为对仓库所有ignored秘密做过取证扫描。公式query被当作数据，不执行命令；非法command字段及单位路径拒绝，合法query只返回固定提示，不回显指令。参数错用-32602，非法JSON用-32700；已知单位异维度属于工具域失败，content/isError=true，不伪装成功。

## 历史失败与Docker隔离

[第一次失败](../../../runtime/week6-phase1-20261018T105137Z-c1bff1ae/result.json)保留：Docker构建成功，docker-up退出1，原因all predefined address pools have been fully subnetted。该轮不是全通过；其failures数组的两条说明同一次地址池失败及异常传播，并非两个失败测试用例。早期默认GOCACHE权限错误通过改用ignored runtime缓存处理，未改系统缓存权限。

随后新建唯一project，先读取已占用子网，指定不重叠10.240.1.0/24内部网络。105339Z、105524Z及最终105629Z三轮成功；未删除既有网络或改Docker默认地址池。配置在创建前校验：只含mcp-demo/probe，无挂载/卷/宿主端口/external网络；不读写现有.env，不复用Demo。

最终project为campusclaw-w6-20261018t105629z-032749c0。[清理日志](../../../runtime/week6-phase1-20261018T105629Z-032749c0/docker-cleanup.stderr)和[本次只读复核](phase1-closeout-docker.json)确认四轮测试project均无残留容器/网络，213个既有容器生命周期字段无变化（健康探针时间戳自然变化不比较）。本轮专属镜像标签按README保留供审阅，未删除既有镜像；无测试卷，未访问数据库/Qdrant数据。此处未宣称检查了所有既有卷的数据内容。

## 文件与范围保护

分支feature/week5-phase1-go-rag，HEAD 51a8749f6550c279efebe63bb67eda724878f5a1。以phase1-preservation-baseline.json中既有文件哈希核对，收尾前全部匹配；原17个tracked修改及大量untracked成果保留，不暂存/清理。第五课仍32/32、29/29，主应用、旧OpenSpec、JWT、RAG、SSE和数据库文件未由本轮修改。最终保护清单见phase1-verification.json及verification-result.json.phase1_review；ignored运行资源/秘密不包含在Git文件哈希承诺内。

上一轮新增mcp-server共13个交付文件：go.mod、cmd/mcp-server/main.go、internal/server/{server.go,tools.go,server_test.go}、internal/checks/checks.go、cmd/probe/main.go、tests/process_test.go、Dockerfile、.dockerignore、compose.test.yml、run_phase1.py、README.md；另在本change保存phase1-preservation-baseline.json。本次仅更新tasks/review-summary/verification-result并新增本报告、phase1-verification.json及phase1-closeout-docker.json。

## 启动及真实示例

进入mcp-server目录，将专用MCP_AUTH_TOKEN注入进程环境后运行go run ./cmd/mcp-server，默认:8090；MCP_LISTEN_ADDR可覆盖。不要在命令行/源码写真实凭据。healthcheck子命令按容器默认8090使用，Origin允许列表为可选MCP_ALLOWED_ORIGINS。具体复现步骤见[运行说明](../../../mcp-server/README.md)。

最终真实initialize响应包含protocolVersion=2025-03-26、serverInfo/capabilities.tools；tools/list仅formula_hint和unit_convert且各有description/inputSchema。formula_hint输入query=“去括号这一步我不会”，输出“去括号可用分配律 a(b+c)=ab+ac。先检查括号前的系数是否乘到每一项，再检查负号引起的符号变化。”；unit_convert输入value=1/from=m/to=cm，输出“1 m = 100 cm”。均为content text、isError=false；m→s返回isError=true及可读说明。完整请求信封/id与输出见上述两环境原始记录。

## 门槛与停止

strict校验结果写入机器记录；任务总数42、完成10，Phase 0四项保留，Phase 2–6三十二项未勾选。具备进入Phase 2的本阶段前置条件，无Phase 1阻塞。客户端出站安全、AES-GCM数据库凭据、v4迁移、管理API、技能/SSE/模型接入尚未实施，不以服务端通过代替这些验收。名称复用、审计窗口等冻结风险仍保留；任何后续实施均等待新的明确授权。完成Phase 1后停止，无commit/push/archive或真实模型调用。
