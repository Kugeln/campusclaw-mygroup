package config

import (
	"strings"
	"testing"
)

func TestRAGConfiguration(t *testing.T) {
	values := map[string]string{"RAG_ENABLED": "true", "QDRANT_URL": "http://vectors:6333", "EMBEDDING_BASE_URL": "https://embedding.example/v1", "EMBEDDING_API_KEY": "secret-test-key", "EMBEDDING_MODEL": "test", "EMBEDDING_DIMENSION": "2048", "DIALOGUE_BASE_URL": "https://dialogue.example/v1", "DIALOGUE_API_KEY": "another-test-key", "DIALOGUE_MODEL": "test"}
	get := func(k string) string { return values[k] }
	c, e := LoadRAG(get)
	if e != nil || !c.Enabled || c.Embedding.Format != "ark_multimodal" || c.Vectors.Collection != "campusclaw_chunks" {
		t.Fatal(c.Enabled, e)
	}
	delete(values, "EMBEDDING_MODEL")
	if _, e = LoadRAG(get); e == nil || !strings.Contains(e.Error(), "EMBEDDING_MODEL") || strings.Contains(e.Error(), "secret-test-key") {
		t.Fatal("missing config / secret disclosure")
	}
	values["RAG_ENABLED"] = "false"
	if c, e = LoadRAG(get); e != nil || c.Enabled {
		t.Fatal("opt-in compatibility")
	}
}
