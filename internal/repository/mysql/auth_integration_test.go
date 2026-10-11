//go:build phase3integration

package mysql_test

import (
	"bytes"
	"campusclaw/internal/auth"
	"campusclaw/internal/config"
	httpapi "campusclaw/internal/http"
	"campusclaw/internal/provision"
	repository "campusclaw/internal/repository/mysql"
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	mysqldriver "github.com/go-sql-driver/mysql"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// Wrap real MySQL transactions to deterministically simulate a rejected commit.
// UPDATE executes in InnoDB, then is rolled back before Commit reports failure.
type rejectConnector struct{ driver.Connector }

func (c rejectConnector) Connect(ctx context.Context) (driver.Conn, error) {
	v, e := c.Connector.Connect(ctx)
	if e != nil {
		return nil, e
	}
	return rejectConn{v}, nil
}

type rejectConn struct{ driver.Conn }

func (c rejectConn) Begin() (driver.Tx, error) {
	v, e := c.Conn.Begin()
	if e != nil {
		return nil, e
	}
	return rejectTx{v}, nil
}
func (c rejectConn) BeginTx(ctx context.Context, o driver.TxOptions) (driver.Tx, error) {
	v, e := c.Conn.(driver.ConnBeginTx).BeginTx(ctx, o)
	if e != nil {
		return nil, e
	}
	return rejectTx{v}, nil
}

type rejectTx struct{ driver.Tx }

func (t rejectTx) Commit() error {
	if e := t.Tx.Rollback(); e != nil {
		return e
	}
	return errors.New("injected private commit failure")
}

func TestPhase3RealMySQL(t *testing.T) {
	if os.Getenv("PHASE3_ISOLATED") != "1" || !strings.HasPrefix(os.Getenv("DB_HOST"), "campusclaw-phase3-test-") {
		t.Fatal("isolated test environment required")
	}
	cfg, e := config.Load(os.Getenv)
	if e != nil {
		t.Fatal("test configuration failed")
	}
	db, e := repository.Open(cfg)
	if e != nil {
		t.Fatal("test database unavailable")
	}
	defer db.Close()
	ctx := context.Background()
	var tables int
	if e = db.QueryRow("SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=DATABASE()").Scan(&tables); e != nil || tables != 0 {
		t.Fatal("requires brand new empty database; no cleanup permitted")
	}
	if repository.PrepareSchema(ctx, db) != nil {
		t.Fatal("schema preparation failed")
	}
	root := t.TempDir()
	if _, e = provision.Fresh(ctx, db, root, func(string) string { return "synthetic-phase3-password" }); e != nil {
		t.Fatal("isolated provisioning failed")
	}
	var id int64
	if db.QueryRow("SELECT id FROM users WHERE username='teacher_a'").Scan(&id) != nil {
		t.Fatal("seed account missing")
	}
	secret := os.Getenv("JWT_SIGNING_SECRET")
	tokens, e := auth.NewTokens(secret, time.Now)
	if e != nil {
		t.Fatal("test key unavailable")
	}
	repo := repository.LoginUsers{DB: db}
	a := &auth.Identity{Store: repo, Tokens: tokens}
	unitHandler := func(store auth.IdentityStore) http.Handler {
		return httpapi.WithAuth(nil, nil, &auth.Identity{Store: store, Tokens: tokens})
	}
	callHandler := func(h http.Handler, method, path, token string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, nil)
		r.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	execSQL := func(query string, args ...any) {
		if _, e := db.Exec(query, args...); e != nil {
			t.Fatal("isolated state mutation failed")
		}
	}
	version := func() int64 {
		var v int64
		if db.QueryRow("SELECT session_version FROM users WHERE id=?", id).Scan(&v) != nil {
			t.Fatal("version read failed")
		}
		return v
	}
	// Run the actual Go executable, with the normal initialization gate and no SECRET_KEY.
	var process *exec.Cmd
	var output bytes.Buffer
	stop := func() {
		if process != nil {
			_ = process.Process.Kill()
			_ = process.Wait()
			process = nil
		}
	}
	defer stop()
	for _, missing := range []string{"JWT_SIGNING_SECRET", "DB_HOST"} {
		badCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		bad := exec.CommandContext(badCtx, os.Getenv("PHASE3_SERVER"))
		for _, entry := range os.Environ() {
			if !strings.HasPrefix(entry, missing+"=") && !strings.HasPrefix(entry, "SECRET_KEY=") && !strings.HasPrefix(entry, "STARTUP_MODE=") {
				bad.Env = append(bad.Env, entry)
			}
		}
		b, err := bad.CombinedOutput()
		cancel()
		if err == nil || !strings.Contains(string(b), missing) || strings.Contains(string(b), secret) {
			t.Fatal("missing config did not safely reject startup")
		}
	}
	start := func() {
		output.Reset()
		process = exec.Command(os.Getenv("PHASE3_SERVER"))
		for _, entry := range os.Environ() {
			if !strings.HasPrefix(entry, "SECRET_KEY=") && !strings.HasPrefix(entry, "UPLOAD_ROOT=") && !strings.HasPrefix(entry, "LISTEN_ADDR=") && !strings.HasPrefix(entry, "STARTUP_MODE=") {
				process.Env = append(process.Env, entry)
			}
		}
		process.Env = append(process.Env, "UPLOAD_ROOT="+root, "LISTEN_ADDR=127.0.0.1:18973", "STARTUP_MODE=migrate")
		process.Stdout = &output
		process.Stderr = &output
		if process.Start() != nil {
			t.Fatal("Go server start failed")
		}
		client := http.Client{Timeout: time.Second}
		for i := 0; i < 100; i++ {
			r, e := client.Get("http://127.0.0.1:18973/health")
			if e == nil {
				_ = r.Body.Close()
				if r.StatusCode == 200 {
					return
				}
			}
			time.Sleep(100 * time.Millisecond)
		}
		stop()
		t.Fatal("Go server did not become healthy (output withheld)")
	}
	client := http.Client{Timeout: 5 * time.Second}
	request := func(method, path, token, body string) (int, []byte) {
		r, _ := http.NewRequest(method, "http://127.0.0.1:18973"+path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		res, e := client.Do(r)
		if e != nil {
			t.Fatal("HTTP request failed")
		}
		defer res.Body.Close()
		b, _ := io.ReadAll(res.Body)
		if path != "/health" && (res.Header.Get("Cache-Control") != "no-store" || res.Header.Get("Set-Cookie") != "") {
			t.Fatal("auth response header mismatch")
		}
		return res.StatusCode, b
	}
	login := func() string {
		code, b := request("POST", "/api/auth/login", "", `{"username":"teacher_a","password":"synthetic-phase3-password"}`)
		var v auth.LoginResponse
		if code != 200 || json.Unmarshal(b, &v) != nil || v.AccessToken == "" {
			t.Fatal("real login failed")
		}
		return v.AccessToken
	}
	start()
	first, second := login(), login()
	if code, _ := request("GET", "/api/auth/me", first, ""); code != 200 {
		t.Fatal("real account query failed")
	}
	stop()
	start()
	if code, _ := request("GET", "/api/auth/me", first, ""); code != 200 {
		t.Fatal("valid token lost on process restart")
	}
	if code, b := request("POST", "/api/auth/logout", first, ""); code != 204 || len(b) != 0 {
		t.Fatal("real logout failed")
	}
	if version() != 1 {
		t.Fatal("revocation not committed")
	}
	stop()
	start()
	for _, token := range []string{first, second} {
		if code, _ := request("GET", "/api/auth/me", token, ""); code != 401 {
			t.Fatal("revoked token revived after process restart")
		}
	}
	fresh := login()
	if code, _ := request("GET", "/api/auth/me", fresh, ""); code != 200 {
		t.Fatal("new login rejected")
	}
	stop()
	for _, credential := range []string{secret, first, second, fresh, "synthetic-phase3-password"} {
		if strings.Contains(output.String(), credential) {
			t.Fatal("server logs leaked credential")
		}
	}
	// Live role/class/version changes are authoritative; signed claims never freeze scope.
	execSQL("UPDATE users SET role='student',class_id=(SELECT id FROM classes WHERE name='B') WHERE id=?", id)
	p, e := a.Authenticate(ctx, fresh)
	if e != nil || p.Role() != "student" || p.ClassName() != "B" {
		t.Fatal("live role/class not applied")
	}
	guard := httpapi.Authenticate(a, httpapi.TeacherRequired(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Fatal("student reached teacher action") })))
	if callHandler(guard, "POST", "/test-only?role=teacher&class_id=1", fresh).Code != 403 {
		t.Fatal("student elevation")
	}
	execSQL("UPDATE users SET session_version=-1 WHERE id=?", id)
	if _, e = a.Authenticate(ctx, fresh); e != auth.ErrInvalidToken {
		t.Fatal("invalid account state accepted")
	}
	execSQL("UPDATE users SET session_version=1 WHERE id=?", id)
	// Commit failure path executes a real UPDATE then rollback; DB value must remain 1.
	dc := mysqldriver.NewConfig()
	dc.User = cfg.DBUser
	dc.Passwd = cfg.DBPassword
	dc.Net = "tcp"
	dc.Addr = cfg.DBHost + ":" + cfg.DBPort
	dc.DBName = cfg.DBName
	connector, e := mysqldriver.NewConnector(dc)
	if e != nil {
		t.Fatal("connector failed")
	}
	failingDB := sql.OpenDB(rejectConnector{connector})
	defer failingDB.Close()
	failure := callHandler(unitHandler(repository.LoginUsers{DB: failingDB}), "POST", "/api/auth/logout", fresh)
	if failure.Code != 500 || version() != 1 || strings.Contains(failure.Body.String(), "private") {
		t.Fatal("failed commit did not roll back safely")
	}
	// Real MySQL update error (BIGINT overflow) must also rollback and reject success.
	execSQL("UPDATE users SET session_version=9223372036854775807 WHERE id=?", id)
	if repo.Revoke(ctx, id) != auth.ErrStorageUnavailable || version() != 9223372036854775807 {
		t.Fatal("update failure not rolled back")
	}
	execSQL("UPDATE users SET session_version=2 WHERE id=?", id)
	if _, e = a.Authenticate(ctx, fresh); e != auth.ErrInvalidToken {
		t.Fatal("changed version accepted")
	}
	// Delete a new account with no material references; never delete a database/table.
	deletedID, e := provision.CreateUser(ctx, db, "delete-fixture", "synthetic-phase3-password", "student", p.ClassID())
	if e != nil {
		t.Fatal("account fixture failed")
	}
	deletedToken, _ := tokens.Issue(deletedID, 0)
	execSQL("DELETE FROM users WHERE id=?", deletedID)
	if _, e = a.Authenticate(ctx, deletedToken); e != auth.ErrInvalidToken {
		t.Fatal("deleted user accepted")
	}
	var sessionCount int
	if db.QueryRow("SELECT COUNT(*) FROM sessions").Scan(&sessionCount) != nil || sessionCount != 0 {
		t.Fatal("authentication created sessions")
	}
	_ = failingDB.Close()
	if callHandler(unitHandler(repository.LoginUsers{DB: failingDB}), "GET", "/api/auth/me", fresh).Code != 500 {
		t.Fatal("query failure not 500")
	}
	t.Log("PASS real MySQL login, process restart, all-device revocation, current role/class, invalid/deleted account, update failure, commit rollback, query failure, no sessions")
}
