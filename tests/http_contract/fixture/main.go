// Fixture-only executable. Never used by the application or real migration.
package main

import (
	"campusclaw/internal/config"
	"campusclaw/internal/provision"
	repository "campusclaw/internal/repository/mysql"
	"campusclaw/internal/transfer"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func run() error {
	fail := func() error { return fmt.Errorf("isolated contract fixture preparation failed") }
	if os.Getenv("PHASE5_CONTRACT_ISOLATED") != "1" || os.Getenv("DB_HOST") != "127.0.0.1" || !strings.HasPrefix(os.Getenv("DB_NAME"), "phase5_contract_") {
		return fail()
	}
	c, e := config.Load(os.Getenv)
	if e != nil {
		return fail()
	}
	password := os.Getenv("CONTRACT_PASSWORD")
	if len(password) < 32 {
		return fail()
	}
	var f struct {
		Created string `json:"created_at"`
		Mtime   int64  `json:"mtime"`
		Classes []struct {
			ID   int64
			Name string
		}
		Users []struct {
			ID             int64
			Username, Role string
			Class          int64 `json:"class_id"`
		}
		Materials []struct {
			ID, Name, Raw string
			Class         int64 `json:"class_id"`
			User          int64 `json:"uploader_id"`
		}
	}
	b, e := os.ReadFile(os.Getenv("CONTRACT_FIXTURES"))
	if e != nil || json.Unmarshal(b, &f) != nil {
		return fail()
	}
	db, e := repository.Open(c)
	if e != nil {
		return fail()
	}
	defer db.Close()
	ctx := context.Background()
	var n int
	if db.QueryRow("SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=DATABASE()").Scan(&n) != nil || n != 0 {
		return fmt.Errorf("refuse nonempty contract target")
	}
	if _, e = os.Stat(c.UploadRoot); !os.IsNotExist(e) {
		return fmt.Errorf("refuse existing contract upload root")
	}
	if repository.PrepareSchema(ctx, db) != nil {
		return fail()
	}
	for _, dir := range []string{"tmp", "files"} {
		if os.MkdirAll(filepath.Join(c.UploadRoot, dir), 0700) != nil {
			return fail()
		}
	}
	hash, e := provision.PasswordHash(password)
	if e != nil {
		return fail()
	}
	tx, e := db.BeginTx(ctx, nil)
	if e != nil {
		return fail()
	}
	defer tx.Rollback()
	for _, v := range f.Classes {
		if _, e = tx.Exec("INSERT INTO classes(id,name) VALUES (?,?)", v.ID, v.Name); e != nil {
			return fail()
		}
	}
	for _, v := range f.Users {
		if _, e = tx.Exec("INSERT INTO users(id,username,password_hash,role,class_id,session_version) VALUES (?,?,?,?,?,0)", v.ID, v.Username, hash, v.Role, v.Class); e != nil {
			return fail()
		}
	}
	for i, v := range f.Materials {
		raw := []byte(v.Raw)
		path := filepath.Join(c.UploadRoot, "files", v.ID)
		if os.WriteFile(path, raw, 0600) != nil || os.Chtimes(path, time.Unix(f.Mtime, 0), time.Unix(f.Mtime, 0)) != nil {
			return fail()
		}
		if _, e = tx.Exec("INSERT INTO materials(id,class_id,uploader_id,original_name,size,content_type,storage_key,created_at) VALUES (?,?,?,?,?,'text/plain',?,?)", v.ID, v.Class, v.User, v.Name, len(raw), v.ID, f.Created); e != nil {
			return fail()
		}
		if _, e = tx.Exec("INSERT INTO knowledge_entries(id,material_id,class_id,plain_text,created_at) VALUES (?,?,?,?,?)", i+1, v.ID, v.Class, strings.TrimPrefix(v.Raw, "\ufeff"), f.Created); e != nil {
			return fail()
		}
	}
	if tx.Commit() != nil {
		return fail()
	}
	s, e := transfer.Read(ctx, db)
	if e != nil || s.Validate(c.UploadRoot) != nil {
		return fail()
	}
	if transfer.WriteReport(filepath.Join(c.UploadRoot, transfer.ReceiptName), s.Report("fresh", true)) != nil {
		return fail()
	}
	return nil
}
func main() {
	if e := run(); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
	fmt.Println("PASS isolated equivalent fixture")
}
