package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestRegistryRejectsDuplicatesAndOverLimit(t *testing.T) {
	tool := Builtins().tools[0]
	if _, e := NewRegistry([]Tool{tool, tool}); e == nil {
		t.Fatal("duplicate registration accepted")
	}
	if _, e := NewRegistry(make([]Tool, MaxTools+1)); e == nil {
		t.Fatal("oversize registry accepted")
	}
	if _, e := NewRegistry(nil); e == nil {
		t.Fatal("empty registry accepted")
	}
}

func TestConfiguration(t *testing.T) {
	for _, c := range []Config{{}, {Token: "has whitespace"}, {Token: "test-only", AllowedOrigins: []string{"*"}}, {Token: "test-only", AllowedOrigins: []string{"https://example.test/path"}}} {
		if _, e := New(c, Builtins()); e == nil {
			t.Fatal("invalid configuration accepted")
		}
	}
}

func TestTimeoutAndCancellationNoRetry(t *testing.T) {
	for _, cancelEarly := range []bool{false, true} {
		t.Run(map[bool]string{false: "three_second_deadline", true: "request_cancel"}[cancelEarly], func(t *testing.T) {
			var calls atomic.Int32
			started, stopped := make(chan struct{}), make(chan struct{})
			tool := Builtins().tools[0]
			tool.execute = func(ctx context.Context, _ any) CallResult {
				calls.Add(1)
				close(started)
				<-ctx.Done()
				close(stopped)
				return result("cancelled", true)
			}
			registry, e := NewRegistry([]Tool{tool})
			if e != nil {
				t.Fatal(e)
			}
			h, e := New(Config{Token: "test-only"}, registry)
			if e != nil {
				t.Fatal(e)
			}
			s := httptest.NewServer(h)
			defer s.Close()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			req, _ := http.NewRequestWithContext(ctx, "POST", s.URL+"/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"formula_hint","arguments":{"query":"test"}}}`))
			req.Header.Set("Authorization", "Bearer test-only")
			req.Header.Set("Content-Type", "application/json")
			if cancelEarly {
				go func() { <-started; cancel() }()
			}
			begin := time.Now()
			res, e := http.DefaultClient.Do(req)
			if cancelEarly {
				if e == nil {
					res.Body.Close()
					t.Fatal("cancelled HTTP succeeded")
				}
			} else {
				if e != nil {
					t.Fatal(e)
				}
				defer res.Body.Close()
				var envelope struct{ Result CallResult }
				if json.NewDecoder(res.Body).Decode(&envelope) != nil || !envelope.Result.IsError || !strings.Contains(envelope.Result.Content[0].Text, "超时") {
					t.Fatal("timeout not a tool error")
				}
				if elapsed := time.Since(begin); elapsed < ToolTimeout || elapsed > ToolTimeout+time.Second {
					t.Fatalf("deadline duration %v", elapsed)
				}
			}
			select {
			case <-stopped:
			case <-time.After(time.Second):
				t.Fatal("cancellation not propagated")
			}
			if calls.Load() != 1 {
				t.Fatal("tool retried")
			}
		})
	}
}

func TestChunkedBodyLimitAndNoExecution(t *testing.T) {
	var calls atomic.Int32
	tool := Builtins().tools[0]
	tool.execute = func(context.Context, any) CallResult { calls.Add(1); return result("unexpected", false) }
	registry, _ := NewRegistry([]Tool{tool})
	h, _ := New(Config{Token: "test-only"}, registry)
	req := httptest.NewRequest("POST", "/mcp", strings.NewReader(strings.Repeat(" ", MaxBody+1)))
	req.ContentLength = -1
	req.Header.Set("Authorization", "Bearer test-only")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != 413 || calls.Load() != 0 {
		t.Fatal("chunked limit bypassed")
	}
}

func TestRejectedRequestsNeverExecute(t *testing.T) {
	var calls atomic.Int32
	tool := Builtins().tools[0]
	tool.execute = func(context.Context, any) CallResult { calls.Add(1); return result("unexpected", false) }
	registry, _ := NewRegistry([]Tool{tool})
	h, _ := New(Config{Token: "test-only"}, registry)
	valid := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"formula_hint","arguments":{"query":"去括号"}}}`
	for _, mode := range []string{"missing_auth", "wrong_auth", "origin", "invalid_arguments"} {
		t.Run(mode, func(t *testing.T) {
			body := valid
			if mode == "invalid_arguments" {
				body = strings.Replace(body, `"query":"去括号"`, `"query":false`, 1)
			}
			req := httptest.NewRequest("POST", "/mcp", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			if mode != "missing_auth" {
				req.Header.Set("Authorization", "Bearer test-only")
			}
			if mode == "wrong_auth" {
				req.Header.Set("Authorization", "Bearer wrong")
			}
			if mode == "origin" {
				req.Header.Set("Origin", "https://untrusted.invalid")
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, req)
			if calls.Load() != 0 {
				t.Fatal("rejected request executed tool")
			}
			if mode == "invalid_arguments" && !strings.Contains(w.Body.String(), "-32602") {
				t.Fatal("argument error missing")
			}
		})
	}
}

func TestPublishedSchemaAndNumericAccuracy(t *testing.T) {
	registry := Builtins()
	query := registry.tools[0].InputSchema["properties"].(map[string]any)["query"].(map[string]any)
	if query["minLength"] != 2 || query["maxLength"] != 500 {
		t.Fatal("query schema drift")
	}
	tool := registry.tools[1]
	for _, pair := range []struct{ raw, expected string }{
		{`{"value":1,"from":"mm","to":"m"}`, "1 mm = 0.001 m"},
		{`{"value":-2,"from":"km","to":"m"}`, "-2 km = -2000 m"},
		{`{"value":1000000000000,"from":"km","to":"mm"}`, "1e+12 km = 1e+18 mm"},
		{`{"value":0,"from":"mg","to":"kg"}`, "0 mg = 0 kg"},
	} {
		args, err := tool.validate(json.RawMessage(pair.raw))
		if err != nil {
			t.Fatal(err)
		}
		value := tool.execute(context.Background(), args)
		if value.IsError || value.Content[0].Text != pair.expected {
			t.Fatalf("conversion mismatch: %s", value.Content[0].Text)
		}
	}
}
