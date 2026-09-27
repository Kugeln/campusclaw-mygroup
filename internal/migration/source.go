// Package migration is used only by the offline migration command, never server.
package migration

import (
	"campusclaw/internal/transfer"
	"context"
	"database/sql"
	_ "embed"
	"fmt"
	_ "modernc.org/sqlite"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

//go:embed sqlite_schema.sql
var sourceSchema string

//go:embed sqlite_sessions.sql
var sourceSessions string

func catalog(ctx context.Context, db *sql.DB) (map[string]string, error) {
	rows, err := db.QueryContext(ctx, "SELECT type,name,sql FROM sqlite_master WHERE sql IS NOT NULL AND name NOT LIKE 'sqlite_%' ORDER BY name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := map[string]string{}
	for rows.Next() {
		var kind, name, statement string
		if err = rows.Scan(&kind, &name, &statement); err != nil {
			return nil, err
		}
		statement = strings.ToLower(strings.Join(strings.Fields(statement), ""))
		statement = strings.ReplaceAll(statement, "ifnotexists", "")
		result[kind+":"+name] = statement
	}
	return result, rows.Err()
}
func Source(ctx context.Context, path, root string) (transfer.Snapshot, error) {
	var empty transfer.Snapshot
	absolute, err := filepath.Abs(path)
	if err != nil {
		return empty, fmt.Errorf("invalid source path")
	}
	info, err := os.Lstat(absolute)
	if err != nil || !info.Mode().IsRegular() {
		return empty, fmt.Errorf("source must be a regular snapshot file")
	}
	before, err := os.ReadFile(absolute)
	if err != nil {
		return empty, fmt.Errorf("source fingerprint failed")
	}
	beforeHash := transfer.Hash(before)
	for _, suffix := range []string{"-wal", "-shm", "-journal"} {
		if _, err = os.Lstat(absolute + suffix); !os.IsNotExist(err) {
			return empty, fmt.Errorf("source must be an offline checkpointed snapshot")
		}
	}
	uri := url.URL{Scheme: "file", Path: filepath.ToSlash(absolute)}
	query := uri.Query()
	query.Set("mode", "ro")
	query.Add("_pragma", "query_only(1)")
	uri.RawQuery = query.Encode()
	db, err := sql.Open("sqlite", uri.String())
	if err != nil {
		return empty, fmt.Errorf("source open failed")
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	expected, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		return empty, err
	}
	defer expected.Close()
	if _, err = expected.ExecContext(ctx, sourceSchema+sourceSessions); err != nil {
		return empty, fmt.Errorf("reference schema unavailable")
	}
	want, err := catalog(ctx, expected)
	if err != nil {
		return empty, err
	}
	got, err := catalog(ctx, db)
	if err != nil {
		return empty, fmt.Errorf("source schema unreadable")
	}
	if len(got) != len(want) {
		return empty, fmt.Errorf("source schema differs from approved eight-table schema")
	}
	for k, v := range want {
		if got[k] != v {
			return empty, fmt.Errorf("source schema mismatch: %s", k)
		}
	}
	var integrity string
	if err = db.QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&integrity); err != nil || integrity != "ok" {
		return empty, fmt.Errorf("source integrity check failed")
	}
	rows, err := db.QueryContext(ctx, "PRAGMA foreign_key_check")
	if err != nil {
		return empty, fmt.Errorf("source foreign key check failed")
	}
	bad := rows.Next()
	rows.Close()
	if bad {
		return empty, fmt.Errorf("source foreign key violation")
	}
	if err = rows.Err(); err != nil {
		return empty, fmt.Errorf("source foreign key check incomplete")
	}
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return empty, fmt.Errorf("source read transaction failed")
	}
	defer tx.Rollback()
	snapshot, err := transfer.Read(ctx, tx)
	if err != nil {
		return empty, err
	}
	if err = snapshot.Validate(root); err != nil {
		return empty, err
	}
	bytes, err := os.ReadFile(absolute)
	if err != nil {
		return empty, fmt.Errorf("source fingerprint failed")
	}
	snapshot.SourceSHA256 = transfer.Hash(bytes)
	if snapshot.SourceSHA256 != beforeHash {
		return empty, fmt.Errorf("source snapshot changed during preflight")
	}
	return snapshot, nil
}
