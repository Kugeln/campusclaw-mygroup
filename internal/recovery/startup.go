// Package recovery coordinates verified startup only, never live maintenance.
package recovery

import (
	repository "campusclaw/internal/repository/mysql"
	"campusclaw/internal/storage"
	"campusclaw/internal/transfer"
	"context"
	"database/sql"
)

func Startup(ctx context.Context, db *sql.DB, root string) error {
	// No file mutation (even creation of tmp/files) before the existing schema,
	// initialization receipt and all referenced contents have passed the gate.
	if e := transfer.Ready(ctx, db, root); e != nil {
		return e
	}
	files, e := storage.OpenPrivate(root)
	if e != nil {
		return e
	}
	defer files.Close()
	return (repository.Materials{DB: db}).RecoverFiles(ctx, files)
}
