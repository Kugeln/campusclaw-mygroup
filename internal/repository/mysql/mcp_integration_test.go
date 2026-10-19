//go:build mcpstorage

package mysql

import (
	"campusclaw/internal/auth"
	"campusclaw/internal/mcp"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	driver "github.com/go-sql-driver/mysql"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func mcpDB(t *testing.T) *sql.DB {
	t.Helper()
	cfg, e := driver.ParseDSN(os.Getenv("MCP_TEST_DSN"))
	host := os.Getenv("MCP_TEST_MYSQL")
	if e != nil || cfg == nil || !strings.HasPrefix(host, "campusclaw-w6-") || cfg.Addr != host+":3306" || cfg.DBName != "" || cfg.Passwd == "" {
		t.Fatal("isolated Phase2 MySQL required")
	}
	admin, e := sql.Open("mysql", cfg.FormatDSN())
	if e != nil {
		t.Fatal("test server")
	}
	defer admin.Close()
	var version string
	if admin.QueryRow("SELECT VERSION()").Scan(&version) != nil || !strings.HasPrefix(version, "8.4.") {
		t.Fatal("MySQL 8.4 required")
	}
	var random [8]byte
	rand.Read(random[:])
	cfg.DBName = "w6_" + hex.EncodeToString(random[:])
	if _, e = admin.Exec("CREATE DATABASE " + cfg.DBName + " CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_bin"); e != nil {
		t.Fatal("create test database")
	}
	db, e := sql.Open("mysql", cfg.FormatDSN())
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { db.Close() })
	t.Log("isolated_database=" + cfg.DBName)
	return db
}
func mcpSQL(t *testing.T, db *sql.DB, q string, args ...any) {
	t.Helper()
	if _, e := db.Exec(q, args...); e != nil {
		t.Fatalf("fixture query failed code=%d", ErrorCode(e))
	}
}
func legacyMCP(t *testing.T) *sql.DB {
	t.Helper()
	db := mcpDB(t)
	if e := PrepareSchema(context.Background(), db); e != nil {
		t.Fatal(e)
	}
	if e := prepareTutorSchema(context.Background(), db, nil); e != nil {
		t.Fatal(e)
	}
	return db
}
func TestMCPCapture(t *testing.T) {
	if os.Getenv("MCP_CAPTURE") != "true" {
		t.Skip("explicit isolated catalog capture only")
	}
	db := legacyMCP(t)
	previous, _ := readSchemaCatalog(context.Background(), db)
	steps := []schemaStep{}
	for _, s := range ddlStatements(mcpDDL) {
		mcpSQL(t, db, s)
		next, e := readSchemaCatalog(context.Background(), db)
		if e != nil {
			t.Fatal(e)
		}
		add, remove := make(schemaCatalogRows, len(next)), make(schemaCatalogRows, len(next))
		for i, rows := range next {
			old := map[string]bool{}
			for _, r := range previous[i] {
				old[strings.Join(r, "\x00")] = true
			}
			add[i] = [][]string{}
			remove[i] = [][]string{}
			for _, r := range rows {
				k := strings.Join(r, "\x00")
				if old[k] {
					delete(old, k)
				} else {
					add[i] = append(add[i], r)
				}
			}
			for _, r := range previous[i] {
				if old[strings.Join(r, "\x00")] {
					remove[i] = append(remove[i], r)
				}
			}
		}
		steps = append(steps, schemaStep{SHA256: fmt.Sprintf("%x", sha256.Sum256([]byte(s))), Add: add, Remove: remove})
		previous = next
	}
	b, _ := json.MarshalIndent(steps, "", "  ")
	if e := os.WriteFile(filepath.Join(os.Getenv("MCP_TEST_EVIDENCE"), "schema_mcp_steps.json"), b, 0600); e != nil {
		t.Fatal(e)
	}
}
func TestMCPMigrationPaths(t *testing.T) {
	for _, path := range []string{"fresh", "v1", "v2", "v3"} {
		t.Run(path, func(t *testing.T) {
			db := mcpDB(t)
			ctx := context.Background()
			if e := PrepareSchema(ctx, db); e != nil {
				t.Fatal(e)
			}
			if path != "fresh" {
				mcpSQL(t, db, "INSERT INTO skills(id,name) VALUES(77,'legacy untouched placeholder')")
			}
			if path == "v2" {
				if e := prepareRAGDDL(ctx, db); e != nil {
					t.Fatal(e)
				}
			}
			if path == "v3" {
				if e := prepareTutorSchema(ctx, db, nil); e != nil {
					t.Fatal(e)
				}
			}
			if e := PrepareRAGSchema(ctx, db); e != nil {
				t.Fatal(e)
			}
			if e := ValidateRAGSchema(ctx, db); e != nil {
				t.Fatal(e)
			}
			before, _ := readSchemaCatalog(ctx, db)
			for i := 0; i < 2; i++ {
				if e := PrepareRAGSchema(ctx, db); e != nil {
					t.Fatal(e)
				}
				if e := PrepareSchema(ctx, db); e != nil {
					t.Fatal("repeat initializer", e)
				}
			}
			after, _ := readSchemaCatalog(ctx, db)
			if !reflect.DeepEqual(before, after) {
				t.Fatal("repeat catalog changed")
			}
			var n int
			if db.QueryRow("SELECT COUNT(*) FROM schema_upgrades").Scan(&n) != nil || n != 2 {
				t.Fatal("old/new ledger not retained")
			}
			if path != "fresh" {
				var name string
				if db.QueryRow("SELECT name FROM skills WHERE id=77 AND source IS NULL AND class_id IS NULL").Scan(&name) != nil || name != "legacy untouched placeholder" {
					t.Fatal("placeholder changed")
				}
			}
		})
	}
}
func TestMCPInterruptedRecovery(t *testing.T) {
	stop := errors.New("injected stop")
	for _, phase := range []string{"before_ddl", "after_ddl", "after_ledger", "before_complete"} {
		for step := 1; step <= len(ddlStatements(mcpDDL)); step++ {
			if phase == "before_complete" && step != len(ddlStatements(mcpDDL)) {
				continue
			}
			t.Run(fmt.Sprintf("%s_%d", phase, step), func(t *testing.T) {
				db := legacyMCP(t)
				ctx := context.Background()
				e := prepareMCPSchema(ctx, db, func(p string, s int) error {
					if p == phase && s == step {
						return stop
					}
					return nil
				})
				if !errors.Is(e, stop) {
					t.Fatal("boundary not reached", e)
				}
				if ValidateMCPSchema(ctx, db) == nil {
					t.Fatal("incomplete ready")
				}
				for i := 0; i < 2; i++ {
					if e = PrepareRAGSchema(ctx, db); e != nil {
						t.Fatal("resume", e)
					}
				}
			})
		}
	}
}

func TestMCPLock(t *testing.T) {
	db := legacyMCP(t)
	ctx := context.Background()
	var name string
	db.QueryRow("SELECT DATABASE()").Scan(&name)
	key := fmt.Sprintf("campusclaw-schema-%x", sha256.Sum256([]byte(name)))[:60]
	conn, err := db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	var locked int
	if conn.QueryRowContext(ctx, "SELECT GET_LOCK(?,0)", key).Scan(&locked) != nil || locked != 1 {
		t.Fatal("test lock")
	}
	before := databaseFingerprint(t, db, true)
	if PrepareRAGSchema(ctx, db) == nil {
		t.Fatal("concurrent upgrade accepted")
	}
	if databaseFingerprint(t, db, true) != before {
		t.Fatal("busy upgrader wrote data")
	}
	conn.ExecContext(ctx, "SELECT RELEASE_LOCK(?)", key)
	if err = PrepareRAGSchema(ctx, db); err != nil {
		t.Fatal(err)
	}
}
func TestMCPDriftRefusal(t *testing.T) {
	queries := []string{"UPDATE schema_upgrades SET version=5 WHERE version=4", "UPDATE schema_upgrades SET ddl_step=0 WHERE version=4", "UPDATE schema_upgrades SET ddl_hash=REPEAT('a',64) WHERE version=4", "DELETE FROM schema_upgrades WHERE version=3", "CREATE TABLE unknown_table(id INT)", "ALTER TABLE mcp_servers ADD unexpected INT", "ALTER TABLE mcp_tools ADD INDEX unexpected(tool_name)", "ALTER TABLE mcp_tool_calls DROP CHECK mcp_call_status"}
	for i, q := range queries {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			db := legacyMCP(t)
			ctx := context.Background()
			if e := PrepareRAGSchema(ctx, db); e != nil {
				t.Fatal(e)
			}
			mcpSQL(t, db, q)
			before := databaseFingerprint(t, db, true)
			if ValidateMCPSchema(ctx, db) == nil || PrepareRAGSchema(ctx, db) == nil {
				t.Fatal("drift accepted")
			}
			if databaseFingerprint(t, db, true) != before {
				t.Fatal("drift rejection wrote data/catalog")
			}
		})
	}
	for _, q := range []string{"UPDATE schema_upgrades SET completed=1 WHERE version=4", "UPDATE schema_upgrades SET ddl_step=9 WHERE version=4", "DELETE FROM schema_upgrades WHERE version=4"} {
		t.Run("partial_"+q, func(t *testing.T) {
			db := legacyMCP(t)
			_ = prepareMCPSchema(context.Background(), db, func(phase string, step int) error {
				if phase == "after_ledger" && step == 2 {
					return errors.New("stop")
				}
				return nil
			})
			mcpSQL(t, db, q)
			if PrepareRAGSchema(context.Background(), db) == nil {
				t.Fatal("invalid partial ledger accepted")
			}
		})
	}
	t.Run("DDL_hash", func(t *testing.T) {
		db := mcpDB(t)
		if err := PrepareSchema(context.Background(), db); err != nil {
			t.Fatal(err)
		}
		fingerprint := databaseFingerprint(t, db, true)
		before := mcpStepsJSON
		defer func() { mcpStepsJSON = before }()
		var steps []schemaStep
		json.Unmarshal(before, &steps)
		steps[0].SHA256 = strings.Repeat("0", 64)
		mcpStepsJSON, _ = json.Marshal(steps)
		if _, e := expectedMCPCatalogs(); e == nil {
			t.Fatal("hash mismatch accepted")
		}
		if PrepareRAGSchema(context.Background(), db) == nil {
			t.Fatal("invalid compiled hash accepted")
		}
		if databaseFingerprint(t, db, true) != fingerprint {
			t.Fatal("invalid v4 contract advanced legacy schema")
		}
	})
}
func databaseFingerprint(t *testing.T, db *sql.DB, all bool) string {
	t.Helper()
	catalog, e := readSchemaCatalog(context.Background(), db)
	if e != nil {
		t.Fatal(e)
	}
	b, _ := json.Marshal(catalog)
	h := sha256.New()
	h.Write(b)
	for _, table := range catalog[0] {
		name := table[0]
		if !all && (strings.HasPrefix(name, "mcp_") || name == "schema_upgrades") {
			continue
		}
		rows, e := db.Query("SELECT * FROM " + name + " ORDER BY 1")
		if e != nil {
			t.Fatal(e)
		}
		cols, _ := rows.Columns()
		for rows.Next() {
			values := make([]sql.NullString, len(cols))
			args := make([]any, len(cols))
			for i := range values {
				args[i] = &values[i]
			}
			if rows.Scan(args...) != nil {
				t.Fatal("fingerprint read")
			}
			b, _ := json.Marshal(values)
			h.Write(b)
		}
		if rows.Err() != nil {
			t.Fatal(rows.Err())
		}
		rows.Close()
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

// Data digest deliberately excludes the added MCP catalog, but includes every
// existing table column and NULL/byte distinction in deterministic row order.
func legacyDataFingerprint(t *testing.T, db *sql.DB) string {
	t.Helper()
	catalog, _ := readSchemaCatalog(context.Background(), db)
	h := sha256.New()
	for _, table := range catalog[0] {
		name := table[0]
		if strings.HasPrefix(name, "mcp_") || name == "schema_upgrades" {
			continue
		}
		rows, e := db.Query("SELECT * FROM " + name + " ORDER BY 1")
		if e != nil {
			t.Fatal(e)
		}
		cols, _ := rows.Columns()
		for rows.Next() {
			v := make([]sql.NullString, len(cols))
			a := make([]any, len(cols))
			for i := range v {
				a[i] = &v[i]
			}
			if rows.Scan(a...) != nil {
				t.Fatal("digest")
			}
			b, _ := json.Marshal(v)
			h.Write(b)
		}
		rows.Close()
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

type mcpIdentityStore struct{ u auth.CurrentUser }

func (s mcpIdentityStore) FindCurrentUser(context.Context, int64) (auth.CurrentUser, bool, error) {
	return s.u, true, nil
}
func (s mcpIdentityStore) Revoke(context.Context, int64) error { return nil }
func principal(t *testing.T, id, class int64, role string) auth.Principal {
	t.Helper()
	tokens, e := auth.NewTokens(strings.Repeat("isolated-test-key-", 4), time.Now)
	if e != nil {
		t.Fatal(e)
	}
	token, e := tokens.Issue(id, 0)
	if e != nil {
		t.Fatal(e)
	}
	identity := auth.Identity{Tokens: tokens, Store: mcpIdentityStore{auth.CurrentUser{ID: id, ClassID: class, Role: role, Version: 0}}}
	p, e := identity.Authenticate(context.Background(), token)
	if e != nil {
		t.Fatal(e)
	}
	return p
}
func mcpFixture(t *testing.T) (*sql.DB, MCP, auth.Principal, auth.Principal) {
	db := legacyMCP(t)
	mcpSQL(t, db, "INSERT INTO classes(id,name) VALUES(1,'A'),(2,'B')")
	mcpSQL(t, db, "INSERT INTO users(id,username,password_hash,role,class_id,session_version) VALUES(1,'teacher-a','fixture','teacher',1,0),(2,'teacher-b','fixture','teacher',2,0),(3,'student-a','fixture','student',1,0)")
	if e := PrepareRAGSchema(context.Background(), db); e != nil {
		t.Fatal(e)
	}
	var key [32]byte
	rand.Read(key[:])
	vault, _ := mcp.NewVault("test", map[string][]byte{"test": key[:]})
	var targets []mcp.Target
	if json.Unmarshal([]byte(os.Getenv("MCP_ALLOWED_TARGETS")), &targets) != nil {
		t.Fatal("targets")
	}
	policy, e := mcp.NewPolicy(targets, nil)
	if e != nil {
		t.Fatal(e)
	}
	return db, MCP{DB: db, Vault: vault, Policy: policy}, principal(t, 1, 1, "teacher"), principal(t, 2, 2, "teacher")
}
func TestMCPStoreAndRealService(t *testing.T) {
	db, r, a, b := mcpFixture(t)
	ctx := context.Background()
	base := "http://mcp-demo:8090"
	token := os.Getenv("MCP_AUTH_TOKEN")
	s, e := r.Create(ctx, a, "campus-tools", base, token)
	if e != nil {
		t.Fatal(e)
	}
	encoded, _ := json.Marshal(s)
	if strings.Contains(string(encoded), token) || !s.HasAuth {
		t.Fatal("view leaked token")
	}
	var stored []byte
	if db.QueryRow("SELECT token_cipher FROM mcp_servers WHERE id=?", s.ID).Scan(&stored) != nil || strings.Contains(string(stored), token) {
		t.Fatal("plaintext stored")
	}
	if _, e = r.Get(ctx, b, s.ID); !errors.Is(e, MCPNotFound) {
		t.Fatal("cross class read")
	}
	if _, e = r.Update(ctx, b, s.ID, "other", base, ""); !errors.Is(e, MCPNotFound) {
		t.Fatal("cross class update")
	}
	if e = r.Delete(ctx, b, s.ID); !errors.Is(e, MCPNotFound) {
		t.Fatal("cross class delete")
	}
	lease, e := r.BeginDiscovery(ctx, a, s.ID)
	if e != nil {
		t.Fatal(e)
	}
	var tools []mcp.Tool
	e = r.WithClient(ctx, a, s.ID, func(client *mcp.Client) error {
		if e := client.Initialize(ctx); e != nil {
			return e
		}
		var e error
		tools, e = client.List(ctx)
		if e != nil {
			return e
		}
		v, e := client.Call(ctx, "formula_hint", map[string]any{"query": "圆面积公式"})
		if e != nil || v.IsError || !strings.Contains(v.Content[0].Text, "π") {
			return mcp.RemoteError
		}
		v, e = client.Call(ctx, "unit_convert", map[string]any{"value": 1, "from": "m", "to": "cm"})
		if e != nil || v.IsError || v.Content[0].Text != "1 m = 100 cm" {
			return mcp.RemoteError
		}
		return nil
	})
	if e != nil {
		t.Fatal("real MCP network", e)
	}
	if len(tools) != 2 {
		t.Fatal("discovery")
	}
	if e = r.PublishDiscovery(ctx, a, s.ID, lease, tools, true); e != nil {
		t.Fatal(e)
	}
	t.Run("cache_atomic_and_CAS", func(t *testing.T) {
		first, _ := r.BeginDiscovery(ctx, a, s.ID)
		second, _ := r.BeginDiscovery(ctx, a, s.ID)
		if e = r.PublishDiscovery(ctx, a, s.ID, first, tools, true); !errors.Is(e, MCPStale) {
			t.Fatal("stale response published")
		}
		if e = r.PublishDiscovery(ctx, a, s.ID, second, []mcp.Tool{tools[0], tools[0]}, true); !errors.Is(e, mcp.Invalid) {
			t.Fatal("duplicate tool")
		}
		if e = r.PublishDiscovery(ctx, a, s.ID, second, tools, true); e != nil {
			t.Fatal(e)
		}
		var n int
		db.QueryRow("SELECT COUNT(*) FROM mcp_tools WHERE server_id=?", s.ID).Scan(&n)
		if n != 2 {
			t.Fatal("duplicate cache")
		}
	})
	t.Run("cache_insert_failure_rolls_back", func(t *testing.T) {
		l, e := r.BeginDiscovery(ctx, a, s.ID)
		if e != nil {
			t.Fatal(e)
		}
		mcpSQL(t, db, "CREATE TRIGGER mcp_fail_cache BEFORE INSERT ON mcp_tools FOR EACH ROW BEGIN IF NEW.tool_name='unit_convert' THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='controlled cache failure'; END IF; END")
		err := r.PublishDiscovery(ctx, a, s.ID, l, tools, true)
		mcpSQL(t, db, "DROP TRIGGER mcp_fail_cache")
		if err == nil {
			t.Fatal("cache failure accepted")
		}
		var n int
		db.QueryRow("SELECT COUNT(*) FROM mcp_tools WHERE server_id=?", s.ID).Scan(&n)
		if n != 0 {
			t.Fatal("partial cache published")
		}
		if e = r.PublishDiscovery(ctx, a, s.ID, l, tools, true); e != nil {
			t.Fatal(e)
		}
	})
	t.Run("audit_and_delete", func(t *testing.T) {
		ok, e := r.AppendTerminal(ctx, a, "campus-tools", "formula_hint", "ok", token+" Bearer secret-canary "+strings.Repeat("字", 300), nil)
		if e != nil || !ok {
			t.Fatal(e)
		}
		calls, e := r.RecentCalls(ctx, a)
		if e != nil || len(calls) != 1 || len([]rune(calls[0].Detail)) > 200 || strings.Contains(calls[0].Detail, token) || strings.Contains(calls[0].Detail, "secret-canary") {
			t.Fatal("audit not safe")
		}
		if e = r.Delete(ctx, a, s.ID); !errors.Is(e, MCPUnavailable) {
			t.Fatal("audit did not restrict deletion")
		}
		cached, cacheErr := r.Tools(ctx, a, s.ID)
		if cacheErr != nil || len(cached) != 2 {
			t.Fatal("restricted delete changed cache")
		}
		if e = r.Disconnect(ctx, a, s.ID); e != nil {
			t.Fatal(e)
		}
		ok, e = r.AppendTerminal(ctx, a, "campus-tools", "formula_hint", "ok", "not executed", nil)
		if e != nil || !ok {
			t.Fatal(e)
		}
		calls, _ = r.RecentCalls(ctx, a)
		if calls[0].Status != "denied" {
			t.Fatal("disconnected audit")
		}
		if _, e = r.Update(ctx, a, s.ID, "renamed", base, ""); e != nil {
			t.Fatal(e)
		}
		if yes, e := r.AppendTerminal(ctx, a, "campus-tools", "formula_hint", "denied", "", nil); e != nil || yes {
			t.Fatal("old name got audit")
		}
		if e = r.WithClient(ctx, a, s.ID, func(c *mcp.Client) error { return c.Initialize(ctx) }); e != nil {
			t.Fatal("empty token lost on rename", e)
		}
	})
	t.Run("cache_cascade", func(t *testing.T) {
		x, e := r.Create(ctx, a, "delete-me", base, token)
		if e != nil {
			t.Fatal(e)
		}
		l, _ := r.BeginDiscovery(ctx, a, x.ID)
		if e = r.PublishDiscovery(ctx, a, x.ID, l, tools, true); e != nil {
			t.Fatal(e)
		}
		if e = r.Delete(ctx, a, x.ID); e != nil {
			t.Fatal(e)
		}
		var n int
		db.QueryRow("SELECT COUNT(*) FROM mcp_tools WHERE server_id=?", x.ID).Scan(&n)
		if n != 0 {
			t.Fatal("cache not cascaded")
		}
		if yes, e := r.AppendTerminal(ctx, a, "delete-me", "formula_hint", "denied", "", nil); e != nil || yes {
			t.Fatal("deleted name audited")
		}
	})
	t.Run("audit_failure", func(t *testing.T) {
		bad := principal(t, 99, 1, "student")
		if ok, e := r.AppendTerminal(ctx, bad, "renamed", "formula_hint", "error", "sensitive input", nil); e == nil || ok {
			t.Fatal("failed audit claimed success")
		}
	})
	t.Run("constraints", func(t *testing.T) {
		for _, q := range []string{fmt.Sprintf("UPDATE mcp_servers SET connected=2 WHERE id=%d", s.ID), fmt.Sprintf("INSERT INTO mcp_tools(server_id,class_id,tool_name,description,schema_json) VALUES(%d,2,'invalid','x','{}')", s.ID), "UPDATE mcp_tool_calls SET status='pending'"} {
			if _, e = db.Exec(q); e == nil {
				t.Fatal("constraint accepted violation")
			}
		}
	})
	t.Run("audit_limits", func(t *testing.T) {
		for i := 0; i < 105; i++ {
			if _, e = r.AppendTerminal(ctx, a, "renamed", "formula_hint", "error", "", nil); e != nil {
				t.Fatal(e)
			}
		}
		fifty, _ := r.RecentCalls(ctx, a)
		hundred, _ := r.Calls(ctx, a, 101)
		other, _ := r.RecentCalls(ctx, b)
		if len(fifty) != 50 || len(hundred) != 100 || len(other) != 0 {
			t.Fatal("audit class/limit")
		}
		if _, e = r.Calls(ctx, a, 0); e == nil {
			t.Fatal("invalid limit")
		}
	})
	t.Run("same_name_classes_and_roles", func(t *testing.T) {
		if _, e = r.Create(ctx, a, "renamed", base, token); !errors.Is(e, mcp.Invalid) {
			t.Fatal("same-class duplicate accepted")
		}
		if _, e = r.Create(ctx, b, "renamed", base, token); e != nil {
			t.Fatal("other class same name", e)
		}
		student := principal(t, 3, 1, "student")
		if _, e = r.Servers(ctx, student); !errors.Is(e, MCPForbidden) {
			t.Fatal("student management access")
		}
		if _, e = r.RecentCalls(ctx, student); !errors.Is(e, MCPForbidden) {
			t.Fatal("student audit read")
		}
	})
}

func TestMCPBackupAndLegacyProtection(t *testing.T) {
	db := legacyMCP(t)
	ctx := context.Background()
	mcpSQL(t, db, "INSERT INTO classes(id,name) VALUES(1,'A')")
	mcpSQL(t, db, "INSERT INTO users(id,username,password_hash,role,class_id,session_version) VALUES(1,'old-user','old-hash','teacher',1,7)")
	mcpSQL(t, db, "INSERT INTO sessions VALUES(?, 'legacy session bytes', 9999999999)", strings.Repeat("a", 64))
	id := "00000000-0000-4000-8000-000000000001"
	fileRoot := filepath.Join(os.Getenv("MCP_TEST_EVIDENCE"), "material-files")
	if e := os.MkdirAll(fileRoot, 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(filepath.Join(fileRoot, id), []byte("abc"), 0600); e != nil {
		t.Fatal(e)
	}
	fileHash := fmt.Sprintf("%x", sha256.Sum256([]byte("abc")))
	mcpSQL(t, db, "INSERT INTO materials(id,class_id,uploader_id,original_name,size,content_type,storage_key,created_at) VALUES(?,1,1,'old material',3,'text/plain',?,'2026-10-01')", id, id)
	mcpSQL(t, db, "INSERT INTO knowledge_entries(id,material_id,class_id,plain_text,created_at,index_status) VALUES(1,?,1,'原切片','2026-10-01','ready')", id)
	mcpSQL(t, db, "INSERT INTO knowledge_chunks(id,knowledge_entry_id,material_id,class_id,chunk_index,chunk_text,index_status) VALUES(?,1,?,1,1,'原切片','ready')", id, id)
	source := "名称：旧技能\r\n工具：knowledge_search\r\n\r\n保留原始字节"
	mcpSQL(t, db, "INSERT INTO skills(id,name,class_id,skill_key,enabled,instruction_version,revision,source,source_origin,source_version,source_hash,tools) VALUES(7,'旧技能',1,'problem-guidance',0,'1',4,?,'teacher_upload',8,?,'[\"knowledge_search\"]')", source, fmt.Sprintf("%x", sha256.Sum256([]byte(source))))
	mcpSQL(t, db, "INSERT INTO assistants(id,name,class_id,prompt,enabled,revision,updated_by) VALUES(9,'old assistant',1,'原提示词',1,2,1)")
	mcpSQL(t, db, "INSERT INTO assistant_skill_bindings VALUES(9,1,7)")
	mcpSQL(t, db, "INSERT INTO assistant_material_bindings VALUES(9,1,?)", id)
	mcpSQL(t, db, "INSERT INTO conversations(id,user_id,class_id,agent_id,title,created_at,updated_at) VALUES(1,1,1,9,'old','2026-10-01','2026-10-01')")
	mcpSQL(t, db, "INSERT INTO conversation_messages(conversation_id,class_id,turn_id,sequence_number,role,content,status,created_at,configuration_snapshot,evidence_snapshot,citations) VALUES(1,1,?,1,'assistant','原回答[1]','completed','2026-10-01','{\"version\":2}','[{\"material_id\":\"old\"}]','[{\"number\":1}]')", id)
	before := legacyDataFingerprint(t, db)
	if e := PrepareRAGSchema(ctx, db); e != nil {
		t.Fatal(e)
	}
	if legacyDataFingerprint(t, db) != before {
		t.Fatal("legacy data changed")
	}
	cfg, _ := driver.ParseDSN(os.Getenv("MCP_TEST_DSN"))
	var name string
	db.QueryRow("SELECT DATABASE()").Scan(&name)
	restore := mcpDB(t)
	var target string
	restore.QueryRow("SELECT DATABASE()").Scan(&target)
	dump := exec.Command("mysqldump", "--skip-ssl", "--no-tablespaces", "--single-transaction", "-h", os.Getenv("MCP_TEST_MYSQL"), "-uroot", name)
	dump.Env = append(os.Environ(), "MYSQL_PWD="+cfg.Passwd)
	data, e := dump.Output()
	if e != nil {
		if failure, ok := e.(*exec.ExitError); ok {
			t.Fatal("backup failed: " + strings.ReplaceAll(string(failure.Stderr), cfg.Passwd, "[redacted]"))
		}
		t.Fatal("backup failed")
	}
	if e = os.WriteFile(filepath.Join(os.Getenv("MCP_TEST_EVIDENCE"), "isolated-backup.sql"), data, 0600); e != nil {
		t.Fatal(e)
	}
	load := exec.Command("mysql", "--skip-ssl", "-h", os.Getenv("MCP_TEST_MYSQL"), "-uroot", target)
	load.Env = append(os.Environ(), "MYSQL_PWD="+cfg.Passwd)
	load.Stdin = strings.NewReader(string(data))
	if e = load.Run(); e != nil {
		t.Fatal("restore failed")
	}
	backupRoot := filepath.Join(os.Getenv("MCP_TEST_EVIDENCE"), "backup-files")
	restoreRoot := filepath.Join(os.Getenv("MCP_TEST_EVIDENCE"), "restored-files")
	for _, dir := range []string{backupRoot, restoreRoot} {
		if e = os.MkdirAll(dir, 0700); e != nil {
			t.Fatal(e)
		}
	}
	fileBytes, e := os.ReadFile(filepath.Join(fileRoot, id))
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(filepath.Join(backupRoot, id), fileBytes, 0600); e != nil {
		t.Fatal(e)
	}
	fileBytes, e = os.ReadFile(filepath.Join(backupRoot, id))
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(filepath.Join(restoreRoot, id), fileBytes, 0600); e != nil {
		t.Fatal(e)
	}
	restoredBytes, e := os.ReadFile(filepath.Join(restoreRoot, id))
	if e != nil || fmt.Sprintf("%x", sha256.Sum256(restoredBytes)) != fileHash {
		t.Fatal("restored material file differs")
	}
	if e = ValidateMCPSchema(ctx, restore); e != nil {
		t.Fatal(e)
	}
	if databaseFingerprint(t, db, true) != databaseFingerprint(t, restore, true) {
		t.Fatal("backup catalog/ledger/data mismatch")
	}
	report := map[string]any{"legacy_before_sha256": before, "legacy_after_sha256": legacyDataFingerprint(t, db), "restored_sha256": databaseFingerprint(t, restore, true), "source_sha256": databaseFingerprint(t, db, true), "backup_sha256": fmt.Sprintf("%x", sha256.Sum256(data)), "source_database": name, "restore_database": target, "qdrant_reindex": false}
	report["material_file_sha256"] = fileHash
	report["restored_material_file_sha256"] = fmt.Sprintf("%x", sha256.Sum256(restoredBytes))
	b, _ := json.MarshalIndent(report, "", "  ")
	os.WriteFile(filepath.Join(os.Getenv("MCP_TEST_EVIDENCE"), "preservation-restore.json"), b, 0600)
}
