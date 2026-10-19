package httpapi

import (
	"campusclaw/internal/auth"
	"campusclaw/internal/conversation"
	"campusclaw/internal/mcp"
	"campusclaw/internal/rag"
	"campusclaw/internal/tutor"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
	"unicode/utf8"
)

type ToolRunner interface {
	Run(context.Context, auth.Principal, []string, string, int64) ([]mcp.ToolCall, error)
}
type ConversationHandler struct {
	Tools     ToolRunner
	Store     conversation.Store
	Tutor     tutor.SnapshotStore
	Retrieval tutor.Retriever
	Model     conversation.Model
	Timeout   time.Duration
	mu        sync.Mutex
	active    map[int64]bool
}

func studentRequired(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if scope(r).Role != "student" {
			authError(w, 403, "Student role required")
			return
		}
		next.ServeHTTP(w, r)
	})
}
func conversationError(w http.ResponseWriter, e error) {
	code := 503
	switch {
	case errors.Is(e, conversation.Invalid), errors.Is(e, tutor.Invalid), errors.Is(e, rag.Invalid):
		code = 400
	case errors.Is(e, conversation.NotFound), errors.Is(e, tutor.NotFound):
		code = 404
	case errors.Is(e, conversation.Busy):
		code = 409
	case errors.Is(e, rag.InvalidAnswer):
		code = 502
	}
	authError(w, code, e.Error())
}

// List documents this registered handler.
// @Summary 学生自己的会话列表
// @Description owner+当前class隔离，按updated_at倒序。
// @Tags conversations
// @Produce json
// @Security bearerAuth
// @Success 200 {object} ConversationList
// @Failure 401 {object} APIError
// @Failure 403 {object} APIError
// @Failure 500 {object} APIError
// @Failure 503 {object} APIError
// @Router /api/conversations [get]
func (h *ConversationHandler) List(w http.ResponseWriter, r *http.Request) {
	v, e := h.Store.List(r.Context(), scope(r))
	if e != nil {
		conversationError(w, e)
		return
	}
	tutorJSON(w, 200, map[string]any{"conversations": v})
}

// Create documents this registered handler.
// @Summary 学生创建锁定智能体的私有会话
// @Description 只接受agent_id。本班无启用助手400；外班/不存在/停用404。已有会话可在助手停用后续聊；不得换绑。
// @Tags conversations
// @Accept json
// @Produce json
// @Security bearerAuth
// @Param body body CreateConversationInput true "本班启用智能体"
// @Success 201 {object} conversation.Conversation
// @Failure 400 {object} APIError
// @Failure 401 {object} APIError
// @Failure 403 {object} APIError
// @Failure 404 {object} APIError
// @Failure 500 {object} APIError
// @Failure 503 {object} APIError
// @x-strict-request true
// @Router /api/conversations [post]
func (h *ConversationHandler) Create(w http.ResponseWriter, r *http.Request) {
	v, ok := tutorObject(w, r, "agent_id")
	if !ok {
		return
	}
	var id int64
	if json.Unmarshal(v["agent_id"], &id) != nil || id <= 0 {
		conversationError(w, conversation.Invalid)
		return
	}
	c, e := h.Store.Create(r.Context(), scope(r), id)
	if e != nil {
		conversationError(w, e)
		return
	}
	tutorJSON(w, 201, c)
}

// Get documents this registered handler.
// @Summary 本人会话及完整消息历史
// @Description owner+当前class限定；外班/他人/不存在404，教师403。保留失败前缀和最终引用；done丢失时据此确认completed，不自动重试模型。首问标题前24个Unicode字符。
// @Tags conversations
// @Produce json
// @Security bearerAuth
// @Param id path int true "本人会话ID" minimum(1)
// @Success 200 {object} ConversationOutput
// @Failure 400 {object} APIError
// @Failure 401 {object} APIError
// @Failure 403 {object} APIError
// @Failure 404 {object} APIError
// @Failure 500 {object} APIError
// @Failure 503 {object} APIError
// @Router /api/conversations/{id} [get]
func (h *ConversationHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, ok := tutorID(w, r)
	if !ok {
		return
	}
	c, m, e := h.Store.Get(r.Context(), scope(r), id)
	if e != nil {
		conversationError(w, e)
		return
	}
	tutorJSON(w, 200, map[string]any{"conversation": c, "messages": m})
}

// Messages streams a durably accepted, privately scoped turn.
// @Summary 学生发送问题并读取实时SSE
// @Description 仅content，trim后2–500字，owner+class限定，单会话并发409。原子保存user/assistant占位，准备本轮快照和末尾6条历史（含当前user一次，每条历史≤500字），再meta/Flush，之后才连接模型。有据/无据均生成，空绑定不检索；无据不虚构数字引用。每片先落库再delta，completed提交后done；15秒心跳。meta.citations是完整候选/空数组，done.citations是实际引用子集且保留原号。meta前参数/权限/准备故障返回JSON（503可能已保留failed轮次），meta后模型连接/生成/引用/持久化失败保持HTTP200 + error，无成功done。取消保留interrupted草稿；EOF无done请读历史确认，不自动重试。Swagger即时增量请使用实时SSE面板。
// @Tags conversations
// @Accept json
// @Produce text/event-stream
// @Security bearerAuth
// @Param id path int true "本人会话ID" minimum(1)
// @Param body body SendMessageInput true "只允许content"
// @Success 200 {object} SSEEnvelope "meta → delta* → done，或meta → delta* → error"
// @Failure 400 {object} APIError
// @Failure 401 {object} APIError
// @Failure 403 {object} APIError
// @Failure 404 {object} APIError
// @Failure 409 {object} APIError
// @Failure 500 {object} APIError "身份存储故障"
// @Failure 502 {object} APIError "仅meta前模型输入预算校验失败；模型尚未连接"
// @Failure 503 {object} APIError "meta前准备或数据库故障，模型尚未调用"
// @x-strict-request true
// @x-sse {"heartbeat_seconds":15,"meta_before_model":true,"examples":{"no_evidence":{"summary":"空候选仍调用模型，清洗数字引用","value":"event: meta\ndata: {\"status\":\"ok\",\"citations\":[],\"skill_name\":\"\",\"message_id\":12}\n\nevent: delta\ndata: {\"text\":\"无资料也按角色解释问题。\"}\n\nevent: done\ndata: {\"message_id\":12,\"citations\":[],\"skill_name\":\"\",\"state\":\"completed\"}\n\n"},"failure":{"summary":"meta后连接/生成失败：HTTP200，无done","value":"event: meta\ndata: {\"status\":\"ok\",\"citations\":[],\"skill_name\":\"\",\"message_id\":12}\n\nevent: error\ndata: {\"error\":\"回答失败\",\"reason\":\"model_connection_failed\"}\n\n"}}}
// @Router /api/conversations/{id}/messages [post]
func (h *ConversationHandler) Messages(w http.ResponseWriter, r *http.Request) {
	id, ok := tutorID(w, r)
	if !ok {
		return
	}
	data, ok := tutorObject(w, r, "content")
	if !ok {
		return
	}
	var q string
	if json.Unmarshal(data["content"], &q) != nil {
		conversationError(w, conversation.Invalid)
		return
	}
	q, e := conversation.Question(q)
	if e != nil {
		conversationError(w, e)
		return
	}
	s := scope(r)
	c, _, e := h.Store.Get(r.Context(), s, id)
	if e != nil {
		conversationError(w, e)
		return
	}
	if c.Active != "" {
		conversationError(w, conversation.Busy)
		return
	}
	h.mu.Lock()
	if h.active == nil {
		h.active = map[int64]bool{}
	}
	if h.active[id] {
		h.mu.Unlock()
		conversationError(w, conversation.Busy)
		return
	}
	h.active[id] = true
	h.mu.Unlock()
	defer func() { h.mu.Lock(); delete(h.active, id); h.mu.Unlock() }()
	timeout := h.Timeout
	if timeout == 0 || timeout > 120*time.Second {
		timeout = 120 * time.Second
	}
	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()
	// Freeze routing/configuration before accepting a turn. All preparation after
	// Start uses this snapshot even if a teacher commits a new configuration.
	snapshot, e := tutor.Capture(ctx, h.Tutor, s, c.AgentID)
	if e != nil {
		conversationError(w, e)
		return
	}
	snapshot = tutor.Route(snapshot, q)
	turn, e := h.Store.Start(ctx, s, c, q, tutor.Prepared{Snapshot: snapshot})
	if e != nil {
		conversationError(w, e)
		return
	}
	terminal := false
	fail := func(state, reason string) {
		recovery, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		if h.Store.Finish(recovery, s, id, turn, state, nil, reason) == nil {
			terminal = true
		}
	}
	defer func() {
		if !terminal {
			fail("interrupted", "request_interrupted")
		}
	}()
	prepareFailure := func(err error) { fail("failed", "preparation_failed"); conversationError(w, err) }
	pairs, e := h.Store.History(ctx, s, id)
	if e != nil {
		prepareFailure(e)
		return
	}
	history, previous := conversation.Context(pairs)
	query := q
	if previous != "" {
		query = previous + "\n" + q
	}
	runes := []rune(query)
	if len(runes) > 1000 {
		query = string(runes[len(runes)-1000:])
	}
	prepared, e := tutor.Retrieve(ctx, h.Retrieval, s, snapshot, query, "hybrid")
	if e != nil {
		prepareFailure(e)
		return
	}
	if prepared.Actual != nil && prepared.Snapshot.Selected != nil {
		parsed, err := tutor.ParseSkillSource(prepared.Snapshot.Selected.Source)
		if err != nil {
			prepareFailure(tutor.Unavailable)
			return
		}
		refs := []string{}
		for _, ref := range parsed.Tools {
			if ref != "knowledge_search" {
				refs = append(refs, ref)
			}
		}
		if len(refs) > 0 {
			if h.Tools == nil {
				prepareFailure(tutor.Unavailable)
				return
			}
			principal, _ := auth.FromContext(r.Context())
			calls, err := h.Tools.Run(ctx, principal, refs, q, id)
			if err != nil {
				prepareFailure(conversation.Unavailable)
				return
			}
			prepared.Snapshot.ToolCalls = calls
		}
	}
	if e = h.Store.Prepare(ctx, s, id, turn, prepared); e != nil {
		prepareFailure(e)
		return
	}
	// Assemble/validate the bounded input before committing the SSE response.
	if _, e = conversation.ModelMessages(prepared, history, q); e != nil {
		prepareFailure(e)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-store")
	w.Header().Set("X-Accel-Buffering", "no")
	if r.ProtoMajor == 1 {
		w.Header().Set("Connection", "keep-alive")
	}
	controller := http.NewResponseController(w)
	emit := func(event string, value any) error {
		if r.Context().Err() != nil {
			return r.Context().Err()
		}
		b, err := json.Marshal(value)
		if err != nil {
			return err
		}
		if _, err = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, b); err != nil {
			return err
		}
		return controller.Flush()
	}
	actualName := ""
	if prepared.Actual != nil {
		actualName = prepared.Actual.Name
	}
	candidates := []rag.Source{}
	for i, v := range prepared.Evidence {
		candidates = append(candidates, rag.Source{Number: i + 1, ChunkID: v.ChunkID, MaterialID: v.MaterialID, Title: v.Title, Index: v.Index, Text: v.Text})
	}
	meta := map[string]any{"status": "ok", "citations": candidates, "skill_name": actualName, "message_id": turn.ID}
	if len(prepared.Snapshot.ToolCalls) > 0 {
		meta["tool_calls"] = prepared.Snapshot.ToolCalls
	}
	if emit("meta", meta) != nil {
		return
	}
	streamError := func(reason string) {
		if r.Context().Err() != nil {
			fail("interrupted", "request_interrupted")
			return
		}
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			reason = "model_timeout"
		}
		fail("failed", reason)
		_ = emit("error", map[string]string{"error": "回答失败", "reason": reason})
	}
	if h.Model == nil {
		streamError("model_unavailable")
		return
	}
	stream, e := h.Model.Open(ctx, prepared, history, q)
	if e != nil {
		if stream != nil {
			stream.Close()
		}
		streamError("model_connection_failed")
		return
	}
	if stream == nil {
		streamError("model_unavailable")
		return
	}
	defer stream.Close()
	if e = h.Store.ModelStarted(ctx, s, id, turn, prepared); e != nil {
		streamError("model_audit_write_failed")
		return
	}
	text := ""
	rawBytes := 0
	filter := conversation.CitationFilter{}
	push := func(fragment string) bool {
		if fragment == "" {
			return true
		}
		if len(text)+len(fragment) > 1<<20 {
			streamError("answer_too_large")
			return false
		}
		if !utf8.ValidString(fragment) {
			streamError("invalid_answer")
			return false
		}
		if err := h.Store.Append(ctx, s, id, turn, fragment); err != nil {
			streamError("append_failed")
			return false
		}
		text += fragment
		if emit("delta", map[string]string{"text": fragment}) != nil {
			fail("interrupted", "transport_failed")
			return false
		}
		return true
	}
	type part struct {
		text string
		err  error
	}
	parts := make(chan part, 1)
	go func() {
		for {
			fragment, err := stream.Next()
			select {
			case parts <- part{fragment, err}:
			case <-ctx.Done():
				return
			}
			if err != nil {
				return
			}
		}
	}()
	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()
	for {
		select {
		case <-ctx.Done():
			if r.Context().Err() != nil {
				return
			}
			streamError("model_timeout")
			return
		case <-heartbeat.C:
			if _, e = io.WriteString(w, ": heartbeat\n\n"); e != nil {
				return
			}
			if controller.Flush() != nil {
				return
			}
		case fragment := <-parts:
			if ctx.Err() != nil {
				if r.Context().Err() == nil {
					streamError("model_timeout")
				}
				return
			}
			rawBytes += len(fragment.text)
			if rawBytes > 1<<20 {
				streamError("answer_too_large")
				return
			}
			piece := fragment.text
			if len(prepared.Evidence) == 0 {
				piece = filter.Feed(piece)
			}
			if !push(piece) {
				return
			}
			if fragment.err == io.EOF {
				if len(prepared.Evidence) == 0 {
					if !push(filter.End()) {
						return
					}
				}
				sources, err := conversation.ValidateAnswer(text, prepared.Evidence)
				if err != nil {
					streamError("invalid_citations_or_answer")
					return
				}
				if err = h.Store.Finish(ctx, s, id, turn, "completed", sources, ""); err != nil {
					streamError("completion_write_failed")
					return
				}
				terminal = true
				_ = emit("done", map[string]any{"message_id": turn.ID, "citations": sources, "skill_name": actualName, "state": "completed"})
				return
			}
			if fragment.err != nil {
				streamError("model_failed")
				return
			}
		}
	}
}
