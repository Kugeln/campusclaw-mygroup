# Phase 0 baseline

2026-09-27，Windows，Python 3.12.7 / pytest 7.4.4。
`python -m pytest -q`: **36 passed in 186.42s**。使用 tests/conftest.py 的临时 SQLite、文件和随机凭据，未操作真实数据。原业务、测试及规划文件 SHA-256 见 baseline-sha256.json（生成于实施前，排除缓存、环境秘密文件）。

## 路由及后续等价验证映射

| 契约 | 当前证据 | 后续 Go 验证 |
|---|---|---|
| GET /login；POST /api/auth/login：CSRF403，凭据失败统一401，成功200 redirect | auth.py/csrf.py；test_auth.py | auth/security unit + HTTP 登录矩阵 |
| POST /api/auth/logout：认证→CSRF，204；全设备版本失效 | test_auth.py、test_sessions.py | session repository/unit + 双客户端 HTTP |
| GET /materials：匿名302；教师上传控件、学生只读、正文textContent | test_auth.py、test_uploads.py | HTTP/模板回归 |
| GET /api/materials：匿名401、query400、本班items按created_at/id倒序 | db.py/routes.py；test_db.py、test_material_routes.py | MySQL scoped repository + HTTP |
| POST /api/materials：认证→teacher→CSRF→输入；学生403、教师伪造字段400、成功201 material_id | test_uploads.py | unit/HTTP A/B两方向伪造矩阵 |
| GET详情：同班200 id/original_name/size/created_at/uploader/plain_text，外班/不存在404 | test_material_routes.py | repository + HTTP |
| GET文件：同班原字节附件、nosniff、private no-store，外班404；无静态直达 | test_material_routes.py | HTTP/代理测试 |
| 单txt/md、20MiB、UTF8/BOM、NUL/空/非法400、超限413、路径净化、无修改接口405 | test_uploads.py | material/storage unit + HTTP |
| SID随机/摘要、服务器身份核验、轮换、8小时不续期、删除/过期/伪造拒绝 | test_auth.py、test_sessions.py | session/security unit + MySQL + HTTP |
| Session读写故障500、不发成功Cookie、不泄密、旧保存不复活 | test_sessions.py | repository故障 + service/HTTP |
| 双表事务、rename/commit失败补偿、孤立清理和缺失拒绝启动 | test_recovery.py | material/storage故障 + MySQL事务 |
| GET /health：匿名200/503 JSON，检查库及文件读写 | test_health.py | health unit + 实际MySQL故障 |
| 初始化无导入副作用、幂等、种子哈希、失败重试 | test_factory.py、test_seed.py、test_startup.py | config/admin unit + MySQL集成 |
| 八表/约束/占位无业务 | test_schema.py | MySQL schema集成（Phase 2） |

旧测试全部保留；Flask内部 session_transaction/SQLite trigger 测试不直接冒充 Go 验证。Go无SECRET_KEY要求，正式cutover重新登录是已批准差异；Go新会话安全测试不减。

## 实际 SQLite schema

来源 app/schema.sql + app/session_schema.sql；主键、非空、唯一、外键由 test_schema.py 验证。

| 表 | 全部字段 |
|---|---|
| classes | id, name |
| users | id, username, password_hash, role, class_id, session_version |
| sessions | sid_hash, data, expires_at |
| materials | id, class_id, uploader_id, original_name, size, content_type, storage_key, created_at |
| knowledge_entries | id, material_id, class_id, plain_text, created_at |
| assignments / assistants / skills | 各自 id, name |

users.role CHECK teacher/student；用户class外键RESTRICT；materials.class/uploader外键、size>0、storage_key唯一、(id,class_id)唯一及(class_id,created_at,id)索引；knowledge_entries.material_id唯一、(material_id,class_id)复合外键；created_at为UTC毫秒文本；sessions为SHA-256摘要主键/JSON文本/Unix到期秒。占位三表不新增业务。本轮不创建MySQL业务schema、不迁移数据。
