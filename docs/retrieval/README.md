# 最小课堂 Demo

本轮使用 Flask + 当前 Bearer Token，SQLite 只保存可信账号/班级，MySQL 保存材料、正文及切片，Qdrant 保存向量和五个关联编号。前端只请求本站 `/api`。没有迁移旧材料，也没有修改 Go 服务或 Token change。

当前已支持两种 Embedding 协议，并补齐隔离迁移、MySQL 事务补偿、失败恢复及维度保护验证。课堂现有 vision/2048 与已有向量保持不变。最新结果见 [verification.md](../../openspec/changes/add-traceable-vector-retrieval/verification.md)。

## 启动

使用独立 `docker-compose.retrieval.yml`，不要使用根目录默认的 Go Compose。Demo 使用独立命名卷，不会读取或清理原有上传目录。首次创建 A/B 两班的四个示范账号，并经 MySQL repository 保存两份很小的样本材料；不会自动调用模型。已有课堂数据库不会重新初始化或重置账号。

课堂 Compose 将 `app/` 和 `scripts/` 以只读方式挂载，便于使用本轮最终代码，无需重复构建依赖镜像。此配置是本地课堂开发入口。

在服务端 PowerShell 进程配置以下变量；所有密码和 key 通过秘密管理设施或安全输入注入，不写入源码或浏览器。

| 变量 | 用途 |
| --- | --- |
| `JWT_SIGNING_SECRET` | 保持当前 Bearer 签名秘密；首次使用至少 32 字节高熵值 |
| `RETRIEVAL_MYSQL_PASSWORD`、`RETRIEVAL_MYSQL_ROOT_PASSWORD` | 独立 Demo MySQL 凭据；以后沿用同一值 |
| `SEED_TEACHER_A_PASSWORD`、`SEED_STUDENT_A1_PASSWORD`、`SEED_TEACHER_B_PASSWORD`、`SEED_STUDENT_B1_PASSWORD` | 首次账号密码；已有账号不被重置 |
| `EMBEDDING_BASE_URL` | 网关基础地址（当前火山方舟为 `https://ark.cn-beijing.volces.com/api/v3`），不含请求端点 |
| `EMBEDDING_API_FORMAT` | `ark_multimodal`（默认，当前课堂方案）或 `openai`（标准 `/embeddings`）；显式选择，不自动回退 |
| `EMBEDDING_API_KEY`、`EMBEDDING_MODEL`、`EMBEDDING_DIMENSION` | 独立 Embedding 配置；维度必须与实际模型一致 |
| `DIALOGUE_BASE_URL` | 独立对话基础地址，不含 `/chat/completions` |
| `DIALOGUE_API_KEY`、`DIALOGUE_MODEL` | 独立对话配置 |

安全输入秘密的例子：

```powershell
$name = 'EMBEDDING_API_KEY' # 逐项替换为需配置的秘密变量名
$value = Read-Host $name -AsSecureString
$ptr = [Runtime.InteropServices.Marshal]::SecureStringToBSTR($value)
try {
  [Environment]::SetEnvironmentVariable($name, [Runtime.InteropServices.Marshal]::PtrToStringBSTR($ptr), 'Process')
} finally {
  [Runtime.InteropServices.Marshal]::ZeroFreeBSTR($ptr)
}
```

非秘密的地址、模型、维度使用 `$env:变量名 = '课程提供的值'`。配置完成后：

```powershell
# 本轮验收已构建 campusclaw-retrieval-demo:local；无需重复构建。
docker compose -f docker-compose.retrieval.yml up -d --no-build mysql qdrant app
docker compose -f docker-compose.retrieval.yml ps
# 新机器尚无镜像时才执行一次：
# docker compose -f docker-compose.retrieval.yml build app
```

打开 `http://localhost:8080/login`，登录 `teacher_a`，上传 [demo-a.md](demo-a.md)（约 1.2 KB，默认只有一个切片）。看到“已加入知识库”后，点击“打开知识库问答”。提问：**星河读书活动什么时候开始，在哪里举行？** 页面应显示回答以及 `[1] demo-a.md` 和原文摘录。

无依据示例：**木星探测器的燃料配方是什么？** 无有效检索命中时，服务端精确返回“资料中未找到相关内容”，且不调用对话模型。注意现有 spec 的固定拒答条件是“零检索证据”，不是对所有材料未记载的问题做语义判定；真实模型可能把无关问题召回为相似证据（阈值固定 0.35），应在上课前用实际模型核验此示例。若命中但证据不足，模型应说明无法回答并引用检查过的资料；服务端不会伪造引用。

Qdrant 必须运行才能上传成功索引以及使用默认 hybrid 问答。keyword API 在 Qdrant 停止时仍可读取已经 ready 的 MySQL 切片。浏览器不直接访问 MySQL/Qdrant，Compose 不公布它们的端口。

## API 与失败重建

所有请求使用 `Authorization: Bearer <access_token>`，不提交 Session Cookie 或 CSRF。账号、角色、class_id 由 Token 校验后的数据库用户决定；检索/问答中客户端自报身份一律忽略。

```text
GET /api/search?q=星河&mode=hybrid
POST /api/ask                  {"question":"星河活动在哪里举行？"}
POST /api/materials/{id}/reindex  {"strategy":"fixed"}
```

重建仅限本班 teacher；也支持 `markdown` 或 `delimiter`（附 `delimiter`），以及 `size`、`overlap`。依赖恢复后对失败材料调用重建。失败上传仍返回 201 和 `index_status=failed`，原文件可读；删除旧向量未经确认时不会发布新索引。当前保留 `doubao-embedding-vision-251215` / 2048 与现有 collection；对话使用 `doubao-seed-2-0-lite-260428`。配置维度不符时，重建在旧索引变动前失败；响应/写入维度同样校验。不自动删除、清空或重建现有 collection，禁止将不同模型向量混入。标准协议在隔离环境验证，不要求切换现有课堂模型。

## 数据与验证边界

已提供显式材料两表迁移命令，仅在隔离临时数据库验收；没有对现有课堂数据执行迁移。命令保持源库/文件只读、原始 ID/关联/时间/正文不变；重复执行一致，冲突整体回滚且不覆盖，不迁移账号或 Token 版本。目标时间精度不足时拒绝提交。不要将旧上传目录绑定到新的空 MySQL 库；不要把 MySQL 切回旧 SQLite 快照运行 recovery。备份时停止写入，同步保留身份数据库、MySQL、上传文件及 Qdrant 卷；`down` 保留卷，不执行 `down -v`。

最小本地测试：

```powershell
.venv/Scripts/python.exe -B -m pytest -p no:cacheprovider tests/retrieval/test_core.py tests/retrieval/test_clients.py -q
# 真实 MySQL/Qdrant + 受控 HTTP 网关，仅两份 1–3 KB 文档：
.venv/Scripts/python.exe scripts/check_retrieval_demo.py --no-build
openspec validate add-traceable-vector-retrieval --strict
```

集成脚本生成独立临时 Compose 项目与随机数据库密码，完成后只清理自己的测试卷。受控网关仅存在于测试中，不能代替课堂真实 Embedding/LLM。当前已获授权运行 Bearer 等价全量回归与小规模故障用例；不运行 Go migration Phase 1/2，不做 benchmark。

## 显式迁移与样本初始化（不要用于本次已有课堂库）

在停写维护窗口先成套备份，服务端提供 MYSQL_HOST / MYSQL_PORT / MYSQL_DATABASE / MYSQL_USER / MYSQL_PASSWORD。以下命令只针对操作员明确选定的目标库，不在普通启动时迁移：

```powershell
.venv/Scripts/python.exe scripts/migrate_retrieval_materials.py --source <旧SQLite路径> --uploads <对应上传根目录>
# MySQL 模式初始化样本；沿用已有身份，不重置密码
# MATERIAL_BACKEND=mysql，DATABASE 指定身份库，UPLOAD_ROOT 指定对应目录
.venv/Scripts/python.exe scripts/init_db.py --seed
```

迁移后列出/读取/下载/上传/seed/recovery 均使用同一选定的 MySQL authority；原文件不移动。新增材料不会再写入旧 SQLite 材料表。旧库不可直接作为在线回退快照。

现有容器已保留完整运行环境时，课堂重启优先使用 `docker compose -f docker-compose.retrieval.yml start mysql qdrant app`，避免在未注入原秘密的新终端中执行 up 导致环境被空值替换。不要使用宿主机端口覆盖文件用于课堂默认部署。
