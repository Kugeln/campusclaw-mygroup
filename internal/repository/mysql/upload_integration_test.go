//go:build phase4uploadintegration

package mysql_test

import (
	"bytes"
	"campusclaw/internal/auth"
	"campusclaw/internal/config"
	"campusclaw/internal/health"
	httpapi "campusclaw/internal/http"
	"campusclaw/internal/material"
	"campusclaw/internal/provision"
	repository "campusclaw/internal/repository/mysql"
	"campusclaw/internal/storage"
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	mysqldriver "github.com/go-sql-driver/mysql"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// Real InnoDB commits/rollbacks with injected lost replies. The delayed variant
// leaves the insertion transaction open briefly, exercising the locking read.
type outcomeConnector struct {
	driver.Connector
	mode string
}

func (c outcomeConnector) Connect(ctx context.Context) (driver.Conn, error) {
	v, e := c.Connector.Connect(ctx)
	if e != nil {
		return nil, e
	}
	return outcomeConn{v, c.mode}, nil
}

type outcomeConn struct {
	driver.Conn
	mode string
}

func (c outcomeConn) Begin() (driver.Tx, error) {
	v, e := c.Conn.Begin()
	if e != nil {
		return nil, e
	}
	return outcomeTx{v, c.mode}, nil
}
func (c outcomeConn) BeginTx(ctx context.Context, o driver.TxOptions) (driver.Tx, error) {
	v, e := c.Conn.(driver.ConnBeginTx).BeginTx(ctx, o)
	if e != nil {
		return nil, e
	}
	return outcomeTx{v, c.mode}, nil
}

type outcomeTx struct {
	driver.Tx
	mode string
}

func (t outcomeTx) Commit() error {
	switch t.mode {
	case "rollback":
		if e := t.Tx.Rollback(); e != nil {
			return e
		}
	case "delay":
		go func() { time.Sleep(200 * time.Millisecond); _ = t.Tx.Commit() }()
	default:
		if e := t.Tx.Commit(); e != nil {
			return e
		}
	}
	return errors.New("synthetic lost commit reply")
}

type missingSource struct{ *storage.Upload }

func (f missingSource) Publish() error { _ = f.Upload.Close(); return f.Upload.Publish() }

type corruptPublished struct {
	*storage.Upload
	root string
}

func (f corruptPublished) Publish() error {
	if e := f.Upload.Publish(); e != nil {
		return e
	}
	return os.WriteFile(filepath.Join(f.root, "files", f.Info().ID), []byte("damaged"), 0600)
}

func TestPhase4UploadRealMySQL(t *testing.T) {
	if os.Getenv("PHASE4_UPLOAD_ISOLATED") != "1" || !strings.HasPrefix(os.Getenv("DB_HOST"), "campusclaw-phase4-upload-") {
		t.Fatal("isolated database required")
	}
	cfg, e := config.Load(os.Getenv)
	if e != nil {
		t.Fatal("configuration failed")
	}
	db, e := repository.Open(cfg)
	if e != nil {
		t.Fatal("database unavailable")
	}
	defer db.Close()
	ctx := context.Background()
	var n int
	if db.QueryRow("SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=DATABASE()").Scan(&n) != nil || n != 0 {
		t.Fatal("requires new empty database; refuses cleanup")
	}
	if repository.PrepareSchema(ctx, db) != nil {
		t.Fatal("schema failed")
	}
	root := t.TempDir()
	if _, e := provision.Fresh(ctx, db, root, func(string) string { return "upload-fixture-password" }); e != nil {
		t.Fatal("fake fresh data failed")
	}
	files, e := storage.OpenPrivate(root)
	if e != nil {
		t.Fatal("private store failed")
	}
	defer files.Close()
	tokens, e := auth.NewTokens(os.Getenv("JWT_SIGNING_SECRET"), time.Now)
	if e != nil {
		t.Fatal("signing setup failed")
	}
	users := repository.LoginUsers{DB: db}
	identity := &auth.Identity{Store: users, Tokens: tokens}
	login := &auth.Login{Users: users, Tokens: tokens}
	response, e := login.Authenticate(ctx, "teacher_a", "upload-fixture-password")
	if e != nil {
		t.Fatal("login failed")
	}
	principal, e := identity.Authenticate(ctx, response.AccessToken)
	if e != nil {
		t.Fatal("trusted principal failed")
	}
	reconnect := func() (*sql.DB, error) { return repository.Open(cfg) }
	baseRepo := repository.Materials{DB: db, Reconnect: reconnect}
	handler := func(repo material.Repository, store *storage.PrivateStore) http.Handler {
		return httpapi.WithUpload(health.Service{DB: db, Probe: func() error { return storage.Probe(root) }}, login, identity, &httpapi.UploadHandler{Store: store, Uploader: material.Uploader{Repository: repo}, Limit: storage.DefaultUploadLimit})
	}
	post := func(h http.Handler, name, content string) *httptest.ResponseRecorder {
		var b bytes.Buffer
		mw := multipart.NewWriter(&b)
		p, _ := mw.CreateFormFile("file", name)
		p.Write([]byte(content))
		mw.Close()
		r := httptest.NewRequest("POST", "/api/materials", &b)
		r.Header.Set("Content-Type", mw.FormDataContentType())
		r.Header.Set("Authorization", "Bearer "+response.AccessToken)
		r.Header.Set("X-Class-Id", "999")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	counts := func() (int, int) {
		var m, k int
		if db.QueryRow("SELECT COUNT(*) FROM materials").Scan(&m) != nil || db.QueryRow("SELECT COUNT(*) FROM knowledge_entries").Scan(&k) != nil {
			t.Fatal("count query failed")
		}
		return m, k
	}
	assertStored := func(id, body string) {
		var class, user, kclass, size int64
		var text, name, key, mime, created, kcreated string
		// Independent pool observes the rows only after the response.
		check, e := reconnect()
		if e != nil {
			t.Fatal("independent connection failed")
		}
		defer check.Close()
		e = check.QueryRow(`SELECT m.class_id,m.uploader_id,m.size,m.original_name,m.storage_key,m.content_type,m.created_at,k.class_id,k.plain_text,k.created_at FROM materials m JOIN knowledge_entries k ON k.material_id=m.id WHERE m.id=?`, id).Scan(&class, &user, &size, &name, &key, &mime, &created, &kclass, &text, &kcreated)
		if e != nil || class != principal.ClassID() || user != principal.UserID() || kclass != class || size != int64(len(body)) || key != id || mime != "text/plain" || created == "" || kcreated != created || text != strings.TrimPrefix(body, "\ufeff") {
			t.Fatal("persisted double-table contract mismatch")
		}
		raw, e := os.ReadFile(filepath.Join(root, "files", id))
		if e != nil || string(raw) != body {
			t.Fatal("committed raw file missing or changed")
		}
	}
	decodeID := func(w *httptest.ResponseRecorder) string {
		var r map[string]string
		if w.Code != 201 || json.Unmarshal(w.Body.Bytes(), &r) != nil || len(r) != 1 || r["material_id"] == "" {
			t.Fatalf("invalid success response: status %d", w.Code)
		}
		return r["material_id"]
	}
	good := post(handler(baseRepo, files), "lesson.MD", "\ufeff# 中文\n<script>text</script>")
	goodID := decodeID(good)
	assertStored(goodID, "\ufeff# 中文\n<script>text</script>")
	// Faults are scoped by fixture content/name; existing rows are never changed.
	for _, statement := range []string{
		`CREATE TRIGGER upload_fail_first BEFORE INSERT ON materials FOR EACH ROW BEGIN IF NEW.original_name='fail-first.txt' THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='fixture first insert failure'; END IF; END`,
		`CREATE TRIGGER upload_fail_second BEFORE INSERT ON knowledge_entries FOR EACH ROW BEGIN IF NEW.plain_text='fail-second' THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='fixture second insert failure'; END IF; END`,
	} {
		if _, e = db.Exec(statement); e != nil {
			t.Fatal("isolated fault trigger failed")
		}
	}
	for _, tc := range []struct{ name, body string }{{"fail-first.txt", "first"}, {"fail-second.txt", "fail-second"}} {
		t.Run(tc.name, func(t *testing.T) {
			m, k := counts()
			before, _ := os.ReadDir(filepath.Join(root, "files"))
			w := post(handler(baseRepo, files), tc.name, tc.body)
			m2, k2 := counts()
			after, _ := os.ReadDir(filepath.Join(root, "files"))
			if w.Code != 500 || m != m2 || k != k2 || len(before) != len(after) {
				t.Fatal("insert failure leaked rows/file or success")
			}
		})
	}
	t.Run("file-write-and-move-failures", func(t *testing.T) {
		closed, e := storage.OpenPrivate(root)
		if e != nil {
			t.Fatal("store fixture")
		}
		closed.Close()
		m, k := counts()
		if post(handler(baseRepo, closed), "write.txt", "write").Code != 500 {
			t.Fatal("write failure succeeded")
		}
		u, e := files.Prepare("move.txt", strings.NewReader("move"), 100)
		if e != nil {
			t.Fatal("staging failed")
		}
		defer u.Close()
		if _, e = (material.Uploader{Repository: baseRepo}).Save(ctx, principal, missingSource{u}); e == nil {
			t.Fatal("rename failure succeeded")
		}
		m2, k2 := counts()
		if m != m2 || k != k2 {
			t.Fatal("move failure did not rollback")
		}
		if _, e = os.Stat(filepath.Join(root, "files", u.Info().ID)); !os.IsNotExist(e) {
			t.Fatal("move reservation leaked")
		}
	})
	for _, mode := range []string{"rollback", "commit", "delay", "unknown"} {
		t.Run(mode, func(t *testing.T) {
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
			wrapped := sql.OpenDB(outcomeConnector{connector, mode})
			defer wrapped.Close()
			r := repository.Materials{DB: wrapped, Reconnect: reconnect}
			if mode == "unknown" {
				r.Reconnect = func() (*sql.DB, error) { return nil, errors.New("synthetic unavailable verifier") }
			}
			m, k := counts()
			before, _ := os.ReadDir(filepath.Join(root, "files"))
			w := post(handler(r, files), mode+".txt", mode)
			m2, k2 := counts()
			after, _ := os.ReadDir(filepath.Join(root, "files"))
			if mode == "rollback" {
				if w.Code != 500 || m2 != m || k2 != k || len(after) != len(before) {
					t.Fatal("confirmed rollback not compensated")
				}
			} else if mode == "unknown" {
				if w.Code != 500 || m2 != m+1 || k2 != k+1 || len(after) != len(before)+1 {
					t.Fatal("unknown commit lost evidence or returned success")
				}
			} else {
				id := decodeID(w)
				assertStored(id, mode)
			}
		})
	}
	t.Run("committed-file-never-deleted", func(t *testing.T) {
		u, e := files.Prepare("verify.txt", strings.NewReader("original"), 100)
		if e != nil {
			t.Fatal("prepare")
		}
		defer u.Close()
		if _, e = (material.Uploader{Repository: baseRepo}).Save(ctx, principal, corruptPublished{u, root}); e == nil {
			t.Fatal("corrupted committed file returned success")
		}
		var rows int
		if db.QueryRow("SELECT COUNT(*) FROM materials WHERE id=?", u.Info().ID).Scan(&rows) != nil || rows != 1 {
			t.Fatal("commit missing")
		}
		if _, e = os.Stat(filepath.Join(root, "files", u.Info().ID)); e != nil {
			t.Fatal("committed file was deleted")
		}
	})
	t.Run("same-name-concurrency", func(t *testing.T) {
		var wg sync.WaitGroup
		results := make(chan *httptest.ResponseRecorder, 6)
		for i := 0; i < 6; i++ {
			wg.Add(1)
			go func() { defer wg.Done(); results <- post(handler(baseRepo, files), "same.txt", "concurrent") }()
		}
		wg.Wait()
		close(results)
		seen := map[string]bool{}
		for w := range results {
			id := decodeID(w)
			if seen[id] {
				t.Fatal("same name collided")
			}
			seen[id] = true
			assertStored(id, "concurrent")
		}
	})
	assertStored(goodID, "\ufeff# 中文\n<script>text</script>")
	tmp, _ := os.ReadDir(filepath.Join(root, "tmp"))
	if len(tmp) != 0 {
		t.Fatal("temporary residue")
	}
	t.Log("PASS real MySQL dual inserts, independent post-201 reads, first/second insert rollback, file write/move failure, confirmed commit failure, lost reply, in-flight commit locking, unknown evidence retention, committed file preservation and concurrent same-name uploads")
}
