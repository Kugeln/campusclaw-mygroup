//go:build mcpstorage

package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestMCPClientRealService(t *testing.T) {
	var targets []Target
	if json.Unmarshal([]byte(os.Getenv("MCP_ALLOWED_TARGETS")), &targets) != nil {
		t.Fatal("isolated targets required")
	}
	p, e := NewPolicy(targets, nil)
	if e != nil {
		t.Fatal(e)
	}
	c, e := NewClient("http://mcp-demo:8090/", os.Getenv("MCP_AUTH_TOKEN"), p)
	if e != nil {
		t.Fatal(e)
	}
	ctx := context.Background()
	if e = c.Initialize(ctx); e != nil {
		t.Fatal(e)
	}
	tools, e := c.List(ctx)
	if e != nil || len(tools) != 2 {
		t.Fatal("discover", e)
	}
	a, e := c.Call(ctx, "formula_hint", map[string]any{"query": "去括号"})
	if e != nil || a.IsError || !strings.Contains(a.Content[0].Text, "分配律") {
		t.Fatal("formula", e)
	}
	b, e := c.Call(ctx, "unit_convert", map[string]any{"value": 1, "from": "m", "to": "cm"})
	if e != nil || b.IsError || b.Content[0].Text != "1 m = 100 cm" {
		t.Fatal("conversion", e)
	}
	bad, _ := NewClient("http://mcp-demo:8090", "wrong-credential", p)
	if bad.Initialize(ctx) == nil {
		t.Fatal("wrong bearer accepted")
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if !errors.Is(c.Initialize(cancelled), Timeout) {
		t.Fatal("cancel not propagated")
	}
	report := map[string]any{"tools": tools, "formula": a, "conversion": b, "wrong_bearer_rejected": true, "cancelled_context_rejected": true}
	data, _ := json.MarshalIndent(report, "", "  ")
	if e = os.WriteFile(os.Getenv("MCP_TEST_EVIDENCE")+"/client-real-service.json", data, 0600); e != nil {
		t.Fatal(e)
	}
}

func fakeNetwork(t *testing.T, handler http.Handler) (string, *Policy, func()) {
	t.Helper()
	host, _ := os.Hostname()
	ips, e := net.DefaultResolver.LookupNetIP(context.Background(), "ip4", host)
	if e != nil || len(ips) == 0 || !safeIP(ips[0]) {
		t.Fatal("dedicated nonloopback test container required")
	}
	ips[0] = ips[0].Unmap()
	l, e := net.Listen("tcp", "0.0.0.0:0")
	if e != nil {
		t.Fatal(e)
	}
	port := l.Addr().(*net.TCPAddr).Port
	server := &http.Server{Handler: handler}
	go server.Serve(l)
	p, e := NewPolicy([]Target{{Scheme: "http", Host: ips[0].String(), Port: port, CIDRs: []string{ips[0].String() + "/32"}}}, nil)
	if e != nil {
		t.Fatal(e)
	}
	return "http://" + net.JoinHostPort(ips[0].String(), strconv.Itoa(port)), p, func() { server.Close() }
}
func TestMCPClientFaults(t *testing.T) {
	for _, mode := range []string{"http401", "http500", "bad_json", "wrong_id", "wrong_version", "result_and_error", "rpc_error", "oversize", "wrong_content", "no_isError", "isError_null", "text_null", "unsupported_content", "timeout", "cancel", "redirect"} {
		t.Run(mode, func(t *testing.T) {
			var calls atomic.Int32
			var trapCalls atomic.Int32
			trap, _, stopTrap := fakeNetwork(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { trapCalls.Add(1) }))
			defer stopTrap()
			began := make(chan struct{})
			base, p, stop := fakeNetwork(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				close(began)
				var req map[string]json.RawMessage
				json.NewDecoder(r.Body).Decode(&req)
				w.Header().Set("Content-Type", "application/json")
				switch mode {
				case "http401":
					w.WriteHeader(401)
					return
				case "http500":
					w.WriteHeader(500)
					return
				case "redirect":
					http.Redirect(w, r, trap, 302)
					return
				case "timeout", "cancel":
					<-r.Context().Done()
					return
				case "oversize":
					w.Write([]byte(strings.Repeat("x", MaxPayload+1)))
					return
				case "bad_json":
					w.Write([]byte("{remote-secret-canary"))
					return
				case "wrong_content":
					w.Header().Set("Content-Type", "text/plain")
				}
				id := req["id"]
				version := "2.0"
				if mode == "wrong_id" {
					id = json.RawMessage(`"mismatch"`)
				}
				if mode == "wrong_version" {
					version = "1.0"
				}
				out := map[string]any{"jsonrpc": version, "id": id, "result": map[string]any{"content": []map[string]string{{"type": "text", "text": "test"}}, "isError": false}}
				if mode == "result_and_error" || mode == "rpc_error" {
					out["error"] = map[string]any{"code": -32603, "message": "remote-secret-canary"}
					if mode == "rpc_error" {
						delete(out, "result")
					}
				}
				if mode == "no_isError" {
					out["result"] = map[string]any{"content": []map[string]string{{"type": "text", "text": "test"}}}
				}
				if mode == "isError_null" {
					out["result"] = map[string]any{"content": []map[string]string{{"type": "text", "text": "test"}}, "isError": nil}
				}
				if mode == "unsupported_content" {
					out["result"] = map[string]any{"content": []map[string]string{{"type": "image", "text": "test"}}, "isError": false}
				}
				if mode == "text_null" {
					out["result"] = map[string]any{"content": []map[string]any{{"type": "text", "text": nil}}, "isError": false}
				}
				json.NewEncoder(w).Encode(out)
			}))
			defer stop()
			c, e := NewClient(base, "ephemeral-network-test", p)
			if e != nil {
				t.Fatal(e)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if mode == "cancel" {
				go func() { <-began; cancel() }()
			}
			started := time.Now()
			_, e = c.Call(ctx, "formula_hint", map[string]string{"query": "test"})
			if e == nil {
				t.Fatal("invalid response accepted")
			}
			if strings.Contains(e.Error(), "canary") {
				t.Fatal("remote secret leaked")
			}
			if calls.Load() != 1 || trapCalls.Load() != 0 {
				t.Fatal("retry/redirect")
			}
			if mode == "timeout" && time.Since(started) > CallBudget+time.Second {
				t.Fatal("timeout unbounded")
			}
		})
	}
}
func TestMCPClientDNSPinningAndStoppedServer(t *testing.T) {
	var calls atomic.Int32
	base, original, stop := fakeNetwork(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		var req map[string]json.RawMessage
		json.NewDecoder(r.Body).Decode(&req)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req["id"], "result": map[string]any{"tools": []any{}}})
	}))
	defer stop()
	target := original.targets[0]
	ip := netip.MustParseAddr(target.Host)
	target.Host = "pinned.invalid"
	resolver := &fakeResolver{ips: []netip.Addr{ip}}
	policy, _ := NewPolicy([]Target{target}, resolver)
	c, e := NewClient("http://pinned.invalid:"+strconv.Itoa(target.Port), "ephemeral", policy)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = c.List(context.Background()); e != nil {
		t.Fatal("verified IP not pinned", e)
	}
	resolver.ips = []netip.Addr{netip.MustParseAddr("127.0.0.1")}
	if _, e = c.List(context.Background()); !errors.Is(e, Denied) {
		t.Fatal("rebinding not rejected")
	}
	if calls.Load() != 1 {
		t.Fatal("DNS rebind reached target")
	}
	stop()
	normal, _ := NewClient(base, "ephemeral", original)
	if _, e = normal.List(context.Background()); e == nil {
		t.Fatal("stopped server succeeded")
	}
}
