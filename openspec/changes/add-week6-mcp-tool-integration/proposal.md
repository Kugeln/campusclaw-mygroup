> **Phase 6 最终状态（2026-10-19）：42/42，按用户批准的项目调整标准完成。** 6.5/T71采用受控HTTP模型、真实MCP和自动技术复核；6.7/T68采用独立账号模拟交叉验证4/4。未调用真实付费模型、未执行用户人工审阅、未邀请真实同伴，不代表课程原始参与要求已完成。72项技术断言、strict及保护检查通过；下方旧状态及原始要求保留历史时点。详见[最终报告](phase6-verification.md)、[逐项覆盖](phase6-coverage.md)和[用户调整](phase6-user-decisions.md)。已停止，不提交、推送或归档。

## Why

第五课技能已有knowledge_search，但缺少独立可发现、受鉴权约束并可审计的外部工具。第六课在现有助教链路中接入同仓MCP网络服务，以确定性白名单调用提升公式提示和单位换算能力，保护原有班级隔离及流式回答。

**本版：官方要求对齐审核已完成，待用户审核，尚未实施。** 已亲自读取任务清单、概览及全部9个课件子页，并核对官方OpenAPI的8个MCP操作；来源冲突与项目扩展详见追踪表，不能声明业务已验收。

## What Changes

- 新增同仓mcp-server/独立HTTP JSON-RPC服务、/health、Bearer、initialize/list/call及两个真实工具；项目选formula_hint、unit_convert，旧草案unit_converter不作默认别名。
- 以官方8个管理操作为兼容基线，采用base_url/auth_token输入、has_auth/connected/tool_count输出；移除不必要的额外详情GET和discover接口，重新发现复用connect。
- 规划三张MCP表和显式v4/catalog升级；审计tool_ref/detail、最近50上限100、只追加终态；无审计删除级联缓存，有审计删除503，旧引用改名/删除失配denied无新审计。
- 扩展技能工具栏多分隔符和32字引用规则，保存四道校验，按命中启用技能逐个调用；不加入模型自主工具选择。
- meta前备齐工具结果，meta.tool_calls兼容ref/status/text，无调用省略；危险句式或超过1200字整条丢弃换固定说明，工具仅作user数据，保留RAG/引用/历史。
- 最小扩展现有tutor的MCP页签及学生反馈，沿用注解生成Swagger；72需求/72场景保留编号，42项阶段任务，加入真实同伴四项人工验收。HTML为官方建议、项目交付要求。

## Capabilities

### New Capabilities

- `week6-mcp-service`: 独立网络协议、两工具、鉴权及执行边界。
- `week6-mcp-management`: 官方8接口、三表、凭据、缓存与审计例外。
- `week6-skill-tools`: 多分隔符、引用规则、保存/运行校验和改名失效。
- `week6-tool-conversations`: 确定性meta前调用、ref/status/text、安全数据与引用边界。
- `week6-mcp-acceptance`: 最小UI、Swagger、官方双向追踪、真实同伴及项目测试。

### Modified Capabilities

无已归档main specs，仍使用现有五个ADDED能力，不重建change、不改旧Week5。旧MCP禁令限于Week5旧范围，由本次经审核的Week6增量取代；其余现有行为继续保护。

## Impact

后续预计影响internal/tutor/conversation/http/repository/mysql、cmd配置与初始化、同仓mcp-server、Compose及环境模板、现有tutor前端与注解生成链。保护非流式/api/ask、JWT当前Principal/撤销机制、材料UUID、数据库旧数据与未提交文件。本轮仅修订本change文档，无代码、迁移、Docker、真实模型或Git提交。
