package mysql

import (
	"campusclaw/internal/material"
	"campusclaw/internal/storage"
	"context"
	"database/sql"
)

// RecoverFiles takes current locking reads, rather than a historical consistent
// snapshot: an in-flight COMMIT must settle before a UUID is declared orphaned.
// It is only called after the initialization gate and before the sole listener.
func (r Materials) RecoverFiles(ctx context.Context, files *storage.PrivateStore) error {
	tx, e := r.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if e != nil {
		return material.ErrUnavailable
	}
	defer tx.Rollback()
	rows, e := tx.QueryContext(ctx, `SELECT id,class_id,storage_key,size FROM materials ORDER BY id FOR UPDATE`)
	if e != nil {
		return material.ErrUnavailable
	}
	type meta struct {
		class int64
		ref   storage.Reference
	}
	records := map[string]meta{}
	for rows.Next() {
		var id string
		var v meta
		if rows.Scan(&id, &v.class, &v.ref.Key, &v.ref.Size) != nil {
			rows.Close()
			return material.ErrUnavailable
		}
		records[id] = v
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return material.ErrUnavailable
	}
	rows, e = tx.QueryContext(ctx, `SELECT material_id,class_id,plain_text FROM knowledge_entries ORDER BY material_id FOR UPDATE`)
	if e != nil {
		return material.ErrUnavailable
	}
	refs := []storage.Reference{}
	for rows.Next() {
		var id, text string
		var class int64
		if rows.Scan(&id, &class, &text) != nil {
			rows.Close()
			return material.ErrUnavailable
		}
		v, ok := records[id]
		if !ok || class != v.class {
			rows.Close()
			return storage.ErrRecovery
		}
		v.ref.Text = text
		refs = append(refs, v.ref)
		delete(records, id)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return material.ErrUnavailable
	}
	if len(records) != 0 {
		return storage.ErrRecovery
	}
	confirm := func() error { var n int; return tx.QueryRowContext(ctx, `SELECT 1`).Scan(&n) }
	if e = files.Recover(refs, confirm); e != nil {
		return e
	}
	// No database data is modified. Rollback releases all locks; no commit result
	// is used as permission to delete files.
	if tx.Rollback() != nil {
		return material.ErrUnavailable
	}
	return nil
}
