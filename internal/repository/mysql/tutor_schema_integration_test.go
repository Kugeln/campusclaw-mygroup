//go:build alignstorage

package mysql

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	driver "github.com/go-sql-driver/mysql"
)

func newAlignDB(t *testing.T) *sql.DB {
	t.Helper()
	container := os.Getenv("ALIGN_STORAGE_CONTAINER")
	cfg, err := driver.ParseDSN(os.Getenv("ALIGN_STORAGE_DSN"))
	if err != nil || !strings.HasPrefix(container, "campusclaw-align-storage-") || cfg == nil || cfg.Addr != container+":3306" || cfg.DBName != "" || cfg.Passwd == "" {
		t.Fatal("dedicated new align-storage server required")
	}
	admin, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatal("test server unavailable")
	}
	defer admin.Close()
	var version string
	if admin.QueryRow("SELECT VERSION()").Scan(&version) != nil || !strings.HasPrefix(version, "8.4.") {
		t.Fatal("MySQL 8.4 test server required")
	}
	var suffix [8]byte
	if _, err = rand.Read(suffix[:]); err != nil {
		t.Fatal(err)
	}
	name := fmt.Sprintf("align_w5_%x", suffix)
	if _, err = admin.Exec("CREATE DATABASE " + name + " CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_bin"); err != nil {
		t.Fatal(err)
	}
	cfg.DBName = name
	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() }) // Retain every test database. No DROP/volume removal.
	t.Logf("new isolated database=%s mysql=%s", name, version)
	return db
}

func mustSQL(t *testing.T, db *sql.DB, query string, args ...any) {
	t.Helper()
	if _, err := db.Exec(query, args...); err != nil {
		t.Fatalf("fixture SQL failed: code=%d", ErrorCode(err))
	}
}

// Capture is a separate, explicit development action on a new database only.
// It never runs as part of normal tests, migration or startup validation.
func TestAlignCapture(t *testing.T) {
	db := newAlignDB(t)
	ctx := context.Background()
	if err := PrepareSchema(ctx, db); err != nil {
		t.Fatal(err)
	}
	previous, err := readSchemaCatalog(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	var steps []schemaStep
	for _, statement := range append(ddlStatements(ragDDL), ddlStatements(tutorDDL)...) {
		mustSQL(t, db, statement)
		current, err := readSchemaCatalog(ctx, db)
		if err != nil {
			t.Fatal(err)
		}
		add := make(schemaCatalogRows, len(current))
		removed := make(schemaCatalogRows, len(current))
		for i, rows := range current {
			old := map[string]bool{}
			for _, row := range previous[i] {
				old[strings.Join(row, "\x00")] = true
			}
			add[i] = [][]string{}
			removed[i] = [][]string{}
			for _, row := range rows {
				key := strings.Join(row, "\x00")
				if old[key] {
					delete(old, key)
				} else {
					add[i] = append(add[i], row)
				}
			}
			// MySQL may replace an implicit FK-support index with a newly
			// declared covering index. Freeze that exact metadata delta too.
			for _, row := range previous[i] {
				if old[strings.Join(row, "\x00")] {
					removed[i] = append(removed[i], row)
				}
			}
		}
		steps = append(steps, schemaStep{fmt.Sprintf("%x", sha256.Sum256([]byte(statement))), add, removed})
		previous = current
	}
	b, _ := json.MarshalIndent(steps, "", "  ")
	if err = os.WriteFile(filepath.Join(os.Getenv("ALIGN_STORAGE_EVIDENCE"), "schema_upgrade_steps.json"), b, 0600); err != nil {
		t.Fatal(err)
	}
	t.Logf("captured %d approved additive DDL catalogs", len(steps))
}

func TestAlignSchemaPaths(t *testing.T) {
	ctx := context.Background()
	for _, start := range []string{"fresh", "v1", "v2"} {
		t.Run(start, func(t *testing.T) {
			db := newAlignDB(t)
			if err := PrepareSchema(ctx, db); err != nil {
				t.Fatal(err)
			}
			if start == "v1" {
				mustSQL(t, db, "INSERT INTO skills(id,name) VALUES(77,'legacy placeholder')")
				report, err := PreflightTutorUpgrade(ctx, db)
				if err != nil || report.FromVersion != 1 || report.UnscopedSkills != 1 {
					t.Fatal("read-only v1 preflight", report, err)
				}
			}
			if start == "v2" {
				if err := prepareRAGDDL(ctx, db); err != nil {
					t.Fatal(err)
				}
				if err := validateRAGV2(ctx, db); err != nil {
					t.Fatal(err)
				}
			}
			if err := PrepareRAGSchema(ctx, db); err != nil {
				t.Fatal(err)
			}
			before, err := readSchemaCatalog(ctx, db)
			if err != nil {
				t.Fatal(err)
			}
			if err = PrepareRAGSchema(ctx, db); err != nil {
				t.Fatal("repeat", err)
			}
			after, _ := readSchemaCatalog(ctx, db)
			if !reflect.DeepEqual(before, after) {
				t.Fatal("repeat changed catalog")
			}
			if err = ValidateRAGSchema(ctx, db); err != nil {
				t.Fatal(err)
			}
			if err = validateRAGV2(ctx, db); err == nil {
				t.Fatal("old v2 strict catalog accepted v3")
			}
			if start == "v1" {
				var name string
				if db.QueryRow("SELECT name FROM skills WHERE id=77 AND class_id IS NULL AND source IS NULL").Scan(&name) != nil || name != "legacy placeholder" {
					t.Fatal("placeholder changed")
				}
			}
		})
	}
}

func TestAlignSchemaInterruptedSteps(t *testing.T) {
	ctx := context.Background()
	stopped := errors.New("controlled crash after DDL commit")
	for stop := 1; stop <= len(ddlStatements(ragDDL))+len(ddlStatements(tutorDDL)); stop++ {
		t.Run(fmt.Sprint(stop), func(t *testing.T) {
			db := newAlignDB(t)
			if err := PrepareSchema(ctx, db); err != nil {
				t.Fatal(err)
			}
			err := prepareTutorSchema(ctx, db, func(step int) error {
				if step == stop {
					return stopped
				}
				return nil
			})
			if !errors.Is(err, stopped) {
				t.Fatal("crash not reached", err)
			}
			if ValidateRAGSchema(ctx, db) == nil {
				t.Fatal("partial upgrade ready")
			}
			if err = PrepareRAGSchema(ctx, db); err != nil {
				t.Fatal("resume", err)
			}
			if err = ValidateRAGSchema(ctx, db); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestAlignSchemaDrift(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct{ name, from, to string }{
		{"column", "source LONGTEXT NULL", "source TEXT NULL"},
		{"index", "tutor_skill_name", "unexpected_skill_name"},
		{"foreign_key", "CONSTRAINT tutor_skill_resource FOREIGN KEY(skill_id,class_id) REFERENCES skills(id,class_id) ON DELETE CASCADE", "CONSTRAINT tutor_skill_resource FOREIGN KEY(skill_id,class_id) REFERENCES skills(id,class_id) ON DELETE RESTRICT"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := newAlignDB(t)
			if err := PrepareSchema(ctx, db); err != nil {
				t.Fatal(err)
			}
			if err := prepareRAGDDL(ctx, db); err != nil {
				t.Fatal(err)
			}
			for _, statement := range ddlStatements(strings.Replace(tutorDDL, tc.from, tc.to, 1)) {
				mustSQL(t, db, statement)
			}
			mustSQL(t, db, "INSERT INTO schema_upgrades VALUES(3,?,1)", len(ddlStatements(tutorDDL)))
			before, _ := readSchemaCatalog(ctx, db)
			if ValidateRAGSchema(ctx, db) == nil || PrepareRAGSchema(ctx, db) == nil {
				t.Fatal("drift accepted/repaired")
			}
			after, _ := readSchemaCatalog(ctx, db)
			if !reflect.DeepEqual(before, after) {
				t.Fatal("rejected drift mutated")
			}
		})
	}
	// v4 is now valid and coexists with v3. Change only the current marker to
	// unknown v5 so the fixture reaches validation instead of a PK collision.
	for _, sql := range []string{"UPDATE schema_upgrades SET version=5 WHERE version=4", "UPDATE schema_upgrades SET ddl_step=0", "UPDATE schema_upgrades SET completed=0"} {
		t.Run(sql, func(t *testing.T) {
			db := newAlignDB(t)
			if err := PrepareSchema(ctx, db); err != nil {
				t.Fatal(err)
			}
			if err := PrepareRAGSchema(ctx, db); err != nil {
				t.Fatal(err)
			}
			mustSQL(t, db, sql)
			if ValidateRAGSchema(ctx, db) == nil {
				t.Fatal("marker drift accepted")
			}
		})
	}
}
