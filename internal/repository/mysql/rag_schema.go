package mysql

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
	"strings"
)

//go:embed schema_rag.sql
var ragDDL string

const RAGSchemaVersion = 4

// PrepareRAGSchema is explicit offline administration. HTTP startup never runs DDL.
// MySQL DDL is not atomic: every addition checks its existence on retry, and the
// strict complete catalog is required before the application can start.
func PrepareRAGSchema(ctx context.Context, db *sql.DB) error {
	return prepareMCPSchema(ctx, db, nil)
}
func prepareRAGDDL(ctx context.Context, db *sql.DB) error {
	var size int
	if e := db.QueryRowContext(ctx, "SELECT @@ngram_token_size").Scan(&size); e != nil || size != 2 {
		return fmt.Errorf("RAG requires ngram_token_size=2")
	}
	for _, statement := range strings.Split(ragDDL, ";") {
		statement = strings.TrimSpace(statement)
		if statement == "" {
			continue
		}
		if strings.HasPrefix(statement, "ALTER TABLE ") {
			fields := strings.Fields(statement)
			table, name := fields[2], fields[5]
			var count int
			query := "SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME=? AND COLUMN_NAME=?"
			if fields[4] == "INDEX" || fields[4] == "UNIQUE" {
				query = "SELECT COUNT(*) FROM information_schema.STATISTICS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME=? AND INDEX_NAME=?"
			}
			if fields[4] == "CONSTRAINT" {
				query = "SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS WHERE CONSTRAINT_SCHEMA=DATABASE() AND TABLE_NAME=? AND CONSTRAINT_NAME=?"
			}
			if e := db.QueryRowContext(ctx, query, table, name).Scan(&count); e != nil {
				return fmt.Errorf("RAG schema metadata unavailable")
			}
			if count > 0 {
				continue
			}
		}
		if _, e := db.ExecContext(ctx, statement); e != nil {
			return fmt.Errorf("RAG schema preparation failed (code=%d)", ErrorCode(e))
		}
	}
	return nil
}

// ValidateRAGSchema refuses incomplete upgrades and verifies the ngram parser.
func ValidateRAGSchema(ctx context.Context, db *sql.DB) error {
	return ValidateMCPSchema(ctx, db)
}

func validateRAGV2(ctx context.Context, db *sql.DB) error {
	if err := validateCatalog(ctx, db, ragCatalog); err != nil {
		return err
	}
	return validateNgram(ctx, db)
}

func validateNgram(ctx context.Context, db *sql.DB) error {
	var size int
	var table, ddl string
	if db.QueryRowContext(ctx, "SELECT @@ngram_token_size").Scan(&size) != nil || size != 2 {
		return fmt.Errorf("RAG requires ngram_token_size=2")
	}
	if db.QueryRowContext(ctx, "SHOW CREATE TABLE knowledge_chunks").Scan(&table, &ddl) != nil || !strings.Contains(strings.ToLower(ddl), "with parser `ngram`") {
		return fmt.Errorf("RAG fulltext parser unavailable")
	}
	return nil
}
