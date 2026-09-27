## Purpose

提供本项目内部的教学材料库，保存教师上传的原始文件及可查询的材料记录，使同班用户能够在上传成功后查看列表和读取内容，并保证失败上传不形成可见的不完整材料。

## ADDED Requirements

### Requirement: Persist uploaded material
系统 SHALL 接收教师上传的一个非空 .txt 或 .md 文件，并解析为 plain text。Material MUST 保存材料元数据，包含材料 ID、原始文件名、文件大小、上传时间、上传者和班级；KnowledgeEntry（knowledge_entries）MUST 保存解析文本，并关联该 material_id 和同一 class_id。两类记录 MUST 在同一数据库事务中写入。成功响应 SHALL 为 201 并包含材料 ID；只有原始文件、Material 和 KnowledgeEntry 均成功持久化后才能返回成功，本班教师和学生刷新列表 SHALL 均能看到新材料。学生 MUST 保持只读，无上传、修改或覆盖材料的权限。

#### Scenario: Upload parse and knowledge entry
- **WHEN** 教师 A 上传合法 .txt 或 .md 且解析成功
- **THEN** Material 中新增对应材料元数据记录，班级归属为教师 A 所在班级
- **AND** KnowledgeEntry 中新增关联该 material_id 和 class_id 的 plain text 记录
- **AND** 原始文件成功持久化，返回 201 和材料 ID
- **AND** 本班教师刷新列表可查询到新材料

#### Scenario: Same class student sees uploaded material
- **WHEN** 教师 A 成功上传材料后，同班学生 A1 查询本班列表
- **THEN** 学生 A1 能看到该材料
- **AND** 学生仍无上传、修改或覆盖权限，直接上传返回 403；任何修改或覆盖尝试均不能改变文件、Material 或 KnowledgeEntry，系统不因此新增修改接口

#### Scenario: Upload and query
- **WHEN** A 班教师成功上传文件后，本班教师或学生立即查询材料列表
- **THEN** 列表包含同一材料 ID 的记录，详情可查，下载内容与上传文件一致

#### Scenario: Empty class
- **WHEN** 已登录用户查询尚无材料的本班列表
- **THEN** 返回 200 和空列表

### Requirement: Upload validation and failure consistency
系统 MUST 拒绝缺失文件、空文件、不支持的文件格式、文本解析失败和超过配置大小上限的上传；MVP 仅支持 .txt、.md（扩展名不区分大小写）。文本 MUST 按 UTF-8（允许 BOM）严格解码，无法解码或包含 NUL 时判定解析失败；Markdown 保留为文本，不渲染或执行。失败上传 MUST 不留下本次 Material 或 KnowledgeEntry 记录，不产生列表可见记录，并清理本次临时或孤立文件。材料 SHALL 作为附件下载，不执行上传内容或将其作为可执行页面公开。

#### Scenario: Unsupported format or parse failure
- **WHEN** 教师上传不支持的文件格式、空文件或文本解析失败
- **THEN** 返回 400
- **AND** 本次 Material 与 KnowledgeEntry 均不得留下不完整记录
- **AND** 不保留本次孤立上传文件或临时文件

#### Scenario: Invalid file input
- **WHEN** 教师提交缺失文件或空文件的上传请求
- **THEN** 返回 400，不创建材料记录或持久化文件
- **AND** 不创建 KnowledgeEntry

#### Scenario: Oversized upload
- **WHEN** 教师上传超过服务端配置大小上限的文件
- **THEN** 返回 413，不创建材料记录，清理本次临时文件
- **AND** 不创建 KnowledgeEntry

#### Scenario: Storage or database failure
- **WHEN** 保存文件、写入 Material 或 KnowledgeEntry、或提交双表事务失败
- **THEN** 返回 5xx，不返回成功或展示不完整材料，并清理残留文件；进程意外中断的残留在下次启动恢复时清理
- **AND** 回滚本次 Material 与 KnowledgeEntry 写入，不留下单表成功的记录

#### Scenario: Filename collision or traversal
- **WHEN** 教师上传同名文件或包含路径分隔符的文件名
- **THEN** 不覆盖其他材料，不写入材料存储目录之外的位置，使用独立材料 ID 保存内容

#### Scenario: Download as attachment
- **WHEN** 本班用户下载包含 HTML 或脚本的材料
- **THEN** 文件以附件形式返回，不作为应用页面执行
