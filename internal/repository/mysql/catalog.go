package mysql

import (
	"context"
	"database/sql"
	_ "embed"
	"encoding/json"
	"fmt"
	"reflect"
)

// Version 1 metadata, recorded from the approved DDL on isolated MySQL 8.4.4.
// AUTO_INCREMENT counters are deliberately excluded: imported IDs change them.
//
//go:embed schema_v1_catalog.json
var schemaCatalog []byte

var catalogQueries = []string{
	`SELECT TABLE_NAME,ENGINE,TABLE_COLLATION FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() ORDER BY TABLE_NAME`,
	`SELECT TABLE_NAME,ORDINAL_POSITION,COLUMN_NAME,COLUMN_TYPE,IS_NULLABLE,COALESCE(COLUMN_DEFAULT,'<NULL>'),EXTRA,COALESCE(COLLATION_NAME,'') FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() ORDER BY TABLE_NAME,ORDINAL_POSITION`,
	`SELECT TABLE_NAME,INDEX_NAME,NON_UNIQUE,SEQ_IN_INDEX,COLUMN_NAME,COALESCE(SUB_PART,0) FROM information_schema.STATISTICS WHERE TABLE_SCHEMA=DATABASE() ORDER BY TABLE_NAME,INDEX_NAME,SEQ_IN_INDEX`,
	`SELECT k.TABLE_NAME,k.CONSTRAINT_NAME,k.ORDINAL_POSITION,k.COLUMN_NAME,k.REFERENCED_TABLE_NAME,k.REFERENCED_COLUMN_NAME,r.UPDATE_RULE,r.DELETE_RULE FROM information_schema.KEY_COLUMN_USAGE k JOIN information_schema.REFERENTIAL_CONSTRAINTS r ON k.CONSTRAINT_SCHEMA=r.CONSTRAINT_SCHEMA AND k.CONSTRAINT_NAME=r.CONSTRAINT_NAME AND k.TABLE_NAME=r.TABLE_NAME WHERE k.CONSTRAINT_SCHEMA=DATABASE() ORDER BY k.TABLE_NAME,k.CONSTRAINT_NAME,k.ORDINAL_POSITION`,
	`SELECT t.TABLE_NAME,c.CONSTRAINT_NAME,c.CHECK_CLAUSE,t.ENFORCED FROM information_schema.CHECK_CONSTRAINTS c JOIN information_schema.TABLE_CONSTRAINTS t ON c.CONSTRAINT_SCHEMA=t.CONSTRAINT_SCHEMA AND c.CONSTRAINT_NAME=t.CONSTRAINT_NAME WHERE c.CONSTRAINT_SCHEMA=DATABASE() ORDER BY t.TABLE_NAME,c.CONSTRAINT_NAME`,
}

// ValidateSchema is read-only; startup never repairs or silently accepts drift.
func ValidateSchema(ctx context.Context, db *sql.DB) error {
	var expected [][][]string
	if err := json.Unmarshal(schemaCatalog, &expected); err != nil {
		return fmt.Errorf("schema catalog unavailable")
	}
	for i, query := range catalogQueries {
		rows, err := db.QueryContext(ctx, query)
		if err != nil {
			return fmt.Errorf("schema metadata unavailable")
		}
		columns, err := rows.Columns()
		if err != nil {
			rows.Close()
			return fmt.Errorf("schema metadata unreadable")
		}
		actual := [][]string{}
		for rows.Next() {
			values := make([]string, len(columns))
			args := make([]any, len(columns))
			for n := range values {
				args[n] = &values[n]
			}
			if err = rows.Scan(args...); err != nil {
				rows.Close()
				return fmt.Errorf("schema metadata unreadable")
			}
			actual = append(actual, values)
		}
		err = rows.Err()
		rows.Close()
		if err != nil || !reflect.DeepEqual(expected[i], actual) {
			return fmt.Errorf("schema version %d mismatch (section %d)", SchemaVersion, i)
		}
	}
	return nil
}
