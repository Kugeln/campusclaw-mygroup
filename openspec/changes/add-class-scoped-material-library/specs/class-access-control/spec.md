## Purpose

以服务端保存的角色和单一班级归属作为访问教学材料的授权依据，明确教师与学生的上传权限，保证列表、记录和文件访问均不能越过班级数据边界。

## ADDED Requirements

### Requirement: Single class identity
系统 MUST 为每个用户保存且仅保存一个有效班级和教师或学生角色；授权 MUST 使用服务端可信身份数据，不信任客户端传入的用户、角色或班级。

#### Scenario: Invalid account membership
- **WHEN** 初始化账号缺少有效班级或提供多个班级或不支持的角色
- **THEN** 拒绝创建该账号，不产生可登录的无效身份

#### Scenario: Forged upload identity
- **WHEN** A 班教师上传时在请求中附带 B 班 class_id、他人的 uploader_id 或伪造 role 字段
- **THEN** 请求返回 400，不创建记录或保留文件，客户端不能改变上传归属

### Requirement: Teacher only upload
上传接口 MUST 在服务端要求已认证用户为教师；拒绝请求 MUST 不写入材料记录或持久化文件。

#### Scenario: Student calls upload directly
- **WHEN** 已登录学生绕过页面直接调用上传 API，包括附带教师角色字段的请求
- **THEN** 返回 403，不产生材料或文件

#### Scenario: Teacher uploads within own class
- **WHEN** 已登录教师提交有效材料
- **THEN** 允许上传，班级和上传者由该教师的服务端身份确定

### Requirement: Class scoped reads
系统 MUST 在服务端对材料列表、详情及文件读取实施班级限制。跨班材料和不存在的材料 SHALL 均返回 404，避免泄露材料是否存在；本班教师和学生 SHALL 均可读取。

#### Scenario: Class filtered list
- **WHEN** A 班教师或学生查询材料列表，且 A 班和 B 班均有材料
- **THEN** 结果及任何计数仅包含 A 班材料

#### Scenario: Forged list scope
- **WHEN** A 班用户通过列表查询参数指定 B 班 class_id
- **THEN** 返回 400，不返回 B 班记录或计数

#### Scenario: Cross class resource identifier
- **WHEN** A 班教师或学生使用已知 B 班材料 ID 请求详情或下载文件
- **THEN** 返回 404，不返回该材料的元数据、路径或文件字节

#### Scenario: Same class resource read
- **WHEN** 本班教师或学生请求本班已成功上传的材料详情或文件
- **THEN** 返回对应元数据或文件内容

#### Scenario: Direct storage access
- **WHEN** 用户尝试通过静态 URL 或存储路径直接访问上传文件
- **THEN** 不提供绕过认证和班级校验的文件访问路径
