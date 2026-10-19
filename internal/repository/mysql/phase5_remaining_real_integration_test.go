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

// Remaining real scenarios use the original cumulative ledger.
type remainingRealTransport struct {
	mu       sync.Mutex
	budget   *alignRealBudget
	question string
}

func (r *remainingRealTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if req.URL.Path == "/v1/chat/completions" {
		raw, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, errors.New("capture failed")
		}
		req.Body = io.NopCloser(bytes.NewReader(raw))
		var input struct {
			Messages []struct{ Role, Content string }
		}
		if json.Unmarshal(raw, &input) != nil || len(input.Messages) < 2 || len(input.Messages) > 7 {
			return nil, errors.New("invalid message window; no dispatch")
		}
		var payload struct {
			Question string
			Evidence []any
		}
		if json.Unmarshal([]byte(input.Messages[len(input.Messages)-1].Content), &payload) != nil || payload.Question != r.question {
			return nil, errors.New("current question missing or duplicated")
		}
		if len(payload.Evidence) > 0 && !strings.Contains(input.Messages[0].Content, "number=2对应[2]，不是chunk_index") {
			return nil, errors.New("citation rule missing")
		}
	}
	return r.budget.RoundTrip(req)
}
func TestAlignPhase5RemainingReal(t *testing.T) {
	if os.Getenv("ALIGN_REAL_AUTHORIZED") != "remaining-real-9-4" || os.Getenv("ALIGN_REAL_EVIDENCE") != "/evidence" {
		t.Fatal("explicit single-scenario authorization required")
	}
	root := "/evidence"
	attempt := os.Getenv("ALIGN_REAL_ATTEMPT")
	if !regexp.MustCompile(`^remaining-real-[a-f0-9]{8}$`).MatchString(attempt) {
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
	if json.Unmarshal(ledger, &budget.state) != nil || budget.state["dialogues"] != float64(3) || budget.state["embeddings"] != float64(4) || budget.state["blocked"] != true {
		t.Fatal("expected original stopped 3/4 ledger; cannot reset counts")
	}
	os.WriteFile(filepath.Join(out, "budget-before.json"), ledger, 0600)
	var previous struct {
		Database, Collection string
		SkillID              int64  `json:"skill_id"`
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

	budget.state["previous_stop"] = map[string]any{"reason": budget.state["stopped_reason"], "utc": budget.state["stopped_utc"]}
	budget.state["blocked"] = false
	budget.state["active_remaining_attempt"] = map[string]any{"id": attempt, "baseline_dialogues": 3, "baseline_embeddings": 4, "new_dialogue_limit": 9, "new_embedding_limit": 4, "utc": time.Now().UTC().Format(time.RFC3339Nano)}
	budget.save()
	defer func() {
		budget.mu.Lock()
		defer budget.mu.Unlock()
		budget.state["blocked"] = true
		budget.state["stopped_reason"] = "Remaining authorized teaching attempt ended; user review required before further calls"
		budget.state["stopped_utc"] = time.Now().UTC().Format(time.RFC3339Nano)
		budget.save()
	}()
	transport := &remainingRealTransport{budget: budget}
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
	report := map[string]any{"attempt": attempt, "database": previous.Database, "collection": previous.Collection, "material_text": materialText, "prior_no_skill_user_review": "passed by explicit user confirmation before this run", "rounds": []any{}, "user_review": "all new answers pending"}
	save := func() {
		data, _ := json.MarshalIndent(report, "", "  ")
		os.WriteFile(filepath.Join(out, "result.json"), data, 0600)
	}
	save()
	defer save()
	type scenario struct {
		name, question                             string
		evidence, bind, enabled, history, selected bool
	}
	cases := []scenario{
		{"evidence_skill_enabled", q, true, true, true, false, true},
		{"empty_no_skill", "我的学习代号是蓝鲸。我在练习等式变形，请用一般知识说明如何保持两边相等，并明确没有本轮材料依据。", false, false, true, true, false},
		{"empty_skill", "继续刚才的目标，帮我解题：2x+3=11。请只给下一步提示，不给最终答案。", false, true, true, true, true},
		{"evidence_skill_disabled", q, true, true, false, false, false},
		{"evidence_trigger_miss", "请计算2x+3=11中的x，并说明你使用的资料。", true, true, true, false, false},
		{"evidence_skill_unbound", q, true, false, true, false, false},
		{"history_round3", "还记得我的学习代号吗？先复述代号，再解释等式两边都减3会怎样，暂不计算最终结果。", false, true, true, true, false},
		{"history_round4", "继续刚才的讨论，请用方程解题引导方式提示系数处理，只给一个检查问题，不给最终答案。", false, true, true, true, true},
	}
	historyID := int64(0)
	for index, s := range cases {
		if budget.state["blocked"] == true {
			t.Fatal("upstream blocked; no retry")
		}
		if _, err = repo.Toggle(ctx, alignTeacher, previous.SkillID, s.enabled); err != nil {
			t.Fatal("test skill toggle failed")
		}
		materials := []string{}
		skills := []int64{}
		if s.evidence {
			materials = []string{previous.MaterialID}
		}
		if s.bind {
			skills = []int64{previous.SkillID}
		}
		phase3Bind(t, f, materials, skills)
		cid := int64(0)
		if s.history {
			cid = historyID
		}
		if cid == 0 {
			conv, e := (Conversations{DB: db}).Create(ctx, alignStudent, 10)
			if e != nil {
				t.Fatal("create failed")
			}
			cid = conv.ID
			if s.history {
				historyID = cid
			}
		}
		control.Lock()
		control.MetaFlushed = 0
		control.OpenInvoked = 0
		control.FirstDelta = 0
		control.Done = 0
		control.Unlock()
		transport.question = s.question
		beforeD, beforeE := budget.state["dialogues"].(float64), budget.state["embeddings"].(float64)
		body, _ := json.Marshal(map[string]string{"content": s.question})
		req, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/conversations/%d/messages", server.URL, cid), bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		started := time.Now()
		res, e := (&http.Client{Timeout: 150 * time.Second}).Do(req)
		if e != nil {
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
						if db.QueryRow("SELECT content FROM conversation_messages WHERE conversation_id=? AND role='assistant' ORDER BY sequence_number DESC LIMIT 1", cid).Scan(&content) != nil || !strings.HasPrefix(content, answer) {
							durable = false
						}
					}
				}
			}
		}
		res.Body.Close()
		os.WriteFile(filepath.Join(out, s.name+".sse"), []byte(wire.String()), 0600)
		current, messages, e := (Conversations{DB: db}).Get(ctx, alignStudent, cid)
		if e != nil || len(messages) < 2 {
			t.Fatal("history failed")
		}
		last := messages[len(messages)-1]
		rows := budget.state["requests"].([]any)
		row := rows[len(rows)-1].(map[string]any)
		var rawAnswer strings.Builder
		var modelMessages []any
		if row["kind"] == "dialogues" {
			modelMessages = row["request"].(map[string]any)["messages"].([]any)
			if name, ok := row["raw_response_file"].(string); ok {
				raw, _ := os.ReadFile(filepath.Join(root, name))
				for _, line := range strings.Split(string(raw), "\n") {
					if strings.HasPrefix(line, "data: ") {
						var packet struct {
							Choices []struct{ Delta struct{ Content string } }
						}
						if json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &packet) == nil {
							for _, choice := range packet.Choices {
								rawAnswer.WriteString(choice.Delta.Content)
							}
						}
					}
				}
			}
		}
		flags := []string{}
		if scanner.Err() != nil || budget.state["blocked"] == true || res.StatusCode != 200 {
			flags = append(flags, "http_or_stream_error")
		}
		if last.State != "completed" {
			flags = append(flags, "not_completed")
		}
		if !durable || answer != last.Content || current.Active != "" {
			flags = append(flags, "persistence_contract")
		}
		if s.selected != (last.SkillName != "") {
			flags = append(flags, "unexpected_skill_route")
		}
		if !s.evidence && (len(last.Citations) != 0 || budget.state["embeddings"].(float64) != beforeE || regexp.MustCompile(`\[\d+\]`).MatchString(rawAnswer.String())) {
			flags = append(flags, "empty_evidence_or_raw_citation_violation")
		}
		if s.evidence && len(last.Citations) == 0 {
			flags = append(flags, "missing_citation")
		}
		if s.selected && (regexp.MustCompile(`(?i)x\s*[=＝]\s*4`).MatchString(rawAnswer.String()) || (strings.Contains(rawAnswer.String(), "2x=8") && strings.Contains(rawAnswer.String(), "除以2"))) {
			flags = append(flags, "possible_guidance_answer_leak")
		}
		if len(modelMessages) < 2 || len(modelMessages) > 7 {
			flags = append(flags, "context_window_invalid")
		}
		if s.name == "history_round4" && len(modelMessages) != 7 {
			flags = append(flags, "six_message_window_not_reached")
		}
		record := map[string]any{"name": s.name, "question": s.question, "conversation_id": cid, "configuration": map[string]any{"material_ids": materials, "skill_ids": skills, "skill_enabled": s.enabled, "prompt": ""}, "expected_skill_selected": s.selected, "actual_skill_name": last.SkillName, "http_status": res.StatusCode, "answer": answer, "raw_answer": rawAnswer.String(), "events": events, "message": last, "model_messages": modelMessages, "new_dialogues": budget.state["dialogues"].(float64) - beforeD, "new_embeddings": budget.state["embeddings"].(float64) - beforeE, "prefix_persisted_before_delta": durable, "delta_equals_database": answer == last.Content, "flags": flags, "subjective_user_review": "pending"}
		control.Lock()
		record["wire_timings_ns"] = map[string]int64{"meta_flush": control.MetaFlushed, "model_open": control.OpenInvoked, "first_delta": control.FirstDelta, "done": control.Done}
		control.Unlock()
		report["rounds"] = append(report["rounds"].([]any), record)
		save()
		t.Logf("round=%d scenario=%s state=%s flags=%v cumulative=%.0f/%.0f", index+1, s.name, last.State, flags, budget.state["dialogues"], budget.state["embeddings"])
		if len(flags) > 0 {
			report["stopped_on"] = s.name
			save()
			t.Fatal("scenario anomaly; saved full evidence; no retry")
		}
		if index < len(cases)-1 {
			deadline := time.Now().Add(10 * time.Minute)
			for {
				if _, e := os.Stat(filepath.Join(out, fmt.Sprintf("continue-%02d", index+1))); e == nil {
					break
				}
				if time.Now().After(deadline) {
					t.Fatal("review gate timeout; no later calls")
				}
				time.Sleep(250 * time.Millisecond)
			}
		}
	}
	report["technical_matrix_finished"] = true
	save()
}
