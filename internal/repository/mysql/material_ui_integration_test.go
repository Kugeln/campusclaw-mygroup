//go:build phase4uiintegration

package mysql_test

import (
	"bytes"
	"campusclaw/internal/auth"
	"campusclaw/internal/config"
	"campusclaw/internal/provision"
	repository "campusclaw/internal/repository/mysql"
	"campusclaw/internal/transfer"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPhase4UIRealMySQL(t *testing.T) {
	if os.Getenv("PHASE4_UI_ISOLATED") != "1" || os.Getenv("DB_HOST") != "127.0.0.1" || !strings.HasPrefix(os.Getenv("DB_NAME"), "phase4_ui_") {
		t.Fatal("new isolated loopback database required")
	}
	cfg, e := config.Load(os.Getenv)
	if e != nil {
		t.Fatal("configuration")
	}
	db, e := repository.Open(cfg)
	if e != nil {
		t.Fatal("database")
	}
	defer db.Close()
	ctx := context.Background()
	var count int
	if db.QueryRow("SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=DATABASE()").Scan(&count) != nil || count != 0 {
		t.Fatal("refuse nonempty database")
	}
	if repository.PrepareSchema(ctx, db) != nil {
		t.Fatal("schema")
	}
	root := t.TempDir()
	password := os.Getenv("PHASE4_UI_PASSWORD")
	if len(password) < 24 {
		t.Fatal("generated fixture password required")
	}
	if _, e := provision.Fresh(ctx, db, root, func(string) string { return password }); e != nil {
		t.Fatal("fresh")
	}
	listener, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal("port")
	}
	address := listener.Addr().String()
	listener.Close()
	executable := os.Getenv("PHASE4_UI_SERVER")
	if executable == "" {
		t.Fatal("real server required")
	}
	command := exec.Command(executable)
	command.Env = append(os.Environ(), "UPLOAD_ROOT="+root, "LISTEN_ADDR="+address, "STARTUP_MODE=")
	var logs bytes.Buffer
	command.Stdout = &logs
	command.Stderr = &logs
	if command.Start() != nil {
		t.Fatal("server start")
	}
	defer func() {
		command.Process.Kill()
		command.Wait()
		for _, s := range []string{password, os.Getenv("JWT_SIGNING_SECRET")} {
			if strings.Contains(logs.String(), s) {
				t.Error("sensitive server output")
			}
		}
	}()
	origin := "http://" + address
	client := http.Client{Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	ready := false
	for i := 0; i < 100; i++ {
		response, e := client.Get(origin + "/health")
		if e == nil {
			response.Body.Close()
			if response.StatusCode == 200 {
				ready = true
				break
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !ready {
		t.Fatal("server not ready")
	}
	t.Log("isolated Go listener: " + origin)
	type result struct {
		code   int
		body   []byte
		header http.Header
	}
	request := func(method, path, token, kind string, body []byte) result {
		t.Helper()
		r, _ := http.NewRequest(method, origin+path, bytes.NewReader(body))
		r.Header.Set("Content-Type", kind)
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		r.Header.Set("X-Role", "teacher")
		r.Header.Set("X-Class-Id", "999")
		r.Header.Set("X-User-Id", "999")
		r.Header.Set("Cookie", "role=teacher; class_id=999; uploader_id=999; session=obsolete")
		response, e := client.Do(r)
		if e != nil {
			t.Fatal("HTTP request failed")
		}
		defer response.Body.Close()
		b, e := io.ReadAll(response.Body)
		if e != nil {
			t.Fatal("response read")
		}
		if response.Header.Get("Set-Cookie") != "" {
			t.Fatal("cookie introduced")
		}
		return result{response.StatusCode, b, response.Header}
	}
	upload := func(token, name string, data []byte, fields map[string]string, files int) result {
		var b bytes.Buffer
		mw := multipart.NewWriter(&b)
		for k, v := range fields {
			mw.WriteField(k, v)
		}
		for i := 0; i < files; i++ {
			p, _ := mw.CreateFormFile("file", name)
			p.Write(data)
		}
		mw.Close()
		return request("POST", "/api/materials", token, mw.FormDataContentType(), b.Bytes())
	}
	type actor struct {
		token     string
		id, class int64
		role      string
	}
	actors := map[string]actor{}
	for _, name := range []string{"teacher_a", "student_a1", "teacher_b", "student_b1"} {
		b, _ := json.Marshal(map[string]string{"username": name, "password": password, "role": "teacher", "class_id": "999", "uploader_id": "999"})
		r := request("POST", "/api/auth/login", "", "application/json", b)
		var v auth.LoginResponse
		// Decode without printing credentials or the login response on any failure.
		if r.code != 200 || json.Unmarshal(r.body, &v) != nil {
			t.Fatal("login contract")
		}
		me := request("GET", "/api/auth/me", v.AccessToken, "", nil)
		var p struct {
			ID    int64  `json:"user_id"`
			Class int64  `json:"class_id"`
			Role  string `json:"role"`
		}
		json.Unmarshal(me.body, &p)
		if me.code != 200 || p.Class == 999 || p.ID == 999 || (strings.HasPrefix(name, "student") && p.Role != "student") {
			t.Fatal("forged login identity")
		}
		actors[name] = actor{v.AccessToken, p.ID, p.Class, p.Role}
	}
	snapshot := func() string {
		t.Helper()
		s, e := transfer.Read(ctx, db)
		if e != nil || s.Validate(root) != nil {
			t.Fatal("snapshot failed")
		}
		r := s.Report("fresh", true)
		// Include all private files, including unreferenced/staging files.
		all := r.Fingerprint
		filepath.WalkDir(root, func(path string, d os.DirEntry, e error) error {
			if e != nil {
				t.Fatal("snapshot traversal")
			}
			if !d.IsDir() {
				b, e := os.ReadFile(path)
				if e != nil {
					t.Fatal("snapshot read")
				}
				all += path + transfer.Hash(b)
			}
			return nil
		})
		return transfer.Hash([]byte(all))
	}
	ids := map[string]string{}
	raw := []byte("\xef\xbb\xbf中文材料\n<script>window.pwned=true</script>")
	t.Run("teacher-uploads-and-bidirectional-reads", func(t *testing.T) {
		for _, group := range []string{"a", "b"} {
			r := upload(actors["teacher_"+group].token, "lesson-"+group+".md", raw, nil, 1)
			var v map[string]string
			json.Unmarshal(r.body, &v)
			if r.code != 201 || len(v) != 1 || v["material_id"] == "" {
				t.Fatal("upload")
			}
			ids[group] = v["material_id"]
		}
		before := snapshot()
		for name, a := range actors {
			group, other := "a", "b"
			if strings.Contains(name, "_b") {
				group, other = "b", "a"
			}
			r := request("GET", "/api/materials", a.token, "", nil)
			var list struct {
				Items []map[string]any `json:"items"`
			}
			json.Unmarshal(r.body, &list)
			own := false
			for _, item := range list.Items {
				if item["id"] == ids[other] {
					t.Fatal("foreign list")
				}
				if item["id"] == ids[group] {
					own = true
				}
				if len(item) != 5 {
					t.Fatal("list fields")
				}
			}
			if r.code != 200 || !own {
				t.Fatal("list")
			}
			for _, suffix := range []string{"", "/file"} {
				r := request("GET", "/api/materials/"+ids[group]+suffix, a.token, "", nil)
				if r.code != 200 {
					t.Fatal("own read")
				}
				if suffix == "/file" {
					if !bytes.Equal(r.body, raw) || r.header.Get("Cache-Control") != "private, no-store" || r.header.Get("X-Content-Type-Options") != "nosniff" || !strings.HasPrefix(r.header.Get("Content-Disposition"), "attachment;") {
						t.Fatal("download safety")
					}
				} else {
					var d map[string]any
					json.Unmarshal(r.body, &d)
					if len(d) != 6 || d["plain_text"] != strings.TrimPrefix(string(raw), "\ufeff") {
						t.Fatal("detail")
					}
				}
				for _, query := range []string{"", "?role=teacher&class_id=999&uploader_id=999"} {
					foreign := request("GET", "/api/materials/"+ids[other]+suffix+query, a.token, "", nil)
					absent := request("GET", "/api/materials/absent"+suffix, a.token, "", nil)
					if foreign.code != 404 || absent.code != 404 || !bytes.Equal(foreign.body, absent.body) {
						t.Fatal("foreign disclosure")
					}
				}
			}
		}
		if snapshot() != before {
			t.Fatal("reads changed data")
		}
	})
	t.Run("role-forgery-input-and-method-matrix", func(t *testing.T) {
		before := snapshot()
		for _, a := range actors {
			want := 400
			if a.role == "student" {
				want = 403
				if upload(a.token, "a.txt", []byte("text"), nil, 1).code != 403 {
					t.Fatal("student upload")
				}
			}
			for _, field := range []string{"role", "class_id", "uploader_id", "user_id"} {
				if upload(a.token, "a.txt", []byte("text"), map[string]string{field: "999"}, 1).code != want {
					t.Fatal("forged field")
				}
				if request("GET", "/api/materials?"+field+"=999", a.token, "", nil).code != 400 {
					t.Fatal("list query")
				}
			}
			for _, method := range []string{"PUT", "PATCH", "DELETE"} {
				for _, path := range []string{"/api/materials", "/api/materials/" + ids["a"], "/api/materials/" + ids["a"] + "/file"} {
					if request(method, path, a.token, "application/json", []byte(`{"role":"teacher"}`)).code != 405 {
						t.Fatal("method accepted")
					}
				}
			}
			for _, path := range []string{"/api/materials/" + ids["a"], "/api/materials/" + ids["a"] + "/file"} {
				if request("POST", path, a.token, "", nil).code != 405 {
					t.Fatal("POST mutation")
				}
			}
		}
		teacher := actors["teacher_a"].token
		for _, tc := range []struct {
			name        string
			data        []byte
			files, code int
		}{{"x.pdf", []byte("text"), 1, 400}, {"x.txt", nil, 1, 400}, {"x.txt", []byte{255}, 1, 400}, {"x.md", []byte("x\x00y"), 1, 400}, {"x.txt", []byte("text"), 0, 400}, {"x.txt", []byte("text"), 2, 400}, {"x.txt", bytes.Repeat([]byte("x"), 20*1024*1024+1), 1, 413}} {
			if upload(teacher, tc.name, tc.data, nil, tc.files).code != tc.code {
				t.Fatal("invalid file status")
			}
		}
		if request("POST", "/api/materials?class_id=999", teacher, "application/json", []byte(`{}`)).code != 400 {
			t.Fatal("upload query")
		}
		if snapshot() != before {
			t.Fatal("denied requests changed data/files")
		}
	})
	t.Run("missing-invalid-expired-revoked-bearer", func(t *testing.T) {
		a := actors["teacher_a"]
		old, _ := auth.NewTokens(os.Getenv("JWT_SIGNING_SECRET"), func() time.Time { return time.Now().Add(-9 * time.Hour) })
		expired, _ := old.Issue(a.id, 0)
		wrong, _ := auth.NewTokens("wrong-fixture-signing-key-more-than-32-bytes", time.Now)
		badSignature, _ := wrong.Issue(a.id, 0)
		before := snapshot()
		check := func(token string) {
			for _, path := range []string{"/api/auth/me", "/api/materials", "/api/materials/" + ids["a"], "/api/materials/" + ids["a"] + "/file"} {
				r := request("GET", path, token, "", nil)
				if r.code != 401 || r.header.Get("WWW-Authenticate") != "Bearer" {
					t.Fatal("Bearer rejection")
				}
			}
			if upload(token, "denied.txt", []byte("text"), nil, 1).code != 401 {
				t.Fatal("anonymous upload")
			}
		}
		for _, token := range []string{"", "not-a-jwt", "a.b.c", expired, badSignature, a.token + "tampered"} {
			check(token)
		}
		if snapshot() != before {
			t.Fatal("invalid token side effects")
		}
		if request("POST", "/api/auth/logout", a.token, "", nil).code != 204 {
			t.Fatal("logout")
		}
		before = snapshot()
		check(a.token)
		if snapshot() != before {
			t.Fatal("revoked token side effects")
		}
	})
	t.Run("page-static-and-browser-workflows", func(t *testing.T) {
		for _, path := range []string{"/login", "/materials", "/static/styles.css", "/static/logo.png", "/static/api.js", "/static/login.js", "/static/materials.js"} {
			r := request("GET", path, "", "", nil)
			if r.code != 200 {
				t.Fatal("page/static route")
			}
		}
		for _, path := range []string{"/knowledge", "/static/knowledge.js", "/uploads/files/" + ids["a"], "/static/uploads/files/" + ids["a"]} {
			if request("GET", path, "", "", nil).code != 404 {
				t.Fatal("out-of-scope route")
			}
		}
		// Historical Unicode and hostile display metadata belong to data, not markup.
		if _, e := db.Exec("UPDATE materials SET original_name=? WHERE id=?", "中文材料-<img src=x onerror=window.pwned=true>.md", ids["a"]); e != nil {
			t.Fatal("metadata fixture")
		}
		python, script := os.Getenv("PHASE4_UI_PYTHON"), os.Getenv("PHASE4_UI_BROWSER_SCRIPT")
		if python == "" || script == "" {
			t.Fatal("real browser required")
		}
		browser := exec.Command(python, script)
		browser.Env = append(os.Environ(), "PHASE4_UI_ORIGIN="+origin, "PHASE4_UI_MATERIAL_A="+ids["a"])
		output, e := browser.CombinedOutput()
		for _, s := range []string{password, os.Getenv("JWT_SIGNING_SECRET")} {
			if bytes.Contains(output, []byte(s)) {
				t.Fatal("sensitive browser output suppressed")
			}
		}
		if e != nil {
			t.Fatalf("browser workflow failed: %s", output)
		}
		t.Log(strings.TrimSpace(string(output)))
	})
}
