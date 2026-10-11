## Why

现有系统已支持可信 Bearer Token 身份、教师上传和班级材料隔离，但尚不能从本班材料检索证据并回答问题。第 4 课增加切片、向量检索和来源标注，使回答能追溯到本班材料，且没有证据时不调用对话模型。

## What Changes

- 保留 Flask/Python、SQLite 中的账号/班级/Token 撤销版本、teacher/student 权限和上传文件行为；不迁移 Go/Nginx。
- 根据本次明确指定的 MySQL FULLTEXT/ngram 要求，仅将材料元数据及原文存储迁入 MySQL，并增加 knowledge_chunks。保留上传中材料/原文同事务、文件失败补偿和既有 API 契约；原文提交后才开始索引。
- **BREAKING**：启用第 4 课前需要完成材料数据迁移并配置 MySQL、Qdrant 与两个课程网关；不能将现有 SQLite FULLTEXT 替代方案视为本次课程验收。账号与 Token 撤销版本不迁移，不因此要求重新登录。
- 默认约 800 字、80 字重叠切片，支持自定义字面分隔符与 Markdown 标题；教师可重建本班材料索引，学生不能上传或重建。
- Embedding、对话和 Qdrant 分别封装。Qdrant 的 campusclaw_chunks 只保存向量及五个关联编号，正文和最终摘录全部来自 MySQL。
- 提供 keyword/vector/hybrid 检索（默认 hybrid），余弦阈值 0.35、RRF k=60，所有检索路径以服务端注入班级范围过滤。
- POST /api/ask 复用检索模块的前 4 个有效切片，有证据才生成简短回答及 [1] 等来源；无证据直接返回「资料中未找到相关内容」。
- 索引失败保留原材料；先取得全部 Embedding 再写向量，失败切片不可检索，必要时补偿删除并记录待清理状态，禁止把半成品发布为 ready。

## Capabilities

### New Capabilities

- `traceable-vector-retrieval`: 班级材料索引、三种检索、RRF、可信班级范围、来源问答、失败一致性及必要的材料存储迁移。

### Modified Capabilities

无已发布 capability：`openspec list --specs` 为空。既有 `user-auth`、`class-access-control`、`material-library`、`app-runtime` 和 `server-side-session` 存在于两个已有 change 中，本次不改写或归档它们。新 capability 补充其行为；仅替代材料正文存储位置和此前“不做检索问答”的范围决定，保留Bearer 认证及材料访问契约。

## Impact

- HTTP 薄路由、独立 indexing/retrieval/answering 模块、小型 repository 和三个独立 client；不采用 LangChain/LlamaIndex、通用插件框架或微服务。
- 必要接入点是 `app/routes.py` 的上传提交后钩子、材料数据访问/恢复/初始化路径、Compose 中 MySQL/Qdrant 和服务端配置。保留材料页面并接入已获授权的最小问答页，以 API 和最小浏览器流程验收。
- SQLite 继续保存可信身份与 Token 撤销版本；MySQL 保存 materials、knowledge_entries.body_text、knowledge_chunks.chunk_text。保留既有详情响应字段 plain_text，避免影响现有页面。
- 按 Bearer 契约等价适配的全量旧行为测试和新增真实 MySQL/Qdrant 集成测试共同验收，不能用测试替身替代 ngram/向量过滤验收，不能删减第 3 课安全断言。
- 当前进入 Apply；迁移命令只在隔离测试数据上验证，不对现有课堂数据执行迁移，不自动部署或归档。

## 已确认的契约调整

保留现有 vision/2048 多模态 Embedding 与 Qdrant 数据；新增服务端 EMBEDDING_API_FORMAT 配置，支持 ark_multimodal 与 openai。两者使用相同向量校验和独立对话客户端。维度必须匹配目标 collection，禁止自动删建、清空或更换现有课堂索引。旧 Session 专属验收按当前 Bearer 契约等价验证，保留权限、隔离、撤销和事务边界。
