//go:build alignstorage

package mysql

import (
	"campusclaw/internal/tutor"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
)

const alignMaterialA = "00000000-0000-4000-8000-000000000001"
const alignMaterialB = "00000000-0000-4000-8000-000000000002"

func legacyFixture(t *testing.T) *sql.DB {
	t.Helper()
	db := newAlignDB(t)
	ctx := context.Background()
	if err := PrepareSchema(ctx, db); err != nil {
		t.Fatal(err)
	}
	if err := prepareRAGDDL(ctx, db); err != nil {
		t.Fatal(err)
	}
	if err := validateRAGV2(ctx, db); err != nil {
		t.Fatal(err)
	}
	mustSQL(t, db, "INSERT INTO classes(id,name) VALUES(1,'A'),(2,'B')")
	mustSQL(t, db, "INSERT INTO users(id,username,password_hash,role,class_id,session_version) VALUES(1,'sa','test','student',1,9),(2,'ta','test','teacher',1,4),(3,'sb','test','student',2,8),(4,'tb','test','teacher',2,5)")
	mustSQL(t, db, "INSERT INTO sessions VALUES(?, 'opaque session', 9999999999)", strings.Repeat("a", 64))
	for i, id := range []string{alignMaterialA, alignMaterialB} {
		mustSQL(t, db, "INSERT INTO materials VALUES(?,?,?,'original.txt',8,'text/plain',?,'2026-10-01')", id, i+1, (i+1)*2, id)
		mustSQL(t, db, "INSERT INTO knowledge_entries(id,material_id,class_id,plain_text,created_at,index_status) VALUES(?,?,?,'existing source','2026-10-01','ready')", i+1, id, i+1)
		mustSQL(t, db, "INSERT INTO knowledge_chunks(id,knowledge_entry_id,material_id,class_id,chunk_index,chunk_text,index_status) VALUES(?,?,?,?,1,'existing chunk','ready')", id, i+1, id, i+1)
	}
	mustSQL(t, db, "INSERT INTO skills(id,name,class_id,skill_key,enabled,instruction_version,revision) VALUES(20,?,1,?,1,'1',9),(21,?,2,?,0,'1',3)", tutor.SkillName, tutor.SkillKey, tutor.SkillName, tutor.SkillKey)
	mustSQL(t, db, "INSERT INTO skills(id,name) VALUES(22,?)", strings.Repeat("legacy placeholder ", 5))
	mustSQL(t, db, "INSERT INTO assistants(id,name,class_id,prompt,enabled,revision,updated_by,skill_ids,material_ids) VALUES(10,'A tutor',1,'original role',1,7,2,'[20]',?),(11,'empty',1,'empty role',1,1,2,'[]','[]'),(12,'json null',1,'null role',1,1,2,'null','null'),(13,'sql null managed',1,'managed role',1,1,2,NULL,NULL)", `["`+alignMaterialA+`"]`)
	mustSQL(t, db, "INSERT INTO assistants(id,name) VALUES(99,'legacy assistant placeholder')")
	mustSQL(t, db, "INSERT INTO conversations(id,user_id,class_id,agent_id,title,created_at,updated_at) VALUES(40,1,1,10,'old title','2026-10-01','2026-10-01')")
	mustSQL(t, db, "INSERT INTO conversation_messages(id,conversation_id,class_id,turn_id,sequence_number,role,content,status,result_status,created_at,configuration_snapshot,citations,skill_name,skill_hash) VALUES(50,40,1,?,1,'user','old question','completed','ok','2026-10-01',NULL,JSON_ARRAY(),'',''),(51,40,1,?,2,'assistant','历史回答[1]','completed','ok','2026-10-01',JSON_OBJECT('revision',7),JSON_ARRAY(JSON_OBJECT('number',1,'title','old source')),?,?)", alignMaterialA, alignMaterialA, tutor.SkillName, strings.Repeat("b", 64))
	mustSQL(t, db, "UPDATE conversation_messages SET selected_skill_id=20,skill_id=20,skill_version='1',prompt_snapshot='original role',prompt_revision=7,selection_reason='configured',evidence_snapshot=JSON_ARRAY(JSON_OBJECT('material_id',?,'chunk_index',1,'text','existing chunk')) WHERE id=51", alignMaterialA)
	return db
}

func legacyFingerprint(t *testing.T, db *sql.DB) string {
	t.Helper()
	h := sha256.New()
	queries := []string{
		"SELECT * FROM classes ORDER BY id", "SELECT * FROM users ORDER BY id", "SELECT * FROM sessions ORDER BY sid_hash", "SELECT * FROM materials ORDER BY id", "SELECT * FROM knowledge_entries ORDER BY id", "SELECT * FROM knowledge_chunks ORDER BY id",
		"SELECT id,name,class_id,skill_key,enabled,instruction_version,revision,updated_by,updated_at FROM skills ORDER BY id",
		"SELECT id,name,class_id,prompt,enabled,revision,updated_by,updated_at,skill_ids,material_ids FROM assistants ORDER BY id",
		"SELECT * FROM conversations ORDER BY id", "SELECT * FROM conversation_messages ORDER BY id",
	}
	for _, query := range queries {
		rows, err := db.Query(query)
		if err != nil {
			t.Fatal(err)
		}
		cols, _ := rows.Columns()
		for rows.Next() {
			v := make([]sql.NullString, len(cols))
			a := make([]any, len(cols))
			for i := range v {
				a[i] = &v[i]
			}
			if err = rows.Scan(a...); err != nil {
				t.Fatal(err)
			}
			b, _ := json.Marshal(v)
			h.Write(b)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			t.Fatal(err)
		}
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

func scalar(t *testing.T, db *sql.DB, q string, args ...any) int {
	t.Helper()
	var n int
	if err := db.QueryRow(q, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestAlignLegacyPreservation(t *testing.T) {
	db := legacyFixture(t)
	ctx := context.Background()
	before := legacyFingerprint(t, db)
	report, err := PreflightTutorUpgrade(ctx, db)
	if err != nil || report.Skills != 2 || report.UnscopedSkills != 1 || len(report.EmptyMaterialAgents) != 3 {
		t.Fatal(report, err)
	}
	if err = PrepareRAGSchema(ctx, db); err != nil {
		t.Fatal(err)
	}
	if after := legacyFingerprint(t, db); after != before {
		t.Fatal("legacy identifiers/content/audit/JSON/token versions changed", before, after)
	}
	if scalar(t, db, "SELECT COUNT(*) FROM assistant_skill_bindings") != 1 || scalar(t, db, "SELECT COUNT(*) FROM assistant_material_bindings") != 1 {
		t.Fatal("bindings expanded")
	}
	var source, origin, hash string
	var enabled bool
	var revision int64
	if err = db.QueryRow("SELECT source,source_origin,source_hash,enabled,revision FROM skills WHERE id=20").Scan(&source, &origin, &hash, &enabled, &revision); err != nil {
		t.Fatal(err)
	}
	parsed, err := tutor.ParseSkillSource(source)
	if err != nil || !parsed.Enabled || !enabled || revision != 9 || origin != "system_migration_v3" || parsed.Trigger != tutor.MigrationTrigger || strings.Contains(parsed.Description, "只依据本轮合法本班证据") || hash != fmt.Sprintf("%x", sha256.Sum256([]byte(source))) {
		t.Fatal("migration source", err)
	}
	t.Logf("legacy fingerprint preserved=%s; IDs and all historical fields unchanged", before)
}

func TestAlignLegacyConflicts(t *testing.T) {
	for _, tc := range []struct{ name, sql string }{
		{"unknown_skill", "UPDATE skills SET skill_key='custom' WHERE id=20"},
		{"long_name", "UPDATE skills SET name=REPEAT('长',33) WHERE id=20"},
		{"duplicate_name", "UPDATE skills SET class_id=1,skill_key='other' WHERE id=21"},
		{"wrong_json_type", "UPDATE assistants SET skill_ids=JSON_OBJECT() WHERE id=10"},
		{"foreign_skill", "UPDATE assistants SET skill_ids='[21]' WHERE id=10"},
		{"missing_skill", "UPDATE assistants SET skill_ids='[999]' WHERE id=10"},
		{"duplicate_binding", "UPDATE assistants SET skill_ids='[20,20]' WHERE id=10"},
		{"unknown_scope", "UPDATE assistants SET prompt=NULL,updated_by=NULL WHERE id=10"},
		{"foreign_material", `UPDATE assistants SET material_ids='["` + alignMaterialB + `"]' WHERE id=10`},
		{"active_turn", "UPDATE conversations SET active_turn_id='busy' WHERE id=40"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := legacyFixture(t)
			mustSQL(t, db, tc.sql)
			before := legacyFingerprint(t, db)
			report, err := PreflightTutorUpgrade(context.Background(), db)
			var conflict *MigrationConflictError
			if !errors.As(err, &conflict) || len(report.Conflicts) == 0 {
				t.Fatal("conflict not reported", report, err)
			}
			if err = PrepareRAGSchema(context.Background(), db); !errors.As(err, &conflict) {
				t.Fatal("unsafe upgrade not blocked", err)
			}
			if err = validateRAGV2(context.Background(), db); err != nil {
				t.Fatal("conflict mutated schema", err)
			}
			if legacyFingerprint(t, db) != before {
				t.Fatal("conflict silently repaired")
			}
		})
	}
}

func TestAlignLegacyBackfillRollback(t *testing.T) {
	db := legacyFixture(t)
	ctx := context.Background()
	before := legacyFingerprint(t, db)
	injected := errors.New("backfill interrupted")
	err := prepareTutorSchema(ctx, db, func(step int) error {
		if step == len(ddlStatements(ragDDL))+len(ddlStatements(tutorDDL)) {
			return injected
		}
		return nil
	})
	if !errors.Is(err, injected) {
		t.Fatal(err)
	}
	mustSQL(t, db, "UPDATE schema_upgrades SET ddl_step=?", len(ddlStatements(tutorDDL)))
	if err = backfillTutorLegacy(ctx, db, func() error { return injected }); !errors.Is(err, injected) {
		t.Fatal(err)
	}
	if scalar(t, db, "SELECT COUNT(*) FROM skills WHERE source IS NOT NULL") != 0 || scalar(t, db, "SELECT COUNT(*) FROM assistant_skill_bindings") != 0 || ValidateRAGSchema(ctx, db) == nil {
		t.Fatal("failed backfill committed or ready")
	}
	if err = PrepareRAGSchema(ctx, db); err != nil {
		t.Fatal(err)
	}
	if legacyFingerprint(t, db) != before {
		t.Fatal("retry rewrote history")
	}
	// A completed ledger makes obsolete JSON ineligible for later re-import.
	mustSQL(t, db, "DELETE FROM assistant_skill_bindings WHERE agent_id=10")
	mustSQL(t, db, "UPDATE skills SET source='teacher edited source',source_origin='teacher' WHERE id=20")
	if err = PrepareRAGSchema(ctx, db); err != nil {
		t.Fatal(err)
	}
	var source string
	db.QueryRow("SELECT source FROM skills WHERE id=20").Scan(&source)
	if scalar(t, db, "SELECT COUNT(*) FROM assistant_skill_bindings") != 0 || source != "teacher edited source" {
		t.Fatal("retry resurrected stale bindings or source")
	}
}

func TestAlignLegacyRelationFailureRollsBackAll(t *testing.T) {
	db := legacyFixture(t)
	ctx := context.Background()
	before := legacyFingerprint(t, db)
	stopped := errors.New("before backfill")
	err := prepareTutorSchema(ctx, db, func(step int) error {
		if step == len(ddlStatements(ragDDL))+len(ddlStatements(tutorDDL)) {
			return stopped
		}
		return nil
	})
	if !errors.Is(err, stopped) {
		t.Fatal(err)
	}
	mustSQL(t, db, "UPDATE schema_upgrades SET ddl_step=?", len(ddlStatements(tutorDDL)))
	mustSQL(t, db, "CREATE TRIGGER reject_legacy_material BEFORE INSERT ON assistant_material_bindings FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='controlled backfill rejection'")
	if err = backfillTutorLegacy(ctx, db, nil); err == nil {
		t.Fatal("real relation rejection not reached")
	}
	if scalar(t, db, "SELECT COUNT(*) FROM skills WHERE source IS NOT NULL") != 0 || scalar(t, db, "SELECT COUNT(*) FROM assistant_skill_bindings") != 0 || scalar(t, db, "SELECT COUNT(*) FROM schema_upgrades WHERE completed=1") != 0 || legacyFingerprint(t, db) != before {
		t.Fatal("partial legacy backfill committed")
	}
}

func TestAlignLegacyMarkerTamperingDoesNotReimport(t *testing.T) {
	db := legacyFixture(t)
	ctx := context.Background()
	if err := PrepareRAGSchema(ctx, db); err != nil {
		t.Fatal(err)
	}
	mustSQL(t, db, "UPDATE schema_upgrades SET completed=0")
	before := legacyFingerprint(t, db)
	if err := PrepareRAGSchema(ctx, db); err == nil {
		t.Fatal("inconsistent completion marker silently repaired")
	}
	if legacyFingerprint(t, db) != before {
		t.Fatal("marker tampering overwrote data")
	}
}
