//go:build mcpstorage

package httpapi

import (
	"campusclaw/internal/auth"
	"campusclaw/internal/mcp"
	repository "campusclaw/internal/repository/mysql"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	driver "github.com/go-sql-driver/mysql"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type mcpHTTPFixture struct {
	db       *sql.DB
	store    repository.MCP
	identity *auth.Identity
	tokens   map[int64]string
	url      string
	client   *http.Client
	results  []map[string]any
	mu       sync.Mutex
}

func newMCPHTTP(t *testing.T) *mcpHTTPFixture {
	t.Helper()
	cfg, e := driver.ParseDSN(os.Getenv("MCP_TEST_DSN"))
	host := os.Getenv("MCP_TEST_MYSQL")
	if e != nil || cfg == nil || (!strings.HasPrefix(host, "campusclaw-w6-p3-") && !strings.HasPrefix(host, "campusclaw-w6-p4-")) || cfg.Addr != host+":3306" || cfg.DBName != "" || cfg.Passwd == "" {
		t.Fatal("dedicated Phase3 MySQL required")
	}
	admin, e := sql.Open("mysql", cfg.FormatDSN())
	if e != nil {
		t.Fatal(e)
	}
	defer admin.Close()
	var random [8]byte
	rand.Read(random[:])
	cfg.DBName = "w6p3_" + hex.EncodeToString(random[:])
	if _, e = admin.Exec("CREATE DATABASE " + cfg.DBName + " CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_bin"); e != nil {
		t.Fatal(e)
	}
	db, e := sql.Open("mysql", cfg.FormatDSN())
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { db.Close() })
	ctx := context.Background()
	if e = repository.PrepareSchema(ctx, db); e != nil {
		t.Fatal(e)
	}
	if e = repository.PrepareRAGSchema(ctx, db); e != nil {
		t.Fatal(e)
	}
	exec := func(q string) {
		if _, e := db.Exec(q); e != nil {
			t.Fatal(e)
		}
	}
	exec("INSERT INTO classes(id,name) VALUES(1,'A'),(2,'B')")
	exec("INSERT INTO users(id,username,password_hash,role,class_id,session_version) VALUES(1,'teacher-a','fixture','teacher',1,0),(2,'teacher-b','fixture','teacher',2,0),(3,'student-a','fixture','student',1,0),(4,'student-b','fixture','student',2,0),(5,'teacher-a2','fixture','teacher',1,0)")
	var key [32]byte
	rand.Read(key[:])
	vault, e := mcp.NewVault("fixture", map[string][]byte{"fixture": key[:]})
	if e != nil {
		t.Fatal(e)
	}
	var targets []mcp.Target
	if json.Unmarshal([]byte(os.Getenv("MCP_ALLOWED_TARGETS")), &targets) != nil {
		t.Fatal("targets")
	}
	policy, e := mcp.NewPolicy(targets, nil)
	if e != nil {
		t.Fatal(e)
	}
	var signing [32]byte
	rand.Read(signing[:])
	tokens, e := auth.NewTokens(hex.EncodeToString(signing[:]), time.Now)
	if e != nil {
		t.Fatal(e)
	}
	identity := &auth.Identity{Tokens: tokens, Store: repository.LoginUsers{DB: db}}
	f := &mcpHTTPFixture{db: db, identity: identity, store: repository.MCP{DB: db, Vault: vault, Policy: policy}, tokens: map[int64]string{}, client: &http.Client{Timeout: 15 * time.Second}}
	for _, id := range []int64{1, 2, 3, 4, 5} {
		f.tokens[id], e = tokens.Issue(id, 0)
		if e != nil {
			t.Fatal(e)
		}
	}
	server := httptest.NewServer(WithMCP(WithAuth(healthy{}, nil, identity), identity, &MCPHandler{Store: f.store}))
	f.url = server.URL
	t.Cleanup(server.Close)
	t.Cleanup(func() {
		f.mu.Lock()
		defer f.mu.Unlock()
		b, _ := json.MarshalIndent(map[string]any{"database": cfg.DBName, "requests": f.results}, "", "  ")
		if e := os.WriteFile(filepath.Join(os.Getenv("MCP_TEST_EVIDENCE"), strings.ReplaceAll(t.Name(), "/", "__")+".json"), b, 0600); e != nil {
			t.Error(e)
		}
	})
	return f
}
func (f *mcpHTTPFixture) request(t *testing.T, user int64, method, path string, body any, want int) []byte {
	t.Helper()
	var data []byte
	if raw, ok := body.(string); ok {
		data = []byte(raw)
	} else if body != nil {
		data, _ = json.Marshal(body)
	}
	req, e := http.NewRequest(method, f.url+path, strings.NewReader(string(data)))
	if e != nil {
		t.Fatal(e)
	}
	if user != 0 {
		req.Header.Set("Authorization", "Bearer "+f.tokens[user])
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, e := f.client.Do(req)
	if e != nil {
		t.Fatal(e)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	// Store only safe response bodies, never request bodies, credentials or JWTs.
	for _, secret := range append([]string{os.Getenv("MCP_AUTH_TOKEN")}, mapValues(f.tokens)...) {
		if secret != "" && strings.Contains(string(b), secret) {
			t.Fatal("response credential leak")
		}
	}
	for _, key := range []string{`"auth_token"`, `"token_cipher"`, `"token_nonce"`, `"key_id"`, `"op_id"`} {
		if strings.Contains(string(b), key) {
			t.Fatal("internal credential field leak")
		}
	}
	if strings.HasPrefix(path, "/api/mcp/") && strings.Contains(string(b), `"revision"`) {
		t.Fatal("internal MCP revision exposed")
	}
	f.mu.Lock()
	f.results = append(f.results, map[string]any{"user": user, "method": method, "path": path, "status": res.StatusCode, "body": string(b)})
	f.mu.Unlock()
	if res.StatusCode != want {
		t.Fatalf("%s %s user=%d got=%d want=%d body=%s", method, path, user, res.StatusCode, want, b)
	}
	return b
}
func mapValues(m map[int64]string) []string {
	v := []string{}
	for _, s := range m {
		v = append(v, s)
	}
	return v
}
func (f *mcpHTTPFixture) principal(t *testing.T, id int64) auth.Principal {
	t.Helper()
	p, e := f.identity.Authenticate(context.Background(), f.tokens[id])
	if e != nil {
		t.Fatal(e)
	}
	return p
}
func (f *mcpHTTPFixture) create(t *testing.T, user int64, name, base, token string) int64 {
	t.Helper()
	b := f.request(t, user, "POST", "/api/mcp/servers", map[string]string{"name": name, "base_url": base, "auth_token": token}, 201)
	var v repository.MCPServer
	if json.Unmarshal(b, &v) != nil || v.ID <= 0 || v.Connected || v.ToolCount != 0 || v.HasAuth != (token != "") {
		t.Fatal("initial view")
	}
	return v.ID
}
func resource(id int64) string { return "/api/mcp/servers/" + strconv.FormatInt(id, 10) }
func toolsCount(t *testing.T, b []byte, want int) {
	t.Helper()
	var v MCPToolList
	if json.Unmarshal(b, &v) != nil || v.Tools == nil || len(v.Tools) != want {
		t.Fatalf("tool count got %s want %d", b, want)
	}
}
func TestMCPHTTPManagement(t *testing.T) {
	f := newMCPHTTP(t)
	base := "http://mcp-demo:8090"
	token := os.Getenv("MCP_AUTH_TOKEN")
	id := f.create(t, 1, "math-demo", base, token)
	path := resource(id)
	t.Run("guards_all_eight", func(t *testing.T) {
		for _, op := range []struct{ method, path string }{{"GET", "/api/mcp/servers"}, {"POST", "/api/mcp/servers"}, {"PUT", path}, {"DELETE", path}, {"POST", path + "/connect"}, {"POST", path + "/disconnect"}, {"GET", path + "/tools"}, {"GET", "/api/mcp/tool-calls"}} {
			f.request(t, 0, op.method, op.path, nil, 401)
			f.request(t, 3, op.method, op.path, nil, 403)
			f.request(t, 4, op.method, op.path, nil, 403)
		}
	})
	t.Run("class_scope_and_no_override", func(t *testing.T) {
		for _, op := range []struct{ method, suffix string }{{"PUT", ""}, {"DELETE", ""}, {"POST", "/connect"}, {"POST", "/disconnect"}, {"GET", "/tools"}} {
			b := f.request(t, 2, op.method, path+op.suffix, nil, 404)
			other := f.request(t, 2, op.method, resource(9999999)+op.suffix, nil, 404)
			if string(b) != string(other) {
				t.Fatal("existence leak")
			}
		}
		f.request(t, 1, "GET", "/api/mcp/servers?class_id=2", nil, 400)
		f.request(t, 1, "GET", "/api/mcp/tool-calls?class_id=2", nil, 400)
		f.request(t, 1, "POST", "/api/mcp/servers", map[string]any{"name": "forged", "base_url": base, "auth_token": "", "class_id": 2}, 400)
		b := f.request(t, 2, "GET", "/api/mcp/servers", nil, 200)
		var v MCPServerList
		json.Unmarshal(b, &v)
		if len(v.Servers) != 0 {
			t.Fatal("cross class list")
		}
		f.request(t, 5, "GET", path+"/tools", nil, 200)
	})
	t.Run("duplicate_and_encryption", func(t *testing.T) {
		f.request(t, 1, "POST", "/api/mcp/servers", map[string]string{"name": "math-demo", "base_url": base, "auth_token": token}, 400)
		f.create(t, 2, "math-demo", base, token)
		var cipher, nonce []byte
		var key string
		if f.db.QueryRow("SELECT token_cipher,token_nonce,key_id FROM mcp_servers WHERE id=?", id).Scan(&cipher, &nonce, &key) != nil || strings.Contains(string(cipher), token) || len(nonce) != 12 || key == "" {
			t.Fatal("encryption")
		}
	})
	t.Run("real_connect_reconnect_cache", func(t *testing.T) {
		for i := 0; i < 2; i++ {
			b := f.request(t, 5, "POST", path+"/connect", nil, 200)
			var v MCPConnected
			if json.Unmarshal(b, &v) != nil || !v.Connected || len(v.Tools) != 2 {
				t.Fatal("real discovery")
			}
		}
		toolsCount(t, f.request(t, 1, "GET", path+"/tools", nil, 200), 2)
		b := f.request(t, 1, "GET", "/api/mcp/servers", nil, 200)
		var v MCPServerList
		json.Unmarshal(b, &v)
		if len(v.Servers) != 1 || v.Servers[0].ToolCount != 2 || !v.Servers[0].Connected {
			t.Fatal("cache view")
		}
	})
	t.Run("rename_empty_token_and_update_rollback", func(t *testing.T) {
		f.request(t, 1, "PUT", path, map[string]string{"name": "renamed", "base_url": base, "auth_token": ""}, 200)
		toolsCount(t, f.request(t, 1, "GET", path+"/tools", nil, 200), 0)
		f.request(t, 1, "POST", path+"/connect", nil, 200)
		other := f.create(t, 1, "occupied", base, "")
		f.request(t, 1, "PUT", path, map[string]string{"name": "occupied", "base_url": base}, 400)
		toolsCount(t, f.request(t, 1, "GET", path+"/tools", nil, 200), 2)
		f.request(t, 1, "DELETE", resource(other), nil, 204)
	})
	t.Run("audit_limits_and_restrict", func(t *testing.T) {
		p := f.principal(t, 1)
		for i := 0; i < 105; i++ {
			if ok, e := f.store.AppendTerminal(context.Background(), p, "renamed", "formula_hint", "ok", "safe summary "+token, nil); !ok || e != nil {
				t.Fatal(e)
			}
		}
		p2 := f.principal(t, 2)
		if ok, e := f.store.AppendTerminal(context.Background(), p2, "math-demo", "formula_hint", "denied", "", nil); !ok || e != nil {
			t.Fatal(e)
		}
		for _, test := range []struct {
			query string
			count int
		}{{"", 50}, {"?limit=101", 100}, {"?limit=1", 1}} {
			b := f.request(t, 5, "GET", "/api/mcp/tool-calls"+test.query, nil, 200)
			var v MCPCallList
			json.Unmarshal(b, &v)
			if len(v.Calls) != test.count {
				t.Fatal("audit limit")
			}
			for i, c := range v.Calls {
				if c.ServerName != "renamed" || c.UserID != 1 || c.Ref != "formula_hint@renamed" || c.ConversationID != nil || c.CreatedAt == "" {
					t.Fatal("audit scope or fields")
				}
				if i > 0 && (v.Calls[i-1].CreatedAt < c.CreatedAt || (v.Calls[i-1].CreatedAt == c.CreatedAt && v.Calls[i-1].ID < c.ID)) {
					t.Fatal("audit order")
				}
			}
		}
		for _, q := range []string{"0", "-1", "bad", "", "1&limit=2"} {
			f.request(t, 1, "GET", "/api/mcp/tool-calls?limit="+q, nil, 400)
		}
		f.request(t, 1, "DELETE", path, nil, 503)
		toolsCount(t, f.request(t, 1, "GET", path+"/tools", nil, 200), 2)
		for i := 0; i < 2; i++ {
			f.request(t, 1, "POST", path+"/disconnect", nil, 200)
		}
		toolsCount(t, f.request(t, 1, "GET", path+"/tools", nil, 200), 0)
		var n int
		if f.db.QueryRow("SELECT COUNT(*) FROM mcp_tool_calls WHERE server_id=?", id).Scan(&n) != nil || n != 105 {
			t.Fatal("history changed")
		}
	})
	t.Run("delete_cascade_repeated", func(t *testing.T) {
		n := f.create(t, 1, "disposable", base, token)
		f.request(t, 1, "POST", resource(n)+"/connect", nil, 200)
		f.request(t, 1, "DELETE", resource(n), nil, 204)
		f.request(t, 1, "DELETE", resource(n), nil, 404)
		var count int
		f.db.QueryRow("SELECT COUNT(*) FROM mcp_tools WHERE server_id=?", n).Scan(&count)
		if count != 0 {
			t.Fatal("orphan cache")
		}
	})
	t.Run("invalid_inputs_and_no_extra_operations", func(t *testing.T) {
		for _, body := range []string{`{}`, `{"name":"x","name":"y","base_url":"http://mcp-demo:8090","auth_token":""}`, `{"name":"Upper","base_url":"http://mcp-demo:8090","auth_token":""}`, `{"name":"valid","base_url":"http://mcp-demo:8090"}`, `{"name":null,"base_url":"http://mcp-demo:8090","auth_token":""}`, `{"name":"valid","base_url":"http://mcp-demo:8090","auth_token":3}`} {
			f.request(t, 1, "POST", "/api/mcp/servers", body, 400)
		}
		f.request(t, 1, "GET", path, nil, 404)
		f.request(t, 1, "POST", path+"/discover", nil, 404)
		f.request(t, 1, "GET", "/api/mcp/servers/0/tools", nil, 400)
	})
	t.Run("wrong_service_token_502", func(t *testing.T) {
		f.request(t, 1, "PUT", path, map[string]string{"name": "renamed", "base_url": base, "auth_token": "wrong-fixture-token"}, 200)
		f.request(t, 1, "POST", path+"/connect", nil, 502)
		toolsCount(t, f.request(t, 1, "GET", path+"/tools", nil, 200), 0)
	})
	t.Run("revocation_and_current_class", func(t *testing.T) {
		if _, e := f.db.Exec("UPDATE users SET class_id=2 WHERE id=5"); e != nil {
			t.Fatal(e)
		}
		f.request(t, 5, "GET", path+"/tools", nil, 404)
		if _, e := f.db.Exec("UPDATE users SET session_version=1 WHERE id=5"); e != nil {
			t.Fatal(e)
		}
		f.request(t, 5, "GET", "/api/mcp/servers", nil, 401)
	})
}

// Controlled network fixtures are only for faults and races; the normal
// acceptance above always connects to the independent Phase1 service.
func faultMCP(t *testing.T, fn http.HandlerFunc) (string, *mcp.Policy, func()) {
	t.Helper()
	host, _ := os.Hostname()
	ips, e := net.DefaultResolver.LookupNetIP(context.Background(), "ip4", host)
	if e != nil || len(ips) == 0 {
		t.Fatal("container address")
	}
	ip := ips[0].Unmap()
	l, e := net.Listen("tcp", "0.0.0.0:0")
	if e != nil {
		t.Fatal(e)
	}
	port := l.Addr().(*net.TCPAddr).Port
	server := &http.Server{Handler: fn}
	go server.Serve(l)
	policy, e := mcp.NewPolicy([]mcp.Target{{Scheme: "http", Host: ip.String(), Port: port, CIDRs: []string{ip.String() + "/32"}}}, nil)
	if e != nil {
		t.Fatal(e)
	}
	return "http://" + net.JoinHostPort(ip.String(), strconv.Itoa(port)), policy, func() { server.Close() }
}
func faultReply(w http.ResponseWriter, r *http.Request, tools any) {
	var req struct {
		ID     json.RawMessage `json:"id"`
		Method string          `json:"method"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	w.Header().Set("Content-Type", "application/json")
	result := any(map[string]any{"protocolVersion": mcp.ProtocolVersion, "capabilities": map[string]any{"tools": map[string]any{}}, "serverInfo": map[string]string{"name": "fixture", "version": "1"}})
	if req.Method == "tools/list" {
		result = map[string]any{"tools": tools}
	}
	json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": result})
}
func (f *mcpHTTPFixture) useStore(t *testing.T) {
	s := httptest.NewServer(WithMCP(WithAuth(healthy{}, nil, f.identity), f.identity, &MCPHandler{Store: f.store}))
	f.url = s.URL
	t.Cleanup(s.Close)
}
func TestMCPHTTPFaults(t *testing.T) {
	f := newMCPHTTP(t)
	for _, mode := range []string{"stopped", "schema", "timeout", "redirect", "protocol"} {
		t.Run(mode, func(t *testing.T) {
			var count atomic.Int32
			var trap atomic.Int32
			trapBase, _, stopTrap := faultMCP(t, func(w http.ResponseWriter, r *http.Request) { trap.Add(1) })
			defer stopTrap()
			base, policy, stop := faultMCP(t, func(w http.ResponseWriter, r *http.Request) {
				count.Add(1)
				switch mode {
				case "timeout":
					<-r.Context().Done()
				case "redirect":
					http.Redirect(w, r, trapBase, 302)
				case "schema":
					faultReply(w, r, []any{map[string]any{"name": "bad", "description": "x", "inputSchema": map[string]any{"type": "array"}}})
				case "protocol":
					w.Header().Set("Content-Type", "application/json")
					io.WriteString(w, `{"jsonrpc":"1.0","id":"bad","result":{}}`)
				default:
					faultReply(w, r, []any{})
				}
			})
			defer stop()
			f.store.Policy = policy
			f.useStore(t)
			id := f.create(t, 1, "fault-"+mode, base, "")
			if mode == "stopped" {
				stop()
			}
			start := time.Now()
			f.request(t, 1, "POST", resource(id)+"/connect", nil, 502)
			if time.Since(start) > 5*time.Second {
				t.Fatal("timeout budget")
			}
			toolsCount(t, f.request(t, 1, "GET", resource(id)+"/tools", nil, 200), 0)
			if trap.Load() != 0 {
				t.Fatal("redirect followed")
			}
			want := int32(1)
			if mode == "stopped" {
				want = 0
			}
			if mode == "schema" {
				want = 2
			}
			if count.Load() != want {
				t.Fatal("automatic retry or unexpected RPC count")
			}
		})
	}
	t.Run("ssrf_registration", func(t *testing.T) {
		f.useStore(t)
		for i, base := range []string{"http://localhost:8090", "http://127.0.0.1:8090", "http://169.254.169.254", "http://100.100.100.200", "http://168.63.129.16", "http://[::1]:8090", "http://user:pass@mcp-demo:8090", "http://mcp-demo:8090/path", "http://mcp-demo:8090?secret=x", "http://mcp-demo:8091"} {
			f.request(t, 1, "POST", "/api/mcp/servers", map[string]string{"name": fmt.Sprintf("denied%d", i), "base_url": base, "auth_token": ""}, 400)
		}
	})
	t.Run("business_503_auth_500", func(t *testing.T) {
		broken, _ := sql.Open("mysql", "root:no@tcp(127.0.0.1:1)/none?timeout=100ms")
		defer broken.Close()
		f.store.DB = broken
		f.useStore(t)
		f.request(t, 1, "GET", "/api/mcp/servers", nil, 503)
		badIdentity := &auth.Identity{Tokens: f.identity.Tokens, Store: repository.LoginUsers{DB: broken}}
		s := httptest.NewServer(WithMCP(Handler(healthy{}), badIdentity, &MCPHandler{Store: f.store}))
		defer s.Close()
		f.url = s.URL
		f.request(t, 1, "GET", "/api/mcp/servers", nil, 500)
	})
}

type rebindResolver struct {
	count atomic.Int32
	first netip.Addr
}

func (r *rebindResolver) LookupNetIP(context.Context, string, string) ([]netip.Addr, error) {
	if r.count.Add(1) == 1 {
		return []netip.Addr{r.first}, nil
	}
	return []netip.Addr{netip.MustParseAddr("169.254.169.254")}, nil
}
func TestMCPHTTPDNSRebinding(t *testing.T) {
	f := newMCPHTTP(t)
	var visits atomic.Int32
	base, _, stop := faultMCP(t, func(w http.ResponseWriter, r *http.Request) { visits.Add(1); faultReply(w, r, []any{}) })
	defer stop()
	addr := strings.TrimPrefix(base, "http://")
	host, port, _ := net.SplitHostPort(addr)
	n, _ := strconv.Atoi(port)
	res := &rebindResolver{first: netip.MustParseAddr(host)}
	f.store.Policy, _ = mcp.NewPolicy([]mcp.Target{{Scheme: "http", Host: "rebind.fixture", Port: n, CIDRs: []string{host + "/32"}}}, res)
	f.useStore(t)
	id := f.create(t, 1, "rebind", "http://rebind.fixture:"+port, "")
	f.request(t, 1, "POST", resource(id)+"/connect", nil, 502)
	if visits.Load() != 0 {
		t.Fatal("rebinding dialled")
	}
}
func TestMCPHTTPRaces(t *testing.T) {
	f := newMCPHTTP(t)
	base := "http://mcp-demo:8090"
	token := os.Getenv("MCP_AUTH_TOKEN")
	t.Run("concurrent_unique_create", func(t *testing.T) {
		var wg sync.WaitGroup
		var created, invalid atomic.Int32
		for i := 0; i < 8; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				req, _ := http.NewRequest("POST", f.url+"/api/mcp/servers", strings.NewReader(fmt.Sprintf(`{"name":"race-name","base_url":%q,"auth_token":""}`, base)))
				req.Header.Set("Authorization", "Bearer "+f.tokens[1])
				req.Header.Set("Content-Type", "application/json")
				r, e := f.client.Do(req)
				if e != nil {
					t.Error(e)
					return
				}
				io.Copy(io.Discard, r.Body)
				r.Body.Close()
				if r.StatusCode == 201 {
					created.Add(1)
				} else if r.StatusCode == 400 {
					invalid.Add(1)
				} else {
					t.Errorf("unexpected status %d", r.StatusCode)
				}
			}()
		}
		wg.Wait()
		if created.Load() != 1 || invalid.Load() != 7 {
			t.Fatal("uniqueness race")
		}
	})
	for _, action := range []string{"update", "disconnect", "delete", "connect"} {
		t.Run("late_discovery_after_"+action, func(t *testing.T) {
			began, release := make(chan struct{}), make(chan struct{})
			var once sync.Once
			var requests atomic.Int32
			remote, policy, stop := faultMCP(t, func(w http.ResponseWriter, r *http.Request) {
				n := requests.Add(1)
				if n == 2 {
					once.Do(func() { close(began) })
					select {
					case <-release:
					case <-r.Context().Done():
						return
					}
				}
				faultReply(w, r, []any{map[string]any{"name": "fixture_tool", "description": "controlled", "inputSchema": map[string]any{"type": "object"}}})
			})
			defer stop()
			f.store.Policy = policy
			f.useStore(t)
			id := f.create(t, 1, "race-"+action, remote, "")
			path := resource(id)
			done := make(chan []byte, 1)
			go func() {
				done <- f.request(t, 1, "POST", path+"/connect", nil, map[bool]int{true: 404, false: 502}[action == "delete"])
			}()
			select {
			case <-began:
			case <-time.After(4 * time.Second):
				t.Fatal("discovery did not start")
			}
			switch action {
			case "update":
				f.request(t, 1, "PUT", path, map[string]string{"name": "updated", "base_url": remote}, 200)
			case "disconnect":
				for i := 0; i < 4; i++ {
					f.request(t, 1, "POST", path+"/disconnect", nil, 200)
				}
			case "delete":
				f.request(t, 1, "DELETE", path, nil, 204)
			case "connect":
				f.request(t, 1, "POST", path+"/connect", nil, 200)
			}
			close(release)
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				t.Fatal("late discovery blocked")
			}
			if action != "delete" {
				want := 0
				if action == "connect" {
					want = 1
				}
				toolsCount(t, f.request(t, 1, "GET", path+"/tools", nil, 200), want)
			} else {
				var n int
				f.db.QueryRow("SELECT COUNT(*) FROM mcp_tools WHERE server_id=?", id).Scan(&n)
				if n != 0 {
					t.Fatal("orphan")
				}
			}
		})
	}
	// Ensure the normal independent service remains usable after all races.
	var targets []mcp.Target
	json.Unmarshal([]byte(os.Getenv("MCP_ALLOWED_TARGETS")), &targets)
	f.store.Policy, _ = mcp.NewPolicy(targets, nil)
	f.useStore(t)
	id := f.create(t, 1, "after-races", base, token)
	f.request(t, 1, "POST", resource(id)+"/connect", nil, 200)
}

func TestMCPHTTPRefreshAndDBFailure(t *testing.T) {
	f := newMCPHTTP(t)
	var mode atomic.Int32
	var calls atomic.Int32
	base, policy, stop := faultMCP(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		tools := []any{map[string]any{"name": "first_tool", "description": "one", "inputSchema": map[string]any{"type": "object"}}, map[string]any{"name": "removed_tool", "description": "two", "inputSchema": map[string]any{"type": "object"}}}
		if mode.Load() == 1 {
			tools = tools[:1]
		}
		faultReply(w, r, tools)
	})
	defer stop()
	f.store.Policy = policy
	f.useStore(t)
	id := f.create(t, 1, "refresh", base, "")
	path := resource(id)
	t.Run("refresh_removes_tools", func(t *testing.T) {
		f.request(t, 1, "POST", path+"/connect", nil, 200)
		toolsCount(t, f.request(t, 1, "GET", path+"/tools", nil, 200), 2)
		mode.Store(1)
		f.request(t, 1, "POST", path+"/connect", nil, 200)
		toolsCount(t, f.request(t, 1, "GET", path+"/tools", nil, 200), 1)
		if calls.Load() != 4 {
			t.Fatal("GET contacted remote")
		}
	})
	t.Run("rejection_no_network_or_write", func(t *testing.T) {
		before := calls.Load()
		var revision int64
		f.db.QueryRow("SELECT revision FROM mcp_servers WHERE id=?", id).Scan(&revision)
		for _, user := range []int64{0, 2, 3, 4} {
			want := 403
			if user == 0 {
				want = 401
			}
			if user == 2 {
				want = 404
			}
			f.request(t, user, "POST", path+"/connect", nil, want)
			f.request(t, user, "PUT", path, map[string]any{"name": "override", "base_url": base, "auth_token": "", "class_id": 1}, want)
		}
		var after int64
		f.db.QueryRow("SELECT revision FROM mcp_servers WHERE id=?", id).Scan(&after)
		if revision != after || calls.Load() != before {
			t.Fatal("denial side effect")
		}
	})
	t.Run("publish_failure_503_no_partial_cache", func(t *testing.T) {
		if _, e := f.db.Exec("CREATE TRIGGER fixture_cache_failure BEFORE INSERT ON mcp_tools FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='synthetic failure'"); e != nil {
			t.Fatal(e)
		}
		defer f.db.Exec("DROP TRIGGER fixture_cache_failure")
		f.request(t, 1, "POST", path+"/connect", nil, 503)
		toolsCount(t, f.request(t, 1, "GET", path+"/tools", nil, 200), 0)
		v, e := f.store.Get(context.Background(), f.principal(t, 1), id)
		if e != nil || v.Connected {
			t.Fatal("false success")
		}
	})
	t.Run("parallel_disconnect_idempotent", func(t *testing.T) {
		f.request(t, 1, "POST", path+"/connect", nil, 200)
		var wg sync.WaitGroup
		for i := 0; i < 8; i++ {
			wg.Add(1)
			go func() { defer wg.Done(); f.request(t, 1, "POST", path+"/disconnect", nil, 200) }()
		}
		wg.Wait()
		toolsCount(t, f.request(t, 1, "GET", path+"/tools", nil, 200), 0)
	})
	t.Run("delete_audit_race_restrict", func(t *testing.T) {
		for i := 0; i < 20; i++ {
			id := f.create(t, 1, fmt.Sprintf("audit-race%d", i), base, "")
			p := f.principal(t, 1)
			var wg sync.WaitGroup
			var deleted bool
			var recorded bool
			var de, ae error
			wg.Add(2)
			go func() { defer wg.Done(); de = f.store.Delete(context.Background(), p, id); deleted = de == nil }()
			go func() {
				defer wg.Done()
				recorded, ae = f.store.AppendTerminal(context.Background(), p, fmt.Sprintf("audit-race%d", i), "formula_hint", "denied", "", nil)
			}()
			wg.Wait()
			if ae != nil {
				t.Fatal(ae)
			}
			if deleted == recorded {
				t.Fatal("delete and audit inconsistent")
			}
			if !deleted && de != repository.MCPUnavailable {
				t.Fatal(de)
			}
			var n int
			f.db.QueryRow("SELECT COUNT(*) FROM mcp_tool_calls c LEFT JOIN mcp_servers s ON s.id=c.server_id WHERE s.id IS NULL").Scan(&n)
			if n != 0 {
				t.Fatal("orphan audit")
			}
		}
	})
	t.Run("cancelled_HTTP_does_not_publish", func(t *testing.T) {
		began := make(chan struct{})
		var once sync.Once
		remote, p, closeRemote := faultMCP(t, func(w http.ResponseWriter, r *http.Request) { once.Do(func() { close(began) }); <-r.Context().Done() })
		defer closeRemote()
		f.store.Policy = p
		f.useStore(t)
		id := f.create(t, 1, "cancelled", remote, "")
		ctx, cancel := context.WithCancel(context.Background())
		req, _ := http.NewRequestWithContext(ctx, "POST", f.url+resource(id)+"/connect", nil)
		req.Header.Set("Authorization", "Bearer "+f.tokens[1])
		done := make(chan error, 1)
		go func() {
			r, e := f.client.Do(req)
			if r != nil {
				r.Body.Close()
			}
			done <- e
		}()
		select {
		case <-began:
		case <-time.After(4 * time.Second):
			t.Fatal("cancel fixture did not start")
		}
		cancel()
		select {
		case e := <-done:
			if e == nil {
				t.Fatal("cancellation ignored")
			}
		case <-time.After(time.Second):
			t.Fatal("cancel propagation slow")
		}
		v, e := f.store.Get(context.Background(), f.principal(t, 1), id)
		if e != nil || v.Connected || v.ToolCount != 0 {
			t.Fatal("cancelled publication")
		}
	})
}
