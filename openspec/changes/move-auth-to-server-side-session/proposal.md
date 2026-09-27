## Why

当前 Flask signed-cookie session 虽然经过签名和数据库身份核验，但身份字段仍存储在客户端 Cookie，未满足课程对严格服务端 Session 的要求。需要将会话状态移入服务端持久化存储，同时保持现有登录、角色权限、班级隔离及材料行为。

## What Changes

- 使用现有 SQLite 数据库保存会话，浏览器仅保存服务端生成的高熵、不透明 Session ID；身份、角色、班级和 CSRF 状态不再序列化进 Cookie。
- 每次受保护请求从服务端会话和用户记录确定可信身份；保留用户存在性、角色、班级、session_version 核验，以及 teacher/student 两角色限制。
- 保留简单的登录 ID 轮换、现有 8 小时有效期及退出全部设备失效；过期记录仅在启动时清理，存储失败不得绕过认证。
- **BREAKING**：旧 signed-cookie session 不再接受，升级后已有登录用户需要重新登录；API 路径、成功/拒绝状态码及材料权限保持不变。
- 为新库和已有数据库提供幂等、增量的会话表初始化；复用现有 Docker Compose 持久卷，不引入 Redis、ORM 或额外服务。
- 复用既有测试，补充服务端存储、轮换/退出/过期及 A/B 双向隔离缺失的覆盖；新旧库兼容仅做一次幂等准备与数据保留验证。
- 不实现 revision 乐观锁、并发冲突恢复框架、专用维护命令、故障注入矩阵或额外容器迁移演练；保留必要的失效校验和禁止恢复已删除会话的简单保存规则。

## Capabilities

### New Capabilities

- `server-side-session`: 定义不透明 Cookie、SQLite 会话生命周期、可信身份解析、持久化升级和现有授权行为兼容契约。

### Modified Capabilities

无已发布 capability 需要提交 MODIFIED delta：`openspec list --specs` 当前为空。已有 `user-auth`、`class-access-control`、`material-library`、`app-runtime` 仅存在于未归档 change `add-class-scoped-material-library` 中，本 change 不重复创建或改写它们；新增会话 capability 是对其行为契约的补充。旧 change 的 design 中 signed-cookie/no Session entity 决策由本方案取代，旧 artifact 本次保持原样；后续同步/归档须保留两者的依赖关系与本方案优先级。

## Impact

- 预计实现涉及 `app/__init__.py`、`app/auth.py`、新增会话模块、`app/schema.sql`、`scripts/init_db.py`、`scripts/serve.py`；仅在接入会话所需范围调整 CSRF 或身份读取。
- 会话表使用当前 `DATABASE` 和持久卷；保留 `SECRET_KEY` 必填、Cookie 安全配置和现有启动先检查/初始化再服务的边界。
- `tests/test_auth.py`、数据库结构/启动测试及新增会话测试覆盖变更。适配新增表的精确表集合断言，以及原先包含整个数据库的材料快照：材料无副作用断言继续覆盖全部原业务表和文件，会话生命周期另行断言，不把合法登录会话写入误判为材料副作用。不删除或弱化原安全及业务断言；全量现有测试必须通过。
- 不新增管理员、注册或角色管理，不修改无关 UI、材料解析/存储业务、权限规则和班级范围，不新增基础设施。
- 本次只更新和校验现有规划 artifact；不执行迁移、修改业务代码或启动 Apply。
