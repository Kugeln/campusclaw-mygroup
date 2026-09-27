## Purpose

为教师与学生提供统一的账号密码登录与会话能力，确保受保护页面和接口只对已认证用户开放，并约束密码的持久化方式，避免凭据泄露或匿名访问教学材料。

## ADDED Requirements

### Requirement: Account password login
系统 SHALL 允许已配置的教师与学生使用账号密码登录，成功后建立有有效期的会话；错误凭据 MUST 不建立会话。

#### Scenario: Teacher and student login
- **WHEN** 教师或学生提交自己的正确账号密码
- **THEN** 登录成功，后续请求可以识别该用户及服务端保存的角色、班级

#### Scenario: Invalid credentials
- **WHEN** 提交不存在的账号或错误密码
- **THEN** 返回 401 和统一的认证失败信息，不创建会话，不透露账号是否存在

### Requirement: Protected pages and APIs
系统 MUST 在服务端保护材料页面与 API；登录页和 GET /health SHALL 允许匿名访问。

#### Scenario: Anonymous page visit
- **WHEN** 无有效会话的用户访问材料页面
- **THEN** 跳转登录页，不返回受保护材料内容

#### Scenario: Anonymous API request
- **WHEN** 无会话、伪造会话或会话已过期的请求调用材料列表、上传、详情或文件接口
- **THEN** 返回 401，不返回材料数据且不产生上传副作用

### Requirement: Session termination
系统 SHALL 提供退出登录；退出后的会话 MUST 不再授权任何受保护请求。

#### Scenario: Reuse logged out session
- **WHEN** 用户退出后重用之前的会话访问材料接口
- **THEN** 返回 401

### Requirement: Password hashing
系统 MUST 仅持久化使用独立随机盐的自适应密码哈希，禁止保存明文密码或可逆加密密码；响应和日志 MUST 不包含密码或密码哈希。

#### Scenario: Provisioned account storage
- **WHEN** 创建两个使用相同密码的账号并检查持久化数据
- **THEN** 两个账号保存不同的加盐哈希，均能验证正确密码且拒绝错误密码，不存在明文密码字段或副本

#### Scenario: Credential output
- **WHEN** 执行账号初始化或成功及失败的登录请求
- **THEN** 响应和日志不输出密码或密码哈希
