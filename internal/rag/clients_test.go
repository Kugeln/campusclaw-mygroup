package rag

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestEmbeddingProtocolsAndValidation(t *testing.T) {
	invalid := false
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if len(body) != 2 || body["model"] != "test" || body["dimensions"] != nil {
			t.Error("embedding must send only model and input")
		}
		if r.Header.Get("Authorization") != "Bearer test-only" {
			t.Error("missing server credential")
		}
		vector := []float64{1, 2, 3}
		if invalid {
			vector = []float64{0, 0, 0}
		}
		if r.URL.Path == "/embeddings" {
			if body["input"] != "正文" {
				t.Error("openai input")
			}
			json.NewEncoder(w).Encode(map[string]any{"data": []any{map[string]any{"embedding": vector}}})
		} else if r.URL.Path == "/embeddings/multimodal" {
			if _, ok := body["input"].([]any); !ok {
				t.Error("ark input")
			}
			json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"embedding": vector}})
		} else {
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	for _, format := range []string{"openai", "ark_multimodal"} {
		e := Embedding{Gateway: Gateway{Base: server.URL, Key: "test-only", Model: "test"}, Dimension: 3, Format: format}
		if _, err := e.Embed(context.Background(), "正文"); err != nil {
			t.Fatal(err)
		}
		invalid = true
		if _, err := e.Embed(context.Background(), "正文"); err != Unavailable {
			t.Fatal("zero vector accepted")
		}
		invalid = false
		e.Dimension = 4
		if _, err := e.Embed(context.Background(), "正文"); err != Unavailable {
			t.Fatal("wrong dimension accepted")
		}
	}
	if calls != 6 {
		t.Fatal(calls)
	}
}

type staticEmbedding struct{}

func (staticEmbedding) Embed(context.Context, string) ([]float64, error) { return []float64{1, 0}, nil }

type staticVectors struct{ hits []Hit }

func (v staticVectors) Search(context.Context, int64, []float64, []string, int) ([]Hit, error) {
	return v.hits, nil
}
func TestThresholdAndPayloadDoubleCheck(t *testing.T) {
	items := []Evidence{{ChunkID: "a", MaterialID: "m", EntryID: 1, Index: 1, Text: "正文", Score: 1}, {ChunkID: "b", MaterialID: "m", EntryID: 1, Index: 2, Text: "正文", Score: 1}, {ChunkID: "c", MaterialID: "m", EntryID: 1, Index: 3, Text: "正文", Score: 1}}
	hits := []Hit{}
	for i, item := range items {
		scores := []float64{.34, .35, .8}
		hits = append(hits, Hit{ID: item.ChunkID, Score: scores[i], Payload: map[string]any{"class_id": float64(1), "material_id": "m", "knowledge_entry_id": float64(1), "chunk_id": item.ChunkID, "chunk_index": float64(item.Index)}})
	}
	hits = append(hits, Hit{ID: "foreign", Score: .99, Payload: map[string]any{"class_id": float64(1)}})
	r := Retrieval{Store: memoryStore{items}, Embedding: staticEmbedding{}, Vectors: staticVectors{hits}}
	result, e := r.Search(context.Background(), Scope{ClassID: 1}, "问题", "vector", nil, 4)
	if e != nil || len(result) != 2 || result[0].ChunkID != "c" || result[1].ChunkID != "b" {
		t.Fatal(result, e)
	}
	hits[2].Payload["class_id"] = float64(2)
	r.Vectors = staticVectors{hits}
	result, e = r.Search(context.Background(), Scope{ClassID: 1}, "问题", "vector", nil, 4)
	if e != nil || len(result) != 1 || result[0].ChunkID != "b" {
		t.Fatal("payload mismatch", result, e)
	}
}
