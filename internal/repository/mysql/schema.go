package mysql

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
	"strings"
)

// SchemaVersion is the explicit offline schema preparation version.
const SchemaVersion = 1

//go:embed schema_v1.sql
var schemaV1 string

// PrepareSchema is an administrative operation, never called by HTTP startup.
// DDL is deliberately separate from the subsequent data transaction.
func PrepareSchema(ctx context.Context, db *sql.DB) error {
	var padding string
	if err := db.QueryRowContext(ctx, "SELECT PAD_ATTRIBUTE FROM information_schema.COLLATIONS WHERE COLLATION_NAME='utf8mb4_0900_bin'").Scan(&padding); err != nil || padding != "NO PAD" {
		return fmt.Errorf("required collation unavailable")
	}
	var packet int64
	if err := db.QueryRowContext(ctx, "SELECT @@max_allowed_packet").Scan(&packet); err != nil || packet < 32*1024*1024 {
		return fmt.Errorf("max_allowed_packet must be at least 32 MiB")
	}
	for _, statement := range strings.Split(schemaV1, ";") {
		if strings.TrimSpace(statement) == "" {
			continue
		}
		if _, err := db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("schema version 1 preparation failed (code=%d)", ErrorCode(err))
		}
	}
	return ValidateSchema(ctx, db)
}
