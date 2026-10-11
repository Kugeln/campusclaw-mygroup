# Session 专属验收到 Bearer 的等价映射

用户已确认保留 Bearer。只替换认证机制专属断言，原材料、角色、隔离、上传限制、事务与文件补偿测试继续执行。

| 旧契约 | 当前等价验收 |
| --- | --- |
| 服务端 Session 身份字段 | Bearer 校验后 me 从用户表取得身份；claims 不承载角色/班级 |
| Cookie / CSRF | API 必须显式 Authorization；Cookie 与伪造 CSRF 不能授权 |
| Session 固定 8 小时 / 轮换 | Token 固定 28800 秒，读请求不续期；过期、坏签名拒绝；允许多端同时登录 |
| Session 跨重启持久化 / 撤销 | 同签名秘密及用户版本下跨应用重启有效；版本更新后所有旧 Token 失效 |
| 旧 Session 保存不能复活 | 旧 Token/Cookie 重放及重启均不能绕过撤销版本 |
| Session 读取、登录保存失败 | 身份读取失败不签发 Token且拒绝访问；撤销提交失败不报告成功、不泄漏秘密 |
| 启动创建/清理 Session 表 | Token 不依赖 Session 表；重复启动不改变账号、材料或文件 |
| 页面直接渲染材料 | 页面壳无敏感数据；Bearer API 取得数据；textContent 显示，学生写入仍 403 |

tests/test_auth.py、tests/test_sessions.py、共享夹具与材料页面断言按此调整；独立 tests/token_auth 保留原断言，不修改其他 change。
