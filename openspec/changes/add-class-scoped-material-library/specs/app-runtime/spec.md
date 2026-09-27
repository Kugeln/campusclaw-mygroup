## Purpose

定义应用可重复运行的交付约束，通过 Docker Compose 启动服务并保存账号和教学材料，提供可被自动化检查的健康端点，同时防止部署密钥进入客户端或版本控制。

## ADDED Requirements

### Requirement: Compose startup and durable data
项目 SHALL 提供 Docker Compose 启动方式及环境配置、账号初始化说明。完成文档规定的配置后，docker compose up --build MUST 能启动应用；普通容器重建或重启 MUST 保留用户、班级、材料记录和上传文件。

#### Scenario: Fresh deployment
- **WHEN** 操作者按文档配置环境、启动 Compose 并执行班级和账号初始化命令
- **THEN** 应用可访问，初始化的教师和学生均能登录

#### Scenario: Restart persistence
- **WHEN** 教师上传成功后执行不删除数据卷的 Compose 停止、重建和启动
- **THEN** 原账号仍能登录，本班列表包含原材料且文件内容完整
- **AND** 对应 KnowledgeEntry 的解析文本及 material_id、class_id 关联仍然存在且正确

### Requirement: Preloaded core data
初始化/种子数据 MUST 创建班级 A 和班级 B，提供教师 A（班级 A）、学生 A1（班级 A）、学生 B1（班级 B）的样本账号，密码 MUST 仅以安全加盐哈希保存。系统 MUST 提供至少一条 A 班和一条 B 班标题可明确区分的样本材料；标题可使用已有原始文件名表达，无需新增标题字段。样本材料 MUST 包含原始文件、Material 及关联同班的 KnowledgeEntry，满足材料库的持久化约束。上述数据仅用于验证登录、角色权限、班级隔离及本班材料查询。

系统 MUST 建立班级、用户、讲义、作业、助手、技能六类可持久化核心结构。讲义结构 SHALL 复用 Material 与 KnowledgeEntry，不另建业务能力；讲义占位种子可为 0～1 条，但不替代上述每班至少一条样本材料的要求。作业、助手、技能各自仅要求结构存在，各允许 0～1 条占位种子数据，不要求业务功能、页面或 API；不得由此引入作业提交与批改、对话助手、RAG、向量检索或智能问答。

#### Scenario: Initial classes and sample accounts
- **WHEN** 在空数据卷上首次完成初始化及种子数据命令
- **THEN** 存在班级 A 和班级 B
- **AND** 存在教师 A、学生 A1、学生 B1，角色和班级归属分别为教师/A、学生/A、学生/B
- **AND** 样本账号可使用配置的密码登录

#### Scenario: Sample passwords are hashed
- **WHEN** 初始化完成后检查样本账号的密码字段
- **THEN** 保存的是安全加盐哈希而非明文或可逆加密密码
- **AND** 正确密码验证成功，错误密码验证失败，初始化输出不包含密码或哈希

#### Scenario: Distinguishable class sample materials
- **WHEN** 初始化后检查样本材料并分别以学生 A1、学生 B1 查询材料列表
- **THEN** 至少存在一条标题可辨识为 A 班和一条标题可辨识为 B 班的样本材料，原始文件与两类记录完整且归属正确
- **AND** 学生 A1 仅查到 A 班材料，学生 B1 仅查到 B 班材料
- **AND** 跨班请求样本材料详情或文件返回 404

#### Scenario: Core structures without additional business features
- **WHEN** 初始化完成后检查持久化结构及占位数据
- **THEN** 班级、用户、讲义、作业、助手、技能六类结构均存在，讲义复用材料库结构
- **AND** 作业、助手、技能各有 0～1 条占位数据；讲义可有 0～1 条额外占位数据，同时保留两班必需样本材料
- **AND** 验收不要求作业、助手或技能的业务功能，也不新增对应页面或 API

### Requirement: Server only secrets
系统 MUST 仅通过服务端环境变量接收密钥；真实密钥 MUST 不出现在源码、提交的配置、镜像构建参数、浏览器资源、API 响应或日志中。缺少必需密钥时 MUST 拒绝启动，不使用内置默认密钥。

#### Scenario: Missing required secret
- **WHEN** 未提供必需的服务端密钥时启动应用
- **THEN** 启动失败并提示缺少的配置项名称，不输出密钥值

#### Scenario: Secret exposure check
- **WHEN** 使用测试密钥构建并运行应用，检查版本控制文件、镜像配置、浏览器资源、响应与日志
- **THEN** 除服务端运行时环境之外不出现测试密钥值，示例配置仅包含占位说明

### Requirement: Health endpoint
应用 SHALL 提供不要求登录的 GET /health；应用能访问数据库及材料存储目录时 MUST 返回 200 和 {"status":"ok"}，任一依赖不可用时 MUST 返回 503 和 {"status":"unhealthy"}，不得泄露连接信息、路径或密钥。

#### Scenario: Healthy application
- **WHEN** 数据库和材料存储可访问时匿名请求 GET /health
- **THEN** 返回 200 和约定的健康 JSON，不跳转登录页

#### Scenario: Unhealthy dependencies
- **WHEN** 数据库不可访问或材料存储不可读写时请求 GET /health
- **THEN** 返回 503 和约定的不健康 JSON，不包含内部错误详情
