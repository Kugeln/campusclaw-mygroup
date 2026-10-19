package conversation

import (
	"bytes"
	"campusclaw/internal/rag"
	"campusclaw/internal/tutor"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestTrueIncrementalModelStream(t *testing.T) {
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		frame := []byte("data: {\"choices\":\n" + "data: [{\"delta\":{\"content\":\"首段\"}}]}\n\n")
		split := bytes.Index(frame, []byte("首")) + 1
		w.Write(frame[:split])
		w.(http.Flusher).Flush()
		w.Write(frame[split:])
		w.(http.Flusher).Flush()
		select {
		case <-release:
		case <-r.Context().Done():
			return
		}
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"尾段 [1]\"}}]}\n\ndata: [DONE]\n\n")
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	model := StreamingModel{Gateway: rag.Gateway{Base: server.URL, Key: "test-only", Model: "test"}}
	stream, e := model.Open(ctx, tutor.Prepared{System: "教学原则", Evidence: []rag.Evidence{{Title: "教材", Text: "事实", Index: 1}}}, nil, "问题")
	if e != nil {
		t.Fatal(e)
	}
	defer stream.Close()
	first, e := stream.Next()
	if e != nil || first != "首段" {
		t.Fatal("first fragment unavailable before completion", first, e)
	}
	close(release)
	second, e := stream.Next()
	if e != nil || second != "尾段 [1]" {
		t.Fatal(second, e)
	}
	if _, e = stream.Next(); e != io.EOF {
		t.Fatal("terminal", e)
	}
}
func TestStreamProtocolFailures(t *testing.T) {
	for _, body := range []string{"data: malformed\n\n", "data: {\"choices\":[{\"delta\":{\"content\":\"前缀\"}}]}\n\n", "data: {\"choices\":[{\"finish_reason\":\"length\"}]}\n\n"} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprint(w, body)
		}))
		model := StreamingModel{Gateway: rag.Gateway{Base: server.URL, Key: "test-only"}}
		stream, e := model.Open(context.Background(), tutor.Prepared{System: "教学"}, nil, "问题")
		if e != nil {
			t.Fatal(e)
		}
		_, e = stream.Next()
		if e == nil {
			_, e = stream.Next()
		}
		stream.Close()
		server.Close()
		if e == nil || e == io.EOF {
			t.Fatal("incomplete/invalid stream treated completed")
		}
	}
}
func TestHistoryBudgetAndQuestions(t *testing.T) {
	history := []ModelMessage{{Role: "user", Content: strings.Repeat("字", 6100)}, {Role: "assistant", Content: "旧"}, {Role: "user", Content: "新问题"}, {Role: "assistant", Content: "完成回答"}}
	messages, previous := Context(history)
	if len(messages) != 4 || previous != "新问题" || len([]rune(messages[0].Content)) != 500 {
		t.Fatal("per-message window")
	}
	if _, e := Question(" "); e != Invalid {
		t.Fatal("blank question")
	}
	if _, e := Question(strings.Repeat("字", 501)); e != Invalid {
		t.Fatal("question limit")
	}
}
