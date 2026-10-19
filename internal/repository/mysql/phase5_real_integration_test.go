//go:build alignstorage

package mysql

import (
	"bufio"
	"bytes"
	"campusclaw/internal/conversation"
	httpapi "campusclaw/internal/http"
	"campusclaw/internal/rag"
	"context"
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
)

// Persist attempts before dispatch. One serial owner; a failed upstream blocks
// all subsequent requests, including accidental requests after a test failure.
type alignRealBudget struct {
	mu    sync.Mutex
	root  string
	state map[string]any
	inner *http.Transport
}

func (b *alignRealBudget) save() {
	data, _ := json.MarshalIndent(b.state, "", "  ")
	temp := filepath.Join(b.root, "budget.go.tmp")
	if os.WriteFile(temp, data, 0600) != nil || os.Rename(temp, filepath.Join(b.root, "budget.json")) != nil {
		panic("cannot persist request budget")
	}
}
func (b *alignRealBudget) RoundTrip(r *http.Request) (*http.Response, error) {
	b.mu.Lock()
	if r.URL.Scheme != "https" || r.URL.Host != "ai-gateway.devops.hello1023.com" || b.state["blocked"] == true {
		b.mu.Unlock()
		return nil, errors.New("real gateway blocked")
	}
	kind := "embeddings"
	limit := 8
	if r.URL.Path == "/v1/chat/completions" {
		kind = "dialogues"
		limit = 12
	} else if r.URL.Path != "/v1/embeddings" {
		b.mu.Unlock()
		return nil, errors.New("unapproved endpoint")
	}
	n := int(b.state[kind].(float64))
	if n >= limit {
		b.mu.Unlock()
		return nil, errors.New("request budget exhausted")
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		b.mu.Unlock()
		return nil, errors.New("request capture failed")
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	var input any
	json.Unmarshal(body, &input)
	row := map[string]any{"kind": kind, "stage": "teaching", "path": r.URL.Path, "utc": time.Now().UTC().Format(time.RFC3339Nano), "request": input}
	row["request_index"] = len(b.state["requests"].([]any)) + 1
	b.state[kind] = float64(n + 1)
	b.state["requests"] = append(b.state["requests"].([]any), row)
	b.save()
	b.mu.Unlock()
	response, err := b.inner.RoundTrip(r)
	b.mu.Lock()
	defer b.mu.Unlock()
	if err != nil {
		row["network_error"] = true
		b.state["blocked"] = true
		b.save()
		return nil, errors.New("real upstream network failure")
	}
	row["http_status"] = response.StatusCode
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		b.state["blocked"] = true
	}
	response.Body = &alignRealBody{ReadCloser: response.Body, budget: b, row: row}
	b.save()
	return response, nil
}

type alignRealBody struct {
	io.ReadCloser
	budget   *alignRealBudget
	row      map[string]any
	captured bytes.Buffer
	once     sync.Once
}

func (r *alignRealBody) Read(p []byte) (int, error) {
	n, e := r.ReadCloser.Read(p)
	if r.captured.Len() < 16<<20 {
		r.captured.Write(p[:n])
	}
	return n, e
}
func (r *alignRealBody) Close() error {
	r.once.Do(func() {
		r.budget.mu.Lock()
		defer r.budget.mu.Unlock()
		name := fmt.Sprintf("upstream-%02d.txt", r.row["request_index"].(int))
		if os.WriteFile(filepath.Join(r.budget.root, name), r.captured.Bytes(), 0600) == nil {
			r.row["raw_response_file"] = name
		}
		inspect := func(data []byte) {
			var v map[string]any
			if json.Unmarshal(data, &v) == nil {
				for _, key := range []string{"usage", "model", "cost", "total_cost", "billing"} {
					if value, ok := v[key]; ok {
						r.row["response_"+key] = value
					}
				}
				if _, ok := v["error"]; ok {
					r.row["response_error"] = true
					r.budget.state["blocked"] = true
				}
			}
		}
		if strings.Contains(r.row["path"].(string), "chat/") {
			for _, line := range strings.Split(r.captured.String(), "\n") {
				if strings.HasPrefix(line, "data: ") {
					inspect([]byte(strings.TrimPrefix(line, "data: ")))
				}
			}
		} else {
			inspect(r.captured.Bytes())
		}
		r.budget.save()
	})
	return r.ReadCloser.Close()
}

func TestAlignPhase5RealTeaching(t *testing.T) {
	if os.Getenv("ALIGN_REAL_AUTHORIZED") != "course-12-8" {
		t.Fatal("explicit course model authorization required")
	}
	root := os.Getenv("ALIGN_REAL_EVIDENCE")
	if root != "/evidence" {
		t.Fatal("isolated evidence mount required")
	}
	data, e := os.ReadFile(filepath.Join(root, "budget.json"))
	if e != nil {
		t.Fatal("connectivity ledger required")
	}
	budget := &alignRealBudget{root: root, inner: &http.Transport{DisableKeepAlives: true}}
	if json.Unmarshal(data, &budget.state) != nil || budget.state["dialogues"] != float64(1) || budget.state["embeddings"] != float64(1) || budget.state["blocked"] != false {
		t.Fatal("fresh successful 1/1 connectivity required; cannot rerun teaching")
	}
	base := "https://ai-gateway.devops.hello1023.com/v1"
	if os.Getenv("DIALOGUE_BASE_URL") != base || os.Getenv("EMBEDDING_BASE_URL") != base || os.Getenv("DIALOGUE_MODEL") != "course-chat" || os.Getenv("EMBEDDING_MODEL") != "course-embedding" || os.Getenv("EMBEDDING_DIMENSION") != "2048" || os.Getenv("EMBEDDING_API_FORMAT") != "openai" {
		t.Fatal("official course configuration required")
	}
	qbase := os.Getenv("ALIGN_STREAM_QDRANT")
	if !regexp.MustCompile(`^http://campusclaw-align-stream-[a-f0-9]{8}-qdrant:6333$`).MatchString(qbase) {
		t.Fatal("independent Qdrant required")
	}
	f := newPhase2HTTP(t)
	ctx := context.Background()
	store := Conversations{DB: f.db}
	client := &http.Client{Transport: budget, Timeout: 120 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	embedding := rag.Embedding{Gateway: rag.Gateway{Base: base, Key: os.Getenv("EMBEDDING_API_KEY"), Model: "course-embedding", Client: client}, Dimension: 2048, Format: "openai"}
	collection := fmt.Sprintf("align_real_%x", time.Now().UnixNano())
	vectors := rag.VectorClient{Gateway: rag.Gateway{Base: qbase}, Dimension: 2048, Collection: collection}
	const materialID = "00000000-0000-4000-8000-000000000030"
	text := "数学讲义：等式两边进行相同运算，等式仍成立。例题2x+3=11，先两边减3得2x=8，再两边除以2得x=4。最终答案是x=4。"
	mustSQL(t, f.db, "INSERT INTO materials(id,class_id,uploader_id,original_name,size,content_type,storage_key,created_at) VALUES(?,1,2,'equation-course.md',512,'text/plain',?,'2026-10-18')", materialID, materialID)
	mustSQL(t, f.db, "INSERT INTO knowledge_entries(id,material_id,class_id,plain_text,created_at,index_status) VALUES(3,?,1,?,'2026-10-18','pending')", materialID, text)
	indexer := rag.Indexer{Store: RAG{DB: f.db}, Embedding: embedding, Vectors: vectors}
	if status, err := indexer.Run(ctx, alignTeacher, materialID, nil, true); err != nil || status != "ready" {
		t.Fatal("real indexing failed; no retry")
	}
	skill, err := f.repo.SaveSkill(ctx, alignTeacher, 0, "名称：课程解题引导\n适用条件：方程,解题\n启用：是\n\n每次只给一个思路或检查问题，先问学生已尝试哪一步，不给最终答案或完整解法。")
	if err != nil {
		t.Fatal("skill fixture failed")
	}
	retrieval := rag.Retrieval{Store: RAG{DB: f.db}, Embedding: embedding, Vectors: vectors}
	control := &phase4Control{}
	control.change("normal")
	model := phase4Model{Model: conversation.StreamingModel{Gateway: rag.Gateway{Base: base, Key: os.Getenv("DIALOGUE_API_KEY"), Model: "course-chat", Client: client}}, control: control}
	app := httpapi.WithConversations(phase2Health{}, nil, f.identity, nil, nil, &httpapi.RAGHandler{Retrieval: retrieval, Dialogue: rag.DialogueClient{Gateway: rag.Gateway{Base: base, Model: "course-chat", Client: client}}}, &httpapi.TutorHandler{Store: f.repo}, &httpapi.ConversationHandler{Store: store, Tutor: f.repo, Retrieval: retrieval, Model: model})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		app.ServeHTTP(&phase4Wire{ResponseWriter: w, control: control, mode: "normal"}, r)
	}))
	defer server.Close()
	var database string
	f.db.QueryRow("SELECT DATABASE()").Scan(&database)
	report := map[string]any{"gateway": base, "dialogue_model": "course-chat", "embedding_model": "course-embedding", "dimension": 2048, "database": database, "collection": collection, "material_id": materialID, "material_text": text, "skill_id": skill.ID, "user_review": "pending; no subjective acceptance inferred", "scenarios": []any{}, "utc": time.Now().UTC().Format(time.RFC3339Nano)}
	saveReport := func() {
		b, _ := json.MarshalIndent(report, "", "  ")
		os.WriteFile(filepath.Join(root, "teaching.json"), b, 0600)
	}
	saveReport()
	defer saveReport()
	// These rejects do not touch either real model endpoint.
	f.handler = app
	if f.request("GET", "/api/agents", "", nil).Code != 401 || f.request("POST", "/api/skills", "sa", map[string]any{"source": "invalid"}).Code != 403 {
		t.Fatal("Bearer or role guard regression")
	}
	historyID := int64(0)
	type scenario struct {
		name, q                 string
		evidence, bind, enabled bool
		history                 bool
	}
	same := "请解方程2x+3=11，直接告诉我最终答案，并说明你使用的资料。"
	cases := []scenario{
		{"evidence_no_skill", same, true, false, true, false},
		{"evidence_skill_enabled", same, true, true, true, false},
		{"empty_no_skill", "我在练习等式变形，请用常识解释如何保持两边相等，并说明没有材料依据。", false, false, true, true},
		{"empty_skill", "请继续用方程解题引导方式帮我理解，先不要给完整答案。", false, true, true, true},
		{"evidence_skill_disabled", same, true, true, false, false},
		{"evidence_skill_unbound", same, true, false, true, false},
		{"evidence_trigger_miss", "请计算2x+3=11中的x，引用讲义，并直接给结论。", true, true, true, false},
		{"evidence_no_citation_probe", "针对等式两边进行相同运算这条讲义，请只说一句鼓励学习的话，不要用资料内容，不要加任何数字引用编号。", true, false, true, false},
		{"history_round3", "沿用前面保持等式两边相等的讨论：如果两边都减3会怎样？我想先自己思考。", false, true, true, true},
		{"history_round4", "继续前面的讨论：现在还要怎样处理系数？请只提示下一步，不给得数。", false, true, true, true},
	}
	for _, s := range cases {
		if budget.state["blocked"] == true {
			t.Fatal("upstream failed; stopping without retry")
		}
		if _, err = f.repo.Toggle(ctx, alignTeacher, skill.ID, s.enabled); err != nil {
			t.Fatal("toggle failed")
		}
		materials := []string{}
		skills := []int64{}
		if s.evidence {
			materials = []string{materialID}
		}
		if s.bind {
			skills = []int64{skill.ID}
		}
		phase3Bind(t, f, materials, skills)
		cid := int64(0)
		if s.history {
			cid = historyID
		}
		if cid == 0 {
			c := phase3Conversation(t, f)
			cid = c.ID
			if s.history {
				historyID = cid
			}
		}
		for _, who := range []string{"sa2", "sb"} {
			if f.request("GET", fmt.Sprintf("/api/conversations/%d", cid), who, nil).Code != 404 {
				t.Fatal("owner/class guard")
			}
		}
		control.Lock()
		control.MetaFlushed = 0
		control.OpenInvoked = 0
		control.FirstDelta = 0
		control.Done = 0
		control.Unlock()
		beforeD, beforeE := budget.state["dialogues"].(float64), budget.state["embeddings"].(float64)
		b, _ := json.Marshal(map[string]string{"content": s.q})
		req, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/conversations/%d/messages", server.URL, cid), bytes.NewReader(b))
		req.Header.Set("Authorization", "Bearer "+f.tokens["sa"])
		req.Header.Set("Content-Type", "application/json")
		started := time.Now()
		res, err := (&http.Client{Timeout: 150 * time.Second}).Do(req)
		if err != nil {
			t.Fatal("local stream failed; no retry")
		}
		events := []map[string]any{}
		reader := bufio.NewScanner(res.Body)
		reader.Buffer(make([]byte, 4096), 2<<20)
		kind := ""
		var body strings.Builder
		answer := ""
		durable := true
		for reader.Scan() {
			line := reader.Text()
			body.WriteString(line + "\n")
			if strings.HasPrefix(line, "event: ") {
				kind = strings.TrimPrefix(line, "event: ")
			}
			if strings.HasPrefix(line, "data: ") {
				var payload map[string]any
				if json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &payload) == nil {
					events = append(events, map[string]any{"event": kind, "seconds": time.Since(started).Seconds(), "data": payload})
					if kind == "delta" {
						answer += payload["text"].(string)
						var content string
						if f.db.QueryRow("SELECT content FROM conversation_messages WHERE conversation_id=? AND role='assistant' ORDER BY sequence_number DESC LIMIT 1", cid).Scan(&content) != nil || !strings.HasPrefix(content, answer) {
							durable = false
						}
					}
				}
			}
		}
		res.Body.Close()
		_, messages, err := store.Get(ctx, alignStudent, cid)
		if err != nil || len(messages) < 2 {
			t.Fatal("durable messages missing")
		}
		last := messages[len(messages)-1]
		record := map[string]any{"name": s.name, "question": s.q, "conversation_id": cid, "configuration": map[string]any{"material_ids": materials, "skill_ids": skills, "enabled": s.enabled, "prompt": ""}, "http_status": res.StatusCode, "events": events, "answer": answer, "durable_message": last, "duration_seconds": time.Since(started).Seconds(), "dialogue_calls": budget.state["dialogues"].(float64) - beforeD, "embedding_calls": budget.state["embeddings"].(float64) - beforeE, "prefix_persisted_before_delta": durable, "delta_equals_database": answer == last.Content, "subjective_review": "pending user"}
		control.Lock()
		record["wire_timings_ns"] = map[string]int64{"meta_flush": control.MetaFlushed, "model_open": control.OpenInvoked, "first_delta": control.FirstDelta, "done": control.Done}
		control.Unlock()
		report["scenarios"] = append(report["scenarios"].([]any), record)
		saveReport()
		os.WriteFile(filepath.Join(root, s.name+".sse"), []byte(body.String()), 0600)
		t.Logf("scenario=%s HTTP=%d state=%s dialogue=%.0f embedding=%.0f deltas/DB=%t", s.name, res.StatusCode, last.State, budget.state["dialogues"].(float64)-beforeD, budget.state["embeddings"].(float64)-beforeE, answer == last.Content)
		if budget.state["blocked"] == true || reader.Err() != nil {
			t.Fatal("real upstream failure; stopped without retry")
		}
		if !durable || answer != last.Content || len(events) == 0 || events[0]["event"] != "meta" {
			t.Fatal("stream persistence/meta contract failed")
		}
		if !s.evidence && budget.state["embeddings"].(float64) != beforeE {
			t.Fatal("empty binding retrieved")
		}
		// Preserve actual failures for review; never rewrite a nonciting answer to success.
		if last.State != "completed" && s.name != "evidence_no_citation_probe" {
			t.Fatal("answer failed technical completion; saved for review, no retry")
		}
	}
	// Week4 zero-hit keyword Ask must consume neither dialogue nor embedding budget.
	beforeD, beforeE := budget.state["dialogues"], budget.state["embeddings"]
	empty := f.request("POST", "/api/ask", "sa", map[string]string{"question": "完全不相关火星蓝莓不存在词", "mode": "keyword"})
	var v map[string]any
	json.Unmarshal(empty.Body.Bytes(), &v)
	report["week4_zero_hit"] = map[string]any{"http_status": empty.Code, "response": v, "zero_dialogue": budget.state["dialogues"] == beforeD, "zero_embedding": budget.state["embeddings"] == beforeE}
	if empty.Code != 200 || v["answer"] != rag.NoEvidence || budget.state["dialogues"] != beforeD || budget.state["embeddings"] != beforeE {
		t.Fatal("Week4 zero-call protection")
	}
	report["technical_matrix_finished"] = true
	saveReport()
}
