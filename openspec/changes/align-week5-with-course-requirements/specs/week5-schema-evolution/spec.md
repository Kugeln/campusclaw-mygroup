## Purpose

保证课程对齐所需的数据结构通过可审计的离线增量升级进入现有系统，保留课堂与历史数据并检测不完整或漂移状态。升级后的服务须提供明确就绪与恢复行为，避免把MySQL非原子DDL、旧配置回填及严格catalog兼容性误当作无风险操作。

## ADDED Requirements

### Requirement: SE1 Explicit resumable incremental upgrade
系统 SHALL 提供从现有版本到课程对齐版本的显式离线增量准备，兼容新库和已有v1/v2升级路径；重复或中断重试不重建现有表/数据，不重复种子/关系。HTTP启动 MUST 只验证完整schema与catalog，不运行DDL或隐式修复；部分升级或漂移拒绝ready，错误不泄露秘密。正式执行需确认目标、停写窗口、备份和恢复方案，不因实现测试获准而自动升级业务库。

#### Scenario: Fresh upgrade retry and interrupted execution
- **WHEN** 在隔离MySQL分别准备新库、v1/v2旧库、重复执行及逐步中断后重试
- **THEN** 收敛到同一完整新catalog，无重复关系/种子，用户/材料/消息标识保持；部分升级启动拒绝ready，无自动修复

### Requirement: SE2 Legacy configuration and history preservation
升级 MUST 保留用户、Token版本、材料/文件/切片/向量、助手/技能/会话/消息ID、历史正文/引用/技能审计。旧内置技能保持ID/启用状态，补出的规范source标记系统迁移来源，不能伪造教师原文。当前合法本班JSON绑定映射为关系，空集保持空集，不自动补全全班；未知归属、非法引用、名称冲突或无法安全解析的数据须预检报告并阻止升级，不静默删除/截断/猜班级。无班级旧占位行隔离保留。回填完成后的重试不得复活已解绑关系或覆盖新source。

#### Scenario: Scoped backfill preserves identifiers
- **WHEN** 旧库含固定技能、非空/空JSON绑定和无班级占位行，完成合法映射后教师编辑或解绑再重试升级
- **THEN** 原ID/启用/历史hash保持，空集零关系，旧占位不可使用，source来源可辨；重试不覆盖编辑或重建解绑

#### Scenario: Unsafe legacy data blocks mutation
- **WHEN** 预检发现外班/不存在资源、重复或超限技能名、未知scoped技能或无法映射归属
- **THEN** 输出脱敏冲突清单并停止升级，既有行不被自动改名/截断/重归属，修复或明确映射后才可重试

### Requirement: SE3 Complete catalog and rollback safety
新版本 SHALL 校验完整表/列/索引/复合外键/唯一性及现有ngram约束，与离线初始化就绪版本一致；不能只验证新增字段，DDL和数据回填未全部完成不得发布就绪。恢复说明 MUST 明确MySQL多步DDL不可事务回滚、旧严格catalog二进制不能直接连新库、升级后写入的恢复损失风险。禁止自动删表列/卷/collection或跳过catalog假装回退；备份恢复需在独立目标验证，优先前向修复；维护窗口暂不接入Week5流量不等于现有RAG开关可选择性禁用Week5。

#### Scenario: Detect structural drift
- **WHEN** 删除必需索引/外键、篡改列定义或就绪版本、或使ngram不符
- **THEN** 新服务拒绝ready；恢复预期结构后通过，初始化不得仅凭存在标记成功

#### Scenario: Rollback rehearsal without classroom destruction
- **WHEN** 在隔离副本演练升级前备份恢复及升级后新增记录处理
- **THEN** 验证备份可恢复和版本一致，报告新增记录丢失/重放边界；不删除原库/卷/Qdrant，不能宣称直接启动旧二进制即可回滚
