# Phase 4 前端与代理验收（2026-10-14）

当前分支feature/week5-phase1-go-rag，先补齐Phase 3专项，再实施Phase 4；没有进入Phase 5–6。保留全部未提交修改，原受保护文件及其他change哈希差异0；未修改课堂Demo、正式数据库、schema或Compose配置。

## 实现

- Go内嵌/tutor教师配置和学生会话界面。教师保存名称、提示词、启用状态及固定技能绑定，保留已有材料范围；本班技能独立启停。学生看不到配置写入入口，服务器403仍有效。
- 学生选择助手、创建及查看私有会话，继续提问、刷新读取历史；显示状态、实际技能、材料标题和切片序号。引用按钮先校验当前材料权限，再显示历史原文快照；下载通过统一Bearer请求获得文件，不把Token放入URL。
- POST fetch + ReadableStream +流式TextDecoder，按真实meta/delta/done/error逐段显示；异常EOF不视为成功，读取历史确认done丢失。重复点击在请求生命周期内受busy保护，切换会话/退出/认证清理取消旧流；失败、中断保留可辨认状态。
- /knowledge使用原GET search q/mode和POST ask question/mode契约，材料页保持原行为并增加导航。
- Nginx仅针对会话消息路径关闭buffer/cache，HTTP/1.1、Bearer透传、120秒超时匹配15秒心跳；普通路径上传策略不变。

## 验收与命令

`powershell -NoProfile -File tests/go_rag/run_phase4.ps1`最终退出0。真实Edge浏览器经过隔离Nginx → Go →现有隔离MySQL/Qdrant，受控HTTP网关真实流式输出；10组验收通过：

1. 教师提示词保存、刷新持久化和技能启停。
2. 新Go服务实例读取前一实例会话历史及引用。
3. 网关发送首段后等待3秒，浏览器在2秒内显示首段；生成时重复点击不新增消息；真实引用阅读/附件下载和刷新持久。
4. 模型流中失败显示failed，浏览器停止显示interrupted。
5. 网络故障注入只移除真实服务完成提交后的done，浏览器从真实历史确认completed；此注入不作为增量时序证据。
6. B班不能选择A班助手，A班会话及附件访问404。
7. 新会话无命中精确固定文本、无虚假引用。
8. 学生助手/技能修改403，旧材料、检索、单次问答页面可用。
9. 代理上传恰好20MiB成功，超出1字节413。
10. API请求有Bearer Header、无Token URL/Cookie；401清除身份并回到登录。

记录位于Git忽略的`runtime/campusclaw-week5-rag-ee706bec/phase4-dc062e0f/`：browser.json、result.json、fixture.log、teacher.png、student.png、student-mobile.png。截图确认移动端无横向溢出；视觉检查后给长会话列表增加50vh滚动上限。

`node --test tests/go_rag/sse.test.cjs`：3项通过，覆盖逐字节UTF-8、CRLF、多行data、心跳、首段先到、error、EOF及非法事件顺序。

`go test ./internal/http ./internal/conversation ./internal/rag ./internal/tutor ./internal/repository/mysql ./internal/auth ./cmd/server`：全部通过（部分缓存）。本地GOCACHE使用runtime/go-cache；原默认缓存权限失败没有视为通过。`git diff --check`通过，仅LF/CRLF转换提示。`openspec validate add-week5-ai-tutor-and-streaming-chat --strict`退出0，变更有效。

## 文件与测试隔离

新增：internal/http/web/templates/tutor.html、knowledge.html；web/static/tutor.js、tutor.css、sse.js、knowledge.js；internal/http/tutor_browser_integration_test.go；tests/go_rag/sse.test.cjs、check_tutor_browser.py、run_phase4.ps1；本验收记录及phase3-supplement-verification.md。

修改：internal/http/pages.go、pages_test.go、web/templates/materials.html；deploy/nginx.conf；Phase 3的conversation_integration_test.go；本change/tasks.md及phase3-verification.md补验链接。未改原统一api.js、登录脚本、材料脚本及原样式，未修改其他change。

测试数据库、Qdrant和内部网络均复用。每次验收只新增独立Go测试容器、专用collection及Nginx测试代理；internal网络不发布宿主机端口，因此仅新代理加入既有bridge及测试内部网络，端口仅绑定127.0.0.1。代理配置使用deploy/nginx.conf，只替换测试上游名称并设置前台运行；没有应用到课堂实例。早期代理端口及参数转义失败均保留记录，未算通过。现有容器、网络、卷和collection没有删除或重建；测试代理容器保留，Go测试进程已按验收结束标志退出。

最终浏览器夹具文件保存在该run目录uploads子目录，不在课堂文件卷。20MiB测试验证代理和既有材料保存边界，夹具上传路由未对该大文件执行Embedding索引，不能把这项视为重新完成Phase 1大文件RAG验收。重启证据为前一Go测试进程退出后，新进程用新数据库连接读取同库历史；没有重启Docker Desktop或共享服务。

## 当前状态

Phase 3七项及Phase 4四项通过。整体22/32完成，10项未完成：真实模型教学效果2.4、Phase 5三项、Phase 6六项。受控模型不能证明真实教学效果；没有真实外部模型联调，不声称全量官方API兼容或正式部署通过。停止等待审核。
