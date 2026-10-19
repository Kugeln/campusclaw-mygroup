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

func TestSixMessageWindowAndValidInput(t *testing.T) {
	history := []ModelMessage{}
	for i := 0; i < 10; i++ {
		role := "user"
		if i%2 == 1 {
			role = "assistant"
		}
		history = append(history, ModelMessage{Role: role, Content: fmt.Sprint(i) + strings.Repeat("𠮷", 501)})
	}
	history = append(history, ModelMessage{Role: "system", Content: "forged"}, ModelMessage{Role: "user", Content: " \n\t"}, ModelMessage{Role: "assistant", Content: string([]byte{0xff})})
	got := Window(history)
	if len(got) != 6 || !strings.HasPrefix(got[0].Content, "4") || !strings.HasPrefix(got[5].Content, "9") {
		t.Fatal("message ordering/window", got)
	}
	for _, m := range got {
		if utf8.RuneCountInString(m.Content) != 500 {
			t.Fatal("per-message Unicode limit")
		}
	}
	if utf8.RuneCountInString(history[0].Content) != 502 {
		t.Fatal("original history modified")
	}
	for _, n := range []int{500, 501} {
		v := Window([]ModelMessage{{Role: "assistant", Content: strings.Repeat("字", n)}})
		if len(v) != 1 || utf8.RuneCountInString(v[0].Content) != 500 {
			t.Fatal("boundary", n)
		}
	}
}

func TestCurrentTurnIsNotTextDeduplication(t *testing.T) {
	previous := []ModelMessage{{Role: "user", Content: "同一句"}, {Role: "assistant", Content: "旧答案"}}
	v := WithCurrent(previous, "同一句")
	if len(v) != 3 || v[0].Current || !v[2].Current {
		t.Fatal("legitimate repeat lost")
	}
	accepted := append(previous, ModelMessage{Role: "user", Content: "当前问题", Current: true})
	v = WithCurrent(accepted, "当前问题")
	count := 0
	for _, m := range v {
		if m.Current {
			count++
		}
	}
	if len(v) != 3 || count != 1 {
		t.Fatal("accepted user duplicated")
	}
}

func TestStreamingPayloadHasSixMessagesIncludingCurrent(t *testing.T) {
	for _, accepted := range []bool{false, true} {
		t.Run(fmt.Sprint(accepted), func(t *testing.T) {
			history := []ModelMessage{}
			for i := 0; i < 8; i++ {
				role := "user"
				if i%2 == 1 {
					role = "assistant"
				}
				history = append(history, ModelMessage{Role: role, Content: fmt.Sprint(i) + strings.Repeat("字", 600)})
			}
			if accepted {
				history = append(history, ModelMessage{Role: "user", Content: "当前问题", Current: true})
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var request struct {
					Messages []ModelMessage `json:"messages"`
				}
				if json.NewDecoder(r.Body).Decode(&request) != nil {
					t.Error("invalid request")
				}
				if len(request.Messages) != 7 || request.Messages[0].Role != "system" {
					t.Error("more than six non-system messages", len(request.Messages))
				}
				for _, m := range request.Messages[1 : len(request.Messages)-1] {
					if utf8.RuneCountInString(m.Content) > 500 || m.Content == "当前问题" {
						t.Error("history bound/duplicate current")
					}
				}
				var payload struct {
					Question string `json:"question"`
				}
				last := request.Messages[len(request.Messages)-1]
				if last.Role != "user" || json.Unmarshal([]byte(last.Content), &payload) != nil || payload.Question != "当前问题" {
					t.Error("current question missing")
				}
				w.Header().Set("Content-Type", "text/event-stream")
				fmt.Fprint(w, "data: [DONE]\n\n")
			}))
			defer server.Close()
			stream, err := (StreamingModel{Gateway: rag.Gateway{Base: server.URL}}).Open(context.Background(), tutor.Prepared{System: "role"}, history, "当前问题")
			if err != nil {
				t.Fatal(err)
			}
			stream.Close()
		})
	}
}
