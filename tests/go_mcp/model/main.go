// Controlled HTTP model for isolated acceptance only; no upstream forwarding.
package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.Write([]byte(`{"status":"ok","model":"controlled"}`))
			return
		}
		var body struct {
			Stream   bool `json:"stream"`
			Messages []struct {
				Content string `json:"content"`
			} `json:"messages"`
		}
		if json.NewDecoder(r.Body).Decode(&body) != nil {
			w.WriteHeader(400)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/embeddings") {
			json.NewEncoder(w).Encode(map[string]any{"data": []any{map[string]any{"embedding": []float64{1, 0}}}})
			return
		}
		answer := "先列出已知条件，再检查单位与下一步。"
		if len(body.Messages) > 0 {
			var current struct {
				Evidence []any `json:"evidence"`
			}
			if json.Unmarshal([]byte(body.Messages[len(body.Messages)-1].Content), &current) == nil && len(current.Evidence) > 0 {
				answer += "[1]"
			}
		}
		if !body.Stream {
			json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]string{"content": answer}}}})
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		b, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"delta": map[string]string{"content": answer}}}})
		fmt.Fprintf(w, "data: %s\n\ndata: [DONE]\n\n", b)
		w.(http.Flusher).Flush()
	})
	if err := http.ListenAndServe(":8091", nil); err != nil {
		panic("controlled model listener failed")
	}
}
