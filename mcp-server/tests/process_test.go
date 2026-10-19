package tests

import (
	"bytes"
	"campusclaw/mcp-server/internal/checks"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestRealServiceProcess(t *testing.T) {
	name := "mcp-service"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	bin := filepath.Join(t.TempDir(), name)
	build := exec.Command("go", "build", "-o", bin, "../cmd/mcp-server")
	if out, e := build.CombinedOutput(); e != nil {
		t.Fatalf("build: %v %s", e, out)
	}
	baseEnv := []string{}
	for _, v := range os.Environ() {
		if !strings.HasPrefix(v, "MCP_") {
			baseEnv = append(baseEnv, v)
		}
	}
	missing := exec.Command(bin)
	missing.Env = append(baseEnv, "MCP_AUTH_TOKEN=")
	out, e := missing.CombinedOutput()
	if e == nil || !bytes.Contains(out, []byte("configuration invalid")) {
		t.Fatal("missing credential did not fail safely")
	}
	t.Log("missing_credential_startup: pass (nonzero)")
	var random [32]byte
	if _, e = rand.Read(random[:]); e != nil {
		t.Fatal(e)
	}
	token := hex.EncodeToString(random[:])
	l, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	addr := l.Addr().String()
	l.Close()
	cmd := exec.Command(bin)
	cmd.Env = append(baseEnv, "MCP_AUTH_TOKEN="+token, "MCP_LISTEN_ADDR="+addr, "MCP_ALLOWED_ORIGINS=https://teacher.example")
	var logs bytes.Buffer
	cmd.Stdout = &logs
	cmd.Stderr = &logs
	if e = cmd.Start(); e != nil {
		t.Fatal(e)
	}
	stopped := false
	defer func() {
		if !stopped {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	}()
	base := "http://" + addr
	ready := false
	client := http.Client{Timeout: 200 * time.Millisecond}
	for i := 0; i < 100; i++ {
		res, err := client.Get(base + "/health")
		if err == nil {
			res.Body.Close()
			ready = res.StatusCode == 200
			if ready {
				break
			}
		}
		time.Sleep(30 * time.Millisecond)
	}
	if !ready {
		t.Fatal("service did not become ready")
	}
	results := checks.Run(base, token)
	for _, r := range results {
		r := r
		t.Run(r.Name, func(t *testing.T) {
			if !r.Passed {
				t.Fatalf("network check failed: status=%d", r.Status)
			}
		})
	}
	if dest := os.Getenv("MCP_TEST_EVIDENCE"); dest != "" {
		data, _ := json.MarshalIndent(results, "", "  ")
		if e = os.WriteFile(dest, data, 0600); e != nil {
			t.Fatal(e)
		}
	}
	if e = cmd.Process.Kill(); e != nil {
		t.Fatal(e)
	}
	_ = cmd.Wait()
	stopped = true
	req, _ := http.NewRequest("POST", base+"/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := client.Do(req)
	if err == nil {
		res.Body.Close()
		t.Fatal("stopped service still succeeds")
	}
	if bytes.Contains(logs.Bytes(), []byte(token)) {
		t.Fatal("credential leaked in logs")
	}
	t.Log("stopped_service_unavailable: pass; captured service logs contain no credential")
}
