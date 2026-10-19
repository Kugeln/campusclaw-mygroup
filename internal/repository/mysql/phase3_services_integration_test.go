//go:build alignstorage

package mysql

import (
	"bufio"
	"campusclaw/internal/conversation"
	httpapi "campusclaw/internal/http"
	"campusclaw/internal/rag"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// Real isolated MySQL + a new collection in a separately created tmpfs Qdrant;
// only embedding/dialogue HTTP endpoints are local controlled test doubles.
func TestAlignPhase3IntegrationRealServices(t *testing.T) {
	base := os.Getenv("ALIGN_STREAM_QDRANT")
	if !regexp.MustCompile(`^http://campusclaw-align-stream-[a-f0-9]{8}-qdrant:6333$`).MatchString(base) {
		t.Fatal("new isolated align-stream Qdrant required (missing environment is not a pass)")
	}
	f := newPhase2HTTP(t)
	ctx := context.Background()
	const historyHashSQL = "SELECT SHA2(CAST(JSON_ARRAYAGG(JSON_OBJECT('id',id,'content',content,'citations',citations,'configuration',configuration_snapshot,'evidence',evidence_snapshot,'skill',skill_name,'hash',skill_hash)) AS CHAR),256) FROM conversation_messages WHERE conversation_id=40"
	var oldHistory string
	if err := f.db.QueryRow(historyHashSQL).Scan(&oldHistory); err != nil {
		t.Fatal(err)
	}
	collection := fmt.Sprintf("align_w5_%x", time.Now().UnixNano())
	vectors := rag.VectorClient{Gateway: rag.Gateway{Base: base}, Dimension: 2, Collection: collection}
	if err := vectors.Ensure(ctx); err != nil {
		t.Fatal("new Qdrant collection", err)
	}
	t.Logf("isolated Qdrant v1.13.4 target=%s new collection=%s", base, collection)
	mustSQL(t, f.db, "UPDATE knowledge_chunks SET chunk_text='数学方程等式性质' WHERE class_id=1")
	mustSQL(t, f.db, "UPDATE materials SET original_name='数学讲义.txt' WHERE class_id=1")
	third := "00000000-0000-4000-8000-000000000003"
	mustSQL(t, f.db, "INSERT INTO materials VALUES(?,1,2,'未绑定数学.txt',8,'text/plain',?,'2026-10-01')", third, third)
	mustSQL(t, f.db, "INSERT INTO knowledge_entries(id,material_id,class_id,plain_text,created_at,index_status) VALUES(3,?,1,'数学方程','2026-10-01','ready')", third)
	mustSQL(t, f.db, "INSERT INTO knowledge_chunks(id,knowledge_entry_id,material_id,class_id,chunk_index,chunk_text,index_status) VALUES(?,3,?,1,1,'数学方程','ready')", third, third)
	for _, item := range []struct {
		id           string
		class, entry int64
	}{{alignMaterialA, 1, 1}, {alignMaterialB, 2, 2}, {third, 1, 3}} {
		if e := vectors.Upsert(ctx, item.class, []rag.Evidence{{ChunkID: item.id, MaterialID: item.id, EntryID: item.entry, Index: 1}}, [][]float64{{1, 0}}); e != nil {
			t.Fatal(e)
		}
	}
	var embeddings, models atomic.Int32
	var modelMode atomic.Int32
	var embedFails atomic.Bool
	cancelled := make(chan struct{}, 1)
	var gated atomic.Bool
	release := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/embeddings" {
			embeddings.Add(1)
			if embedFails.Load() {
				w.WriteHeader(503)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"data":[{"embedding":[1,0]}]}`)
			return
		}
		if r.URL.Path != "/chat/completions" {
			w.WriteHeader(404)
			return
		}
		models.Add(1)
		if modelMode.Load() == 2 {
			w.WriteHeader(503)
			return
		}
		var body struct {
			Messages []conversation.ModelMessage `json:"messages"`
		}
		if json.NewDecoder(r.Body).Decode(&body) != nil {
			w.WriteHeader(400)
			return
		}
		if len(body.Messages) > 7 || len(body.Messages) < 2 {
			t.Error("model window size", len(body.Messages))
		}
		// Gate the first generated token while the caller reads the flushed meta.
		if gated.Load() {
			select {
			case <-release:
			case <-r.Context().Done():
				return
			}
		}
		w.Header().Set("Content-Type", "text/event-stream")
		if modelMode.Load() == 1 {
			io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\"可靠前缀\"}}]}\n\n")
			w.(http.Flusher).Flush()
			<-r.Context().Done()
			cancelled <- struct{}{}
			return
		}
		for _, part := range []string{"模型回答", "[", "1", "]"} {
			packet, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"delta": map[string]string{"content": part}}}})
			fmt.Fprintf(w, "data: %s\n\n", packet)
			w.(http.Flusher).Flush()
		}
		io.WriteString(w, "data: [DONE]\n\n")
	}))
	defer upstream.Close()
	embedding := rag.Embedding{Gateway: rag.Gateway{Base: upstream.URL, Model: "controlled-embedding"}, Dimension: 2, Format: "openai"}
	retrieval := rag.Retrieval{Store: RAG{DB: f.db}, Embedding: embedding, Vectors: vectors}
	model := conversation.StreamingModel{Gateway: rag.Gateway{Base: upstream.URL, Model: "controlled-stream", Key: "test-only"}}
	store := Conversations{DB: f.db}
	for _, hasEvidence := range []bool{true, false} {
		for _, hasSkill := range []bool{true, false} {
			t.Run(fmt.Sprintf("evidence=%t_skill=%t", hasEvidence, hasSkill), func(t *testing.T) {
				ids := []string{}
				if hasEvidence {
					ids = []string{alignMaterialA}
				}
				skills := []int64{}
				if hasSkill {
					skills = []int64{20}
				}
				phase3Bind(t, f, ids, skills)
				c := phase3Conversation(t, f)
				f.handler = phase3Handler(f, store, retrieval, model, 0)
				server := httptest.NewServer(f.handler)
				defer server.Close()
				beforeModels, beforeEmbeddings := models.Load(), embeddings.Load()
				first := hasEvidence && hasSkill
				gated.Store(first)
				req, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/conversations/%d/messages", server.URL, c.ID), strings.NewReader(`{"content":"方程问题"}`))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Authorization", "Bearer "+f.tokens["sa"])
				client := &http.Client{Timeout: 5 * time.Second}
				res, e := client.Do(req)
				if e != nil {
					if first {
						close(release)
					}
					t.Fatal("meta blocked by upstream", e)
				}
				defer res.Body.Close()
				reader := bufio.NewReader(res.Body)
				var start strings.Builder
				for {
					line, err := reader.ReadString('\n')
					if err != nil {
						t.Fatal(err)
					}
					start.WriteString(line)
					if line == "\n" {
						break
					}
				}
				events := phase3Events(t, start.String())
				if res.StatusCode != 200 || phase3Names(events) != "meta" || string(events[0].data["status"]) != `"ok"` {
					t.Fatal(start.String())
				}
				var candidates []rag.Source
				json.Unmarshal(events[0].data["citations"], &candidates)
				if hasEvidence && (len(candidates) != 1 || candidates[0].MaterialID != alignMaterialA) || !hasEvidence && string(events[0].data["citations"]) != "[]" {
					t.Fatal("binding/class expanded", candidates)
				}
				if first {
					close(release)
				}
				rest, e := io.ReadAll(reader)
				if e != nil {
					t.Fatal(e)
				}
				events = phase3Events(t, start.String()+string(rest))
				if events[len(events)-1].name != "done" || models.Load()-beforeModels != 1 {
					t.Fatal("generation", string(rest), models.Load()-beforeModels)
				}
				if !hasEvidence && embeddings.Load() != beforeEmbeddings {
					t.Fatal("empty binding embedded")
				}
				last := phase3Last(t, f, c)
				if last.Content != phase3Delta(events) || last.State != "completed" || (last.SkillName != "") != hasSkill {
					t.Fatal("event/DB/skill inconsistency", last)
				}
				if !hasEvidence && (strings.Contains(last.Content, "[1]") || len(last.Citations) != 0) {
					t.Fatal("fabricated citation")
				}
				t.Logf("real-services evidence=%t skill=%t model_calls=%d embedding_calls=%d SSE=%s", hasEvidence, hasSkill, models.Load()-beforeModels, embeddings.Load()-beforeEmbeddings, phase3Names(events))
			})
		}
	}
	// Actual HTTP model connection failure occurs after meta, never as HTTP503.
	phase3Bind(t, f, []string{}, []int64{20})
	modelMode.Store(2)
	c := phase3Conversation(t, f)
	f.handler = phase3Handler(f, store, retrieval, model, 0)
	beforeCalls := models.Load()
	w := f.request("POST", fmt.Sprintf("/api/conversations/%d/messages", c.ID), "sa", map[string]any{"content": "方程问题"})
	events := phase3Events(t, w.Body.String())
	if w.Code != 200 || phase3Names(events) != "meta,error" || models.Load()-beforeCalls != 1 || phase3Last(t, f, c).SkillName != "" {
		t.Fatal("real model connection boundary", w.Code, w.Body.String())
	}
	// A real TCP client disconnect cancels the upstream and keeps a durable prefix.
	modelMode.Store(1)
	c = phase3Conversation(t, f)
	server := httptest.NewServer(f.handler)
	clientCtx, stop := context.WithCancel(ctx)
	req, _ := http.NewRequestWithContext(clientCtx, "POST", fmt.Sprintf("%s/api/conversations/%d/messages", server.URL, c.ID), strings.NewReader(`{"content":"方程问题"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+f.tokens["sa"])
	res, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
	if err != nil {
		stop()
		server.Close()
		t.Fatal(err)
	}
	scanner := bufio.NewScanner(res.Body)
	receivedDelta := false
	for scanner.Scan() {
		if scanner.Text() == "event: delta" {
			receivedDelta = true
			break
		}
	}
	stop()
	res.Body.Close()
	if !receivedDelta {
		server.Close()
		t.Fatal("missing streamed prefix")
	}
	select {
	case <-cancelled:
	case <-time.After(3 * time.Second):
		server.Close()
		t.Fatal("upstream not cancelled")
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		actual, m, e := store.Get(ctx, alignStudent, c.ID)
		if e == nil && actual.Active == "" && m[len(m)-1].State == "interrupted" {
			if m[len(m)-1].Content != "可靠前缀" {
				t.Fatal("disconnect lost prefix")
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("disconnect did not release turn")
		}
		time.Sleep(5 * time.Millisecond)
	}
	server.Close()
	modelMode.Store(0)
	// Actual embedding HTTP503 is a preparation failure, unlike normal zero hits.
	phase3Bind(t, f, []string{alignMaterialA}, []int64{20})
	c = phase3Conversation(t, f)
	embedFails.Store(true)
	beforeCalls = models.Load()
	w = f.request("POST", fmt.Sprintf("/api/conversations/%d/messages", c.ID), "sa", map[string]any{"content": "方程问题"})
	embedFails.Store(false)
	if w.Code != 503 || len(phase3Events(t, w.Body.String())) != 0 || models.Load() != beforeCalls || phase3Last(t, f, c).State != "failed" {
		t.Fatal("real retrieval outage boundary", w.Code, w.Body.String())
	}
	// Week 4 retains all-class nil semantics even after Week 5 unbinds everything.
	gated.Store(false)
	dialogue := &phase2Dialogue{}
	f.handler = httpapi.WithTutor(phase2Health{}, nil, f.identity, nil, nil, &httpapi.RAGHandler{Retrieval: retrieval, Dialogue: dialogue}, &httpapi.TutorHandler{Store: f.repo})
	for _, mode := range []string{"keyword", "vector", "hybrid"} {
		before := embeddings.Load()
		w := f.request("GET", "/api/search?q="+url.QueryEscape("方程")+"&mode="+mode, "sa", nil)
		var body struct {
			Items []rag.Evidence `json:"items"`
		}
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &body) != nil || len(body.Items) != 2 {
			t.Fatal("Week4 real three-mode", mode, w.Code, w.Body.String())
		}
		for _, item := range body.Items {
			if item.MaterialID == alignMaterialB || item.Index != 1 {
				t.Fatal("class/1-based leak")
			}
		}
		if mode == "keyword" && embeddings.Load() != before {
			t.Fatal("Week4 keyword embedding")
		}
	}
	before := dialogue.calls
	w = f.request("POST", "/api/ask", "sa", map[string]any{"question": "完全没有命中的词", "mode": "keyword"})
	var answer rag.Answer
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &answer) != nil || answer.Text != rag.NoEvidence || len(answer.Sources) != 0 || dialogue.calls != before {
		t.Fatal("Week4 no-evidence changed", w.Body.String())
	}
	// Real hydration must reject a now-pending row although its vector still exists.
	mustSQL(t, f.db, "UPDATE knowledge_chunks SET index_status='pending' WHERE id=?", alignMaterialA)
	items, e := retrieval.Search(ctx, alignStudent, "方程", "vector", []string{alignMaterialA}, 4)
	if e != nil || len(items) != 0 {
		t.Fatal("not-ready hydration accepted", e, items)
	}
	t.Log("Week4 real keyword/vector/hybrid PASS; Ask zero hit dialogue_calls=0; pending hydration rejects; no classroom collections accessed")
	var currentHistory string
	if err := f.db.QueryRow(historyHashSQL).Scan(&currentHistory); err != nil || oldHistory != currentHistory {
		t.Fatal("legacy message/citation/configuration audit rewritten", err)
	}
	t.Logf("legacy message/citation/configuration/evidence/skill audit SHA256 unchanged=%s", oldHistory)
}
