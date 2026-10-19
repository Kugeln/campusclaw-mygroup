//go:build mcpstorage

package transfer_test

import (
	"campusclaw/internal/provision"
	repository "campusclaw/internal/repository/mysql"
	"campusclaw/internal/transfer"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	driver "github.com/go-sql-driver/mysql"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMCPSchemaReceiptInterruptedInitializer(t *testing.T) {
	cfg, e := driver.ParseDSN(os.Getenv("MCP_TEST_DSN"))
	host := os.Getenv("MCP_TEST_MYSQL")
	if e != nil || cfg == nil || !strings.HasPrefix(host, "campusclaw-w6-") || cfg.Addr != host+":3306" || cfg.DBName != "" {
		t.Fatal("isolated target required")
	}
	admin, e := sql.Open("mysql", cfg.FormatDSN())
	if e != nil {
		t.Fatal(e)
	}
	defer admin.Close()
	var nonce [8]byte
	rand.Read(nonce[:])
	cfg.DBName = "w6_resume_" + hex.EncodeToString(nonce[:])
	if _, e = admin.Exec("CREATE DATABASE " + cfg.DBName + " CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_bin"); e != nil {
		t.Fatal(e)
	}
	db, e := sql.Open("mysql", cfg.FormatDSN())
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	ctx := context.Background()
	root := t.TempDir()
	if e = repository.PrepareSchema(ctx, db); e != nil {
		t.Fatal(e)
	}
	if _, e = provision.Fresh(ctx, db, root, func(string) string { return "isolated-resume-test-only" }); e != nil {
		t.Fatal(e)
	}
	var v3Steps int
	for _, name := range []string{"schema_rag.sql", "schema_tutor_v3.sql"} {
		body, e := os.ReadFile(filepath.Join("..", "repository", "mysql", name))
		if e != nil {
			t.Fatal(e)
		}
		for _, statement := range strings.Split(string(body), ";") {
			if strings.TrimSpace(statement) == "" {
				continue
			}
			if _, e = db.Exec(statement); e != nil {
				t.Fatal("legacy fixture DDL")
			}
			if name == "schema_tutor_v3.sql" {
				v3Steps++
			}
		}
	}
	if _, e = db.Exec("INSERT INTO schema_upgrades(version,ddl_step,completed) VALUES(3,?,1)", v3Steps); e != nil {
		t.Fatal(e)
	}
	// Simulate the first v4 DDL committed before any version=4 ledger row.
	if _, e = db.Exec("ALTER TABLE schema_upgrades ADD COLUMN ddl_hash CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NULL"); e != nil {
		t.Fatal(e)
	}
	if transfer.Ready(ctx, db, root) == nil {
		t.Fatal("partial catalog was ready")
	}
	if e = transfer.PrepareCurrentSchema(ctx, db, root); e != nil {
		t.Fatal("offline initializer cannot resume", e)
	}
	if e = transfer.VerifyMCPSchemaReceipt(ctx, db, root); e != nil {
		t.Fatal(e)
	}
	if e = transfer.PrepareCurrentSchema(ctx, db, root); e != nil {
		t.Fatal("repeat initializer", e)
	}
}

func TestMCPSchemaReceipt(t *testing.T) {
	cfg, e := driver.ParseDSN(os.Getenv("MCP_TEST_DSN"))
	host := os.Getenv("MCP_TEST_MYSQL")
	if e != nil || cfg == nil || !strings.HasPrefix(host, "campusclaw-w6-") || cfg.Addr != host+":3306" || cfg.DBName != "" {
		t.Fatal("isolated target required")
	}
	admin, e := sql.Open("mysql", cfg.FormatDSN())
	if e != nil {
		t.Fatal(e)
	}
	defer admin.Close()
	var random [8]byte
	rand.Read(random[:])
	cfg.DBName = "w6_receipt_" + hex.EncodeToString(random[:])
	if _, e = admin.Exec("CREATE DATABASE " + cfg.DBName + " CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_bin"); e != nil {
		t.Fatal(e)
	}
	db, e := sql.Open("mysql", cfg.FormatDSN())
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	ctx := context.Background()
	root := t.TempDir()
	if e = repository.PrepareSchema(ctx, db); e != nil {
		t.Fatal(e)
	}
	if _, e = provision.Fresh(ctx, db, root, func(string) string { return "only-isolated-seed-password" }); e != nil {
		t.Fatal(e)
	}
	original, e := os.ReadFile(filepath.Join(root, transfer.ReceiptName))
	if e != nil {
		t.Fatal(e)
	}
	if e = repository.PrepareRAGSchema(ctx, db); e != nil {
		t.Fatal(e)
	}
	if e = transfer.VerifyMCPSchemaReceipt(ctx, db, root); e == nil {
		t.Fatal("missing v4 receipt accepted")
	}
	if e = transfer.RecordMCPSchema(ctx, db, root); e != nil {
		t.Fatal(e)
	}
	if e = transfer.VerifyMCPSchemaReceipt(ctx, db, root); e != nil {
		t.Fatal(e)
	}
	if _, e = db.Exec("UPDATE users SET session_version=7 WHERE id=1"); e != nil {
		t.Fatal(e)
	}
	if _, e = db.Exec("INSERT INTO skills(id,name,class_id,enabled) VALUES(900,'teacher deleted',1,0)"); e != nil {
		t.Fatal(e)
	}
	if _, e = db.Exec("DELETE FROM skills WHERE id=900"); e != nil {
		t.Fatal(e)
	}
	// Repeat initializer path must not recreate a teacher-deleted skill or reset
	// passwords/revocations. Fresh's existing receipt path returns without reseed.
	if _, e = provision.Fresh(ctx, db, root, func(string) string { return "" }); e != nil {
		t.Fatal(e)
	}
	if e = repository.PrepareRAGSchema(ctx, db); e != nil {
		t.Fatal(e)
	}
	if e = transfer.RecordMCPSchema(ctx, db, root); e != nil {
		t.Fatal(e)
	}
	now, _ := os.ReadFile(filepath.Join(root, transfer.ReceiptName))
	if string(now) != string(original) {
		t.Fatal("old receipt overwritten")
	}
	var n int
	if db.QueryRow("SELECT COUNT(*) FROM skills WHERE id=900").Scan(&n) != nil || n != 0 {
		t.Fatal("deleted skill recreated")
	}
	if db.QueryRow("SELECT session_version FROM users WHERE id=1").Scan(&n) != nil || n != 7 {
		t.Fatal("revocation reset")
	}
	file := filepath.Join(root, transfer.MCPSchemaReceiptName)
	saved, _ := os.ReadFile(file)
	if e = os.WriteFile(file, []byte(`{"version":4,"ddl_hash":"forged","initialization_hash":"forged"}`), 0600); e != nil {
		t.Fatal(e)
	}
	if e = transfer.VerifyMCPSchemaReceipt(ctx, db, root); e == nil {
		t.Fatal("forged receipt accepted")
	}
	if e = transfer.RecordMCPSchema(ctx, db, root); e == nil {
		t.Fatal("forged receipt silently replaced")
	}
	os.WriteFile(file, saved, 0600)
}
