//go:build phase4readintegration

package mysql_test

import (
	"bytes"
	"campusclaw/internal/auth"
	"campusclaw/internal/config"
	"campusclaw/internal/health"
	httpapi "campusclaw/internal/http"
	"campusclaw/internal/material"
	"campusclaw/internal/provision"
	"campusclaw/internal/recovery"
	repository "campusclaw/internal/repository/mysql"
	"campusclaw/internal/storage"
	"context"
	"encoding/json"
	"fmt"
	"mime"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPhase4ReadRecoveryRealMySQL(t *testing.T) {
	if os.Getenv("PHASE4_READ_ISOLATED") != "1" || !strings.HasPrefix(os.Getenv("DB_HOST"), "campusclaw-phase4-read-") {
		t.Fatal("new isolated database required")
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
	if _, e := provision.Fresh(ctx, db, root, func(string) string { return "isolated-read-fixture-password" }); e != nil {
		t.Fatal("fresh")
	}
	files, e := storage.OpenPrivate(root)
	if e != nil {
		t.Fatal("store")
	}
	defer files.Close()
	repo := repository.Materials{DB: db}
	tokens, _ := auth.NewTokens(os.Getenv("JWT_SIGNING_SECRET"), time.Now)
	users := repository.LoginUsers{DB: db}
	identity := &auth.Identity{Store: users, Tokens: tokens}
	login := &auth.Login{Users: users, Tokens: tokens}
	type actor struct {
		p     auth.Principal
		token string
	}
	actors := map[string]actor{}
	for _, name := range []string{"teacher_a", "student_a1", "teacher_b", "student_b1"} {
		response, e := login.Authenticate(ctx, name, "isolated-read-fixture-password")
		if e != nil {
			t.Fatal("fixture login")
		}
		p, e := identity.Authenticate(ctx, response.AccessToken)
		if e != nil {
			t.Fatal("principal")
		}
		actors[name] = actor{p, response.AccessToken}
	}
	h := httpapi.WithMaterials(health.Service{DB: db, Probe: func() error { return storage.Probe(root) }}, login, identity, &httpapi.UploadHandler{Store: files, Uploader: material.Uploader{Repository: repo}}, &httpapi.MaterialReads{Store: files, Reader: material.Reader{Repository: repo}})
	request := func(method, path, token string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, nil)
		r.Header.Set("Authorization", "Bearer "+token)
		r.Header.Set("X-Class-Id", "999")
		r.Header.Set("Cookie", "class_id=999; role=teacher")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	upload := func(name, text string) string {
		u, e := files.Prepare("new.md", strings.NewReader(text), 1024)
		if e != nil {
			t.Fatal("prepare")
		}
		defer u.Close()
		id, e := (material.Uploader{Repository: repo, Now: func() time.Time { return time.Date(2099, 10, 9, 0, 0, 0, 0, time.UTC) }}).Save(ctx, actors[name].p, u)
		if e != nil {
			t.Fatal("save")
		}
		return id
	}
	raw := "\ufeff# 原文\n<script>不渲染</script>"
	a1 := upload("teacher_a", raw)
	a2 := upload("teacher_a", "same timestamp")
	b1 := upload("teacher_b", "B-only")
	if _, e = db.Exec("UPDATE materials SET original_name=? WHERE id=?", "中文材料.md", a1); e != nil {
		t.Fatal("Unicode fixture")
	}
	t.Run("both-classes-both-roles-and-read-only", func(t *testing.T) {
		for name, a := range actors {
			own, foreign := a1, b1
			if strings.Contains(name, "_b") {
				own, foreign = b1, a1
			}
			items, e := repo.List(ctx, a.p.ClassID())
			if e != nil || len(items) < 2 {
				t.Fatal("repository list")
			}
			for i, v := range items {
				if v.ID == foreign {
					t.Fatal("foreign list")
				}
				if i > 0 && (items[i-1].CreatedAt < v.CreatedAt || items[i-1].CreatedAt == v.CreatedAt && items[i-1].ID < v.ID) {
					t.Fatal("sort")
				}
			}
			if own == a1 {
				expected := a1
				if a2 > a1 {
					expected = a2
				}
				if items[0].ID != expected {
					t.Fatal("tie order")
				}
			}
			w := request("GET", "/api/materials", a.token)
			var listing struct {
				Items []map[string]any `json:"items"`
			}
			json.Unmarshal(w.Body.Bytes(), &listing)
			if w.Code != 200 || len(listing.Items) != len(items) || len(listing.Items[0]) != 5 {
				t.Fatal("list fields")
			}
			if request("GET", "/api/materials?class_id=999", a.token).Code != 400 {
				t.Fatal("client scope")
			}
			for _, suffix := range []string{"", "/file"} {
				w := request("GET", "/api/materials/"+own+suffix, a.token)
				if w.Code != 200 {
					t.Fatal("same class read")
				}
				denied := request("GET", "/api/materials/"+foreign+suffix+"?class_id=999", a.token)
				missing := request("GET", "/api/materials/absent"+suffix, a.token)
				if denied.Code != 404 || missing.Code != 404 || denied.Body.String() != missing.Body.String() {
					t.Fatal("cross-class disclosure")
				}
			}
			if a.p.Role() == "student" {
				if request("POST", "/api/materials", a.token).Code != 403 {
					t.Fatal("student write")
				}
				for _, method := range []string{"PUT", "PATCH", "DELETE"} {
					if request(method, "/api/materials/"+own, a.token).Code != 405 {
						t.Fatal("mutation enabled")
					}
				}
			}
		}
		w := request("GET", "/api/materials/"+a1, actors["student_a1"].token)
		var detail map[string]any
		json.Unmarshal(w.Body.Bytes(), &detail)
		if len(detail) != 6 || detail["plain_text"] != strings.TrimPrefix(raw, "\ufeff") {
			t.Fatal("detail content")
		}
		w = request("GET", "/api/materials/"+a1+"/file", actors["student_a1"].token)
		kind, params, e := mime.ParseMediaType(w.Header().Get("Content-Disposition"))
		if w.Body.String() != raw || kind != "attachment" || e != nil || params["filename"] != "中文材料.md" || w.Header().Get("Cache-Control") != "private, no-store" || w.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Fatal("raw download/header")
		}
		if _, e := db.Exec("INSERT INTO classes(name) VALUES ('Empty read test')"); e != nil {
			t.Fatal("empty class")
		}
		var empty int64
		db.QueryRow("SELECT id FROM classes WHERE name='Empty read test'").Scan(&empty)
		items, e := repo.List(ctx, empty)
		if e != nil || items == nil || len(items) != 0 {
			t.Fatal("empty repository")
		}
		// Current identity takes effect immediately with the same token.
		student := actors["student_a1"]
		db.Exec("UPDATE users SET class_id=? WHERE id=?", empty, student.p.UserID())
		w = request("GET", "/api/materials", student.token)
		db.Exec("UPDATE users SET class_id=? WHERE id=?", student.p.ClassID(), student.p.UserID())
		if w.Code != 200 || strings.TrimSpace(w.Body.String()) != `{"items":[]}` {
			t.Fatal("current empty class")
		}
	})
	marker := filepath.Join(root, "tmp", "interrupted")
	orphan := filepath.Join(root, "files", "uncommitted")
	evidence := func() {
		if os.WriteFile(marker, []byte("pending"), 0600) != nil || os.WriteFile(orphan, []byte("pending"), 0600) != nil {
			t.Fatal("evidence")
		}
	}
	preserved := func() {
		t.Helper()
		for _, p := range []string{marker, orphan} {
			if b, e := os.ReadFile(p); e != nil || string(b) != "pending" {
				t.Fatal("evidence removed on failed recovery")
			}
		}
	}
	t.Run("unverified-and-unavailable-preserve", func(t *testing.T) {
		evidence()
		closed, _ := repository.Open(cfg)
		closed.Close()
		if recovery.Startup(ctx, closed, root) == nil {
			t.Fatal("closed database allowed")
		}
		preserved()
		unverified := t.TempDir()
		os.Mkdir(filepath.Join(unverified, "files"), 0700)
		os.Mkdir(filepath.Join(unverified, "tmp"), 0700)
		os.WriteFile(filepath.Join(unverified, "files", "proof"), []byte("proof"), 0600)
		if recovery.Startup(ctx, db, unverified) == nil {
			t.Fatal("unverified root allowed")
		}
		if _, e := os.Stat(filepath.Join(unverified, "files", "proof")); e != nil {
			t.Fatal("unverified cleaned")
		}
	})
	t.Run("incomplete-and-unsafe-preserve", func(t *testing.T) {
		for _, mode := range []string{"missing-file", "missing-text", "wrong-text", "bad-key", "symlink", "unexpected-directory"} {
			t.Run(mode, func(t *testing.T) {
				evidence()
				path := filepath.Join(root, "files", a1)
				var undo func()
				switch mode {
				case "missing-file":
					os.Rename(path, path+".saved")
					undo = func() { os.Rename(path+".saved", path) }
				case "missing-text":
					var id int64
					var created string
					db.QueryRow("SELECT id,created_at FROM knowledge_entries WHERE material_id=?", a1).Scan(&id, &created)
					db.Exec("DELETE FROM knowledge_entries WHERE material_id=?", a1)
					undo = func() {
						db.Exec("INSERT INTO knowledge_entries(id,material_id,class_id,plain_text,created_at) VALUES (?,?,?,?,?)", id, a1, actors["teacher_a"].p.ClassID(), strings.TrimPrefix(raw, "\ufeff"), created)
					}
				case "wrong-text":
					db.Exec("UPDATE knowledge_entries SET plain_text='damaged' WHERE material_id=?", a1)
					undo = func() {
						db.Exec("UPDATE knowledge_entries SET plain_text=? WHERE material_id=?", strings.TrimPrefix(raw, "\ufeff"), a1)
					}
				case "bad-key":
					db.Exec("UPDATE materials SET storage_key='../escape' WHERE id=?", a1)
					undo = func() { db.Exec("UPDATE materials SET storage_key=? WHERE id=?", a1, a1) }
				case "symlink":
					os.Symlink(marker, filepath.Join(root, "files", "link"))
					undo = func() { os.Remove(filepath.Join(root, "files", "link")) }
				case "unexpected-directory":
					os.Mkdir(filepath.Join(root, "files", "directory"), 0700)
					undo = func() { os.Remove(filepath.Join(root, "files", "directory")) }
				}
				defer undo()
				if recovery.Startup(ctx, db, root) == nil {
					t.Fatal("unsafe startup allowed")
				}
				preserved()
				if mode == "bad-key" || mode == "missing-file" {
					if request("GET", "/api/materials/"+a1+"/file", actors["teacher_a"].token).Code != 500 {
						t.Fatal("file failure success")
					}
				}
			})
		}
	})
	t.Run("in-flight-unknown-commit-and-rollback", func(t *testing.T) {
		for _, commit := range []bool{true, false} {
			u, e := files.Prepare("unknown.txt", strings.NewReader("unknown evidence"), 100)
			if e != nil {
				t.Fatal("prepare")
			}
			tx, e := repo.Begin(ctx)
			if e != nil {
				t.Fatal("transaction")
			}
			p := actors["teacher_a"].p
			if tx.Insert(material.Record{UploadInfo: u.Info(), ClassID: p.ClassID(), UploaderID: p.UserID(), CreatedAt: "2026-10-09T00:00:00.000Z"}) != nil || u.Publish() != nil {
				t.Fatal("pending transaction")
			}
			evidence()
			bounded, cancel := context.WithTimeout(ctx, 150*time.Millisecond)
			err := recovery.Startup(bounded, db, root)
			cancel()
			if err == nil {
				t.Fatal("unresolved transaction allowed cleanup")
			}
			preserved()
			if _, err := os.Stat(filepath.Join(root, "files", u.Info().ID)); err != nil {
				t.Fatal("unknown transaction evidence deleted")
			}
			done := make(chan error, 1)
			go func() { done <- recovery.Startup(ctx, db, root) }()
			select {
			case <-done:
				t.Fatal("did not wait for unresolved transaction")
			case <-time.After(250 * time.Millisecond):
			}
			if commit {
				e = tx.Commit()
			} else {
				e = tx.Rollback()
			}
			if e != nil {
				t.Fatal("transaction completion")
			}
			if e = <-done; e != nil {
				t.Fatal("recovery after transaction")
			}
			_, e = os.Stat(filepath.Join(root, "files", u.Info().ID))
			if commit && e != nil || !commit && !os.IsNotExist(e) {
				t.Fatal("unknown outcome cleanup")
			}
			u.Close()
		}
	})
	t.Run("idempotent-cleanup-and-real-server-startup", func(t *testing.T) {
		evidence()
		if recovery.Startup(ctx, db, root) != nil || recovery.Startup(ctx, db, root) != nil {
			t.Fatal("valid recovery")
		}
		for _, path := range []string{marker, orphan} {
			if _, e := os.Stat(path); !os.IsNotExist(e) {
				t.Fatal("orphan not cleaned")
			}
		}
		if b, e := os.ReadFile(filepath.Join(root, "files", a1)); e != nil || string(b) != raw {
			t.Fatal("committed file deleted")
		}
		executable := os.Getenv("PHASE4_READ_SERVER")
		if executable == "" {
			t.Fatal("real server required")
		}
		evidence()
		listener, e := net.Listen("tcp", "127.0.0.1:0")
		if e != nil {
			t.Fatal("port")
		}
		address := listener.Addr().String()
		listener.Close()
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
			if strings.Contains(logs.String(), os.Getenv("JWT_SIGNING_SECRET")) {
				t.Error("secret logged")
			}
		}()
		client := http.Client{Timeout: time.Second}
		ready := false
		for i := 0; i < 80; i++ {
			response, e := client.Get("http://" + address + "/health")
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
		for _, path := range []string{marker, orphan} {
			if _, e := os.Stat(path); !os.IsNotExist(e) {
				t.Fatal("listener before recovery")
			}
		}
		r, _ := http.NewRequest("GET", "http://"+address+"/api/materials/"+a1, nil)
		r.Header.Set("Authorization", "Bearer "+actors["student_a1"].token)
		response, e := client.Do(r)
		if e != nil {
			t.Fatal("server read")
		}
		response.Body.Close()
		if response.StatusCode != 200 {
			t.Fatal("server read route")
		}
	})
	t.Run("read-database-fault", func(t *testing.T) {
		closed, _ := repository.Open(cfg)
		closed.Close()
		failed := repository.Materials{DB: closed}
		if _, e := failed.List(ctx, actors["teacher_a"].p.ClassID()); e != material.ErrUnavailable {
			t.Fatal("query failure")
		}
		faultHandler := httpapi.WithMaterials(health.Service{DB: db}, nil, identity, nil, &httpapi.MaterialReads{Reader: material.Reader{Repository: failed}, Store: files})
		for _, path := range []string{"/api/materials", "/api/materials/" + a1, "/api/materials/" + a1 + "/file"} {
			r := httptest.NewRequest("GET", path, nil)
			r.Header.Set("Authorization", "Bearer "+actors["teacher_a"].token)
			w := httptest.NewRecorder()
			faultHandler.ServeHTTP(w, r)
			if w.Code != 500 {
				t.Fatal("fault success")
			}
		}
	})
	fmt.Println("PASS scoped reads, original downloads, actual startup, unresolved transactions, complete preflight and preserved evidence")
}
