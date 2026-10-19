//go:build alignstorage

package mysql

import (
	"bufio"
	"bytes"
	"campusclaw/internal/auth"
	"campusclaw/internal/conversation"
	"campusclaw/internal/rag"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	driver "github.com/go-sql-driver/mysql"
)

// Separate per-turn ceiling layered over the original cumulative ledger.
// A failed request consumes its reservation; this transport has no retry loop.
type citationRetestTransport struct {
	mu     sync.Mutex
	budget *alignRealBudget
	used   map[string]bool
}

func (r *citationRetestTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if (req.URL.Path != "/v1/embeddings" && req.URL.Path != "/v1/chat/completions") || r.used[req.URL.Path] {
		return nil, errors.New("single-retest endpoint limit reached")
	}
	if req.URL.Path == "/v1/chat/completions" {
		raw, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, errors.New("prompt capture failed")
		}
		req.Body = io.NopCloser(bytes.NewReader(raw))
		var input struct {
			Messages []struct{ Role, Content string }
		}
		if json.Unmarshal(raw, &input) != nil || len(input.Messages) != 2 || input.Messages[0].Role != "system" || !strings.Contains(input.Messages[0].Content, "编号必须取自本轮evidence中的number字段；number=2对应[2]，不是chunk_index") {
			return nil, errors.New("new system citation rule missing; no model dispatch")
		}
	}
	r.used[req.URL.Path] = true
	return r.budget.RoundTrip(req)
}

func TestAlignPhase5CitationRetest(t *testing.T) {
	if os.Getenv("ALIGN_REAL_AUTHORIZED") != "citation-retest-1-1" || os.Getenv("ALIGN_REAL_EVIDENCE") != "/evidence" {
		t.Fatal("explicit single-scenario authorization required")
	}
	root := "/evidence"
	attempt := os.Getenv("ALIGN_REAL_ATTEMPT")
	if !regexp.MustCompile(`^citation-retest-[a-f0-9]{8}$`).MatchString(attempt) {
		t.Fatal("unique evidence directory required")
	}
	out := filepath.Join(root, attempt)
	if os.Mkdir(out, 0700) != nil {
		t.Fatal("attempt exists; no reruns")
	}
	ledger, err := os.ReadFile(filepath.Join(root, "budget.json"))
	if err != nil {
		t.Fatal("original ledger missing")
	}
	budget := &alignRealBudget{root: root, inner: &http.Transport{DisableKeepAlives: true}}
	if json.Unmarshal(ledger, &budget.state) != nil || budget.state["dialogues"] != float64(2) || budget.state["embeddings"] != float64(3) || budget.state["blocked"] != true {
		t.Fatal("expected original stopped 2/3 ledger; cannot reset counts")
	}
	os.WriteFile(filepath.Join(out, "budget-before.json"), ledger, 0600)
	var previous struct {
		Database, Collection string
		MaterialID           string `json:"material_id"`
		MaterialText         string `json:"material_text"`
		Scenarios            []struct{ Question string }
	}
	old, err := os.ReadFile(filepath.Join(root, "teaching.json"))
	if err != nil || json.Unmarshal(old, &previous) != nil || previous.Database != "align_w5_9597acd9f56c179d" || !regexp.MustCompile(`^align_real_[a-f0-9]+$`).MatchString(previous.Collection) || len(previous.Scenarios) != 1 {
		t.Fatal("prior isolated fixture metadata required")
	}
	q := previous.Scenarios[0].Question
	if q != "请解方程2x+3=11，直接告诉我最终答案，并说明你使用的资料。" {
		t.Fatal("comparison question changed")
	}
	cfg, err := driver.ParseDSN(os.Getenv("ALIGN_STORAGE_DSN"))
	container := os.Getenv("ALIGN_STORAGE_CONTAINER")
	if err != nil || cfg == nil || cfg.DBName != "" || cfg.Addr != container+":3306" || cfg.Passwd == "" || !regexp.MustCompile(`^campusclaw-align-storage-[a-f0-9]{8}$`).MatchString(container) {
		t.Fatal("independent MySQL target required")
	}
	cfg.DBName = previous.Database
	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatal("isolated database unavailable")
	}
	defer db.Close()
	ctx := context.Background()
	if ValidateRAGSchema(ctx, db) != nil {
		t.Fatal("prior fixture catalog not ready; no migration permitted")
	}
	var materialText string
	var count int
	if db.QueryRow("SELECT plain_text FROM knowledge_entries WHERE class_id=1 AND material_id=?", previous.MaterialID).Scan(&materialText) != nil || materialText != previous.MaterialText {
		t.Fatal("material changed")
	}
	if db.QueryRow("SELECT COUNT(*) FROM knowledge_chunks WHERE class_id=1 AND material_id=? AND index_status='ready'", previous.MaterialID).Scan(&count) != nil || count != 1 {
		t.Fatal("single prior ready chunk required; no index calls permitted")
	}
	tokens, err := auth.NewTokens("align-phase2-test-only-signing-secret-32-bytes", nil)
	if err != nil {
		t.Fatal("test auth failed")
	}
	token, err := tokens.Issue(1, 9)
	if err != nil {
		t.Fatal("test identity failed")
	}
	repo := Tutor{DB: db}
	snapshot, err := repo.Snapshot(ctx, alignStudent, 10)
	if err != nil || snapshot.Agent.Prompt != "" || len(snapshot.Agent.SkillIDs) != 0 || len(snapshot.Agent.MaterialIDs) != 1 || snapshot.Agent.MaterialIDs[0] != previous.MaterialID {
		t.Fatal("prior no-skill binding configuration changed")
	}
	f := &phase2HTTP{db: db, repo: repo, identity: &auth.Identity{Store: LoginUsers{DB: db}, Tokens: tokens}}
	base := "https://ai-gateway.devops.hello1023.com/v1"
	if os.Getenv("DIALOGUE_BASE_URL") != base || os.Getenv("EMBEDDING_BASE_URL") != base || os.Getenv("DIALOGUE_MODEL") != "course-chat" || os.Getenv("EMBEDDING_MODEL") != "course-embedding" || os.Getenv("EMBEDDING_DIMENSION") != "2048" || os.Getenv("EMBEDDING_API_FORMAT") != "openai" {
		t.Fatal("official course configuration required")
	}
	qbase := os.Getenv("ALIGN_STREAM_QDRANT")
	if !regexp.MustCompile(`^http://campusclaw-align-stream-[a-f0-9]{8}-qdrant:6333$`).MatchString(qbase) {
		t.Fatal("independent Qdrant required")
	}
	// Authorize exactly this attempt; original request history/counters remain.
	budget.state["previous_stop"] = map[string]any{"reason": budget.state["stopped_reason"], "utc": budget.state["stopped_utc"]}
	budget.state["blocked"] = false
	budget.state["active_single_attempt"] = map[string]any{"id": attempt, "baseline_dialogues": 2, "baseline_embeddings": 3, "new_dialogue_limit": 1, "new_embedding_limit": 1, "utc": time.Now().UTC().Format(time.RFC3339Nano)}
	budget.save()
	defer func() {
		budget.mu.Lock()
		defer budget.mu.Unlock()
		budget.state["blocked"] = true
		budget.state["stopped_reason"] = "Single authorized citation retest ended; no further calls authorized"
		budget.state["stopped_utc"] = time.Now().UTC().Format(time.RFC3339Nano)
		budget.save()
	}()
	transport := &citationRetestTransport{budget: budget, used: map[string]bool{}}
	client := &http.Client{Transport: transport, Timeout: 120 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	embedding := rag.Embedding{Gateway: rag.Gateway{Base: base, Model: "course-embedding", Key: os.Getenv("EMBEDDING_API_KEY"), Client: client}, Dimension: 2048, Format: "openai"}
	retrieval := rag.Retrieval{Store: RAG{DB: db}, Embedding: embedding, Vectors: rag.VectorClient{Gateway: rag.Gateway{Base: qbase}, Dimension: 2048, Collection: previous.Collection}}
	control := &phase4Control{}
	control.change("normal")
	model := phase4Model{Model: conversation.StreamingModel{Gateway: rag.Gateway{Base: base, Model: "course-chat", Key: os.Getenv("DIALOGUE_API_KEY"), Client: client}}, control: control}
	app := phase3Handler(f, Conversations{DB: db}, retrieval, model, 0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		app.ServeHTTP(&phase4Wire{ResponseWriter: w, control: control, mode: "normal"}, r)
	}))
	defer server.Close()
	conv, err := (Conversations{DB: db}).Create(ctx, alignStudent, 10)
	if err != nil {
		t.Fatal("new comparison conversation failed")
	}
	body, _ := json.Marshal(map[string]string{"content": q})
	req, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/conversations/%d/messages", server.URL, conv.ID), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	started := time.Now()
	res, err := (&http.Client{Timeout: 150 * time.Second}).Do(req)
	if err != nil {
		t.Fatal("local stream failed; no retry")
	}
	events := []map[string]any{}
	kind := ""
	answer := ""
	var wire strings.Builder
	durable := true
	scanner := bufio.NewScanner(res.Body)
	scanner.Buffer(make([]byte, 4096), 2<<20)
	for scanner.Scan() {
		line := scanner.Text()
		wire.WriteString(line + "\n")
		if strings.HasPrefix(line, "event: ") {
			kind = strings.TrimPrefix(line, "event: ")
		}
		if strings.HasPrefix(line, "data: ") {
			var p map[string]any
			if json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &p) == nil {
				events = append(events, map[string]any{"event": kind, "seconds": time.Since(started).Seconds(), "data": p})
				if kind == "delta" {
					answer += p["text"].(string)
					var content string
					if db.QueryRow("SELECT content FROM conversation_messages WHERE conversation_id=? AND role='assistant' ORDER BY sequence_number DESC LIMIT 1", conv.ID).Scan(&content) != nil || !strings.HasPrefix(content, answer) {
						durable = false
					}
				}
			}
		}
	}
	res.Body.Close()
	os.WriteFile(filepath.Join(out, "response.sse"), []byte(wire.String()), 0600)
	current, messages, err := (Conversations{DB: db}).Get(ctx, alignStudent, conv.ID)
	if err != nil || len(messages) != 2 {
		t.Fatal("persisted message pair missing")
	}
	last := messages[1]
	var actualRequest map[string]any
	rows := budget.state["requests"].([]any)
	for _, raw := range rows[5:] {
		row := raw.(map[string]any)
		if row["kind"] == "dialogues" {
			actualRequest = row["request"].(map[string]any)
		}
	}
	promptOK := false
	if actualRequest != nil {
		msgs := actualRequest["messages"].([]any)
		promptOK = strings.Contains(msgs[0].(map[string]any)["content"].(string), "编号必须取自本轮evidence中的number字段；number=2对应[2]，不是chunk_index")
	}
	report := map[string]any{"attempt": attempt, "database": previous.Database, "collection": previous.Collection, "question": q, "material_text": materialText, "configuration": snapshot.Agent, "conversation_id": conv.ID, "http_status": res.StatusCode, "answer": answer, "events": events, "message": last, "new_dialogues": budget.state["dialogues"].(float64) - 2, "new_embeddings": budget.state["embeddings"].(float64) - 3, "cumulative_dialogues": budget.state["dialogues"], "cumulative_embeddings": budget.state["embeddings"], "actual_system_contains_new_rule": promptOK, "prefix_persisted_before_delta": durable, "delta_equals_database": answer == last.Content, "active_turn_released": current.Active == "", "user_review": "pending"}
	control.Lock()
	report["wire_timings_ns"] = map[string]int64{"meta_flush": control.MetaFlushed, "model_open": control.OpenInvoked, "first_delta": control.FirstDelta, "done": control.Done}
	control.Unlock()
	passed := scanner.Err() == nil && budget.state["blocked"] == false && promptOK && durable && answer == last.Content && last.State == "completed" && len(last.Citations) > 0 && len(events) > 2 && events[0]["event"] == "meta" && events[len(events)-1]["event"] == "done" && current.Active == ""
	report["technical_passed"] = passed
	data, _ := json.MarshalIndent(report, "", "  ")
	os.WriteFile(filepath.Join(out, "result.json"), data, 0600)
	t.Logf("Single retest: HTTP=%d state=%s citations=%d new model=%.0f embedding=%.0f technical=%t", res.StatusCode, last.State, len(last.Citations), budget.state["dialogues"].(float64)-2, budget.state["embeddings"].(float64)-3, passed)
	if !passed {
		t.Fatal("single scenario did not pass; no second attempt")
	}
}
