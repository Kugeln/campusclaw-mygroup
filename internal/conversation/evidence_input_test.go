package conversation

import (
	"campusclaw/internal/rag"
	"campusclaw/internal/tutor"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestModelEvidenceUnicodeLimits(t *testing.T) {
	for _, size := range []int{799, 800, 801} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			input := strings.Repeat("𠮷", size)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var request struct {
					Messages []ModelMessage `json:"messages"`
				}
				if json.NewDecoder(r.Body).Decode(&request) != nil {
					t.Error("request JSON")
				}
				if len(request.Messages) != 2 || request.Messages[0].Role != "system" || strings.Contains(request.Messages[0].Content, "𠮷") {
					t.Error("prompt boundary")
				}
				var payload struct {
					Evidence []struct {
						Number int    `json:"number"`
						Text   string `json:"chunk_text"`
					} `json:"evidence"`
				}
				if json.Unmarshal([]byte(request.Messages[len(request.Messages)-1].Content), &payload) != nil || len(payload.Evidence) != 1 || utf8.RuneCountInString(payload.Evidence[0].Text) != min(size, 800) || payload.Evidence[0].Number != 1 {
					t.Error("Unicode evidence limit")
				}
				w.Header().Set("Content-Type", "text/event-stream")
				fmt.Fprint(w, "data: [DONE]\n\n")
			}))
			defer server.Close()
			p := tutor.Prepared{System: "role", Evidence: []rag.Evidence{{Text: input, Title: "数学", Index: 4}}}
			stream, err := (StreamingModel{Gateway: rag.Gateway{Base: server.URL}}).Open(context.Background(), p, nil, "问题")
			if err != nil {
				t.Fatal(err)
			}
			stream.Close()
			if p.Evidence[0].Text != input {
				t.Fatal("historical evidence rewritten")
			}
		})
	}
}
