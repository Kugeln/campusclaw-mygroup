//go:build realmodel

package rag_test

import (
	"campusclaw/internal/config"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type realProbeTransport struct {
	status int
	failed bool
}

func (p *realProbeTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	response, err := http.DefaultTransport.RoundTrip(r)
	if response != nil {
		p.status = response.StatusCode
	}
	p.failed = err != nil
	return response, err
}

// Exactly one real Embedding request. No retry, dialogue, database or Docker.
func TestRealEmbeddingPreflight(t *testing.T) {
	if os.Getenv("REAL_MODEL_PREFLIGHT") != "1" {
		t.Fatal("explicit real-model preflight required")
	}
	c, err := config.LoadRAG(os.Getenv)
	if err != nil {
		t.Fatal(err)
	}
	root := os.Getenv("REAL_MODEL_EVIDENCE")
	if root == "" {
		t.Fatal("local ignored evidence directory required")
	}
	transport := &realProbeTransport{}
	c.Embedding.Client = &http.Client{Transport: transport, Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	started := time.Now()
	vector, err := c.Embedding.Embed(context.Background(), "验收资料：解一元一次方程时，先分析已知条件，再使用等式性质。")
	result := map[string]any{"embedding_model": c.Embedding.Model, "dimension": c.Embedding.Dimension, "actual_dimension": len(vector), "http_status": transport.status, "network_error": transport.failed, "elapsed_seconds": time.Since(started).Seconds(), "passed": err == nil, "embedding_requests": 1, "dialogue_requests": 0}
	data, _ := json.MarshalIndent(result, "", "  ")
	if writeErr := os.WriteFile(filepath.Join(root, "embedding-preflight.json"), data, 0600); writeErr != nil {
		t.Fatal("cannot save redacted preflight evidence")
	}
	if err != nil {
		t.Fatalf("real Embedding preflight blocked: HTTP %d, network_error=%t; no retry or dialogue calls", transport.status, transport.failed)
	}
	t.Logf("real Embedding compatible: HTTP %d, dimension=%d", transport.status, len(vector))
}
