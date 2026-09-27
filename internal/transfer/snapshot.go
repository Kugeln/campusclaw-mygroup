// Package transfer provides offline data validation; it has no HTTP dependencies.
package transfer

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

type Table struct {
	Name     string
	Columns  []string
	Integers map[string]bool
}

func table(name, columns, integers string) Table {
	m := map[string]bool{}
	for _, c := range strings.Split(integers, ",") {
		m[c] = true
	}
	return Table{name, strings.Split(columns, ","), m}
}

var Tables = []Table{
	table("classes", "id,name", "id"),
	table("users", "id,username,password_hash,role,class_id,session_version", "id,class_id,session_version"),
	table("materials", "id,class_id,uploader_id,original_name,size,content_type,storage_key,created_at", "class_id,uploader_id,size"),
	table("knowledge_entries", "id,material_id,class_id,plain_text,created_at", "id,class_id"),
	table("assignments", "id,name", "id"), table("assistants", "id,name", "id"), table("skills", "id,name", "id"),
}

type Row map[string]string
type Snapshot struct {
	SourceSHA256 string
	Rows         map[string][]Row
	Files        map[string]string
	Bodies       map[string]string
}
type Querier interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}
type TableReport struct {
	Count        int
	IDs          []string
	RowsSHA256   map[string]string
	FieldsSHA256 map[string]map[string]string
}
type Report struct {
	SourceSHA256 string
	Version      int
	Mode         string
	Verified     bool
	Fingerprint  string
	Tables       map[string]TableReport
	FilesSHA256  map[string]string
	BodiesSHA256 map[string]string
	Sessions     string
}

func Hash(data []byte) string { h := sha256.Sum256(data); return hex.EncodeToString(h[:]) }
func (s Snapshot) Report(mode string, verified bool) Report {
	r := Report{Version: 1, Mode: mode, Verified: verified, Tables: map[string]TableReport{}, FilesSHA256: s.Files, BodiesSHA256: s.Bodies, Sessions: "excluded; target must initially be empty"}
	r.SourceSHA256 = s.SourceSHA256
	for _, t := range Tables {
		tr := TableReport{IDs: []string{}, RowsSHA256: map[string]string{}, FieldsSHA256: map[string]map[string]string{}}
		for _, row := range s.Rows[t.Name] {
			fields := map[string]string{}
			for _, column := range t.Columns {
				kind := "text"
				if t.Integers[column] {
					kind = "int64"
				}
				encoded, _ := json.Marshal([]string{kind, row[column]})
				fields[column] = Hash(encoded)
			}
			tr.FieldsSHA256[row["id"]] = fields
			b, _ := json.Marshal(row)
			tr.IDs = append(tr.IDs, row["id"])
			tr.RowsSHA256[row["id"]] = Hash(b)
		}
		sort.Strings(tr.IDs)
		tr.Count = len(tr.IDs)
		r.Tables[t.Name] = tr
	}
	b, _ := json.Marshal(struct {
		Tables        map[string]TableReport
		Files, Bodies map[string]string
	}{r.Tables, s.Files, s.Bodies})
	r.Fingerprint = Hash(b)
	return r
}
func Read(ctx context.Context, q Querier) (Snapshot, error) {
	s := Snapshot{Rows: map[string][]Row{}, Files: map[string]string{}, Bodies: map[string]string{}}
	for _, t := range Tables {
		rows, err := q.QueryContext(ctx, "SELECT "+strings.Join(t.Columns, ",")+" FROM "+t.Name+" ORDER BY id")
		if err != nil {
			return s, fmt.Errorf("read table %s failed", t.Name)
		}
		s.Rows[t.Name] = []Row{}
		for rows.Next() {
			values := make([]any, len(t.Columns))
			dest := make([]any, len(values))
			for i := range values {
				dest[i] = &values[i]
			}
			if err = rows.Scan(dest...); err != nil {
				rows.Close()
				return s, fmt.Errorf("scan %s failed", t.Name)
			}
			row := Row{}
			for i, c := range t.Columns {
				var v string
				switch x := values[i].(type) {
				case int64:
					v = strconv.FormatInt(x, 10)
				case string:
					v = x
				case []byte:
					v = string(x)
				default:
					rows.Close()
					return s, fmt.Errorf("unsupported type %s.%s", t.Name, c)
				}
				if t.Integers[c] {
					n, e := strconv.ParseInt(v, 10, 64)
					if e != nil || strconv.FormatInt(n, 10) != v {
						rows.Close()
						return s, fmt.Errorf("invalid integer %s.%s", t.Name, c)
					}
				} else if !utf8.ValidString(v) {
					rows.Close()
					return s, fmt.Errorf("invalid UTF-8 %s.%s", t.Name, c)
				}
				row[c] = v
			}
			s.Rows[t.Name] = append(s.Rows[t.Name], row)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return s, fmt.Errorf("read table %s incomplete", t.Name)
		}
	}
	return s, nil
}

var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
var passwordPattern = regexp.MustCompile(`^pbkdf2:sha256:1000000\$[A-Za-z0-9]+\$[0-9a-f]{64}$`)

func regular(path string) error {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return fmt.Errorf("missing or unsafe file")
	}
	return nil
}
func SafeDirectory(path string) error {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("invalid directory")
	}
	for current := absolute; ; current = filepath.Dir(current) {
		info, e := os.Lstat(current)
		if e != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("missing or unsafe directory")
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
	}
	return nil
}
func timestamp(v string) bool {
	t, e := time.Parse("2006-01-02T15:04:05.000Z", v)
	return e == nil && t.Format("2006-01-02T15:04:05.000Z") == v
}

// Validate checks semantics and every referenced file, without touching source data.
func (s *Snapshot) Validate(root string) error {
	if err := SafeDirectory(root); err != nil {
		return err
	}
	if err := SafeDirectory(filepath.Join(root, "files")); err != nil {
		return err
	}
	classes := map[string]bool{}
	users := map[string]Row{}
	materials := map[string]Row{}
	entries := map[string]bool{}
	keys := map[string]bool{}
	for _, r := range s.Rows["classes"] {
		if utf8.RuneCountInString(r["name"]) > 255 {
			return fmt.Errorf("classes.name exceeds target capacity")
		}
		classes[r["id"]] = true
	}
	for _, r := range s.Rows["users"] {
		if !classes[r["class_id"]] || (r["role"] != "teacher" && r["role"] != "student") {
			return fmt.Errorf("invalid users relationship or role")
		}
		if utf8.RuneCountInString(r["username"]) > 255 || len(r["password_hash"]) > 65535 || !passwordPattern.MatchString(r["password_hash"]) {
			return fmt.Errorf("unsupported users field format")
		}
		users[r["id"]] = r
	}
	s.Files = map[string]string{}
	s.Bodies = map[string]string{}
	for _, r := range s.Rows["materials"] {
		if !uuidPattern.MatchString(r["id"]) || !uuidPattern.MatchString(r["storage_key"]) || !timestamp(r["created_at"]) {
			return fmt.Errorf("unsupported material ID key or time")
		}
		u, ok := users[r["uploader_id"]]
		if !ok || !classes[r["class_id"]] || u["class_id"] != r["class_id"] {
			return fmt.Errorf("invalid material class or uploader")
		}
		if keys[r["storage_key"]] {
			return fmt.Errorf("duplicate storage key")
		}
		keys[r["storage_key"]] = true
		path := filepath.Join(root, "files", r["storage_key"])
		if err := regular(path); err != nil {
			return err
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("file read failed")
		}
		size, e := strconv.ParseInt(r["size"], 10, 64)
		if e != nil || size <= 0 || size != int64(len(b)) {
			return fmt.Errorf("file size mismatch")
		}
		if len(r["content_type"]) > 65535 {
			return fmt.Errorf("content type exceeds capacity")
		}
		s.Files[r["storage_key"]] = Hash(b)
		materials[r["id"]] = r
	}
	for _, r := range s.Rows["knowledge_entries"] {
		m, ok := materials[r["material_id"]]
		if !ok || m["class_id"] != r["class_id"] || entries[r["material_id"]] || !timestamp(r["created_at"]) {
			return fmt.Errorf("invalid knowledge entry relationship or time")
		}
		b, err := os.ReadFile(filepath.Join(root, "files", m["storage_key"]))
		if err != nil {
			return fmt.Errorf("file read failed")
		}
		body := strings.TrimPrefix(string(b), "\ufeff")
		if !utf8.Valid(b) || strings.ContainsRune(body, 0) || body != r["plain_text"] {
			return fmt.Errorf("body content mismatch")
		}
		entries[r["material_id"]] = true
		s.Bodies[r["id"]] = Hash([]byte(r["plain_text"]))
	}
	if len(entries) != len(materials) {
		return fmt.Errorf("material missing knowledge entry")
	}
	return nil
}
func (s Snapshot) Empty() bool {
	for _, rows := range s.Rows {
		if len(rows) > 0 {
			return false
		}
	}
	return true
}
