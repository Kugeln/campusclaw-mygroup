//go:build alignstorage

package mysql

import (
	"campusclaw/internal/auth"
	"campusclaw/internal/conversation"
	httpapi "campusclaw/internal/http"
	"campusclaw/internal/material"
	"campusclaw/internal/mcp"
	"campusclaw/internal/rag"
	"campusclaw/internal/storage"
	"campusclaw/internal/tutor"
	"context"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
)

type phase5MCPControl struct {
	sync.Mutex
	Mode               string                      `json:"mode"`
	Models             int                         `json:"models"`
	Embeddings         int                         `json:"embeddings"`
	Cancelled          int                         `json:"cancelled"`
	Completed          int                         `json:"completed"`
	MetaFlushed        int64                       `json:"meta_flushed_ns"`
	OpenInvoked        int64                       `json:"open_invoked_ns"`
	FirstDelta         int64                       `json:"first_delta_ns"`
	Done               int64                       `json:"done_ns"`
	ToolStarted        int64                       `json:"tool_started_ns"`
	ToolFinished       int64                       `json:"tool_finished_ns"`
	ToolRuns           int                         `json:"tool_runs"`
	HeaderViolation    bool                        `json:"identity_header_violation"`
	EmbeddingFailure   bool                        `json:"embedding_failure"`
	BadAnswer          bool                        `json:"bad_answer"`
	LastMessages       []conversation.ModelMessage `json:"last_messages"`
	gateOpen, gateTail chan struct{}
}

func (c *phase5MCPControl) change(mode string) {
	if c.gateOpen != nil {
		select {
		case <-c.gateOpen:
		default:
			close(c.gateOpen)
		}
	}
	if c.gateTail != nil {
		select {
		case <-c.gateTail:
		default:
			close(c.gateTail)
		}
	}
	c.Mode = mode
	c.gateOpen = make(chan struct{})
	c.gateTail = make(chan struct{})
	c.ToolStarted = 0
	c.ToolFinished = 0
	c.MetaFlushed = 0
	c.OpenInvoked = 0
	c.FirstDelta = 0
	c.Done = 0
}
func (c *phase5MCPControl) serve(w http.ResponseWriter, r *http.Request) {
	c.Lock()
	defer c.Unlock()
	if r.Method == "POST" {
		var v struct {
			Mode             string `json:"mode"`
			ReleaseOpen      bool   `json:"release_open"`
			ReleaseTail      bool   `json:"release_tail"`
			EmbeddingFailure bool   `json:"embedding_failure"`
			BadAnswer        bool   `json:"bad_answer"`
		}
		if json.NewDecoder(r.Body).Decode(&v) != nil {
			w.WriteHeader(400)
			return
		}
		if v.Mode != "" {
			c.change(v.Mode)
		}
		c.EmbeddingFailure, c.BadAnswer = v.EmbeddingFailure, v.BadAnswer
		if v.ReleaseOpen {
			select {
			case <-c.gateOpen:
			default:
				close(c.gateOpen)
			}
		}
		if v.ReleaseTail {
			select {
			case <-c.gateTail:
			default:
				close(c.gateTail)
			}
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(c)
}

type phase5MCPModel struct {
	conversation.Model
	control *phase5MCPControl
}

func (m phase5MCPModel) Open(ctx context.Context, p tutor.Prepared, h []conversation.ModelMessage, q string) (conversation.Stream, error) {
	c := m.control
	c.Lock()
	c.OpenInvoked = time.Now().UnixNano()
	mode, gate, flushed := c.Mode, c.gateOpen, c.MetaFlushed
	c.Unlock()
	if flushed == 0 {
		return nil, fmt.Errorf("model invoked before meta Flush")
	}
	if mode == "hold_open" {
		select {
		case <-gate:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return m.Model.Open(ctx, p, h, q)
}

type phase5MCPWire struct {
	http.ResponseWriter
	control         *phase5MCPControl
	mode, lastEvent string
	deltas          int
}

func (w *phase5MCPWire) Unwrap() http.ResponseWriter { return w.ResponseWriter }
func (w *phase5MCPWire) Write(p []byte) (int, error) {
	if strings.HasPrefix(string(p), "event: ") {
		w.lastEvent = strings.SplitN(strings.TrimPrefix(string(p), "event: "), "\n", 2)[0]
	}
	if w.lastEvent == "done" && w.mode == "lost_done" {
		return 0, io.ErrClosedPipe
	}
	if w.lastEvent == "delta" {
		w.deltas++
		w.control.Lock()
		if w.control.FirstDelta == 0 {
			w.control.FirstDelta = time.Now().UnixNano()
		}
		w.control.Unlock()
		if w.mode == "disconnect" && w.deltas == 2 {
			conn, _, err := http.NewResponseController(w.ResponseWriter).Hijack()
			if err == nil {
				conn.Close()
			}
			return 0, io.ErrClosedPipe
		}
	}
	if w.lastEvent == "done" {
		w.control.Lock()
		w.control.Done = time.Now().UnixNano()
		w.control.Unlock()
	}
	return w.ResponseWriter.Write(p)
}
func (w *phase5MCPWire) FlushError() error {
	err := http.NewResponseController(w.ResponseWriter).Flush()
	if err == nil && w.lastEvent == "meta" {
		w.control.Lock()
		w.control.MetaFlushed = time.Now().UnixNano()
		w.control.Unlock()
	}
	return err
}

// This fixture never opens an existing database: currentFixture creates and
// validates a new random v3 database in the previously verified tmpfs instance.
func TestWeek6Phase5BrowserFixture(t *testing.T) {
	root := os.Getenv("W6_BROWSER_EVIDENCE")
	password := os.Getenv("W6_BROWSER_PASSWORD")
	qdrant := os.Getenv("ALIGN_STREAM_QDRANT")
	if root != "/evidence" || len(password) < 32 || !regexp.MustCompile(`^http://campusclaw-w6-p5-qdrant-[a-f0-9]{8}:6333$`).MatchString(qdrant) {
		t.Fatal("isolated browser environment required")
	}
	db, repo := currentFixture(t)
	ctx := context.Background()
	key, err := pbkdf2.Key(sha256.New, password, []byte("phase5MCP-test-only"), 1000000, 32)
	if err != nil {
		t.Fatal(err)
	}
	hash := "pbkdf2:sha256:1000000$phase5MCP-test-only$" + hex.EncodeToString(key)
	mustSQL(t, db, "INSERT INTO users(id,username,password_hash,role,class_id,session_version) VALUES(5,'sa2','test','student',1,0)")
	mustSQL(t, db, "UPDATE users SET password_hash=? WHERE id IN(1,2,3,4,5)", hash)
	if err = os.MkdirAll(filepath.Join(root, "uploads"), 0700); err != nil {
		t.Fatal(err)
	}
	files, err := storage.OpenPrivate(filepath.Join(root, "uploads"))
	if err != nil {
		t.Fatal(err)
	}
	defer files.Close()
	var vaultKey [32]byte
	rand.Read(vaultKey[:])
	vault, err := mcp.NewVault("browser", map[string][]byte{"browser": vaultKey[:]})
	if err != nil {
		t.Fatal(err)
	}
	var targets []mcp.Target
	if json.Unmarshal([]byte(os.Getenv("MCP_ALLOWED_TARGETS")), &targets) != nil {
		t.Fatal("targets")
	}
	policy, err := mcp.NewPolicy(targets, nil)
	if err != nil {
		t.Fatal(err)
	}
	mcpStore := MCP{DB: db, Vault: vault, Policy: policy}
	control := &phase5MCPControl{}
	control.change("normal")
	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var data struct {
			Input    string                      `json:"input"`
			Stream   bool                        `json:"stream"`
			Messages []conversation.ModelMessage `json:"messages"`
		}
		if json.NewDecoder(r.Body).Decode(&data) != nil {
			w.WriteHeader(400)
			return
		}
		if r.URL.Path == "/embeddings" {
			control.Lock()
			control.Embeddings++
			failed := control.EmbeddingFailure
			control.Unlock()
			if failed {
				w.WriteHeader(503)
				return
			}
			v := []float64{1, 0}
			if strings.Contains(data.Input, "海风") {
				v = []float64{-1, 0}
			}
			if strings.Contains(data.Input, "零命中") || strings.Contains(data.Input, "完全") {
				v = []float64{0, 1}
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"data": []any{map[string]any{"embedding": v}}})
			return
		}
		control.Lock()
		control.Models++
		control.LastMessages = data.Messages
		mode, gate := control.Mode, control.gateTail
		badAnswer := control.BadAnswer
		control.Unlock()
		if mode == "connection_fail" {
			w.WriteHeader(503)
			return
		}
		if !data.Stream {
			w.Header().Set("Content-Type", "application/json")
			answer := "单次回答[1]"
			if badAnswer {
				answer = "错误引用[99]"
			}
			json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]string{"content": answer}}}})
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		delta := func(text string) {
			b, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"delta": map[string]string{"content": text}}}})
			fmt.Fprintf(w, "data: %s\n\n", b)
			w.(http.Flusher).Flush()
		}
		delta("先分析𠮷条件，")
		if mode == "hold_tail" || mode == "hold_open" {
			select {
			case <-gate:
			case <-r.Context().Done():
				control.Lock()
				control.Cancelled++
				control.Unlock()
				return
			}
		}
		if mode == "cancel" {
			<-r.Context().Done()
			control.Lock()
			control.Cancelled++
			control.Unlock()
			return
		}
		if mode == "error" {
			io.WriteString(w, "data: {\"error\":{\"message\":\"controlled failure\"}}\n\n")
			return
		}
		time.Sleep(120 * time.Millisecond)
		delta("逐步解释。")
		if mode == "teaching" || mode == "p1p4" {
			guided := len(data.Messages) > 0 && strings.Contains(data.Messages[0].Content, "服务端解题引导边界")
			if guided {
				delta("请先思考等式两边应做什么。")
			} else {
				delta("结论：x=3。")
			}
		}
		// P1-P4 controlled responses expose only evidence actually received from
		// the production prompt assembler; this is not real-model quality evidence.
		if mode == "p1p4" && len(data.Messages) > 0 {
			var payload struct {
				Evidence []struct {
					Text string `json:"chunk_text"`
				} `json:"evidence"`
			}
			if json.Unmarshal([]byte(data.Messages[len(data.Messages)-1].Content), &payload) == nil {
				for _, evidence := range payload.Evidence {
					for _, marker := range []string{"P1P4-A-RED", "P1P4-B-BLUE"} {
						if strings.Contains(evidence.Text, marker) {
							delta("材料标识：" + marker + "。")
						}
					}
				}
			}
		}
		citation := "[1]"
		for _, m := range data.Messages {
			if m.Current {
				continue
			}
			if m.Role == "user" && strings.Contains(m.Content, "第二") {
				citation = "[2]"
			}
		}
		if badAnswer {
			citation = "[99]"
		}
		delta(citation)
		io.WriteString(w, "data: [DONE]\n\n")
		control.Lock()
		control.Completed++
		control.Unlock()
	}))
	defer gateway.Close()
	embedding := rag.Embedding{Gateway: rag.Gateway{Base: gateway.URL, Model: "controlled"}, Dimension: 2, Format: "openai"}
	collection := fmt.Sprintf("align_browser_%x", time.Now().UnixNano())
	vectors := rag.VectorClient{Gateway: rag.Gateway{Base: qdrant}, Dimension: 2, Collection: collection}
	indexer := &rag.Indexer{Store: RAG{DB: db}, Embedding: embedding, Vectors: vectors}
	materialIDs := []string{}
	for i := 0; i < 15; i++ {
		name := fmt.Sprintf("讲义%02d.md", i+1)
		if i < 2 {
			name = fmt.Sprintf("数学讲义%02d.md", i+1)
		}
		text := strings.Repeat("星河方程：理解条件和目标。", 25)
		if i < 2 {
			text += "示例方程 x+2=5，最终答案 x=3。"
		}
		staged, e := files.Prepare(name, strings.NewReader(text), storage.DefaultUploadLimit)
		if e != nil {
			t.Fatal(e)
		}
		tx, e := (Materials{DB: db}).Begin(ctx)
		if e != nil {
			t.Fatal(e)
		}
		if e = tx.Insert(material.Record{UploadInfo: staged.Info(), ClassID: 1, UploaderID: 2, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)}); e != nil {
			tx.Rollback()
			t.Fatal(e)
		}
		if e = staged.Publish(); e != nil {
			tx.Rollback()
			t.Fatal(e)
		}
		if e = tx.Commit(); e != nil {
			t.Fatal(e)
		}
		materialIDs = append(materialIDs, staged.Info().ID)
		if i < 2 {
			if state, e := indexer.Run(ctx, alignTeacher, staged.Info().ID, nil, true); e != nil || state != "ready" {
				t.Fatal("new fixture index", state, e)
			}
		}
	}
	skill, e := repo.SaveSkill(ctx, alignTeacher, 20, "名称：解题引导\n适用条件：星河,方程\n工具：knowledge_search\n启用：是\n\n帮助学生逐步理解。")
	if e != nil {
		t.Fatal(e)
	}
	agent, e := repo.SaveAgent(ctx, alignTeacher, 10, tutor.Config{Name: "浏览器学习助手", Prompt: "", Enabled: true, SkillIDs: []int64{skill.ID}, MaterialIDs: materialIDs[:2]})
	if e != nil {
		t.Fatal(e)
	}
	empty, e := repo.SaveAgent(ctx, alignTeacher, 0, tutor.Config{Name: "空材料助手", Prompt: "", Enabled: true, SkillIDs: []int64{}, MaterialIDs: []string{}})
	if e != nil {
		t.Fatal(e)
	}
	foreign, e := repo.SaveAgent(ctx, alignOtherTeacher, 0, tutor.Config{Name: "乙班助手", Prompt: "", Enabled: true, SkillIDs: []int64{}, MaterialIDs: []string{alignMaterialB}})
	if e != nil {
		t.Fatal(e)
	}
	tokens, e := auth.NewTokens(strings.Repeat("align-browser-test-only-", 3), nil)
	if e != nil {
		t.Fatal(e)
	}
	identity := &auth.Identity{Store: LoginUsers{DB: db}, Tokens: tokens}
	login := &auth.Login{Users: LoginUsers{DB: db}, Tokens: tokens}
	store := Conversations{DB: db}
	retrieval := rag.Retrieval{Store: RAG{DB: db}, Embedding: embedding, Vectors: vectors}
	reads := &httpapi.MaterialReads{Reader: material.Reader{Repository: Materials{DB: db}}, Store: files}
	upload := &httpapi.UploadHandler{Store: files, Uploader: material.Uploader{Repository: Materials{DB: db}}, Indexer: indexer}
	model := phase5MCPModel{Model: conversation.StreamingModel{Gateway: rag.Gateway{Base: gateway.URL, Model: "controlled", Key: "test-only"}}, control: control}
	conversations := &httpapi.ConversationHandler{Store: store, Tutor: repo, Retrieval: retrieval, Model: model, Tools: phase5MCPRunner{Runner: mcp.Runner{Store: mcpStore}, control: control}}
	app := httpapi.WithConversations(phase2Health{}, login, identity, upload, reads, &httpapi.RAGHandler{Retrieval: retrieval, Dialogue: rag.DialogueClient{Gateway: rag.Gateway{Base: gateway.URL, Model: "controlled"}}, Indexer: indexer}, &httpapi.TutorHandler{Store: repo}, conversations)
	app = httpapi.WithMCP(app, identity, &httpapi.MCPHandler{Store: mcpStore})
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/__phase5MCP/control" {
			control.serve(w, r)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/") {
			control.Lock()
			if r.Header.Get("X-User-Id") != "" || r.Header.Get("X-Role") != "" || r.Header.Get("X-Class-Id") != "" {
				control.HeaderViolation = true
			}
			mode := control.Mode
			control.Unlock()
			if strings.HasSuffix(r.URL.Path, "/messages") {
				wire := &phase5MCPWire{ResponseWriter: w, control: control, mode: mode}
				app.ServeHTTP(wire, r)
				return
			}
		}
		app.ServeHTTP(w, r)
	})
	listener, e := net.Listen("tcp", ":8080")
	if e != nil {
		t.Fatal(e)
	}
	server := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second}
	defer server.Close()
	go server.Serve(listener)
	var database string
	if e = db.QueryRow("SELECT DATABASE()").Scan(&database); e != nil {
		t.Fatal(e)
	}
	ready := map[string]any{"database": database, "collection": collection, "agent_id": agent.ID, "empty_agent_id": empty.ID, "foreign_agent_id": foreign.ID, "skill_id": skill.ID, "materials": materialIDs, "foreign_material_id": alignMaterialB, "users": map[string]string{"teacher": "ta", "student": "sa", "other_student": "sa2", "foreign_student": "sb", "foreign_teacher": "tb"}}
	b, _ := json.MarshalIndent(ready, "", "  ")
	if e = os.WriteFile(filepath.Join(root, "ready.json"), b, 0600); e != nil {
		t.Fatal(e)
	}
	deadline := time.Now().Add(20 * time.Minute)
	if os.Getenv("W6_BROWSER_RETAIN") == "1" {
		deadline = time.Now().Add(24 * time.Hour)
	}
	for {
		if _, e = os.Stat(filepath.Join(root, "stop")); e == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("browser acceptance timeout")
		}
		time.Sleep(200 * time.Millisecond)
	}
	control.Lock()
	stats, _ := json.MarshalIndent(control, "", "  ")
	control.Unlock()
	os.WriteFile(filepath.Join(root, "stats.json"), stats, 0600)
}

// Test-only observer delegates to the real runner and real separate MCP process.
type phase5MCPRunner struct {
	mcp.Runner
	control *phase5MCPControl
}

func (r phase5MCPRunner) Run(ctx context.Context, p auth.Principal, refs []string, q string, cid int64) ([]mcp.ToolCall, error) {
	r.control.Lock()
	r.control.ToolStarted = time.Now().UnixNano()
	r.control.ToolRuns++
	r.control.Unlock()
	calls, err := r.Runner.Run(ctx, p, refs, q, cid)
	r.control.Lock()
	r.control.ToolFinished = time.Now().UnixNano()
	r.control.Unlock()
	return calls, err
}
