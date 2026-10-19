//go:build alignstorage

package mysql

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	driver "github.com/go-sql-driver/mysql"
)

type alignRecoveryPlan struct {
	Source       string            `json:"source"`
	Restored     string            `json:"restored"`
	BeforeSHA256 string            `json:"before_sha256"`
	Files        map[string]string `json:"files"`
}

func recoveryEvidence(t *testing.T) string {
	t.Helper()
	run := os.Getenv("ALIGN_STORAGE_RECOVERY")
	if !regexp.MustCompile(`^[a-f0-9]{8}$`).MatchString(run) {
		t.Fatal("unique recovery run ID required")
	}
	return filepath.Join(os.Getenv("ALIGN_STORAGE_EVIDENCE"), "recovery-"+run)
}

func TestAlignRecoveryPrepare(t *testing.T) {
	db := legacyFixture(t)
	var name string
	if db.QueryRow("SELECT DATABASE()").Scan(&name) != nil {
		t.Fatal("database identity")
	}
	p := alignRecoveryPlan{Source: name, BeforeSHA256: legacyFingerprint(t, db), Files: map[string]string{}}
	evidence := recoveryEvidence(t)
	dir := filepath.Join(evidence, "recovery-source-files")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{alignMaterialA, alignMaterialB} {
		b := []byte("隔离恢复文件：" + id)
		if err := os.WriteFile(filepath.Join(dir, id), b, 0600); err != nil {
			t.Fatal(err)
		}
		p.Files[id] = fmt.Sprintf("%x", sha256.Sum256(b))
	}
	b, _ := json.MarshalIndent(p, "", "  ")
	if err := os.WriteFile(filepath.Join(evidence, "recovery-plan.json"), b, 0600); err != nil {
		t.Fatal(err)
	}
	t.Logf("pre-upgrade v2 backup source=%s fingerprint=%s", name, p.BeforeSHA256)
}

func openRecoveryDB(t *testing.T, name string) *sql.DB {
	t.Helper()
	if !regexp.MustCompile(`^align_w5_[a-f0-9]{16}$`).MatchString(name) {
		t.Fatal("unsafe recovery database name")
	}
	cfg, err := driver.ParseDSN(os.Getenv("ALIGN_STORAGE_DSN"))
	if err != nil || cfg.Addr != os.Getenv("ALIGN_STORAGE_CONTAINER")+":3306" {
		t.Fatal("recovery server identity")
	}
	cfg.DBName = name
	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestAlignRecoveryFinish(t *testing.T) {
	evidence := recoveryEvidence(t)
	b, err := os.ReadFile(filepath.Join(evidence, "recovery-plan.json"))
	if err != nil {
		t.Fatal(err)
	}
	var p alignRecoveryPlan
	if json.Unmarshal(b, &p) != nil || p.Source == p.Restored {
		t.Fatal("independent restore required")
	}
	source := openRecoveryDB(t, p.Source)
	restored := openRecoveryDB(t, p.Restored)
	ctx := context.Background()
	if err = validateRAGV2(ctx, restored); err != nil {
		t.Fatal("restored v2 catalog", err)
	}
	if err = ValidateRAGSchema(ctx, restored); err == nil {
		t.Fatal("v2 restore incorrectly marked v3 ready")
	}
	if legacyFingerprint(t, restored) != p.BeforeSHA256 {
		t.Fatal("restored IDs/history/token/audit differ")
	}
	for id, hash := range p.Files {
		file, e := os.ReadFile(filepath.Join(evidence, "recovery-restored-files", id))
		if e != nil || fmt.Sprintf("%x", sha256.Sum256(file)) != hash {
			t.Fatal("restored file mismatch", id, e)
		}
	}
	interrupted := errors.New("controlled recovery interruption")
	err = prepareTutorSchema(ctx, source, func(step int) error {
		if step == len(ddlStatements(ragDDL))+5 {
			return interrupted
		}
		return nil
	})
	if !errors.Is(err, interrupted) || ValidateRAGSchema(ctx, source) == nil {
		t.Fatal("upgrade interruption not detected", err)
	}
	if err = PrepareRAGSchema(ctx, source); err != nil {
		t.Fatal("forward repair", err)
	}
	if legacyFingerprint(t, source) != p.BeforeSHA256 {
		t.Fatal("forward repair changed old data")
	}
	mustSQL(t, source, "INSERT INTO classes(id,name) VALUES(3,'post-upgrade record')")
	if scalar(t, source, "SELECT COUNT(*) FROM classes WHERE id=3") != 1 || scalar(t, restored, "SELECT COUNT(*) FROM classes WHERE id=3") != 0 {
		t.Fatal("restore loss boundary not observed")
	}
	if err = validateRAGV2(ctx, source); err == nil {
		t.Fatal("old binary catalog accepted v3")
	}
	t.Logf("independent restore=%s original=%s fingerprint=%s; post-upgrade record excluded as expected", p.Restored, p.Source, p.BeforeSHA256)
	r := map[string]any{"source": p.Source, "restored": p.Restored, "legacy_sha256": p.BeforeSHA256, "files_verified": len(p.Files), "v2_restore_catalog_passed": true, "v3_forward_repair_passed": true, "v2_catalog_rejects_v3": true, "post_upgrade_rows_excluded": 1, "classroom_resources_touched": false, "qdrant_accessed": false}
	out, _ := json.MarshalIndent(r, "", "  ")
	if err = os.WriteFile(filepath.Join(evidence, "recovery-result.json"), out, 0600); err != nil {
		t.Fatal(err)
	}
}
