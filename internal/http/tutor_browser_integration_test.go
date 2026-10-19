//go:build tutorbrowser

package httpapi

import (
	"campusclaw/internal/auth"
	"campusclaw/internal/config"
	"campusclaw/internal/conversation"
	"campusclaw/internal/material"
	"campusclaw/internal/provision"
	"campusclaw/internal/rag"
	repository "campusclaw/internal/repository/mysql"
	"campusclaw/internal/storage"
	"campusclaw/internal/tutor"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// This fixture is a real Go HTTP service backed by the existing isolated DB.
// It creates only new rows/files/collection and never runs migration/provision.
func TestTutorBrowserFixture(t *testing.T) {
	if os.Getenv("WEEK5_REAL_MODEL") == "1" {
		deadline := time.Now().Add(30 * time.Second)
		for {
			if _, err := os.Stat("/evidence/network-ready"); err == nil {
				break
			}
			if time.Now().After(deadline) {
				t.Fatal("isolated/outbound network attachment unavailable")
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
	ctx := context.Background()
	db, e := sql.Open("mysql", os.Getenv("WEEK5_TEST_DSN"))
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	var name string
	if db.QueryRow("SELECT DATABASE()").Scan(&name) != nil || name != "week5_rag_test" {
		t.Fatal("isolated DB required")
	}
	if e = repository.ValidateRAGSchema(ctx, db); e != nil {
		t.Fatal(e)
	}
	root := os.Getenv("WEEK5_BROWSER_EVIDENCE")
	if root != "/evidence" {
		t.Fatal("isolated evidence mount required")
	}
	fileRoot := filepath.Join(root, "uploads")
	if e = os.MkdirAll(fileRoot, 0700); e != nil {
		t.Fatal(e)
	}
	files, e := storage.OpenPrivate(fileRoot)
	if e != nil {
		t.Fatal(e)
	}
	defer files.Close()
	apiTest := os.Getenv("WEEK5_APITEST") == "1"
	realMode := os.Getenv("WEEK5_REAL_MODEL") == "1"
	if apiTest && realMode {
		t.Fatal("real and controlled modes cannot be combined")
	}
	var controlMu sync.Mutex
	control := struct {
		EmbeddingFailure bool                        `json:"embedding_failure"`
		DialogueFailure  bool                        `json:"dialogue_failure"`
		BadAnswer        bool                        `json:"bad_answer"`
		Embeddings       int                         `json:"embeddings"`
		Dialogues        int                         `json:"dialogues"`
		Cancelled        int                         `json:"cancelled"`
		Completed        int                         `json:"completed"`
		Last             []conversation.ModelMessage `json:"last_messages"`
	}{}
	gateway := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Control belongs only to this controlled model gateway, never business API.
		if apiTest && r.URL.Path == "/control" {
			controlMu.Lock()
			defer controlMu.Unlock()
			if r.Method == "POST" {
				var flags struct {
					EmbeddingFailure bool `json:"embedding_failure"`
					DialogueFailure  bool `json:"dialogue_failure"`
					BadAnswer        bool `json:"bad_answer"`
				}
				if json.NewDecoder(r.Body).Decode(&flags) != nil {
					w.WriteHeader(400)
					return
				}
				control.EmbeddingFailure = flags.EmbeddingFailure
				control.DialogueFailure = flags.DialogueFailure
				control.BadAnswer = flags.BadAnswer
			}
			json.NewEncoder(w).Encode(control)
			return
		}
		var body map[string]json.RawMessage
		if json.NewDecoder(r.Body).Decode(&body) != nil {
			w.WriteHeader(400)
			return
		}
		if r.URL.Path == "/embeddings" {
			controlMu.Lock()
			control.Embeddings++
			failed := control.EmbeddingFailure
			controlMu.Unlock()
			if failed {
				w.WriteHeader(503)
				return
			}
			var input string
			json.Unmarshal(body["input"], &input)
			v := []float64{0, 0, 1}
			if strings.Contains(input, "星河") {
				v = []float64{1, 0, 0}
			}
			if strings.Contains(input, "海风") {
				v = []float64{0, 1, 0}
			}
			json.NewEncoder(w).Encode(map[string]any{"data": []any{map[string]any{"embedding": v}}})
			return
		}
		var messages []conversation.ModelMessage
		json.Unmarshal(body["messages"], &messages)
		controlMu.Lock()
		control.Dialogues++
		control.Last = messages
		dialogueFailure := control.DialogueFailure
		badAnswer := control.BadAnswer
		controlMu.Unlock()
		if dialogueFailure {
			w.WriteHeader(503)
			return
		}
		var current struct {
			Question string `json:"question"`
		}
		if len(messages) > 0 {
			json.Unmarshal([]byte(messages[len(messages)-1].Content), &current)
		}
		var streaming bool
		json.Unmarshal(body["stream"], &streaming)
		if !streaming {
			answer := "单次回答 [1]"
			if badAnswer {
				answer = "非法来源 [999]"
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
		first := "先分析条件，"
		if apiTest && len(messages) > 0 && !strings.Contains(messages[0].Content, "解题引导约束优先") {
			first = "依据资料可得结论，"
		}
		delta(first)
		select {
		case <-time.After(3 * time.Second):
		case <-r.Context().Done():
			controlMu.Lock()
			control.Cancelled++
			controlMu.Unlock()
			return
		}
		if strings.Contains(current.Question, "失败") {
			fmt.Fprint(w, "data: {\"error\":{\"message\":\"controlled\"}}\n\n")
			return
		}
		delta("提示下一步，")
		if badAnswer {
			delta("非法来源 [999]")
		} else {
			delta("依据材料 [1]")
		}
		fmt.Fprint(w, "data: [DONE]\n\n")
		controlMu.Lock()
		control.Completed++
		controlMu.Unlock()
	}))
	if apiTest {
		listener, err := net.Listen("tcp", ":8090")
		if err != nil {
			t.Fatal(err)
		}
		gateway.Listener.Close()
		gateway.Listener = listener
	}
	gateway.Start()
	defer gateway.Close()
	var secret [32]byte
	rand.Read(secret[:])
	tokens, _ := auth.NewTokens(hex.EncodeToString(secret[:]), time.Now)
	login := &auth.Login{Users: repository.LoginUsers{DB: db}, Tokens: tokens}
	identity := &auth.Identity{Store: repository.LoginUsers{DB: db}, Tokens: tokens}
	// Insert the new material via the existing upload transaction, with a fixture record.
	materialText := strings.Repeat("星河解题先分析已知与目标。", 95)
	if realMode {
		materialText = "一元一次方程教学讲义。题目：2x+3=11。等式性质：两边同时减去相同的数，或同时除以相同的非零数，等式仍成立。先识别未知数、已知条件和目标，再移去常数项，然后消去未知数的系数。完整解答是：两边减3得到2x=8，再两边除以2得到x=4。教师启用解题引导时只提示下一步，不直接泄露最后结果。"
	}
	staged, e := files.Prepare("browser-source.md", strings.NewReader(materialText), storage.DefaultUploadLimit)
	if e != nil {
		t.Fatal(e)
	}
	tx, e := (repository.Materials{DB: db}).Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback()
	if e = tx.Insert(material.Record{UploadInfo: staged.Info(), ClassID: 1, UploaderID: 1, CreatedAt: time.Now().UTC().Format("2006-01-02T15:04:05.000Z")}); e != nil {
		t.Fatal(e)
	}
	if e = staged.Publish(); e != nil {
		t.Fatal(e)
	}
	if e = tx.Commit(); e != nil {
		t.Fatal(e)
	}
	embedding := rag.Embedding{Gateway: rag.Gateway{Base: gateway.URL, Key: "controlled", Model: "test"}, Dimension: 3, Format: "openai"}
	vector := rag.VectorClient{Gateway: rag.Gateway{Base: "http://qdrant:6333"}, Dimension: 3, Collection: "week5_browser_" + strings.ReplaceAll(staged.Info().ID, "-", "")}
	dialogueGateway := rag.Gateway{Base: gateway.URL, Key: "controlled", Model: "test"}
	if realMode {
		rc, err := config.LoadRAG(os.Getenv)
		if err != nil {
			t.Fatal(err)
		}
		transport := &realBudgetTransport{path: filepath.Join(root, "real-stats.json"), embeddingModel: rc.Embedding.Model, dialogueModel: rc.Dialogue.Model}
		embedding = rc.Embedding
		embedding.Client = &http.Client{Transport: transport, Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
		vector.Dimension = rc.Embedding.Dimension
		dialogueGateway = rc.Dialogue.Gateway
		dialogueGateway.Client = &http.Client{Transport: transport, Timeout: 120 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	}
	indexer := &rag.Indexer{Store: repository.RAG{DB: db}, Embedding: embedding, Vectors: vector}
	scope := rag.Scope{UserID: 1, ClassID: 1, Role: "teacher"}
	if _, e = indexer.Run(ctx, scope, staged.Info().ID, nil, true); e != nil {
		t.Fatal(e)
	}
	config := repository.Tutor{DB: db}
	skills, e := config.Skills(ctx, scope)
	if e != nil || len(skills) != 1 {
		t.Fatal("skill fixture")
	}
	agent, e := config.SaveAgent(ctx, scope, 0, tutor.Config{Name: "浏览器专项-" + time.Now().Format("150405.000000"), Prompt: "耐心引导", Enabled: true, SkillIDs: []int64{skills[0].ID}, MaterialIDs: []string{staged.Info().ID}})
	if e != nil {
		t.Fatal(e)
	}
	retrieval := rag.Retrieval{Store: repository.RAG{DB: db}, Embedding: embedding, Vectors: vector}
	reads := &MaterialReads{Reader: material.Reader{Repository: repository.Materials{DB: db}}, Store: files}
	upload := &UploadHandler{Store: files, Uploader: material.Uploader{Repository: repository.Materials{DB: db}}}
	if apiTest {
		upload.Indexer = indexer
	}
	conversations := &ConversationHandler{Store: repository.Conversations{DB: db}, Tutor: config, Retrieval: retrieval, Model: conversation.StreamingModel{Gateway: dialogueGateway}}
	h := WithConversations(browserFixtureHealth{}, login, identity, upload, reads, &RAGHandler{Retrieval: retrieval, Dialogue: rag.DialogueClient{Gateway: dialogueGateway}, Indexer: indexer}, &TutorHandler{Store: config}, conversations)
	server := &http.Server{Addr: ":8080", Handler: h, ReadHeaderTimeout: 5 * time.Second}
	defer server.Close()
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			t.Error(err)
		}
	}()
	ready := map[string]any{"agent_id": agent.ID, "agent_name": agent.Name, "material_id": staged.Info().ID}
	if realMode {
		pending, e := files.Prepare("real-pending.md", strings.NewReader("本轮尚未索引的验收材料。"), storage.DefaultUploadLimit)
		if e != nil {
			t.Fatal(e)
		}
		pendingTx, e := (repository.Materials{DB: db}).Begin(ctx)
		if e != nil {
			t.Fatal(e)
		}
		defer pendingTx.Rollback()
		if e = pendingTx.Insert(material.Record{UploadInfo: pending.Info(), ClassID: 1, UploaderID: 1, CreatedAt: time.Now().UTC().Format("2006-01-02T15:04:05.000Z")}); e != nil {
			t.Fatal(e)
		}
		if e = pending.Publish(); e != nil {
			t.Fatal(e)
		}
		if e = pendingTx.Commit(); e != nil {
			t.Fatal(e)
		}
		noEvidence, e := config.SaveAgent(ctx, scope, 0, tutor.Config{Name: "真实无证据-" + staged.Info().ID, Prompt: "只依据材料回答", Enabled: true, SkillIDs: []int64{skills[0].ID}, MaterialIDs: []string{pending.Info().ID}})
		if e != nil {
			t.Fatal(e)
		}
		ready["no_evidence_agent_id"] = noEvidence.ID
	}
	if apiTest {
		a2 := "student_a2_apitest_" + strings.ReplaceAll(staged.Info().ID, "-", "")
		if _, err := provision.CreateUser(ctx, db, a2, os.Getenv("WEEK5_ACCOUNT_PASSWORD"), "student", 1); err != nil {
			t.Fatal(err)
		}
		ready["a2_username"] = a2
	}
	b, _ := json.Marshal(ready)
	if e = os.WriteFile(filepath.Join(root, "ready.json"), b, 0600); e != nil {
		t.Fatal(e)
	}
	deadline := time.Now().Add(8 * time.Minute)
	for time.Now().Before(deadline) {
		if _, e = os.Stat(filepath.Join(root, "stop")); e == nil {
			t.Log("browser fixture completed without migration")
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatal("browser runner timed out")
}

type browserFixtureHealth struct{}

func (browserFixtureHealth) Check(context.Context) error { return nil }
