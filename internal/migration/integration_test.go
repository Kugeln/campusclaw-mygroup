//go:build integration

package migration

import (
	"campusclaw/internal/config"
	"campusclaw/internal/provision"
	repository "campusclaw/internal/repository/mysql"
	"campusclaw/internal/transfer"
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func target(t *testing.T) *sql.DB {
	t.Helper()
	if os.Getenv("PHASE2_ISOLATED") != "1" {
		t.Fatal("integration tests require isolated harness")
	}
	c, e := config.Load(os.Getenv)
	if e != nil {
		t.Fatal(e)
	}
	db, e := repository.Open(c)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { db.Close() })
	for _, table := range []string{"knowledge_entries", "materials", "sessions", "users", "classes", "assignments", "assistants", "skills"} {
		if _, e = db.Exec("DROP TABLE IF EXISTS " + table); e != nil {
			t.Fatal(e)
		}
	}
	if e = repository.PrepareSchema(context.Background(), db); e != nil {
		t.Fatal(e)
	}
	return db
}
func source(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "source.db")
	root := filepath.Join(dir, "source-uploads")
	if e := os.MkdirAll(filepath.Join(root, "files"), 0700); e != nil {
		t.Fatal(e)
	}
	db, e := sql.Open("sqlite", path)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	if _, e = db.Exec(sourceSchema + sourceSessions); e != nil {
		t.Fatal(e)
	}
	hash := "pbkdf2:sha256:1000000$salt123$" + strings.Repeat("a", 64)
	for i, class := range []string{"A", "B"} {
		id := i + 1
		must(t, db, "INSERT INTO classes(id,name) VALUES (?,?)", id, class)
		must(t, db, "INSERT INTO users(id,username,password_hash,role,class_id,session_version) VALUES (?,?,?,?,?,?)", id, "teacher_"+class, hash, "teacher", id, 7)
		key := fmt.Sprintf("00000000-0000-4000-8000-%012d", id)
		body := "中文材料 " + class + "\n"
		b := []byte("\ufeff" + body)
		if e = os.WriteFile(filepath.Join(root, "files", key), b, 0600); e != nil {
			t.Fatal(e)
		}
		must(t, db, "INSERT INTO materials VALUES (?,?,?,?,?,?,?,?)", key, id, id, class+".md", len(b), "text/plain", key, "2026-09-27T00:00:00.123Z")
		must(t, db, "INSERT INTO knowledge_entries VALUES (?,?,?,?,?)", id, key, id, body, "2026-09-27T00:00:00.123Z")
	}
	for _, table := range []string{"assignments", "assistants", "skills"} {
		must(t, db, "INSERT INTO "+table+" VALUES (?,?)", 17, "保留占位")
		must(t, db, "INSERT INTO "+table+" VALUES (?,?)", int64(9007199254740993), "large ID")
		must(t, db, "INSERT INTO "+table+" VALUES (?,?)", 0, "zero ID")
		must(t, db, "INSERT INTO "+table+" VALUES (?,?)", -1, "negative ID")
	}
	must(t, db, "INSERT INTO sessions VALUES (?,?,?)", strings.Repeat("b", 64), `{"user_id":1}`, 9999999999)
	must(t, db, "INSERT INTO sessions VALUES (?,?,?)", strings.Repeat("c", 64), `{"_permanent":true}`, 1)
	return path, root
}
func must(t *testing.T, db *sql.DB, q string, args ...any) {
	t.Helper()
	if _, e := db.Exec(q, args...); e != nil {
		t.Fatalf("fixture/query failed (%d)", repository.ErrorCode(e))
	}
}
func reject(t *testing.T, db *sql.DB, q string, args ...any) {
	t.Helper()
	if _, e := db.Exec(q, args...); e == nil {
		t.Fatal("invalid data accepted")
	}
}
func TestSchemaConstraints(t *testing.T) {
	db := target(t)
	ctx := context.Background()
	if e := repository.PrepareSchema(ctx, db); e != nil {
		t.Fatal(e)
	}
	var mode string
	if e := db.QueryRow("SELECT @@sql_mode").Scan(&mode); e != nil || !strings.Contains(mode, "STRICT_ALL_TABLES") {
		t.Fatal("strict mode missing")
	}
	for i, name := range []string{"A", "a", "A ", "中文"} {
		must(t, db, "INSERT INTO classes(id,name) VALUES (?,?)", i, name)
	}
	reject(t, db, "INSERT INTO classes(name) VALUES ('A')")
	reject(t, db, "INSERT INTO classes(name) VALUES (NULL)")
	reject(t, db, "INSERT INTO classes(name) VALUES (?)", strings.Repeat("x", 256))
	must(t, db, "INSERT INTO users(id,username,password_hash,role,class_id) VALUES (10,'t','hash','teacher',0)")
	reject(t, db, "INSERT INTO users(username,password_hash,role,class_id) VALUES ('x','hash','admin',0)")
	reject(t, db, "INSERT INTO users(username,password_hash,role,class_id) VALUES ('x','hash','teacher',999)")
	key := "00000000-0000-4000-8000-000000000001"
	must(t, db, "INSERT INTO materials VALUES (?,0,10,'file.txt',1,'text/plain',?,'2026-01-01T00:00:00.000Z')", key, key)
	reject(t, db, "INSERT INTO knowledge_entries(material_id,class_id,plain_text,created_at) VALUES (?,1,'x','2026-01-01T00:00:00.000Z')", key)
	body := strings.Repeat("x", 20*1024*1024)
	must(t, db, "INSERT INTO knowledge_entries(id,material_id,class_id,plain_text,created_at) VALUES (41,?,0,?,'2026-01-01T00:00:00.000Z')", key, body)
	var got string
	if e := db.QueryRow("SELECT plain_text FROM knowledge_entries WHERE id=41").Scan(&got); e != nil || transfer.Hash([]byte(got)) != transfer.Hash([]byte(body)) {
		t.Fatal("20 MiB body corrupted")
	}
	reject(t, db, "UPDATE materials SET size=0 WHERE id=?", key)
	reject(t, db, "DELETE FROM classes WHERE id=0")
	var next int64
	r, e := db.Exec("INSERT INTO classes(name) VALUES ('next')")
	if e != nil {
		t.Fatal(e)
	}
	next, _ = r.LastInsertId()
	if next <= 3 {
		t.Fatal("auto increment collision")
	}
	var count int
	if e = db.QueryRow("SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=DATABASE() AND engine='InnoDB'").Scan(&count); e != nil || count != 8 {
		t.Fatal("expected eight InnoDB tables")
	}
}
func TestMigrationAndSafeRerun(t *testing.T) {
	db := target(t)
	path, root := source(t)
	before, _ := os.ReadFile(path)
	ctx := context.Background()
	s, e := Source(ctx, path, root)
	if e != nil {
		t.Fatal(e)
	}
	dest := filepath.Join(t.TempDir(), "target")
	r, e := transfer.Publish(ctx, db, s, root, dest, "migrate", nil)
	if e != nil {
		t.Fatal(e)
	}
	if !r.Verified || len(r.Tables) != 7 || len(r.FilesSHA256) != 2 {
		t.Fatal("incomplete report")
	}
	if r.SourceSHA256 != transfer.Hash(before) {
		t.Fatal("missing source snapshot association")
	}
	if e = transfer.Ready(ctx, db, dest); e != nil {
		t.Fatal("verified target not ready", e)
	}
	for _, table := range transfer.Tables {
		if r.Tables[table.Name].Count != len(s.Rows[table.Name]) {
			t.Fatal("table count mismatch")
		}
	}
	var sessions int
	db.QueryRow("SELECT COUNT(*) FROM sessions").Scan(&sessions)
	if sessions != 0 {
		t.Fatal("old sessions imported")
	}
	again, e := transfer.Publish(ctx, db, s, root, dest, "migrate", nil)
	if e != nil || again.Fingerprint != r.Fingerprint {
		t.Fatal("identical rerun failed", e)
	}
	must(t, db, "UPDATE users SET session_version=99 WHERE id=1")
	if _, e = transfer.Publish(ctx, db, s, root, dest, "migrate", nil); e == nil {
		t.Fatal("conflicting target overwritten")
	}
	var version int
	db.QueryRow("SELECT session_version FROM users WHERE id=1").Scan(&version)
	if version != 99 {
		t.Fatal("conflicting row changed")
	}
	must(t, db, "UPDATE users SET session_version=7 WHERE id=1")
	must(t, db, "UPDATE knowledge_entries SET plain_text='damaged' WHERE id=1")
	if _, e = transfer.Verify(ctx, db, s, dest, "migrate"); e == nil {
		t.Fatal("body damage missed")
	}
	if e = transfer.Ready(ctx, db, dest); e == nil {
		t.Fatal("damaged target ready")
	}
	must(t, db, "UPDATE knowledge_entries SET plain_text=? WHERE id=1", s.Rows["knowledge_entries"][0]["plain_text"])
	key := s.Rows["materials"][0]["storage_key"]
	if e = os.WriteFile(filepath.Join(dest, "files", key), []byte("bad"), 0600); e != nil {
		t.Fatal(e)
	}
	if _, e = transfer.Verify(ctx, db, s, dest, "migrate"); e == nil {
		t.Fatal("file damage missed")
	}
	after, _ := os.ReadFile(path)
	if transfer.Hash(before) != transfer.Hash(after) {
		t.Fatal("source SQLite changed")
	}
}
func TestPreflightRejectsWithoutSourceWrites(t *testing.T) {
	for _, kind := range []string{"missing", "fk", "long", "password", "body", "schema"} {
		t.Run(kind, func(t *testing.T) {
			path, root := source(t)
			db, _ := sql.Open("sqlite", path)
			switch kind {
			case "missing":
				os.Remove(filepath.Join(root, "files", "00000000-0000-4000-8000-000000000001"))
			case "fk":
				must(t, db, "UPDATE users SET class_id=999 WHERE id=1")
			case "long":
				must(t, db, "UPDATE users SET username=? WHERE id=1", strings.Repeat("长", 256))
			case "password":
				must(t, db, "UPDATE users SET password_hash='unknown' WHERE id=1")
			case "body":
				must(t, db, "UPDATE knowledge_entries SET plain_text='corrupt' WHERE id=1")
			case "schema":
				must(t, db, "ALTER TABLE users ADD COLUMN surprise TEXT")
			}
			db.Close()
			before, _ := os.ReadFile(path)
			if _, e := Source(context.Background(), path, root); e == nil {
				t.Fatal("invalid source accepted")
			}
			after, _ := os.ReadFile(path)
			if transfer.Hash(before) != transfer.Hash(after) {
				t.Fatal("preflight changed source")
			}
		})
	}
}
func TestMigrationRollback(t *testing.T) {
	db := target(t)
	path, root := source(t)
	ctx := context.Background()
	s, e := Source(ctx, path, root)
	if e != nil {
		t.Fatal(e)
	}
	dest := filepath.Join(t.TempDir(), "target")
	_, e = transfer.Publish(ctx, db, s, root, dest, "migrate", func() error { return fmt.Errorf("injected failure") })
	if e == nil {
		t.Fatal("failure not propagated")
	}
	empty, e := transfer.Read(ctx, db)
	if e != nil || !empty.Empty() {
		t.Fatal("partial rows after failure")
	}
	entries, _ := os.ReadDir(filepath.Join(dest, "files"))
	if len(entries) != 0 {
		t.Fatal("partial files")
	}
	if _, e = transfer.Receipt(dest); !os.IsNotExist(e) {
		t.Fatal("false success receipt")
	}
	if _, e = transfer.Publish(ctx, db, s, root, dest, "migrate", nil); e != nil {
		t.Fatal("retry failed", e)
	}
}
func TestFreshAndAccountCommands(t *testing.T) {
	db := target(t)
	root := filepath.Join(t.TempDir(), "fresh")
	ctx := context.Background()
	get := func(string) string { return "only-a-test-password" }
	if _, e := provision.Fresh(ctx, db, root, func(string) string { return "" }); e == nil {
		t.Fatal("missing seed secret accepted")
	}
	must(t, db, "CREATE TRIGGER fail_seed BEFORE INSERT ON users FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='test failure'")
	if _, e := provision.Fresh(ctx, db, root, get); e == nil {
		t.Fatal("seed failure ignored")
	}
	must(t, db, "DROP TRIGGER fail_seed")
	snapshot, e := transfer.Read(ctx, db)
	if e != nil || !snapshot.Empty() {
		t.Fatal("failed fresh left data")
	}
	r, e := provision.Fresh(ctx, db, root, get)
	if e != nil {
		t.Fatal(e)
	}
	if r.Mode != "fresh" || r.Tables["users"].Count != 4 || r.Tables["materials"].Count != 2 {
		t.Fatal("missing seeds")
	}
	var hash string
	db.QueryRow("SELECT password_hash FROM users WHERE id=1").Scan(&hash)
	if !passwordPatternForTest(hash) {
		t.Fatal("password not hashed")
	}
	again, e := provision.Fresh(ctx, db, root, func(string) string { return "replacement" })
	if e != nil || again.Fingerprint != r.Fingerprint {
		t.Fatal("fresh rerun changed data", e)
	}
	var after string
	db.QueryRow("SELECT password_hash FROM users WHERE id=1").Scan(&after)
	if after != hash {
		t.Fatal("password reset")
	}
	if _, e = provision.CreateUser(ctx, db, "bad", "test", "admin", 1); e == nil {
		t.Fatal("admin accepted")
	}
	if _, e = provision.CreateUser(ctx, db, "bad", "test", "student", 999); e == nil {
		t.Fatal("missing class accepted")
	}
	id, e := provision.CreateClass(ctx, db, "C")
	if e != nil {
		t.Fatal(e)
	}
	id2, e := provision.CreateClass(ctx, db, "C")
	if e != nil || id != id2 {
		t.Fatal("class creation not idempotent")
	}
	if _, e = provision.CreateUser(ctx, db, "student_c", "test", "student", id); e != nil {
		t.Fatal(e)
	}
	path, files := source(t)
	s, e := Source(ctx, path, files)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = transfer.Publish(ctx, db, s, files, root, "migrate", nil); e == nil {
		t.Fatal("mode conflict accepted")
	}
}
func passwordPatternForTest(value string) bool {
	return strings.HasPrefix(value, "pbkdf2:sha256:1000000$") && !strings.Contains(value, "only-a-test-password")
}

func TestSchemaDriftAndStartupGate(t *testing.T) {
	db := target(t)
	ctx := context.Background()
	root := t.TempDir()
	if e := transfer.Ready(ctx, db, root); e == nil {
		t.Fatal("uninitialized target ready")
	}
	must(t, db, "ALTER TABLE users DROP CHECK users_chk_1")
	if e := repository.ValidateSchema(ctx, db); e == nil {
		t.Fatal("missing CHECK accepted")
	}
	if e := repository.PrepareSchema(ctx, db); e == nil {
		t.Fatal("schema drift silently repaired or accepted")
	}
}

func TestOrderingUniqueAndAutoIncrement(t *testing.T) {
	db := target(t)
	must(t, db, "INSERT INTO classes(id,name) VALUES (9,'A')")
	for i, name := range []string{"User", "user", "User ", "????"} {
		must(t, db, "INSERT INTO users(id,username,password_hash,role,class_id) VALUES (?,?,?,'teacher',9)", i+11, name, "hash")
	}
	reject(t, db, "INSERT INTO users(username,password_hash,role,class_id) VALUES ('User','hash','teacher',9)")
	reject(t, db, "INSERT INTO users(id,username,password_hash,role,class_id) VALUES (11,'other','hash','teacher',9)")
	reject(t, db, "INSERT INTO users(username,password_hash,role,class_id) VALUES ('other',NULL,'teacher',9)")
	for i, stamp := range []string{"2026-01-02T00:00:00.000Z", "2026-01-01T23:59:59.999Z", "2026-01-02T00:00:00.000Z"} {
		key := fmt.Sprintf("00000000-0000-4000-8000-%012d", i+1)
		must(t, db, "INSERT INTO materials VALUES (?,9,11,'x.txt',1,'text/plain',?,?)", key, key, stamp)
	}
	rows, e := db.Query("SELECT id FROM materials ORDER BY created_at DESC,id DESC")
	if e != nil {
		t.Fatal(e)
	}
	var ids []string
	for rows.Next() {
		var id string
		if e = rows.Scan(&id); e != nil {
			t.Fatal(e)
		}
		ids = append(ids, id)
	}
	rows.Close()
	if e = rows.Err(); e != nil {
		t.Fatal(e)
	}
	if len(ids) != 3 || !strings.HasSuffix(ids[0], "003") || !strings.HasSuffix(ids[1], "001") || !strings.HasSuffix(ids[2], "002") {
		t.Fatal("time/id ordering differs")
	}
	for _, table := range []string{"assignments", "assistants", "skills"} {
		must(t, db, "INSERT INTO "+table+"(id,name) VALUES (9007199254740993,'large ID')")
		result, e := db.Exec("INSERT INTO " + table + "(name) VALUES ('next')")
		if e != nil {
			t.Fatal(e)
		}
		id, e := result.LastInsertId()
		if e != nil || id != 9007199254740994 {
			t.Fatal("64 bit identity lost")
		}
	}
}

func TestActiveSessionsPreventReimport(t *testing.T) {
	db := target(t)
	path, root := source(t)
	ctx := context.Background()
	s, e := Source(ctx, path, root)
	if e != nil {
		t.Fatal(e)
	}
	dest := filepath.Join(t.TempDir(), "target")
	if _, e = transfer.Publish(ctx, db, s, root, dest, "migrate", nil); e != nil {
		t.Fatal(e)
	}
	must(t, db, "INSERT INTO sessions VALUES (?,?,?)", strings.Repeat("d", 64), "{}", 9999999999)
	if _, e = transfer.Publish(ctx, db, s, root, dest, "migrate", nil); e == nil {
		t.Fatal("active target reimported")
	}
	var n int
	if e = db.QueryRow("SELECT COUNT(*) FROM sessions").Scan(&n); e != nil || n != 1 {
		t.Fatal("active sessions changed")
	}
	if e = transfer.Ready(ctx, db, dest); e != nil {
		t.Fatal("normal restart rejected active sessions", e)
	}
}

func TestCoherentCorruptionCannotReuseReceipt(t *testing.T) {
	db := target(t)
	path, root := source(t)
	ctx := context.Background()
	s, e := Source(ctx, path, root)
	if e != nil {
		t.Fatal(e)
	}
	dest := filepath.Join(t.TempDir(), "target")
	if _, e = transfer.Publish(ctx, db, s, root, dest, "migrate", nil); e != nil {
		t.Fatal(e)
	}
	key := s.Rows["materials"][0]["storage_key"]
	body := "coherent but wrong data"
	if e = os.WriteFile(filepath.Join(dest, "files", key), []byte(body), 0600); e != nil {
		t.Fatal(e)
	}
	must(t, db, "UPDATE materials SET size=? WHERE storage_key=?", len(body), key)
	must(t, db, "UPDATE knowledge_entries SET plain_text=? WHERE material_id=?", body, key)
	if e = transfer.Ready(ctx, db, dest); e == nil {
		t.Fatal("stale receipt accepted coherent damage")
	}
}
