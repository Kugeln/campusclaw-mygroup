## Why

第 4 课已有班级知识库与可追溯单次问答，但尚无教师教学提示词、解题引导技能、私有多轮会话或流式回答。第 5 课将这些能力组合成学生解题助手，同时保留原认证、权限、检索及无证据行为。

## What Changes

- 新增班级助手提示词保存、版本快照与教师管理权限。
- 新增一个内置“解题引导”技能及教师启停；每轮记录实际加载情况。
- 新增学生私有会话、消息历史、检索先行的多轮回答与 POST SSE。
- 持久化回答文本、完成状态、提示词版本、实际技能及来源快照。
- 新增可浏览和调试的 OpenAPI/Swagger，记录官方契约与现有 API 的差异。
- 规划 `apitest/` 按 URL/方法组织的接口脚本、数据库断言、一键 HTML 报告及第 4 课回归。
- 保留 `/api/auth/*`、材料 API、`/api/search`、`/api/ask` 既有契约，不改 Token 生命周期，不重复三路检索。
- 用户已选择默认 Go 后端：先补接第 4 课检索、索引与单次问答，再实现第 5 课；Python 实现作为行为参考和回归依据，不作为运行依赖。
- 本轮严格限定 OpenSpec 规划，只允许创建或修改本变更的 proposal.md、design.md、tasks.md 和 specs；不修改业务代码、数据库、Docker 配置或现有迁移规约。确认 Go 架构不构成实施授权，规约验证后停止等待审核。

### 第 4 课 Go 接入前置依赖

Go 当前缺失切片与索引、Embedding/Qdrant 接入、班级范围 keyword/vector/hybrid 检索、来源校验、单次问答及无命中处理。这些是已有第 4 课能力在目标后端上的接入前置项，不计为第 5 课新教学能力。后续须先通过 Phase 1 的第 4 课 Go 闭环验收，才能进入第 5 课功能实施；本轮只规划该依赖。

### 第 5 课新增功能

教师提示词、技能启停与实际技能审计、私有多轮会话、SSE、消息与来源快照、对应前端及文档测试属于本课新增。第 5 课调用通过前置验收的共享检索服务，不另建检索算法。

## Capabilities

### New Capabilities

- `tutor-configuration`: 班级提示词及最小解题技能的保存、权限、选择和版本审计。
- `student-streaming-conversations`: 私有多轮会话、检索引用、SSE、失败断连及消息持久化。
- `week5-api-documentation-and-acceptance`: 官方接口兼容边界、Swagger、接口测试规划及第 4 课回归门槛。

### Modified Capabilities

无。`openspec list --specs` 无主规约，已有需求位于未归档的 change；本变更不修改或归档其他 change，不重定义其认证和检索契约。

## Impact

预期涉及 Go 检索接入、现有认证调用方、助手/技能占位模型、MySQL、对话网关、前端交互及接口文档；已有检索算法、Embedding/Qdrant 数据和材料身份保持。默认 Go/MySQL 与 Flask RAG（SQLite 身份库 + MySQL 材料库）并非同一个运行栈，不能假设可直接共享表或用户 ID。本变更明确补接 Go 所缺能力，保留现有 UUID，通过文档记录与官方整数材料 ID 的兼容差异，不扩大为课堂数据整库合并。

不包含作业提交批改、错题讲解、多 Agent、MCP、异步队列、关怀对话、技能市场、通用脚本执行；不引入复杂 Agent 框架。
