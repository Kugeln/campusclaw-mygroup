package mysql

import (
	"campusclaw/internal/auth"
	"context"
	"database/sql"
	"errors"
)

type LoginUsers struct{ DB *sql.DB }

func (r LoginUsers) FindCurrentUser(ctx context.Context, id int64) (auth.CurrentUser, bool, error) {
	var u auth.CurrentUser
	err := r.DB.QueryRowContext(ctx, `SELECT u.id,u.role,u.class_id,u.session_version,c.name FROM users u JOIN classes c ON c.id=u.class_id WHERE u.id=?`, id).Scan(&u.ID, &u.Role, &u.ClassID, &u.Version, &u.ClassName)
	if errors.Is(err, sql.ErrNoRows) {
		return auth.CurrentUser{}, false, nil
	}
	if err != nil {
		return auth.CurrentUser{}, false, auth.ErrStorageUnavailable
	}
	return u, true, nil
}

func (r LoginUsers) Revoke(ctx context.Context, id int64) error {
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return auth.ErrStorageUnavailable
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE users SET session_version=session_version+1 WHERE id=?`, id)
	if err != nil {
		return auth.ErrStorageUnavailable
	}
	count, err := result.RowsAffected()
	if err != nil || count != 1 {
		return auth.ErrStorageUnavailable
	}
	// A failed/uncertain COMMIT must never be represented as a successful logout.
	if err = tx.Commit(); err != nil {
		return auth.ErrStorageUnavailable
	}
	return nil
}

func (r LoginUsers) FindLoginUser(ctx context.Context, username string) (auth.LoginUser, bool, error) {
	var u auth.LoginUser
	err := r.DB.QueryRowContext(ctx, `SELECT u.id,u.password_hash,u.role,u.class_id,u.session_version FROM users u JOIN classes c ON c.id=u.class_id WHERE u.username=?`, username).Scan(&u.ID, &u.PasswordHash, &u.Role, &u.ClassID, &u.Version)
	if errors.Is(err, sql.ErrNoRows) {
		return auth.LoginUser{}, false, nil
	}
	if err != nil {
		return auth.LoginUser{}, false, auth.ErrStorageUnavailable
	}
	return u, true, nil
}
