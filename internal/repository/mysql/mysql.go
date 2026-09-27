package mysql

import (
	"campusclaw/internal/config"
	"database/sql"
	"errors"
	driver "github.com/go-sql-driver/mysql"
	"net"
	"time"
)

// ErrorCode exposes only a numeric server diagnostic, never its message or DSN.
func ErrorCode(err error) uint16 {
	var serverError *driver.MySQLError
	if errors.As(err, &serverError) {
		return serverError.Number
	}
	return 0
}

// Open configures the pool; callers must PingContext before serving requests.
func Open(c config.Config) (*sql.DB, error) {
	d := driver.NewConfig()
	d.User = c.DBUser
	d.Passwd = c.DBPassword
	d.Net = "tcp"
	d.Addr = net.JoinHostPort(c.DBHost, c.DBPort)
	d.DBName = c.DBName
	d.Timeout = 3 * time.Second
	d.ReadTimeout = 3 * time.Second
	d.WriteTimeout = 3 * time.Second
	// Strict conversion and explicit zero IDs preserve SQLite import semantics.
	d.Params = map[string]string{"sql_mode": "'STRICT_ALL_TABLES,NO_ZERO_DATE,NO_ZERO_IN_DATE,ERROR_FOR_DIVISION_BY_ZERO,NO_ENGINE_SUBSTITUTION,NO_AUTO_VALUE_ON_ZERO'"}
	// Params are SQL system variables, not DSN options. Charset emits SET NAMES.
	if err := d.Apply(driver.Charset("utf8mb4", "")); err != nil {
		return nil, err
	}
	connector, err := driver.NewConnector(d)
	if err != nil {
		return nil, err
	}
	db := sql.OpenDB(connector)
	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(3 * time.Minute)
	return db, nil
}
