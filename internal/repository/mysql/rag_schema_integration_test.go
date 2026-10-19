//go:build ragintegration

package mysql

import (
	"context"
	"database/sql"
	"os"
	"testing"
)

func TestRAGSchemaIntegrity(t *testing.T) {
	db, e := sql.Open("mysql", os.Getenv("WEEK5_TEST_DSN"))
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	ctx := context.Background()
	var name string
	if db.QueryRow("SELECT DATABASE()").Scan(&name) != nil || name != "week5_rag_test" {
		t.Fatal("isolated database required")
	}
	if e = PrepareRAGSchema(ctx, db); e != nil {
		t.Fatal(e)
	}
	var before, after int
	db.QueryRow("SELECT COUNT(*) FROM materials").Scan(&before)
	if e = PrepareRAGSchema(ctx, db); e != nil {
		t.Fatal("repeat", e)
	}
	db.QueryRow("SELECT COUNT(*) FROM materials").Scan(&after)
	if before != after {
		t.Fatal("upgrade changed material data")
	}
	if _, e = db.Exec("INSERT INTO assistants(name) VALUES ('legacy-placeholder')"); e != nil {
		t.Fatal(e)
	}
	var id int64
	db.QueryRow("SELECT MAX(id) FROM assistants").Scan(&id)
	if e = PrepareRAGSchema(ctx, db); e != nil {
		t.Fatal(e)
	}
	var class sql.NullInt64
	var text string
	if db.QueryRow("SELECT name,class_id FROM assistants WHERE id=?", id).Scan(&text, &class) != nil || class.Valid || text != "legacy-placeholder" {
		t.Fatal("legacy placeholder reassigned")
	}
	if _, e = db.Exec("INSERT INTO knowledge_chunks(id,knowledge_entry_id,material_id,class_id,chunk_index,chunk_text) VALUES ('11111111-1111-4111-8111-111111111111',1,'22222222-2222-4222-8222-222222222222',999,1,'foreign')"); e == nil {
		t.Fatal("invalid relation accepted")
	}
	// An added column on a transaction-local temporary table cannot test real
	// metadata drift. Change only this explicitly isolated fixture, then restore
	// its definition; never operate on classroom data.
	if _, e = db.Exec("ALTER TABLE skills ADD COLUMN test_drift INT NULL"); e != nil {
		t.Fatal(e)
	}
	if e = ValidateRAGSchema(ctx, db); e == nil {
		t.Fatal("unknown drift accepted")
	}
	if _, e = db.Exec("ALTER TABLE skills DROP COLUMN test_drift"); e != nil {
		t.Fatal(e)
	}
	if e = ValidateRAGSchema(ctx, db); e != nil {
		t.Fatal("restored catalog", e)
	}
	t.Log("PASS strict RAG catalog, ngram, repeat upgrade, preserved materials/placeholders, invalid FK and drift rejection")
}
