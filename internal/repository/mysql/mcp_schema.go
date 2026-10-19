package mysql

import (
	"context"
	"crypto/sha256"
	"database/sql"
	_ "embed"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

//go:embed schema_mcp_v4.sql
var mcpDDL string

//go:embed schema_mcp_steps.json
var mcpStepsJSON []byte

func mcpHash(step int) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(strings.Join(ddlStatements(mcpDDL)[:step], "\n"))))
}
func expectedMCPCatalogs() ([]schemaCatalogRows, error) {
	old, e := expectedCatalogs()
	if e != nil {
		return nil, e
	}
	base := old[len(old)-1]
	var steps []schemaStep
	statements := ddlStatements(mcpDDL)
	if json.Unmarshal(mcpStepsJSON, &steps) != nil || len(steps) != len(statements) {
		return nil, fmt.Errorf("MCP upgrade catalogs unavailable")
	}
	out := []schemaCatalogRows{base}
	for i, s := range steps {
		if s.SHA256 != fmt.Sprintf("%x", sha256.Sum256([]byte(statements[i]))) || len(s.Add) != len(base) || len(s.Remove) != len(base) {
			return nil, fmt.Errorf("MCP DDL/catalog hash mismatch")
		}
		next := make(schemaCatalogRows, len(base))
		for k := range base {
			next[k] = [][]string{}
			remove := map[string]bool{}
			for _, row := range s.Remove[k] {
				remove[strings.Join(row, "\x00")] = true
			}
			for _, row := range base[k] {
				key := strings.Join(row, "\x00")
				if remove[key] {
					delete(remove, key)
				} else {
					next[k] = append(next[k], row)
				}
			}
			if len(remove) != 0 {
				return nil, fmt.Errorf("MCP catalog delta invalid")
			}
			next[k] = append(next[k], s.Add[k]...)
		}
		canonicalCatalog(next)
		out = append(out, next)
		base = next
	}
	return out, nil
}
func mcpStage(ctx context.Context, db *sql.DB) (int, error) {
	expected, e := expectedMCPCatalogs()
	if e != nil {
		return 0, e
	}
	actual, e := readSchemaCatalog(ctx, db)
	if e != nil {
		return 0, e
	}
	canonicalCatalog(actual)
	for i, c := range expected {
		if reflect.DeepEqual(actual, c) {
			return i, nil
		}
	}
	return 0, fmt.Errorf("MCP catalog drift; upgrade refused")
}
func checkMCPLedger(ctx context.Context, db *sql.DB, stage int) (bool, error) {
	if stage == 0 {
		return false, validateTutorReady(ctx, db)
	}
	rows, e := db.QueryContext(ctx, "SELECT version,ddl_step,completed,ddl_hash FROM schema_upgrades ORDER BY version")
	if e != nil {
		return false, fmt.Errorf("MCP ledger unavailable")
	}
	defer rows.Close()
	count := 0
	complete := false
	for rows.Next() {
		var version, step int
		var done bool
		var hash sql.NullString
		if rows.Scan(&version, &step, &done, &hash) != nil {
			return false, fmt.Errorf("MCP ledger unreadable")
		}
		count++
		if version == 3 {
			if step != len(ddlStatements(tutorDDL)) || !done || hash.Valid {
				return false, fmt.Errorf("legacy ledger drift")
			}
		} else if version == 4 {
			if step < 1 || step > stage || step < stage-1 || !hash.Valid || hash.String != mcpHash(step) || (done && (stage != len(ddlStatements(mcpDDL)) || step != stage)) {
				return false, fmt.Errorf("MCP ledger drift")
			}
			complete = done
		} else {
			return false, fmt.Errorf("unknown schema version")
		}
	}
	if rows.Err() != nil || count < 1 || count > 2 || (stage > 1 && count != 2) {
		return false, fmt.Errorf("MCP ledger drift")
	}
	// The v3 marker is mandatory, including immediately after first v4 DDL.
	var n int
	if db.QueryRowContext(ctx, "SELECT COUNT(*) FROM schema_upgrades WHERE version=3 AND completed=1").Scan(&n) != nil || n != 1 {
		return false, fmt.Errorf("legacy marker missing")
	}
	return complete, nil
}

// prepareMCPSchema is offline-only. The same named lock spans v1 through v4.
// Hooks are test-only boundaries; a failure never repairs an unknown catalog.
func prepareMCPSchema(ctx context.Context, db *sql.DB, hook func(string, int) error) error {
	// Reject invalid compiled v4 contracts before even advancing the legacy chain.
	if _, err := expectedMCPCatalogs(); err != nil {
		return err
	}
	return schemaUpgradeLock(ctx, db, func() error {
		stage, e := mcpStage(ctx, db)
		if e != nil {
			if _, legacy := schemaStage(ctx, db); legacy != nil {
				return e
			}
			if e = prepareTutorSchemaLocked(ctx, db, nil); e != nil {
				return e
			}
			stage, e = mcpStage(ctx, db)
			if e != nil {
				return e
			}
		}
		if stage == 0 {
			if e = prepareTutorSchemaLocked(ctx, db, nil); e != nil {
				return e
			}
		}
		complete, e := checkMCPLedger(ctx, db, stage)
		if e != nil {
			return e
		}
		if complete {
			return ValidateMCPSchema(ctx, db)
		}
		statements := ddlStatements(mcpDDL)
		boundary := func(phase string, step int) error {
			if hook != nil {
				return hook(phase, step)
			}
			return nil
		}
		// Recover only the single catalog-proven DDL that may precede its marker.
		if stage > 0 {
			if _, e = db.ExecContext(ctx, "INSERT INTO schema_upgrades(version,ddl_step,completed,ddl_hash) VALUES(4,?,0,?) ON DUPLICATE KEY UPDATE ddl_step=VALUES(ddl_step),ddl_hash=VALUES(ddl_hash)", stage, mcpHash(stage)); e != nil {
				return fmt.Errorf("MCP ledger write failed")
			}
		}
		for i := stage; i < len(statements); i++ {
			if e = boundary("before_ddl", i+1); e != nil {
				return e
			}
			if _, e = db.ExecContext(ctx, statements[i]); e != nil {
				return fmt.Errorf("MCP DDL step %d failed (code=%d)", i+1, ErrorCode(e))
			}
			if e = boundary("after_ddl", i+1); e != nil {
				return e
			}
			actual, e := mcpStage(ctx, db)
			if e != nil || actual != i+1 {
				return fmt.Errorf("MCP catalog mismatch")
			}
			if _, e = db.ExecContext(ctx, "INSERT INTO schema_upgrades(version,ddl_step,completed,ddl_hash) VALUES(4,?,0,?) ON DUPLICATE KEY UPDATE ddl_step=VALUES(ddl_step),ddl_hash=VALUES(ddl_hash)", i+1, mcpHash(i+1)); e != nil {
				return fmt.Errorf("MCP ledger write failed")
			}
			if e = boundary("after_ledger", i+1); e != nil {
				return e
			}
		}
		if e = boundary("before_complete", len(statements)); e != nil {
			return e
		}
		if _, e = db.ExecContext(ctx, "UPDATE schema_upgrades SET completed=1 WHERE version=4 AND ddl_step=? AND ddl_hash=?", len(statements), mcpHash(len(statements))); e != nil {
			return fmt.Errorf("MCP completion write failed")
		}
		return ValidateMCPSchema(ctx, db)
	})
}
func ValidateMCPSchema(ctx context.Context, db *sql.DB) error {
	stage, e := mcpStage(ctx, db)
	if e != nil {
		return e
	}
	if stage != len(ddlStatements(mcpDDL)) {
		return fmt.Errorf("MCP schema incomplete")
	}
	complete, e := checkMCPLedger(ctx, db, stage)
	if e != nil {
		return e
	}
	if !complete {
		return fmt.Errorf("MCP schema not ready")
	}
	return validateNgram(ctx, db)
}
func MCPSchemaFingerprint() string { return mcpHash(len(ddlStatements(mcpDDL))) }

// PreflightMCPSchema validates a recognized catalog/ledger without mutations.
func PreflightMCPSchema(ctx context.Context, db *sql.DB) error {
	if _, err := expectedMCPCatalogs(); err != nil {
		return err
	}
	if stage, err := mcpStage(ctx, db); err == nil && stage > 0 {
		_, err = checkMCPLedger(ctx, db, stage)
		return err
	}
	stage, err := schemaStage(ctx, db)
	if err != nil {
		return err
	}
	_, err = checkTutorLedger(ctx, db, stage)
	return err
}
