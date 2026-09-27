// Package provision creates offline seed data and accounts, not login sessions.
package provision

import (
	"campusclaw/internal/transfer"
	"context"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"
	"unicode/utf8"
)

func PasswordHash(password string) (string, error) {
	if password == "" {
		return "", fmt.Errorf("password environment value required")
	}
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("random source failed")
	}
	encoded := hex.EncodeToString(salt)
	key, err := pbkdf2.Key(sha256.New, password, []byte(encoded), 1000000, 32)
	if err != nil {
		return "", fmt.Errorf("password hashing failed")
	}
	return "pbkdf2:sha256:1000000$" + encoded + "$" + hex.EncodeToString(key), nil
}
func CreateClass(ctx context.Context, db *sql.DB, name string) (int64, error) {
	if name == "" || !utf8.ValidString(name) || utf8.RuneCountInString(name) > 255 {
		return 0, fmt.Errorf("invalid class name")
	}
	var id int64
	err := db.QueryRowContext(ctx, "SELECT id FROM classes WHERE name=?", name).Scan(&id)
	if err == nil {
		return id, nil
	}
	if err != sql.ErrNoRows {
		return 0, fmt.Errorf("class lookup failed")
	}
	r, err := db.ExecContext(ctx, "INSERT INTO classes(name) VALUES (?)", name)
	if err != nil {
		return 0, fmt.Errorf("class creation failed")
	}
	return r.LastInsertId()
}
func CreateUser(ctx context.Context, db *sql.DB, name, password, role string, classID int64) (int64, error) {
	if name == "" || !utf8.ValidString(name) || utf8.RuneCountInString(name) > 255 || (role != "teacher" && role != "student") || classID <= 0 {
		return 0, fmt.Errorf("invalid account fields")
	}
	hash, err := PasswordHash(password)
	if err != nil {
		return 0, err
	}
	r, err := db.ExecContext(ctx, "INSERT INTO users(username,password_hash,role,class_id) VALUES (?,?,?,?)", name, hash, role, classID)
	if err != nil {
		return 0, fmt.Errorf("account creation failed")
	}
	return r.LastInsertId()
}
func Fresh(ctx context.Context, db *sql.DB, root string, get func(string) string) (transfer.Report, error) {
	return fresh(ctx, db, root, get, nil)
}
func fresh(ctx context.Context, db *sql.DB, root string, get func(string) string, hook func() error) (transfer.Report, error) {
	receipt, e := transfer.Receipt(root)
	if e == nil {
		if receipt.Mode != "fresh" {
			return transfer.Report{}, fmt.Errorf("fresh cannot operate on migrated target")
		}
		if err := transfer.Ready(ctx, db, root); err != nil {
			return transfer.Report{}, err
		}
		current, err := transfer.Read(ctx, db)
		if err != nil {
			return transfer.Report{}, err
		}
		if current.Empty() {
			return transfer.Report{}, fmt.Errorf("initialized data missing")
		}
		if err = current.Validate(root); err != nil {
			return transfer.Report{}, err
		}
		return receipt, nil // Do not reset existing accounts, passwords, sessions or files.
	}
	if !os.IsNotExist(e) {
		return transfer.Report{}, e
	}
	current, e := transfer.Read(ctx, db)
	if e != nil {
		return transfer.Report{}, e
	}
	if !current.Empty() {
		return transfer.RecoverFreshReceipt(ctx, db, root)
	}
	stage, e := os.MkdirTemp("", "campusclaw-fresh-")
	if e != nil {
		return transfer.Report{}, fmt.Errorf("seed staging failed")
	}
	defer os.RemoveAll(stage)
	if e = os.Mkdir(filepath.Join(stage, "files"), 0700); e != nil {
		return transfer.Report{}, e
	}
	s := transfer.Snapshot{Rows: map[string][]transfer.Row{}}
	for _, t := range transfer.Tables {
		s.Rows[t.Name] = []transfer.Row{}
	}
	s.Rows["classes"] = []transfer.Row{{"id": "1", "name": "A"}, {"id": "2", "name": "B"}}
	accounts := []struct{ name, role, class, key string }{{"teacher_a", "teacher", "1", "SEED_TEACHER_A_PASSWORD"}, {"student_a1", "student", "1", "SEED_STUDENT_A1_PASSWORD"}, {"teacher_b", "teacher", "2", "SEED_TEACHER_B_PASSWORD"}, {"student_b1", "student", "2", "SEED_STUDENT_B1_PASSWORD"}}
	for i, a := range accounts {
		hash, err := PasswordHash(get(a.key))
		if err != nil {
			return transfer.Report{}, fmt.Errorf("seed password missing or invalid: %s", a.key)
		}
		s.Rows["users"] = append(s.Rows["users"], transfer.Row{"id": strconv.Itoa(i + 1), "username": a.name, "password_hash": hash, "role": a.role, "class_id": a.class, "session_version": "0"})
	}
	now := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	for i, class := range []string{"A", "B"} {
		b := make([]byte, 16)
		if _, e = rand.Read(b); e != nil {
			return transfer.Report{}, fmt.Errorf("random source failed")
		}
		b[6] = (b[6] & 15) | 64
		b[8] = (b[8] & 63) | 128
		key := fmt.Sprintf("%x-%x-%x-%x-%x", b[:4], b[4:6], b[6:8], b[8:10], b[10:])
		body := "Class " + class + " teaching material\n"
		cid := strconv.Itoa(i + 1)
		if e = os.WriteFile(filepath.Join(stage, "files", key), []byte(body), 0600); e != nil {
			return transfer.Report{}, e
		}
		s.Rows["materials"] = append(s.Rows["materials"], transfer.Row{"id": key, "class_id": cid, "uploader_id": strconv.Itoa(i*2 + 1), "original_name": "Class-" + class + "-sample.txt", "size": strconv.Itoa(len(body)), "content_type": "text/plain", "storage_key": key, "created_at": now})
		s.Rows["knowledge_entries"] = append(s.Rows["knowledge_entries"], transfer.Row{"id": cid, "material_id": key, "class_id": cid, "plain_text": body, "created_at": now})
	}
	if e = s.Validate(stage); e != nil {
		return transfer.Report{}, e
	}
	return transfer.Publish(ctx, db, s, stage, root, "fresh", hook)
}
