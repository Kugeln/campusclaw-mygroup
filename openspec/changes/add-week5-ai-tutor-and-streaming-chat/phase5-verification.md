# Phase 5 OpenAPI / Swagger 验收（2026-10-14）

保持feature/week5-phase1-go-rag及全部未提交修改，只使用现有隔离MySQL/Qdrant、独立测试Go进程与Nginx代理。未部署或修改课堂Demo，未迁移数据库。

脚本scripts/generate_week5_openapi.py生成本地实际OpenAPI 3.0.3：23个URL/方法，覆盖登录/身份/退出、材料/附件/重建、GET search、POST ask、助手/技能、会话与SSE及三份文档入口。声明现有Bearer安全、请求/响应/错误、教师与学生权限、UUID/1-based及SSE事件扩展，保留官方差异。JSON是服务端实际提供的唯一契约；Go测试核对它与路由注册完全一致。没有新增官方别名或课外API。

匿名/openapi.json、/swagger/index.html和/swagger/可用。Swagger UI 5.17.14资源及Apache-2.0许可证存于本地，不依赖运行时CDN；Authorize仅页面内存，不保存Token。独立SSE面板使用POST Bearer fetch及Phase 4的真实增量解析器，显示meta/delta/done/error、草稿/终态并说明EOF读历史策略。

`go test ./internal/http -run 'TestDocumentation|TestMaterialPages'`通过；23个操作与真实注册路由及Bearer声明一致，文档静态资源匿名200，未知文档路径404。

`powershell -NoProfile -File tests/go_rag/run_phase4.ps1 -Stage phase5`退出0：真实Edge经隔离Nginx访问Swagger，未授权Try it out返回401、通过实际Authorize对话框后返回200；教师助手读取200；学生SSE首delta在生成结束前展示，三段正常done、故障error无done；无Token URL、无Token浏览器持久化。失败的首轮为授权按钮测试定位不匹配，修正定位后完整重跑，未更改接口来掩盖失败。

初次完整通过记录在runtime/campusclaw-week5-rag-ee706bec/phase5-e55130a2；最终补齐Swagger实际Execute的403/404及最终SSE文档示例后，phase5-544e9700也通过，浏览器及夹具退出码为0。报告不入Git。Phase 5三项完成时进度25/32，随后按授权开始Phase 6。2.4及真实模型教学效果未执行。

新增scripts/generate_week5_openapi.py、internal/http/documentation.go及documentation_test.go、web/swagger/openapi.json、index.html、init.js、本地vendor资源及许可证、tests/go_rag/check_swagger_browser.py；修改handler.go注册文档路由，以及run_phase4.ps1增加独立阶段选择。api-differences.md增加已实现状态，不修改原参考差异或既有接口。
