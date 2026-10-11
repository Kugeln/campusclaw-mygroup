package mysql

import (
	"campusclaw/internal/material"
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"strings"
	"testing"
)

type readConnector struct {
	t      *testing.T
	mode   string
	detail bool
}

func (c readConnector) Connect(context.Context) (driver.Conn, error) { return readConn{c}, nil }
func (c readConnector) Driver() driver.Driver                        { return loginDriver{} }

type readConn struct{ readConnector }

func (c readConn) Prepare(string) (driver.Stmt, error) { return nil, errors.New("unexpected prepare") }
func (c readConn) Close() error                        { return nil }
func (c readConn) Begin() (driver.Tx, error)           { return nil, errors.New("unexpected write") }
func (c readConn) QueryContext(_ context.Context, q string, args []driver.NamedValue) (driver.Rows, error) {
	if !strings.Contains(q, "m.class_id=?") || !strings.Contains(q, "JOIN users u ON u.id=m.uploader_id") {
		c.t.Fatal("unscoped query")
	}
	if c.detail {
		if len(args) != 2 || args[0].Value != "quoted'id" || args[1].Value != int64(7) || !strings.Contains(q, "k.class_id=m.class_id") {
			c.t.Fatal("detail scope/binding")
		}
	} else if len(args) != 1 || args[0].Value != int64(7) || !strings.HasSuffix(q, "ORDER BY m.created_at DESC,m.id DESC") {
		c.t.Fatal("list scope/order")
	}
	if c.mode == "query-failure" {
		return nil, errors.New("private database address/password")
	}
	return &readRows{mode: c.mode, detail: c.detail}, nil
}

type readRows struct {
	mode         string
	detail, done bool
}

func (r *readRows) Columns() []string {
	v := []string{"id", "original_name", "size", "created_at", "username"}
	if r.detail {
		v = append(v, "plain_text", "storage_key")
	}
	return v
}
func (r *readRows) Close() error { return nil }
func (r *readRows) Next(v []driver.Value) error {
	if r.mode == "row-failure" {
		return errors.New("private row error")
	}
	if r.done || r.mode == "empty" {
		return io.EOF
	}
	r.done = true
	values := []driver.Value{"uuid", "中文.txt", int64(4), "2026-10-09T00:00:00.000Z", "teacher"}
	if r.mode == "scan-failure" {
		values[2] = "not-a-size"
	}
	if r.detail {
		values = append(values, "text", "key")
	}
	copy(v, values)
	return nil
}
func TestMaterialReadRepository(t *testing.T) {
	for _, detail := range []bool{false, true} {
		for _, mode := range []string{"ok", "empty", "query-failure", "row-failure", "scan-failure"} {
			db := sql.OpenDB(readConnector{t, mode, detail})
			repo := Materials{DB: db}
			if detail {
				v, e := repo.Find(context.Background(), 7, "quoted'id")
				switch mode {
				case "ok":
					if e != nil || v.PlainText != "text" || v.StorageKey != "key" {
						t.Fatal("detail scan")
					}
				case "empty":
					if e != material.ErrNotFound {
						t.Fatal("not found")
					}
				default:
					if e != material.ErrUnavailable {
						t.Fatal("error leak")
					}
				}
			} else {
				v, e := repo.List(context.Background(), 7)
				switch mode {
				case "ok":
					if e != nil || len(v) != 1 || v[0].OriginalName != "中文.txt" {
						t.Fatal("list scan")
					}
				case "empty":
					if e != nil || v == nil || len(v) != 0 {
						t.Fatal("empty array")
					}
				default:
					if e != material.ErrUnavailable || v != nil {
						t.Fatal("partial/error leak")
					}
				}
			}
			db.Close()
		}
	}
	repo := Materials{}
	if _, e := repo.List(context.Background(), 0); e != material.ErrForbidden {
		t.Fatal("invalid class")
	}
	if _, e := repo.Find(context.Background(), 0, "id"); e != material.ErrForbidden {
		t.Fatal("invalid class")
	}
}
