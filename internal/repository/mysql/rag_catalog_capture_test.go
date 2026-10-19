//go:build ragcatalog

package mysql

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"testing"
)

// Development-only capture of the approved DDL on a brand-new isolated server.
// This is not a production drift-repair facility and is excluded from builds.
func TestCaptureRAGCatalog(t *testing.T) {
	db, e := sql.Open("mysql", os.Getenv("WEEK5_TEST_DSN"))
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	ctx := context.Background()
	var name string
	db.QueryRow("SELECT DATABASE()").Scan(&name)
	if name != "week5_rag_test" {
		t.Fatal("isolated test database required")
	}
	if e = PrepareSchema(ctx, db); e != nil {
		t.Fatal(e)
	}
	// The complete new catalog is derived from approved DDL, then checked into
	// source and used for all subsequent independent migrations and drift tests.
	if e = prepareRAGDDL(ctx, db); e != nil {
		t.Fatal(e)
	}
	var result [][][]string
	for _, q := range catalogQueries {
		rows, e := db.Query(q)
		if e != nil {
			t.Fatal(e)
		}
		cols, _ := rows.Columns()
		values := [][]string{}
		for rows.Next() {
			v := make([]string, len(cols))
			a := make([]any, len(cols))
			for i := range v {
				a[i] = &v[i]
			}
			if e = rows.Scan(a...); e != nil {
				t.Fatal(e)
			}
			values = append(values, v)
		}
		if e = rows.Err(); e != nil {
			t.Fatal(e)
		}
		rows.Close()
		result = append(result, values)
	}
	b, _ := json.MarshalIndent(result, "", "  ")
	if e = os.WriteFile(os.Getenv("WEEK5_CATALOG_OUTPUT"), b, 0600); e != nil {
		t.Fatal(e)
	}
}
