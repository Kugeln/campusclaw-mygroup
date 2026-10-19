package mysql

import (
	"context"
	"crypto/sha256"
	"database/sql"
	_ "embed"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
)

//go:embed schema_tutor_v3.sql
var tutorDDL string

// The approved additive catalog delta of every v1->v2->v3 DDL statement.
// Captured only from an empty, dedicated test database, never from a live target.
//
//go:embed schema_upgrade_steps.json
var upgradeStepsJSON []byte

type schemaCatalogRows [][][]string
type schemaStep struct {
	SHA256 string            `json:"sha256"`
	Add    schemaCatalogRows `json:"add"`
	Remove schemaCatalogRows `json:"remove"`
}

func ddlStatements(ddl string) []string {
	out := []string{}
	for _, s := range strings.Split(ddl, ";") {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func readSchemaCatalog(ctx context.Context, db *sql.DB) (schemaCatalogRows, error) {
	out := make(schemaCatalogRows, len(catalogQueries))
	for i, q := range catalogQueries {
		rows, err := db.QueryContext(ctx, q)
		if err != nil {
			return nil, fmt.Errorf("schema metadata unavailable")
		}
		cols, err := rows.Columns()
		if err != nil {
			rows.Close()
			return nil, fmt.Errorf("schema metadata unreadable")
		}
		out[i] = [][]string{}
		for rows.Next() {
			v := make([]string, len(cols))
			a := make([]any, len(cols))
			for k := range v {
				a[k] = &v[k]
			}
			if err = rows.Scan(a...); err != nil {
				rows.Close()
				return nil, fmt.Errorf("schema metadata unreadable")
			}
			out[i] = append(out[i], v)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, fmt.Errorf("schema metadata unreadable")
		}
	}
	return out, nil
}

func canonicalCatalog(c schemaCatalogRows) {
	for _, rows := range c {
		sort.Slice(rows, func(i, j int) bool { return strings.Join(rows[i], "\x00") < strings.Join(rows[j], "\x00") })
	}
}

func expectedCatalogs() ([]schemaCatalogRows, error) {
	var base schemaCatalogRows
	var steps []schemaStep
	statements := append(ddlStatements(ragDDL), ddlStatements(tutorDDL)...)
	if json.Unmarshal(schemaCatalog, &base) != nil || json.Unmarshal(upgradeStepsJSON, &steps) != nil || len(steps) != len(statements) {
		return nil, fmt.Errorf("upgrade catalogs unavailable")
	}
	canonicalCatalog(base)
	out := []schemaCatalogRows{base}
	for i, step := range steps {
		if step.SHA256 != fmt.Sprintf("%x", sha256.Sum256([]byte(statements[i]))) || len(step.Add) != len(base) || len(step.Remove) != len(base) {
			return nil, fmt.Errorf("upgrade DDL/catalog mismatch")
		}
		next := make(schemaCatalogRows, len(base))
		for section := range base {
			removed := map[string]bool{}
			for _, row := range step.Remove[section] {
				removed[strings.Join(row, "\x00")] = true
			}
			next[section] = [][]string{}
			for _, row := range base[section] {
				key := strings.Join(row, "\x00")
				if removed[key] {
					delete(removed, key)
				} else {
					next[section] = append(next[section], row)
				}
			}
			if len(removed) != 0 {
				return nil, fmt.Errorf("invalid upgrade catalog delta")
			}
			next[section] = append(next[section], step.Add[section]...)
		}
		canonicalCatalog(next)
		out = append(out, next)
		base = next
	}
	return out, nil
}

func schemaStage(ctx context.Context, db *sql.DB) (int, error) {
	expected, err := expectedCatalogs()
	if err != nil {
		return 0, err
	}
	actual, err := readSchemaCatalog(ctx, db)
	if err != nil {
		return 0, err
	}
	canonicalCatalog(actual)
	for stage, catalog := range expected {
		if reflect.DeepEqual(catalog, actual) {
			return stage, nil
		}
	}
	return 0, fmt.Errorf("schema catalog drift; upgrade refused")
}

func validateTutorStructure(ctx context.Context, db *sql.DB) error {
	stage, err := schemaStage(ctx, db)
	if err != nil {
		return err
	}
	if stage != len(ddlStatements(ragDDL))+len(ddlStatements(tutorDDL)) {
		return fmt.Errorf("incomplete schema upgrade")
	}
	return validateNgram(ctx, db)
}

func validateTutorReady(ctx context.Context, db *sql.DB) error {
	if err := validateTutorStructure(ctx, db); err != nil {
		return err
	}
	var count int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM schema_upgrades WHERE version=3 AND ddl_step=? AND completed=1", len(ddlStatements(tutorDDL))).Scan(&count); err != nil || count != 1 {
		return fmt.Errorf("schema v3 backfill not ready")
	}
	if db.QueryRowContext(ctx, "SELECT COUNT(*) FROM schema_upgrades").Scan(&count) != nil || count != 1 {
		return fmt.Errorf("schema version marker drift")
	}
	return nil
}

func checkTutorLedger(ctx context.Context, db *sql.DB, stage int) (bool, error) {
	legacyCount := len(ddlStatements(ragDDL))
	if stage <= legacyCount {
		return false, nil
	}
	var version, step int
	var completed bool
	rows, err := db.QueryContext(ctx, "SELECT version,ddl_step,completed FROM schema_upgrades")
	if err != nil {
		return false, fmt.Errorf("schema ledger unavailable")
	}
	count := 0
	for rows.Next() {
		count++
		if rows.Scan(&version, &step, &completed) != nil {
			rows.Close()
			return false, fmt.Errorf("schema ledger unreadable")
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil || count > 1 || (count == 0 && stage != legacyCount+1) {
		return false, fmt.Errorf("schema ledger drift")
	}
	if count == 1 && (version != 3 || step < 1 || step > stage-legacyCount || step < stage-legacyCount-1) {
		return false, fmt.Errorf("schema ledger drift")
	}
	return completed, nil
}

// prepareTutorSchema runs only in explicit offline administration. The caller
// must stop writes and verify backup/target identity. It never repairs drift.
func prepareTutorSchema(ctx context.Context, db *sql.DB, afterDDL func(int) error) error {
	return schemaUpgradeLock(ctx, db, func() error { return prepareTutorSchemaLocked(ctx, db, afterDDL) })
}

func schemaUpgradeLock(ctx context.Context, db *sql.DB, apply func() error) error {
	conn, err := db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("schema lock unavailable")
	}
	defer conn.Close()
	var name string
	var lock int
	if conn.QueryRowContext(ctx, "SELECT DATABASE()").Scan(&name) != nil || name == "" {
		return fmt.Errorf("explicit database required")
	}
	key := fmt.Sprintf("campusclaw-schema-%x", sha256.Sum256([]byte(name)))[:60]
	if conn.QueryRowContext(ctx, "SELECT GET_LOCK(?,0)", key).Scan(&lock) != nil || lock != 1 {
		return fmt.Errorf("schema upgrade busy")
	}
	defer conn.ExecContext(context.Background(), "SELECT RELEASE_LOCK(?)", key)
	return apply()
}

func prepareTutorSchemaLocked(ctx context.Context, db *sql.DB, afterDDL func(int) error) error {
	stage, err := schemaStage(ctx, db)
	if err != nil {
		return err
	}
	legacyCount := len(ddlStatements(ragDDL))
	var ngram int
	if db.QueryRowContext(ctx, "SELECT @@ngram_token_size").Scan(&ngram) != nil || ngram != 2 {
		return fmt.Errorf("RAG requires ngram_token_size=2")
	}
	all := append(ddlStatements(ragDDL), ddlStatements(tutorDDL)...)
	if stage >= legacyCount {
		if err = validateNgram(ctx, db); err != nil {
			return err
		}
	}
	complete, err := checkTutorLedger(ctx, db, stage)
	if err != nil {
		return err
	}
	if complete {
		return validateTutorReady(ctx, db)
	}
	// Preflight before the first v3 mutation, and again on every partial retry.
	if stage >= legacyCount {
		if _, err = preflightTutorLegacy(ctx, db); err != nil {
			return err
		}
	}
	for i := stage; i < len(all); i++ {
		if i == legacyCount {
			if _, err = preflightTutorLegacy(ctx, db); err != nil {
				return err
			}
		}
		if _, err = db.ExecContext(ctx, all[i]); err != nil {
			return fmt.Errorf("schema upgrade step %d failed (code=%d)", i+1, ErrorCode(err))
		}
		// A process may die immediately after implicit DDL commit. The next run
		// identifies its exact catalog, without assuming the ledger was updated.
		if afterDDL != nil {
			if err = afterDDL(i + 1); err != nil {
				return err
			}
		}
		if i >= legacyCount {
			if _, err = db.ExecContext(ctx, "INSERT INTO schema_upgrades(version,ddl_step,completed) VALUES(3,?,0) ON DUPLICATE KEY UPDATE ddl_step=VALUES(ddl_step)", i+1-legacyCount); err != nil {
				return fmt.Errorf("schema ledger write failed")
			}
		}
		actual, e := schemaStage(ctx, db)
		if e != nil || actual != i+1 {
			return fmt.Errorf("schema upgrade catalog mismatch at step %d", i+1)
		}
	}
	// Handles a crash after the last DDL but before its ledger write as well.
	if _, err = db.ExecContext(ctx, "INSERT INTO schema_upgrades(version,ddl_step,completed) VALUES(3,?,0) ON DUPLICATE KEY UPDATE ddl_step=VALUES(ddl_step)", len(ddlStatements(tutorDDL))); err != nil {
		return fmt.Errorf("schema ledger write failed")
	}
	if err = backfillTutorLegacy(ctx, db, nil); err != nil {
		return err
	}
	return validateTutorReady(ctx, db)
}
