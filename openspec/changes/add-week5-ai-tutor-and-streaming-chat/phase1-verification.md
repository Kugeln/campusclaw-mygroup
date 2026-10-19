# Phase 1 独立 Go 实施记录

本轮仅授权1.1–1.8，不进入Phase 2。开发分支 feature/week5-phase1-go-rag，基于51a8749。未提交或推送。原有 tests/test_recovery.py 修改、未跟踪 Compose/迁移报告和其他 change 均保护；开始哈希保存在被忽略的 runtime/week5-phase1-preflight/protected-hashes.json。

## 1.1 基线与行为映射

Docker Engine29.8.0、Desktop4.92.0、Compose5.5.1可访问。课堂项目 campusclaw-retrieval-demo 的三个容器均已退出，其 demo-materials/demo-mysql/demo-qdrant 卷和 services/outbound 网络禁止使用或修改；历史迁移测试资源同样保留。新测试必须使用随机项目名、新网络、新卷、新库及独立collection，不接受现有课堂地址。

Go1.27.1 Windows，现有基线 `go test ./...` 在沙箱外退出0：8个包测试通过（部分缓存），其余编译且无测试。沙箱内首次尝试因已有runtime目录ACL失败，不修改ACL、不将其报告为业务测试失败。没有运行或复用Flask测试来证明Go通过；真实MySQL集成尚未执行，基线不等于RAG验收。

| 第4课参考 | Go目标与必须保持的契约 |
|---|---|
| app/auth.py / tokens.py | 直接复用internal/auth及/api/auth/*：HS256/sub/ver/iat/exp，28800秒，当前数据库身份、全设备版本撤销；不换官方login路径 |
| app/routes.py / materials.py | 复用Go上传/列表/详情/附件及恢复：UUID、20MiB、原文提交先于索引、学生403、跨班材料404 |
| retrieval/chunking.py | Go Unicode800/80；fixed/delimiter/markdown围栏；1-based切片，无空白块 |
| storage/mysql.py / retrieval/service.py | 本班ready FULLTEXT ngram=2；vector≥0.35、Qdrant与MySQL双过滤；RRF k=60稳定UUID排序 |
| retrieval/indexing.py | pending→全量ready；任一Embedding失败零upsert；重建先确认清理；原文不回滚；故障/中断不可见 |
| retrieval_routes.py GET /api/search | q/mode，默认hybrid；mode/items；伪造scope无效，外班独有200空数组；非法400，依赖503不静默降级 |
| retrieval/answering.py POST /api/ask | question/mode→answer/sources；top4；有效[n]来源；无命中精确“资料中未找到相关内容”且零模型调用；空回答/错误引用502 |
| clients/embedding.py / qdrant.py / dialogue.py | 独立服务端配置，两Embedding协议、有限非零向量/维度校验、五字段payload、非流式独立对话；无客户端密钥 |

旧change任务已完成但未归档，不修改。主要接入冲突为严格v1 schema catalog及初始化receipt；增量升级必须显式管理，不放宽未知漂移校验，不通过CREATE IF NOT EXISTS假装旧表已升级。第5课预备表仅结构，不能接入教师技能或会话API。

## 历史暂停时状态（被下方最终验收覆盖）

1.2–1.8 尚未完成，不以基线或OpenSpec验证代替；真实隔离Go/MySQL/Qdrant闭环通过前Phase 1不报告完成。

## 首次隔离网络阻塞（历史记录，现已解决）

新项目名 campusclaw-week5-rag-26c4d311，使用 tests/go_rag/compose.yml：新私有网络 isolated、随机loopback端口、库week5_rag_test及新mysql-test/qdrant-test卷，不引用external网络或卷。执行创建时Docker返回 `all predefined address pools have been fully subnetted`，isolated网络创建失败；该命令退出1，不认为MySQL/Qdrant已启动或任何真实集成已通过。未操作课堂容器、网络、卷，未prune或修改Docker全局地址池。可能部分创建的新项目卷保留，不自动删除。

已写入1.2的未验收草稿：schema_rag.sql、显式离线PrepareRAGSchema及build-tag开发catalog捕获测试。未执行DDL，未生成/接入RAG catalog，未启用启动流程或修改默认Compose，不能宣称可运行RAG。捕获测试不在普通build/test中，下一轮须在独立空测试库生成并严格核验完整catalog，再接入升级和回归；目前不运行它，不勾选1.2。新增隔离Compose仅供测试，不是部署配置。1.3–1.8尚未开始。

安全停止后，新增Go schema代码的最小编译检查使用 `go test ./internal/repository/mysql`；OpenSpec strict只校验规约，不是RAG验收。恢复实施需要先明确一个经核对不与现有Docker网络及宿主路由冲突的测试子网，或由用户处理Docker可用地址池。不得以使用既有网络绕过隔离要求。

## 最终验收（2026-10-14）

已解决地址池问题：显式子网10.254.241.0/24，核对未与Docker既有网络（包括10.254.240.0/24）、宿主10.7.0.0/16、WSL172.17.80.0/20和VMware/VPN当前路由重叠。新项目campusclaw-week5-rag-ee706bec的内部网络、mysql-test/qdrant-test/uploads-test卷独立；mysql库week5_rag_test，Qdrant每次测试独立week5_go_* collection，受控网关在Go测试容器内。测试服务不发布宿主端口。复用已建立的服务，不停止、清理、删除或重建旧资源。

| 任务 | 结果与证据 |
|---|---|
| 1.1 | 保留前述Go基线和行为映射；最终保护哈希核对原三个工作文件及其他change全部不变 |
| 1.2 | v1保留、v2增量DDL、严格独立catalog/确切ngram解析器；重复升级、材料及旧无班级占位行保留、非法FK拒绝、未知列漂移拒绝；预备教学表只有结构，无Phase2业务API |
| 1.3 | Unicode800/80、固定/字面分隔符/围栏Markdown、RRF手算/去重/稳定序、0.34丢弃与0.35保留；Go核心不导入HTTP/SQL |
| 1.4 | 两Embedding协议与零向量/维度拒绝、真实Qdrant召回及本班回表、伪造payload和外班ID丢弃；不匹配维度不改变旧ready索引。collection只新增/验证，不能删除重建 |
| 1.5 | 真实上传→切片→索引→重建；第二片Embedding失败零本次向量发布、原文保留；Qdrant写入确认丢失且清理失败保持failed/cleanup_required、检索不可见，恢复确认清理后可重建；学生403/跨班404 |
| 1.6 | A/B teacher/student三模式双向矩阵、q/mode与mode/items保持、可信班级、材料额外过滤在limit前；keyword不依赖向量服务、hybrid故障不降级 |
| 1.7 | question/mode→answer/sources、前4证据、来源对应MySQL、非法引用502、无命中精确固定文本/空来源/零模型调用；认证及Token撤销保持 |
| 1.8 | 实际cmd/server启动校验和持久文件/MySQL复用后health、Bearer hybrid search、ask通过；Go依赖图无Flask/Python/SQLite。默认Compose语法及scratch服务镜像构建通过，CA证书已带入镜像；RAG默认false、个人显式启用不影响旧功能 |

最终独立报告：`runtime/campusclaw-week5-rag-ee706bec/results-1a33927f.json`，accepted=true、exit_code=0；对应 `go-tests-1a33927f.log`，6个顶层验收测试通过。Go HTTP链路测试同时覆盖实际cmd/server子进程：15.47秒；schema 1.80秒，核心0.01秒。HTTP测试记录34次Embedding及2次对话（实际cmd/server补验另有调用）；无命中按前后计数断言零新增对话。次数不是用例数。

### 实际命令和结果

- `powershell -NoProfile -File tests/go_rag/run.ps1`：最终退出0，串行真实MySQL/Qdrant + Go核心/HTTP/实际入口受控验收通过，机器报告和日志均在Git忽略的runtime中。
- `go test ./internal/config ./internal/http ./internal/rag ./internal/repository/mysql ./cmd/server ./cmd/bootstrap ./cmd/admin`：最终全部通过；另外原认证/材料/存储包此前针对性回归通过。未反复全仓扫描；普通go test不冒充build-tag真实集成。
- `docker compose --env-file NUL -f docker-compose.yml --profile rag config --quiet`：通过，使用测试占位环境值，不输出密钥。
- `docker build --build-arg GOPROXY=https://goproxy.cn -f Dockerfile.backend --target server -t campusclaw-week5-go-server:phase1 .`：通过，无Python/Flask/SQLite业务镜像；未启动默认Compose或部署。
- `go list -deps ./cmd/server`：无modernc/sqlite/python/flask依赖。
- `git diff --check`：通过；Git仅提示LF/CRLF归一化，不是差异错误。原工作文件和其他OpenSpec change保护哈希未变。

### 失败记录和边界

历史网络创建失败后没有清理旧网络。一次集成失败为不同包并发导致schema故障注入与HTTP启动重叠，runner采用-p 1后通过；一次runner失败为PowerShell模板引号丢失，改为解析docker inspect JSON后通过；故障代理压缩转发导致注入点错误，修正Accept-Encoding处理后重跑通过。早期UUID模块下载超时改为标准库随机UUID，未改依赖清单。失败证据保留，不把它们算通过。

未运行真实外部大模型、完整默认Compose/Nginx下的新RAG端到端或生产迁移；当前证据是实际Go服务和隔离依赖、受控网关与镜像/配置校验，不是正式部署或真实模型教学效果保证。当前测试数据、collection、运行容器和卷均保留，不自动删除。v2升级后旧v1二进制的严格catalog不会接受新结构，不能直接退旧版本；使用本分支RAG_ENABLED=false停用新API，无需删除新表/旧文件。小组集成前重点协调catalog、bootstrap、服务路由、上传后索引钩子和Compose配置修改。

Phase 1为8/8完成；Phase 2–6的24项保持未勾选，不提交、推送、merge、归档或自动继续实施。
