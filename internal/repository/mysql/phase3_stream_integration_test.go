//go:build alignstorage

package mysql

import (
	"campusclaw/internal/conversation"
	httpapi "campusclaw/internal/http"
	"campusclaw/internal/rag"
	"campusclaw/internal/tutor"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type phase3Retrieval func(context.Context, rag.Scope, string, string, []string, int) ([]rag.Evidence, error)

func (f phase3Retrieval) Search(c context.Context, s rag.Scope, q, m string, ids []string, n int) ([]rag.Evidence, error) {
	return f(c, s, q, m, ids, n)
}

type phase3Writer struct {
	*httptest.ResponseRecorder
	failEvent  string
	flushFails bool
	observe    func(string)
}

func (w *phase3Writer) Write(p []byte) (int, error) {
	s := string(p)
	if w.observe != nil {
		w.observe(s)
	}
	if w.failEvent != "" && strings.HasPrefix(s, "event: "+w.failEvent+"\n") {
		return 0, io.ErrClosedPipe
	}
	return w.ResponseRecorder.Write(p)
}
func (w *phase3Writer) FlushError() error {
	if w.flushFails {
		return io.ErrClosedPipe
	}
	w.ResponseRecorder.Flush()
	return nil
}

type phase3Model struct {
	calls int
	open  func(context.Context, tutor.Prepared, []conversation.ModelMessage, string) (conversation.Stream, error)
}

func (m *phase3Model) Open(ctx context.Context, p tutor.Prepared, h []conversation.ModelMessage, q string) (conversation.Stream, error) {
	m.calls++
	return m.open(ctx, p, h, q)
}

type phase3Stream struct {
	pieces []string
	err    error
	closed bool
	next   func() (string, error)
}

func (s *phase3Stream) Next() (string, error) {
	if s.next != nil {
		return s.next()
	}
	if len(s.pieces) > 0 {
		v := s.pieces[0]
		s.pieces = s.pieces[1:]
		return v, nil
	}
	if s.err != nil {
		return "", s.err
	}
	return "", io.EOF
}
func (s *phase3Stream) Close() error { s.closed = true; return nil }

type phase3Fault struct {
	conversation.Store
	stage       string
	appendCount int
}

func (f *phase3Fault) History(ctx context.Context, s rag.Scope, id int64) ([]conversation.ModelMessage, error) {
	if f.stage == "history" {
		return nil, conversation.Unavailable
	}
	return f.Store.History(ctx, s, id)
}
func (f *phase3Fault) Prepare(ctx context.Context, s rag.Scope, id int64, t conversation.Turn, p tutor.Prepared) error {
	if f.stage == "prepare" {
		return conversation.Unavailable
	}
	return f.Store.Prepare(ctx, s, id, t, p)
}
func (f *phase3Fault) ModelStarted(ctx context.Context, s rag.Scope, id int64, t conversation.Turn, p tutor.Prepared) error {
	if f.stage == "audit" {
		return conversation.Unavailable
	}
	return f.Store.ModelStarted(ctx, s, id, t, p)
}
func (f *phase3Fault) Append(ctx context.Context, s rag.Scope, id int64, t conversation.Turn, text string) error {
	f.appendCount++
	if f.stage == "append" || f.stage == "append2" && f.appendCount == 2 {
		return conversation.Unavailable
	}
	return f.Store.Append(ctx, s, id, t, text)
}
func (f *phase3Fault) Finish(ctx context.Context, s rag.Scope, id int64, t conversation.Turn, state string, c []rag.Source, reason string) error {
	if f.stage == "finish_all" || f.stage == "finish" && state == "completed" {
		return conversation.Unavailable
	}
	return f.Store.Finish(ctx, s, id, t, state, c, reason)
}

type phase3Event struct {
	name string
	data map[string]json.RawMessage
}

func phase3Events(t *testing.T, body string) []phase3Event {
	t.Helper()
	out := []phase3Event{}
	for _, block := range strings.Split(body, "\n\n") {
		if !strings.HasPrefix(block, "event: ") {
			continue
		}
		lines := strings.SplitN(block, "\n", 2)
		var value map[string]json.RawMessage
		if len(lines) != 2 || json.Unmarshal([]byte(strings.TrimPrefix(lines[1], "data: ")), &value) != nil {
			t.Fatalf("bad SSE %q", block)
		}
		out = append(out, phase3Event{strings.TrimPrefix(lines[0], "event: "), value})
	}
	return out
}
func phase3Delta(events []phase3Event) string {
	out := ""
	for _, e := range events {
		if e.name == "delta" {
			var s string
			json.Unmarshal(e.data["text"], &s)
			out += s
		}
	}
	return out
}
func phase3Names(events []phase3Event) string {
	v := []string{}
	for _, e := range events {
		v = append(v, e.name)
	}
	return strings.Join(v, ",")
}
func phase3Conversation(t *testing.T, f *phase2HTTP) conversation.Conversation {
	t.Helper()
	c, e := (Conversations{DB: f.db}).Create(context.Background(), alignStudent, 10)
	if e != nil {
		t.Fatal(e)
	}
	return c
}
func phase3Bind(t *testing.T, f *phase2HTTP, materials []string, skills []int64) {
	t.Helper()
	_, e := f.repo.SaveAgent(context.Background(), alignTeacher, 10, tutor.Config{Name: "A tutor", Prompt: "", Enabled: true, MaterialIDs: materials, SkillIDs: skills})
	if e != nil {
		t.Fatal(e)
	}
}
func phase3Handler(f *phase2HTTP, store conversation.Store, r tutor.Retriever, m conversation.Model, timeout time.Duration) http.Handler {
	return httpapi.WithConversations(phase2Health{}, nil, f.identity, nil, nil, nil, &httpapi.TutorHandler{Store: f.repo}, &httpapi.ConversationHandler{Store: store, Tutor: f.repo, Retrieval: r, Model: m, Timeout: timeout})
}
func phase3Last(t *testing.T, f *phase2HTTP, c conversation.Conversation) conversation.Message {
	t.Helper()
	actual, m, e := (Conversations{DB: f.db}).Get(context.Background(), alignStudent, c.ID)
	if e != nil || len(m) < 2 {
		t.Fatal("accepted pair missing", e)
	}
	if actual.Active != "" {
		t.Fatal("active turn not released")
	}
	return m[len(m)-1]
}

func TestAlignPhase3PreparationFailuresAndAtomicStart(t *testing.T) {
	f := newPhase2HTTP(t)
	store := Conversations{DB: f.db}
	for _, stage := range []string{"start", "history", "retrieval", "prepare", "prepare_sql"} {
		t.Run(stage, func(t *testing.T) {
			c := phase3Conversation(t, f)
			fault := &phase3Fault{Store: store, stage: stage}
			r := &phase2Retriever{}
			if stage == "retrieval" {
				r.err = rag.Unavailable
			}
			if stage == "start" {
				mustSQL(t, f.db, "CREATE TRIGGER phase3_reject_start BEFORE INSERT ON conversation_messages FOR EACH ROW BEGIN IF NEW.role='assistant' THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='controlled start rejection'; END IF; END")
			}
			if stage == "prepare_sql" {
				mustSQL(t, f.db, "CREATE TRIGGER phase3_reject_prepare BEFORE UPDATE ON conversation_messages FOR EACH ROW BEGIN IF NEW.selection_reason='prepared' THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='controlled prepare rejection'; END IF; END")
			}
			m := &phase3Model{open: func(context.Context, tutor.Prepared, []conversation.ModelMessage, string) (conversation.Stream, error) {
				t.Fatal("model before preparation")
				return nil, nil
			}}
			f.handler = phase3Handler(f, fault, r, m, 0)
			w := f.request("POST", fmt.Sprintf("/api/conversations/%d/messages", c.ID), "sa", map[string]any{"content": "方程问题"})
			if stage == "prepare_sql" {
				mustSQL(t, f.db, "DROP TRIGGER phase3_reject_prepare")
			}
			if stage == "start" {
				mustSQL(t, f.db, "DROP TRIGGER phase3_reject_start")
			}
			if w.Code != 503 || strings.Contains(w.Body.String(), "event:") || m.calls != 0 {
				t.Fatal(w.Code, w.Body.String())
			}
			if stage == "start" {
				if scalar(t, f.db, "SELECT COUNT(*) FROM conversation_messages WHERE conversation_id=?", c.ID) != 0 {
					t.Fatal("partial initial transaction")
				}
				return
			}
			last := phase3Last(t, f, c)
			if last.State != "failed" || last.SkillName != "" {
				t.Fatal("preparation failure audit", last)
			}
			if scalar(t, f.db, "SELECT COUNT(*) FROM conversation_messages WHERE conversation_id=?", c.ID) != 2 {
				t.Fatal("question/placeholder not retained")
			}
		})
	}
}

func TestAlignPhase3FourCombinationsAndEarlyMeta(t *testing.T) {
	f := newPhase2HTTP(t)
	ctx := context.Background()
	store := Conversations{DB: f.db}
	for _, evidence := range []bool{true, false} {
		for _, skill := range []bool{true, false} {
			t.Run(fmt.Sprintf("evidence=%t_skill=%t", evidence, skill), func(t *testing.T) {
				materials := []string{}
				if evidence {
					materials = []string{alignMaterialA}
				}
				skills := []int64{}
				if skill {
					skills = []int64{20}
				}
				phase3Bind(t, f, materials, skills)
				c := phase3Conversation(t, f)
				r := &phase2Retriever{data: []rag.Evidence{{ChunkID: alignMaterialA, MaterialID: alignMaterialA, Index: 1, Title: "数学一", Text: "第一份全文"}, {ChunkID: alignMaterialA, MaterialID: alignMaterialA, Index: 2, Title: "数学二", Text: "第二份全文"}}}
				w := httptest.NewRecorder()
				m := &phase3Model{open: func(_ context.Context, p tutor.Prepared, h []conversation.ModelMessage, q string) (conversation.Stream, error) {
					events := phase3Events(t, w.Body.String())
					if phase3Names(events) != "meta" || !w.Flushed {
						t.Fatal("model called before meta Flush")
					}
					if string(events[0].data["status"]) != `"ok"` || string(events[0].data["citations"]) == "null" {
						t.Fatal("bad meta")
					}
					var sources []rag.Source
					json.Unmarshal(events[0].data["citations"], &sources)
					if evidence && len(sources) != 2 || !evidence && len(sources) != 0 {
						t.Fatal("incomplete candidates", sources)
					}
					var pendingActual int
					if f.db.QueryRow("SELECT COUNT(*) FROM conversation_messages WHERE conversation_id=? AND skill_id IS NOT NULL", c.ID).Scan(&pendingActual) != nil || pendingActual != 0 {
						t.Fatal("skill falsely recorded before Open")
					}
					// A separate connection can lock the conversation: no gateway-spanning transaction.
					tx, e := f.db.BeginTx(ctx, nil)
					if e != nil {
						t.Fatal(e)
					}
					var id int64
					if e = tx.QueryRow("SELECT id FROM conversations WHERE id=? FOR UPDATE NOWAIT", c.ID).Scan(&id); e != nil {
						t.Fatal("gateway held DB lock", e)
					}
					tx.Rollback()
					if (p.Actual != nil) != skill || !strings.Contains(p.System, tutor.DefaultRole) || p.Answer != "" {
						t.Fatal("independent role/skill inputs")
					}
					messages, e := conversation.ModelMessages(p, h, q)
					if e != nil || len(messages) != 2 {
						t.Fatal("current input duplicated", messages, e)
					}
					body := "独立回答"
					if evidence {
						body = "只依据第二份[2]"
					}
					return &phase3Stream{pieces: []string{body, "。"}}, nil
				}}
				f.handler = phase3Handler(f, store, r, m, 0)
				req := httptest.NewRequest("POST", fmt.Sprintf("/api/conversations/%d/messages", c.ID), strings.NewReader(`{"content":"方程问题"}`))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Authorization", "Bearer "+f.tokens["sa"])
				f.handler.ServeHTTP(w, req)
				events := phase3Events(t, w.Body.String())
				if w.Code != 200 || phase3Names(events) != "meta,delta,delta,done" || m.calls != 1 {
					t.Fatal(w.Code, w.Body.String(), m.calls)
				}
				if !evidence && r.calls != 0 {
					t.Fatal("empty set retrieved")
				}
				last := phase3Last(t, f, c)
				if last.State != "completed" || last.Content != phase3Delta(events) || (last.SkillName != "") != skill {
					t.Fatal("persistence/call audit", last)
				}
				var final []rag.Source
				json.Unmarshal(events[len(events)-1].data["citations"], &final)
				if evidence && (len(final) != 1 || final[0].Number != 2) || !evidence && (len(final) != 0 || string(events[len(events)-1].data["citations"]) != "[]") {
					t.Fatal("final identities", final)
				}
				if w.Header().Get("X-Accel-Buffering") != "no" || w.Header().Get("Connection") != "keep-alive" {
					t.Fatal("stream headers")
				}
				t.Logf("evidence=%t skill=%t model_calls=%d order=%s", evidence, skill, m.calls, phase3Names(events))
			})
		}
	}
}

func TestAlignPhase3StreamFailuresAndCleaning(t *testing.T) {
	f := newPhase2HTTP(t)
	store := Conversations{DB: f.db}
	phase3Bind(t, f, []string{}, []int64{20})
	for _, tc := range []struct {
		stage  string
		pieces []string
		err    error
		want   string
		state  string
	}{
		{"clean", []string{"文字[", "1", "]𠮷[普通]", "[23]末尾["}, nil, "文字𠮷[普通]末尾[", "completed"},
		{"empty", []string{"[", "12", "]"}, nil, "", "failed"},
		{"fixed", []string{rag.NoEvidence}, nil, rag.NoEvidence, "failed"},
		{"open", nil, rag.Unavailable, "", "failed"},
		{"first", nil, rag.Unavailable, "", "failed"},
		{"partial", []string{"可靠前缀"}, rag.Unavailable, "可靠前缀", "failed"},
		{"append", []string{"不应发送"}, nil, "", "failed"},
		{"append2", []string{"第一段", "第二段"}, nil, "第一段", "failed"},
		{"finish", []string{"已存正文"}, nil, "已存正文", "failed"},
		{"audit", []string{"不应生成"}, nil, "", "failed"},
	} {
		t.Run(tc.stage, func(t *testing.T) {
			c := phase3Conversation(t, f)
			fault := &phase3Fault{Store: store, stage: tc.stage}
			stream := &phase3Stream{pieces: append([]string{}, tc.pieces...), err: tc.err}
			m := &phase3Model{open: func(context.Context, tutor.Prepared, []conversation.ModelMessage, string) (conversation.Stream, error) {
				if tc.stage == "open" {
					return nil, rag.Unavailable
				}
				return stream, nil
			}}
			f.handler = phase3Handler(f, fault, &phase2Retriever{}, m, 0)
			w := f.request("POST", fmt.Sprintf("/api/conversations/%d/messages", c.ID), "sa", map[string]any{"content": "方程问题"})
			events := phase3Events(t, w.Body.String())
			if w.Code != 200 || len(events) < 2 || events[0].name != "meta" {
				t.Fatal("HTTP boundary", w.Code, w.Body.String())
			}
			terminal := "error"
			if tc.state == "completed" {
				terminal = "done"
			}
			if events[len(events)-1].name != terminal || strings.Contains(phase3Names(events), "error,done") {
				t.Fatal(w.Body.String())
			}
			last := phase3Last(t, f, c)
			if last.State != tc.state || last.Content != tc.want || phase3Delta(events) != tc.want {
				t.Fatal("prefix mismatch", last.Content, last.State, w.Body.String())
			}
			if tc.stage == "open" && last.SkillName != "" {
				t.Fatal("unsent skill audit")
			}
			if tc.stage != "open" && !stream.closed {
				t.Fatal("stream not closed")
			}
		})
	}
}

func TestAlignPhase3TurnTokenIsolationAndRecovery(t *testing.T) {
	f := newPhase2HTTP(t)
	store := Conversations{DB: f.db}
	ctx := context.Background()
	c := phase3Conversation(t, f)
	snapshot, e := tutor.Capture(ctx, f.repo, alignStudent, 10)
	if e != nil {
		t.Fatal(e)
	}
	snapshot = tutor.Route(snapshot, "方程问题")
	p, e := tutor.Prepare(snapshot, nil)
	if e != nil {
		t.Fatal(e)
	}
	turn, e := store.Start(ctx, alignStudent, c, "方程问题", p)
	if e != nil {
		t.Fatal(e)
	}
	for _, s := range []rag.Scope{{UserID: 5, ClassID: 1}, {UserID: 1, ClassID: 2}} {
		if store.Prepare(ctx, s, c.ID, turn, p) == nil || store.ModelStarted(ctx, s, c.ID, turn, p) == nil || store.Append(ctx, s, c.ID, turn, "foreign") == nil {
			t.Fatal("owner/class bypass")
		}
	}
	bad := turn
	bad.Token = "00000000-0000-4000-8000-000000000000"
	if store.Prepare(ctx, alignStudent, c.ID, bad, p) == nil || store.ModelStarted(ctx, alignStudent, c.ID, bad, p) == nil {
		t.Fatal("turn token bypass")
	}
	if _, e = store.Start(ctx, alignStudent, c, "并发问题", p); !errors.Is(e, conversation.Busy) {
		t.Fatal("missing DB concurrency guard", e)
	}
	if e = store.Prepare(ctx, alignStudent, c.ID, turn, p); e != nil {
		t.Fatal(e)
	}
	if e = store.ModelStarted(ctx, alignStudent, c.ID, turn, p); e != nil {
		t.Fatal(e)
	}
	if e = store.Append(ctx, alignStudent, c.ID, turn, "恢复前缀"); e != nil {
		t.Fatal(e)
	}
	if e = store.Recover(ctx); e != nil {
		t.Fatal(e)
	}
	last := phase3Last(t, f, c)
	if last.State != "interrupted" || last.Content != "恢复前缀" || last.SkillName == "" {
		t.Fatal("recovery lost audit/prefix", last)
	}
}

func TestAlignPhase3PreparationFrozenConfigurationAndZeroResults(t *testing.T) {
	f := newPhase2HTTP(t)
	store := Conversations{DB: f.db}
	ctx := context.Background()
	for _, kind := range []string{"zero", "subject", "update"} {
		t.Run(kind, func(t *testing.T) {
			phase3Bind(t, f, []string{alignMaterialA}, []int64{20})
			source := "名称：解题引导冻结\n适用条件：方程\n资料：数学\n启用：是\n\n原始说明"
			skill, e := f.repo.SaveSkill(ctx, alignTeacher, 20, source)
			if e != nil {
				t.Fatal(e)
			}
			c := phase3Conversation(t, f)
			r := phase3Retrieval(func(_ context.Context, _ rag.Scope, _ string, _ string, ids []string, _ int) ([]rag.Evidence, error) {
				if scalar(t, f.db, "SELECT COUNT(*) FROM conversation_messages WHERE conversation_id=?", c.ID) != 2 {
					t.Fatal("retrieval before accepted pair")
				}
				if kind == "update" {
					phase3Bind(t, f, []string{}, []int64{})
					if _, e = f.repo.SaveSkill(ctx, alignTeacher, 20, strings.ReplaceAll(source, "原始说明", "新说明")); e != nil {
						t.Fatal(e)
					}
				}
				if kind == "zero" {
					return []rag.Evidence{}, nil
				}
				return []rag.Evidence{{ChunkID: alignMaterialA, MaterialID: alignMaterialA, Index: 1, Title: "语文", Text: "不符学科"}}, nil
			})
			m := &phase3Model{open: func(_ context.Context, p tutor.Prepared, _ []conversation.ModelMessage, _ string) (conversation.Stream, error) {
				if len(p.Evidence) != 0 || p.Actual == nil || p.Actual.Hash != skill.SourceHash || !strings.Contains(p.System, "原始说明") || strings.Contains(p.System, "新说明") {
					t.Fatal("recaptured/failed empty prep")
				}
				return &phase3Stream{pieces: []string{"无资料仍解释"}}, nil
			}}
			f.handler = phase3Handler(f, store, r, m, 0)
			w := f.request("POST", fmt.Sprintf("/api/conversations/%d/messages", c.ID), "sa", map[string]any{"content": "方程问题"})
			if w.Code != 200 || !strings.HasSuffix(phase3Names(phase3Events(t, w.Body.String())), "done") || m.calls != 1 {
				t.Fatal(w.Code, w.Body.String())
			}
			last := phase3Last(t, f, c)
			if last.State != "completed" || last.SkillHash != skill.SourceHash || len(last.Evidence) != 0 {
				t.Fatal("snapshot not persisted", last)
			}
		})
	}
}

func TestAlignPhase3LifecyclePersistenceTransportAndRecovery(t *testing.T) {
	f := newPhase2HTTP(t)
	store := Conversations{DB: f.db}
	phase3Bind(t, f, []string{}, []int64{20})
	for _, kind := range []string{"ordered", "delta_lost", "done_lost", "meta_flush", "cancel", "timeout", "finish_all"} {
		t.Run(kind, func(t *testing.T) {
			c := phase3Conversation(t, f)
			fault := &phase3Fault{Store: store, stage: kind}
			w := &phase3Writer{ResponseRecorder: httptest.NewRecorder()}
			if kind == "delta_lost" {
				w.failEvent = "delta"
			}
			if kind == "done_lost" {
				w.failEvent = "done"
			}
			if kind == "meta_flush" {
				w.flushFails = true
			}
			emitted := ""
			w.observe = func(raw string) {
				events := phase3Events(t, raw)
				for _, event := range events {
					if event.name == "delta" {
						var v string
						json.Unmarshal(event.data["text"], &v)
						emitted += v
						var body string
						if e := f.db.QueryRow("SELECT content FROM conversation_messages WHERE conversation_id=? AND role='assistant'", c.ID).Scan(&body); e != nil || body != emitted {
							t.Fatal("delta before Append", e, body, emitted)
						}
					}
					if event.name == "done" {
						last := phase3Last(t, f, c)
						if last.State != "completed" {
							t.Fatal("done before Finish")
						}
					}
				}
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			firstDelta := make(chan struct{}, 1)
			previousObserve := w.observe
			w.observe = func(raw string) {
				previousObserve(raw)
				if strings.HasPrefix(raw, "event: delta\n") {
					select {
					case firstDelta <- struct{}{}:
					default:
					}
				}
			}
			m := &phase3Model{open: func(modelCtx context.Context, _ tutor.Prepared, _ []conversation.ModelMessage, _ string) (conversation.Stream, error) {
				stream := &phase3Stream{pieces: []string{"可靠", "前缀"}}
				if kind == "cancel" || kind == "timeout" {
					i := 0
					stream.next = func() (string, error) {
						i++
						if i == 1 {
							return "可靠", nil
						}
						if kind == "cancel" {
							<-firstDelta
							cancel()
						}
						<-modelCtx.Done()
						return "", modelCtx.Err()
					}
				}
				return stream, nil
			}}
			timeout := time.Duration(0)
			if kind == "timeout" {
				timeout = 100 * time.Millisecond
			}
			f.handler = phase3Handler(f, fault, &phase2Retriever{}, m, timeout)
			req := httptest.NewRequest("POST", fmt.Sprintf("/api/conversations/%d/messages", c.ID), strings.NewReader(`{"content":"方程问题"}`)).WithContext(ctx)
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+f.tokens["sa"])
			f.handler.ServeHTTP(w, req)
			events := phase3Events(t, w.Body.String())
			if kind == "finish_all" {
				actual, all, e := store.Get(context.Background(), alignStudent, c.ID)
				if e != nil || actual.Active == "" || all[len(all)-1].State != "generating" {
					t.Fatal("false terminal write")
				}
				if e = store.Recover(context.Background()); e != nil {
					t.Fatal(e)
				}
			}
			last := phase3Last(t, f, c)
			want := "completed"
			switch kind {
			case "delta_lost", "meta_flush", "cancel", "finish_all":
				want = "interrupted"
			case "timeout":
				want = "failed"
			}
			if last.State != want {
				t.Fatal(kind, last.State, w.Body.String())
			}
			if kind == "meta_flush" {
				if m.calls != 0 || last.Content != "" || last.SkillName != "" {
					t.Fatal("model after failed meta")
				}
				return
			}
			if kind == "done_lost" {
				if last.Content != "可靠前缀" || strings.Contains(phase3Names(events), "done") {
					t.Fatal("lost done recovery")
				}
			}
			if kind == "ordered" && phase3Delta(events) != last.Content {
				t.Fatal("delta/DB divergence")
			}
			if kind == "timeout" && events[len(events)-1].name != "error" {
				t.Fatal("timeout not SSE error")
			}
			if kind == "delta_lost" && (last.Content != "可靠" || phase3Delta(events) != "") {
				t.Fatal("transport reliable prefix")
			}
			if kind == "cancel" && strings.Contains(phase3Names(events), "done") {
				t.Fatal("cancel completed")
			}
			if kind == "cancel" && last.Content != "可靠" {
				t.Fatal("cancel lost persisted prefix", last.Content)
			}
		})
	}
}

func TestAlignPhase3StreamEvidencedValidation(t *testing.T) {
	f := newPhase2HTTP(t)
	store := Conversations{DB: f.db}
	phase3Bind(t, f, []string{alignMaterialA}, []int64{})
	for _, text := range []string{"没有引用", "越界[2]", "零号[0]"} {
		t.Run(text, func(t *testing.T) {
			c := phase3Conversation(t, f)
			r := &phase2Retriever{data: []rag.Evidence{{ChunkID: alignMaterialA, MaterialID: alignMaterialA, Index: 1, Text: "正文"}}}
			m := &phase3Model{open: func(context.Context, tutor.Prepared, []conversation.ModelMessage, string) (conversation.Stream, error) {
				return &phase3Stream{pieces: []string{text}}, nil
			}}
			f.handler = phase3Handler(f, store, r, m, 0)
			w := f.request("POST", fmt.Sprintf("/api/conversations/%d/messages", c.ID), "sa", map[string]any{"content": "方程问题"})
			events := phase3Events(t, w.Body.String())
			last := phase3Last(t, f, c)
			if w.Code != 200 || last.State != "failed" || last.Content != text || events[len(events)-1].name != "error" {
				t.Fatal("citation guard weakened", w.Body.String(), last)
			}
		})
	}
}

func TestAlignPhase3PreparationConcurrentHTTP(t *testing.T) {
	f := newPhase2HTTP(t)
	store := Conversations{DB: f.db}
	c := phase3Conversation(t, f)
	entered, release := make(chan struct{}), make(chan struct{})
	r := phase3Retrieval(func(context.Context, rag.Scope, string, string, []string, int) ([]rag.Evidence, error) {
		close(entered)
		<-release
		return nil, nil
	})
	m := &phase3Model{open: func(context.Context, tutor.Prepared, []conversation.ModelMessage, string) (conversation.Stream, error) {
		return &phase3Stream{pieces: []string{"有效回答"}}, nil
	}}
	f.handler = phase3Handler(f, store, r, m, 0)
	path := fmt.Sprintf("/api/conversations/%d/messages", c.ID)
	result := make(chan *httptest.ResponseRecorder, 1)
	go func() { result <- f.request("POST", path, "sa", map[string]any{"content": "方程问题"}) }()
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("not entered retrieval")
	}
	for _, tc := range []struct {
		who  string
		code int
	}{{"sa", 409}, {"sa2", 404}, {"sb", 404}} {
		w := f.request("POST", path, tc.who, map[string]any{"content": "并发问题"})
		if w.Code != tc.code {
			close(release)
			t.Fatal(tc, w.Code)
		}
	}
	if scalar(t, f.db, "SELECT COUNT(*) FROM conversation_messages WHERE conversation_id=?", c.ID) != 2 {
		t.Fatal("concurrent request inserted pair")
	}
	close(release)
	if w := <-result; w.Code != 200 || !strings.Contains(w.Body.String(), "event: done") {
		t.Fatal(w.Code, w.Body.String())
	}
}
