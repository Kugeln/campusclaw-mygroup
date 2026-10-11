package mysql

import (
	"campusclaw/internal/material"
	"context"
	"database/sql"
	"errors"
)

type Materials struct {
	DB        *sql.DB
	Reconnect func() (*sql.DB, error)
}
type materialTx struct {
	tx  *sql.Tx
	ctx context.Context
}

func (r Materials) Begin(ctx context.Context) (material.Transaction, error) {
	tx, e := r.DB.BeginTx(ctx, nil)
	if e != nil {
		return nil, material.ErrUnavailable
	}
	return &materialTx{tx, ctx}, nil
}
func (t *materialTx) Insert(r material.Record) error {
	_, e := t.tx.ExecContext(t.ctx, `INSERT INTO materials(id,class_id,uploader_id,original_name,size,content_type,storage_key,created_at) VALUES (?,?,?,?,?,'text/plain',?,?)`, r.ID, r.ClassID, r.UploaderID, r.Name, r.Size, r.ID, r.CreatedAt)
	if e != nil {
		return material.ErrUnavailable
	}
	_, e = t.tx.ExecContext(t.ctx, `INSERT INTO knowledge_entries(material_id,class_id,plain_text,created_at) VALUES (?,?,?,?)`, r.ID, r.ClassID, r.Text, r.CreatedAt)
	if e != nil {
		return material.ErrUnavailable
	}
	return nil
}
func (t *materialTx) Commit() error   { return t.tx.Commit() }
func (t *materialTx) Rollback() error { return t.tx.Rollback() }

func (r Materials) Resolve(ctx context.Context, want material.Record) (material.Outcome, error) {
	if r.Reconnect == nil {
		return material.Unknown, material.ErrUnavailable
	}
	db, e := r.Reconnect()
	if e != nil {
		return material.Unknown, material.ErrUnavailable
	}
	defer db.Close()
	tx, e := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if e != nil {
		return material.Unknown, material.ErrUnavailable
	}
	defer tx.Rollback()
	var id, name, mime, key, created string
	var class, user, size int64
	// Locking read waits for the original inserting transaction to settle. A
	// nonlocking SELECT could incorrectly see absence while COMMIT is in flight.
	e = tx.QueryRowContext(ctx, `SELECT id,class_id,uploader_id,original_name,size,content_type,storage_key,created_at FROM materials WHERE id=? FOR UPDATE`, want.ID).Scan(&id, &class, &user, &name, &size, &mime, &key, &created)
	missing := errors.Is(e, sql.ErrNoRows)
	if e != nil && !missing {
		return material.Unknown, material.ErrUnavailable
	}
	var text, kcreated string
	var kclass int64
	ke := tx.QueryRowContext(ctx, `SELECT class_id,plain_text,created_at FROM knowledge_entries WHERE material_id=? FOR UPDATE`, want.ID).Scan(&kclass, &text, &kcreated)
	kmissing := errors.Is(ke, sql.ErrNoRows)
	if ke != nil && !kmissing {
		return material.Unknown, material.ErrUnavailable
	}
	if missing && kmissing {
		return material.Absent, nil
	}
	if missing || kmissing || id != want.ID || class != want.ClassID || user != want.UploaderID || name != want.Name || size != want.Size || mime != "text/plain" || key != want.ID || created != want.CreatedAt || kclass != want.ClassID || text != want.Text || kcreated != want.CreatedAt {
		return material.Unknown, nil
	}
	return material.Committed, nil
}
