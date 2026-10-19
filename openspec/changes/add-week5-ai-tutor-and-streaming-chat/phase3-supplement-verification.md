# Phase 3 专项补验（2026-10-14）

复用campusclaw-week5-rag-ee706bec隔离MySQL/Qdrant和既有网络。只新增本轮材料、文件和专用collection，不重建或删除旧资源，不迁移数据库。

`powershell -NoProfile -File tests/go_rag/run_phase3.ps1`退出0：三个流解析/预算测试和真实MySQL/Qdrant会话综合测试通过，综合测试5.835秒。结果在Git忽略的`runtime/campusclaw-week5-rag-ee706bec/phase3-f50fa4cb.json`及同名日志。之前一次失败由新增测试文本经PowerShell管道转码为问号造成，已恢复UTF-8文本并重跑通过，失败记录保留。

3.5：本轮独立材料生成回答，保存引用和证据快照；将本轮材料的知识正文更新为明确不同的新文本，执行现有Go Indexer重建真实Qdrant/MySQL索引。验证旧chunk UUID不再存在，新检索命中新UUID和新版内容；重新打开数据库读取历史，引用/证据JSON完全相同且仍含原始来源。引用快照不指向新内容，不依赖已删除的chunk关系。未修改原有测试材料、课堂文件或旧collection。

3.7：同一会话顺序发送两次相同问题，均完成各自SSE，恰好两个模型调用、两个distinct turn_id、四条消息，助手消息ID不同、两条用户文本相同；没有覆盖、额外写入或自动重放。并发409、占用释放及异常恢复沿用同次综合测试断言。

Phase 3七项全部通过并勾选。2.4没有真实外部模型教学证据，仍未勾选。受保护旧文件未覆盖。当前总进度18/32，按用户授权继续Phase 4。
