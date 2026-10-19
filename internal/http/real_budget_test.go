//go:build tutorbrowser

package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// Only the isolated real-model fixture uses this no-retry budget recorder.
// It never saves request headers, bodies, response contents or credentials.
type realBudgetTransport struct {
	mu                                  sync.Mutex
	path, embeddingModel, dialogueModel string
	embeddings, dialogues               int
	blocked                             bool
	results                             []map[string]any
}

func (s *realBudgetTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	s.mu.Lock()
	if s.blocked || (r.URL.Hostname() != "ark.cn-beijing.volces.com" && r.URL.Hostname() != "ai-gateway.devops.hello1023.com") {
		s.mu.Unlock()
		return nil, errors.New("real model verification blocked")
	}
	dialogue := strings.HasSuffix(r.URL.Path, "/chat/completions")
	if dialogue {
		s.dialogues++
	} else {
		s.embeddings++
	}
	if s.dialogues > 4 || s.embeddings > 8 {
		s.blocked = true
		s.mu.Unlock()
		return nil, errors.New("real verification request budget exceeded")
	}
	s.mu.Unlock()
	started := time.Now()
	response, err := http.DefaultTransport.RoundTrip(r)
	status := 0
	if response != nil {
		status = response.StatusCode
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err != nil || status >= 400 {
		s.blocked = true
	}
	s.results = append(s.results, map[string]any{"path": r.URL.Path, "http_status": status, "network_error": err != nil, "headers_seconds": time.Since(started).Seconds()})
	data, _ := json.MarshalIndent(map[string]any{"embedding_model": s.embeddingModel, "dialogue_model": s.dialogueModel, "embedding_requests": s.embeddings, "dialogue_requests": s.dialogues, "blocked": s.blocked, "requests": s.results}, "", "  ")
	temp := s.path + ".tmp"
	if os.WriteFile(temp, data, 0600) == nil {
		_ = os.Rename(temp, s.path)
	}
	return response, err
}
