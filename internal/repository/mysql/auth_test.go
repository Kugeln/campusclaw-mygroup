package mysql

import (
	"campusclaw/internal/auth"
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"strings"
	"testing"
)

// A database/sql driver double checks the production query and scan boundary
// without opening any database or running schema/migration commands.
type loginConnector struct {
	t    *testing.T
	mode string
}

func (c loginConnector) Connect(context.Context) (driver.Conn, error) { return loginConn{c}, nil }
func (c loginConnector) Driver() driver.Driver                        { return loginDriver{} }

type loginDriver struct{}

func (loginDriver) Open(string) (driver.Conn, error) { return nil, errors.New("use connector") }

type loginConn struct{ loginConnector }

func (c loginConn) Prepare(string) (driver.Stmt, error) { return nil, errors.New("unexpected prepare") }
func (c loginConn) Close() error                        { return nil }
func (c loginConn) Begin() (driver.Tx, error) {
	c.t.Fatal("login must not write")
	return nil, errors.New("unexpected transaction")
}
func (c loginConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	if !strings.Contains(query, "JOIN classes c ON c.id=u.class_id") || !strings.HasSuffix(query, "WHERE u.username=?") || strings.Contains(query, "sessions") || len(args) != 1 || args[0].Value != "quoted' username" {
		c.t.Fatal("login query must bind credentials and require a valid class")
	}
	if c.mode == "failure" {
		return nil, errors.New("private-driver-error")
	}
	return &loginRows{empty: c.mode == "missing"}, nil
}

type loginRows struct{ empty bool }

func (*loginRows) Columns() []string {
	return []string{"id", "password_hash", "role", "class_id", "session_version"}
}
func (*loginRows) Close() error { return nil }
func (r *loginRows) Next(v []driver.Value) error {
	if r.empty {
		return io.EOF
	}
	r.empty = true
	copy(v, []driver.Value{int64(1), "fixture-hash", "teacher", int64(2), int64(7)})
	return nil
}
func TestLoginUserQuery(t *testing.T) {
	for _, mode := range []string{"found", "missing", "failure"} {
		t.Run(mode, func(t *testing.T) {
			db := sql.OpenDB(loginConnector{t, mode})
			defer db.Close()
			u, found, err := (LoginUsers{DB: db}).FindLoginUser(context.Background(), "quoted' username")
			switch mode {
			case "found":
				if err != nil || !found || u.ID != 1 || u.ClassID != 2 || u.Version != 7 || u.Role != "teacher" || u.PasswordHash != "fixture-hash" {
					t.Fatal("account scan mismatch")
				}
			case "missing":
				if err != nil || found {
					t.Fatal("unknown account mismatch")
				}
			case "failure":
				if err != auth.ErrStorageUnavailable || found {
					t.Fatal("driver error not redacted")
				}
			}
		})
	}
}
