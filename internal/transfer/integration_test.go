//go:build integration

package transfer

import (
	"campusclaw/internal/config"
	repository "campusclaw/internal/repository/mysql"
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func isolated(t *testing.T) *sql.DB {
	t.Helper()
	if os.Getenv("PHASE2_ISOLATED") != "1" {
		t.Fatal("isolated harness required")
	}
	c, err := config.Load(os.Getenv)
	if err != nil {
		t.Fatal(err)
	}
	db, err := repository.Open(c)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	for _, name := range []string{"knowledge_entries", "materials", "sessions", "users", "classes", "assignments", "assistants", "skills"} {
		if _, err = db.Exec("DROP TABLE IF EXISTS " + name); err != nil {
			t.Fatal(err)
		}
	}
	if err = repository.PrepareSchema(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestCommitOutcomeReconciliation(t *testing.T) {
	for _, outcome := range []string{"committed_reply_lost", "rolled_back_reply_lost", "unavailable", "receipt_write_failure"} {
		t.Run(outcome, func(t *testing.T) {
			db := isolated(t)
			ctx := context.Background()
			source := t.TempDir()
			if err := os.Mkdir(filepath.Join(source, "files"), 0700); err != nil {
				t.Fatal(err)
			}
			dest := filepath.Join(t.TempDir(), "target")
			s := Snapshot{Rows: map[string][]Row{"classes": {{"id": "1", "name": "A"}}}}
			key := "00000000-0000-4000-8000-000000000001"
			body := "test material\n"
			if err := os.WriteFile(filepath.Join(source, "files", key), []byte(body), 0600); err != nil {
				t.Fatal(err)
			}
			s.Rows["users"] = []Row{{"id": "1", "username": "teacher", "password_hash": "pbkdf2:sha256:1000000$salt$" + strings.Repeat("a", 64), "role": "teacher", "class_id": "1", "session_version": "7"}}
			s.Rows["materials"] = []Row{{"id": key, "storage_key": key, "class_id": "1", "uploader_id": "1", "original_name": "test.txt", "size": "14", "content_type": "text/plain", "created_at": "2026-01-01T00:00:00.000Z"}}
			s.Rows["knowledge_entries"] = []Row{{"id": "1", "material_id": key, "class_id": "1", "plain_text": body, "created_at": "2026-01-01T00:00:00.000Z"}}
			for _, table := range Tables {
				if s.Rows[table.Name] == nil {
					s.Rows[table.Name] = []Row{}
				}
			}
			commit := func(tx *sql.Tx) error {
				if outcome == "rolled_back_reply_lost" {
					if err := tx.Rollback(); err != nil {
						t.Fatal(err)
					}
					return fmt.Errorf("lost reply")
				}
				if err := tx.Commit(); err != nil {
					t.Fatal(err)
				}
				if outcome == "unavailable" {
					db.Close()
				}
				if outcome == "receipt_write_failure" {
					if err := os.Mkdir(filepath.Join(dest, ReceiptName), 0700); err != nil {
						t.Fatal(err)
					}
				}
				return fmt.Errorf("lost reply")
			}
			r, err := publish(ctx, db, s, source, dest, "fresh", nil, commit)
			switch outcome {
			case "committed_reply_lost":
				if err != nil || !r.Verified {
					t.Fatal("committed outcome not reconciled", err)
				}
			case "rolled_back_reply_lost":
				if err == nil {
					t.Fatal("rollback reported success")
				}
				if entries, e := os.ReadDir(filepath.Join(dest, "files")); e != nil || len(entries) != 0 {
					t.Fatal("rollback left files", e)
				}
				current, e := Read(ctx, db)
				if e != nil || !current.Empty() {
					t.Fatal("rollback not empty", e)
				}
				if _, e = Publish(ctx, db, s, source, dest, "fresh", nil); e != nil {
					t.Fatal("retry failed", e)
				}
			case "unavailable":
				if err == nil {
					t.Fatal("unavailable database accepted")
				}
				if bytes, e := os.ReadFile(filepath.Join(dest, "files", key)); e != nil || string(bytes) != body {
					t.Fatal("uncertain commit lost file evidence", e)
				}
				if _, e := os.Stat(filepath.Join(dest, PendingName)); e != nil {
					t.Fatal("pending evidence lost", e)
				}
				c, e := config.Load(os.Getenv)
				if e != nil {
					t.Fatal(e)
				}
				reopened, e := repository.Open(c)
				if e != nil {
					t.Fatal(e)
				}
				defer reopened.Close()
				if _, e = Publish(ctx, reopened, s, source, dest, "fresh", nil); e != nil {
					t.Fatal("reconnect verification failed", e)
				}
			case "receipt_write_failure":
				if err == nil {
					t.Fatal("receipt write failure ignored")
				}
				if e := os.Remove(filepath.Join(dest, ReceiptName)); e != nil {
					t.Fatal(e)
				}
				recovered, e := RecoverFreshReceipt(ctx, db, dest)
				if e != nil || !recovered.Verified {
					t.Fatal("receipt recovery failed", e)
				}
			}
		})
	}
}
