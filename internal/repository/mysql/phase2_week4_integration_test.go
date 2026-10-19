//go:build alignstorage

package mysql

import (
	httpapi "campusclaw/internal/http"
	"campusclaw/internal/rag"
	"campusclaw/internal/tutor"
	"context"
	"encoding/json"
	"net/url"
	"testing"
)

type phase2Week4Store struct {
	RAG
	keywords     int
	nilMaterials bool
}

func (s *phase2Week4Store) Keyword(ctx context.Context, class int64, q string, materials []string, limit int) ([]rag.Evidence, error) {
	s.keywords++
	s.nilMaterials = materials == nil
	return s.RAG.Keyword(ctx, class, q, materials, limit)
}

type phase2Embedding struct {
	calls int
	err   error
}

func (e *phase2Embedding) Embed(context.Context, string) ([]float64, error) {
	e.calls++
	return []float64{1, 0}, e.err
}

type phase2Vectors struct {
	calls int
	hits  []rag.Hit
}

func (v *phase2Vectors) Search(context.Context, int64, []float64, []string, int) ([]rag.Hit, error) {
	v.calls++
	return v.hits, nil
}

type phase2Dialogue struct{ calls int }

func (d *phase2Dialogue) Answer(context.Context, string, []rag.Evidence) (string, error) {
	d.calls++
	return "依据[1]", nil
}

func TestAlignPhase2Week4RetrievalAndAskProtection(t *testing.T) {
	f := newPhase2HTTP(t)
	ctx := context.Background()
	mustSQL(t, f.db, "UPDATE knowledge_chunks SET chunk_text='甲班数学方程等式性质' WHERE class_id=1")
	mustSQL(t, f.db, "UPDATE knowledge_chunks SET chunk_text='乙班语文阅读作文' WHERE class_id=2")
	store := &phase2Week4Store{RAG: RAG{DB: f.db}}
	embedding := &phase2Embedding{}
	vectors := &phase2Vectors{}
	dialogue := &phase2Dialogue{}
	hit := func(id string, class, entry int64, score float64) rag.Hit {
		return rag.Hit{ID: id, Score: score, Payload: map[string]any{"class_id": float64(class), "knowledge_entry_id": float64(entry), "chunk_index": float64(1), "material_id": id, "chunk_id": id}}
	}
	vectors.hits = []rag.Hit{hit(alignMaterialA, 1, 1, .9), hit(alignMaterialB, 2, 2, .99)}
	retrieval := rag.Retrieval{Store: store, Embedding: embedding, Vectors: vectors}
	f.handler = httpapi.WithTutor(phase2Health{}, nil, f.identity, nil, nil, &httpapi.RAGHandler{Retrieval: retrieval, Dialogue: dialogue}, &httpapi.TutorHandler{Store: f.repo})
	search := func(who, mode string) []rag.Evidence {
		t.Helper()
		w := f.request("GET", "/api/search?q="+url.QueryEscape("方程")+"&mode="+mode, who, nil)
		var body struct {
			Items []rag.Evidence `json:"items"`
		}
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &body) != nil {
			t.Fatal("Week4 search", w.Code, w.Body.String())
		}
		return body.Items
	}
	for _, mode := range []string{"keyword", "vector", "hybrid"} {
		before := embedding.calls
		items := search("sa", mode)
		if len(items) != 1 || items[0].MaterialID != alignMaterialA || items[0].Index != 1 {
			t.Fatal("Week4 scope/UUID/index", mode, items)
		}
		if mode == "keyword" && embedding.calls != before {
			t.Fatal("keyword embedded")
		}
	}
	if !store.nilMaterials {
		t.Fatal("Week4 nil no longer means full class")
	}
	if items := search("sb", "keyword"); len(items) != 0 {
		t.Fatal("keyword cross-class")
	}
	vectors.hits = []rag.Hit{hit(alignMaterialA, 1, 1, .35)}
	if len(search("sa", "vector")) != 1 {
		t.Fatal("threshold .35")
	}
	vectors.hits[0].Score = .349
	if len(search("sa", "vector")) != 0 {
		t.Fatal("threshold below .35")
	}
	vectors.hits = []rag.Hit{hit(alignMaterialA, 2, 1, .9)}
	if len(search("sa", "vector")) != 0 {
		t.Fatal("payload class mismatch accepted")
	}
	vectors.hits = []rag.Hit{hit(alignMaterialA, 1, 1, .9)}
	mustSQL(t, f.db, "UPDATE knowledge_chunks SET index_status='pending' WHERE class_id=1")
	if len(search("sa", "vector")) != 0 {
		t.Fatal("unready hydrated")
	}
	mustSQL(t, f.db, "UPDATE knowledge_chunks SET index_status='ready' WHERE class_id=1")
	if len(search("sb", "vector")) != 0 {
		t.Fatal("foreign ready row hydrated")
	}
	embedding.err = rag.Unavailable
	if w := f.request("GET", "/api/search?q="+url.QueryEscape("方程")+"&mode=hybrid", "sa", nil); w.Code != 503 {
		t.Fatal("hybrid silently fell back", w.Code)
	}
	embedding.err = nil
	beforeMessages := scalar(t, f.db, "SELECT COUNT(*) FROM conversation_messages")
	beforeCalls := dialogue.calls
	w := f.request("POST", "/api/ask", "sa", map[string]any{"question": "zzzzunmatchedzzzz", "mode": "keyword"})
	var answer struct {
		Answer  string       `json:"answer"`
		Sources []rag.Source `json:"sources"`
	}
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &answer) != nil || answer.Answer != rag.NoEvidence || len(answer.Sources) != 0 || dialogue.calls != beforeCalls {
		t.Fatal("Ask no-evidence changed", w.Code, w.Body.String())
	}
	w = f.request("POST", "/api/ask", "sa", map[string]any{"question": "方程", "mode": "keyword"})
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &answer) != nil || len(answer.Sources) != 1 || dialogue.calls != beforeCalls+1 || scalar(t, f.db, "SELECT COUNT(*) FROM conversation_messages") != beforeMessages {
		t.Fatal("Ask shape or persistence changed")
	}
	// The same shared retrieval still serves Week4 nil as full-class, while
	// Tutor's empty binding must not call any shared dependency at all.
	if _, err := f.repo.SaveAgent(ctx, alignTeacher, 10, tutor.Config{Name: "A tutor", Prompt: "role", Enabled: true, SkillIDs: []int64{}, MaterialIDs: []string{}}); err != nil {
		t.Fatal(err)
	}
	if len(search("sa", "keyword")) != 1 {
		t.Fatal("Week4 affected by Tutor empty bindings")
	}
	keywordCalls, embedCalls, vectorCalls := store.keywords, embedding.calls, vectors.calls
	p, err := tutor.PrepareTurn(ctx, f.repo, retrieval, alignStudent, 10, "方程", "hybrid")
	if err != nil || len(p.Evidence) != 0 || store.keywords != keywordCalls || embedding.calls != embedCalls || vectors.calls != vectorCalls {
		t.Fatal("Tutor empty set called retrieval", err)
	}
	t.Log("Week4 three modes/threshold/ready/class/Ask zero-call protected with real MySQL + controlled vectors/embedding/dialogue; no Qdrant accessed")
}
