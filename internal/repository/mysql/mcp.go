package mysql

import (
	"campusclaw/internal/auth"
	"campusclaw/internal/mcp"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

var MCPUnavailable = mcp.Unavailable
var MCPNotFound = mcp.NotFound
var MCPForbidden = mcp.Forbidden
var MCPStale = mcp.Stale

type MCP struct {
	DB      *sql.DB
	Vault   *mcp.Vault
	Policy  *mcp.Policy
	Secrets []string
}
type MCPServer = mcp.Server

type MCPLease struct {
	Revision  int64
	Operation string
}

const mcpView = "id,name,base_url,connected,(token_cipher IS NOT NULL),(SELECT COUNT(*) FROM mcp_tools t WHERE t.server_id=mcp_servers.id AND t.class_id=mcp_servers.class_id),revision"

func mcpTeacher(p auth.Principal) error {
	if p.UserID() <= 0 || p.ClassID() <= 0 || p.Role() != "teacher" {
		return MCPForbidden
	}
	return nil
}
func mcpScope(p auth.Principal) error {
	if p.UserID() <= 0 || p.ClassID() <= 0 {
		return MCPForbidden
	}
	return nil
}
func mcpRead(row interface{ Scan(...any) error }) (MCPServer, error) {
	var s MCPServer
	e := row.Scan(&s.ID, &s.Name, &s.BaseURL, &s.Connected, &s.HasAuth, &s.ToolCount, &s.Revision)
	if errors.Is(e, sql.ErrNoRows) {
		return s, MCPNotFound
	}
	if e != nil {
		return s, MCPUnavailable
	}
	return s, nil
}
func mcpNow() string                                    { return time.Now().UTC().Format("2006-01-02T15:04:05.000000000Z") }
func credentialBinding(class int64, name string) string { return fmt.Sprintf("mcp:%d:%s", class, name) }
func (r MCP) validate(ctx context.Context, p auth.Principal, name, base, token string) error {
	if e := mcpTeacher(p); e != nil {
		return e
	}
	if !mcp.ServerName.MatchString(name) || !utf8.ValidString(base) || utf8.RuneCountInString(base) > 512 || len(token) > 4096 || strings.ContainsAny(token, "\r\n") {
		return mcp.Invalid
	}
	return r.Policy.Validate(ctx, base)
}
func (r MCP) Get(ctx context.Context, p auth.Principal, id int64) (MCPServer, error) {
	if e := mcpTeacher(p); e != nil {
		return MCPServer{}, e
	}
	return mcpRead(r.DB.QueryRowContext(ctx, "SELECT "+mcpView+" FROM mcp_servers WHERE id=? AND class_id=?", id, p.ClassID()))
}
func (r MCP) Servers(ctx context.Context, p auth.Principal) ([]MCPServer, error) {
	if e := mcpTeacher(p); e != nil {
		return nil, e
	}
	rows, e := r.DB.QueryContext(ctx, "SELECT "+mcpView+" FROM mcp_servers WHERE class_id=? ORDER BY id", p.ClassID())
	if e != nil {
		return nil, MCPUnavailable
	}
	defer rows.Close()
	out := []MCPServer{}
	for rows.Next() {
		s, e := mcpRead(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, s)
	}
	if rows.Err() != nil {
		return nil, MCPUnavailable
	}
	return out, nil
}
func (r MCP) Create(ctx context.Context, p auth.Principal, name, base, token string) (MCPServer, error) {
	if e := r.validate(ctx, p, name, base, token); e != nil {
		return MCPServer{}, e
	}
	var cipher, nonce any
	var key any
	if token != "" {
		sealed, e := r.Vault.Seal(token, credentialBinding(p.ClassID(), name))
		if e != nil {
			return MCPServer{}, e
		}
		cipher, nonce, key = sealed.Cipher, sealed.Nonce, sealed.KeyID
	}
	now := mcpNow()
	res, e := r.DB.ExecContext(ctx, "INSERT INTO mcp_servers(class_id,name,base_url,token_cipher,token_nonce,key_id,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?)", p.ClassID(), name, strings.TrimRight(base, "/"), cipher, nonce, key, now, now)
	if e != nil {
		if ErrorCode(e) == 1062 {
			return MCPServer{}, mcp.Invalid
		}
		return MCPServer{}, MCPUnavailable
	}
	id, e := res.LastInsertId()
	if e != nil {
		return MCPServer{}, MCPUnavailable
	}
	return r.Get(ctx, p, id)
}
func (r MCP) Update(ctx context.Context, p auth.Principal, id int64, name, base, token string) (MCPServer, error) {
	// Scope before DNS: an external-class ID must never trigger outbound work.
	if _, e := r.Get(ctx, p, id); e != nil {
		return MCPServer{}, e
	}
	if e := r.validate(ctx, p, name, base, token); e != nil {
		return MCPServer{}, e
	}
	tx, e := r.DB.BeginTx(ctx, nil)
	if e != nil {
		return MCPServer{}, MCPUnavailable
	}
	defer tx.Rollback()
	var old string
	var envelope mcp.Envelope
	var key sql.NullString
	e = tx.QueryRowContext(ctx, "SELECT name,token_cipher,token_nonce,key_id FROM mcp_servers WHERE id=? AND class_id=? FOR UPDATE", id, p.ClassID()).Scan(&old, &envelope.Cipher, &envelope.Nonce, &key)
	if e == sql.ErrNoRows {
		return MCPServer{}, MCPNotFound
	}
	if e != nil {
		return MCPServer{}, MCPUnavailable
	}
	envelope.KeyID = key.String
	if token == "" && key.Valid {
		token, e = r.Vault.Open(envelope, credentialBinding(p.ClassID(), old))
		if e != nil {
			return MCPServer{}, e
		}
	}
	var cipher, nonce, keyID any
	if token != "" {
		sealed, e := r.Vault.Seal(token, credentialBinding(p.ClassID(), name))
		if e != nil {
			return MCPServer{}, e
		}
		cipher, nonce, keyID = sealed.Cipher, sealed.Nonce, sealed.KeyID
	}
	if _, e = tx.ExecContext(ctx, "UPDATE mcp_servers SET name=?,base_url=?,token_cipher=?,token_nonce=?,key_id=?,connected=0,connected_at=NULL,revision=revision+1,op_id=NULL,operation_started_at=NULL,updated_at=? WHERE id=? AND class_id=?", name, strings.TrimRight(base, "/"), cipher, nonce, keyID, mcpNow(), id, p.ClassID()); e != nil {
		if ErrorCode(e) == 1062 {
			return MCPServer{}, mcp.Invalid
		}
		return MCPServer{}, MCPUnavailable
	}
	if _, e = tx.ExecContext(ctx, "DELETE FROM mcp_tools WHERE server_id=? AND class_id=?", id, p.ClassID()); e != nil {
		return MCPServer{}, MCPUnavailable
	}
	if tx.Commit() != nil {
		return MCPServer{}, MCPUnavailable
	}
	return r.Get(ctx, p, id)
}
func (r MCP) Delete(ctx context.Context, p auth.Principal, id int64) error {
	if e := mcpTeacher(p); e != nil {
		return e
	}
	res, e := r.DB.ExecContext(ctx, "DELETE FROM mcp_servers WHERE id=? AND class_id=?", id, p.ClassID())
	if e != nil {
		return MCPUnavailable
	}
	n, e := res.RowsAffected()
	if e != nil {
		return MCPUnavailable
	}
	if n == 0 {
		return MCPNotFound
	}
	return nil
}
func (r MCP) BeginDiscovery(ctx context.Context, p auth.Principal, id int64) (MCPLease, error) {
	if e := mcpTeacher(p); e != nil {
		return MCPLease{}, e
	}
	var random [16]byte
	if _, e := rand.Read(random[:]); e != nil {
		return MCPLease{}, MCPUnavailable
	}
	op := hex.EncodeToString(random[:])
	tx, e := r.DB.BeginTx(ctx, nil)
	if e != nil {
		return MCPLease{}, MCPUnavailable
	}
	defer tx.Rollback()
	var revision int64
	if e = tx.QueryRowContext(ctx, "SELECT revision FROM mcp_servers WHERE id=? AND class_id=? FOR UPDATE", id, p.ClassID()).Scan(&revision); e != nil {
		if e == sql.ErrNoRows {
			return MCPLease{}, MCPNotFound
		}
		return MCPLease{}, MCPUnavailable
	}
	if _, e = tx.ExecContext(ctx, "UPDATE mcp_servers SET connected=0,connected_at=NULL,revision=revision+1,op_id=?,operation_started_at=?,updated_at=? WHERE id=? AND class_id=?", op, mcpNow(), mcpNow(), id, p.ClassID()); e != nil {
		return MCPLease{}, MCPUnavailable
	}
	if _, e = tx.ExecContext(ctx, "DELETE FROM mcp_tools WHERE server_id=? AND class_id=?", id, p.ClassID()); e != nil {
		return MCPLease{}, MCPUnavailable
	}
	if tx.Commit() != nil {
		return MCPLease{}, MCPUnavailable
	}
	return MCPLease{Revision: revision + 1, Operation: op}, nil
}

// PublishDiscovery is network-free. Callers supply only a fully validated remote
// discovery result, after initialize/list finish outside this transaction.
func (r MCP) PublishDiscovery(ctx context.Context, p auth.Principal, id int64, lease MCPLease, tools []mcp.Tool, connected bool) error {
	if e := mcpTeacher(p); e != nil {
		return e
	}
	if mcp.ValidateTools(tools) != nil || (!connected && len(tools) != 0) {
		return mcp.Invalid
	}
	tx, e := r.DB.BeginTx(ctx, nil)
	if e != nil {
		return MCPUnavailable
	}
	defer tx.Rollback()
	var revision int64
	var op sql.NullString
	e = tx.QueryRowContext(ctx, "SELECT revision,op_id FROM mcp_servers WHERE id=? AND class_id=? FOR UPDATE", id, p.ClassID()).Scan(&revision, &op)
	if e == sql.ErrNoRows {
		return MCPNotFound
	}
	if e != nil {
		return MCPUnavailable
	}
	if revision != lease.Revision || !op.Valid || op.String != lease.Operation {
		return MCPStale
	}
	if _, e = tx.ExecContext(ctx, "DELETE FROM mcp_tools WHERE server_id=? AND class_id=?", id, p.ClassID()); e != nil {
		return MCPUnavailable
	}
	for _, t := range tools {
		if _, e = tx.ExecContext(ctx, "INSERT INTO mcp_tools(server_id,class_id,tool_name,description,schema_json) VALUES(?,?,?,?,?)", id, p.ClassID(), t.Name, t.Description, []byte(t.InputSchema)); e != nil {
			return MCPUnavailable
		}
	}
	var at any
	if connected {
		at = mcpNow()
	}
	if _, e = tx.ExecContext(ctx, "UPDATE mcp_servers SET connected=?,connected_at=?,revision=revision+1,op_id=NULL,operation_started_at=NULL,updated_at=? WHERE id=? AND class_id=?", connected, at, mcpNow(), id, p.ClassID()); e != nil {
		return MCPUnavailable
	}
	if tx.Commit() != nil {
		return MCPUnavailable
	}
	return nil
}

// Connect performs network I/O outside transactions. A later update, disconnect,
// deletion or discovery invalidates this lease and can never be resurrected.
func (r MCP) Connect(ctx context.Context, p auth.Principal, id int64) ([]mcp.Tool, error) {
	lease, e := r.BeginDiscovery(ctx, p, id)
	if e != nil {
		return nil, e
	}
	var tools []mcp.Tool
	e = r.WithClient(ctx, p, id, func(client *mcp.Client) error {
		if e := client.Initialize(ctx); e != nil {
			return e
		}
		var err error
		tools, err = client.List(ctx)
		return err
	})
	if e != nil {
		// BeginDiscovery already invalidated the cache. Never clean up using a new
		// lease: doing so could destroy a newer successful concurrent connection.
		return nil, e
	}
	if e = r.PublishDiscovery(ctx, p, id, lease, tools, true); e != nil {
		return nil, e
	}
	return tools, nil
}
func (r MCP) Disconnect(ctx context.Context, p auth.Principal, id int64) error {
	if e := mcpTeacher(p); e != nil {
		return e
	}
	tx, e := r.DB.BeginTx(ctx, nil)
	if e != nil {
		return MCPUnavailable
	}
	defer tx.Rollback()
	var found int64
	e = tx.QueryRowContext(ctx, "SELECT id FROM mcp_servers WHERE id=? AND class_id=? FOR UPDATE", id, p.ClassID()).Scan(&found)
	if e == sql.ErrNoRows {
		return MCPNotFound
	}
	if e != nil {
		return MCPUnavailable
	}
	if _, e = tx.ExecContext(ctx, "UPDATE mcp_servers SET connected=0,connected_at=NULL,revision=revision+1,op_id=NULL,operation_started_at=NULL,updated_at=? WHERE id=? AND class_id=?", mcpNow(), id, p.ClassID()); e != nil {
		return MCPUnavailable
	}
	if _, e = tx.ExecContext(ctx, "DELETE FROM mcp_tools WHERE server_id=? AND class_id=?", id, p.ClassID()); e != nil {
		return MCPUnavailable
	}
	if tx.Commit() != nil {
		return MCPUnavailable
	}
	return nil
}
func (r MCP) Tools(ctx context.Context, p auth.Principal, id int64) ([]mcp.Tool, error) {
	if _, e := r.Get(ctx, p, id); e != nil {
		return nil, e
	}
	rows, e := r.DB.QueryContext(ctx, "SELECT tool_name,description,schema_json FROM mcp_tools WHERE server_id=? AND class_id=? ORDER BY id", id, p.ClassID())
	if e != nil {
		return nil, MCPUnavailable
	}
	defer rows.Close()
	tools := []mcp.Tool{}
	for rows.Next() {
		var t mcp.Tool
		if rows.Scan(&t.Name, &t.Description, &t.InputSchema) != nil {
			return nil, MCPUnavailable
		}
		tools = append(tools, t)
	}
	if rows.Err() != nil {
		return nil, MCPUnavailable
	}
	return tools, nil
}

// WithClient confines decryption to a callback; views never carry credentials.
func (r MCP) WithClient(ctx context.Context, p auth.Principal, id int64, use func(*mcp.Client) error) error {
	if e := mcpTeacher(p); e != nil {
		return e
	}
	var name, base string
	var key sql.NullString
	var env mcp.Envelope
	e := r.DB.QueryRowContext(ctx, "SELECT name,base_url,token_cipher,token_nonce,key_id FROM mcp_servers WHERE id=? AND class_id=?", id, p.ClassID()).Scan(&name, &base, &env.Cipher, &env.Nonce, &key)
	if e == sql.ErrNoRows {
		return MCPNotFound
	}
	if e != nil {
		return MCPUnavailable
	}
	env.KeyID = key.String
	token := ""
	if key.Valid {
		token, e = r.Vault.Open(env, credentialBinding(p.ClassID(), name))
		if e != nil {
			return e
		}
	}
	client, e := mcp.NewClient(base, token, r.Policy)
	if e != nil {
		return e
	}
	return use(client)
}

type MCPCall = mcp.AuditCall

var sensitiveMCP = regexp.MustCompile(`(?i)(authorization|bearer|password|api[_-]?key|auth[_-]?token|secret)\s*[:= ]\s*[^\s,;]+`)

// AppendTerminal performs only a terminal INSERT. Unknown names intentionally
// return recorded=false; there is no fabricated server identity or pending row.
func (r MCP) AppendTerminal(ctx context.Context, p auth.Principal, server, tool, status, text string, conversationID *int64) (bool, error) {
	return r.appendTerminal(ctx, p, server, tool, status, text, conversationID, nil)
}
func (r MCP) appendTerminal(ctx context.Context, p auth.Principal, server, tool, status, text string, conversationID *int64, expected *runtimeStamp) (bool, error) {
	if e := mcpScope(p); e != nil {
		return false, e
	}
	if !mcp.ServerName.MatchString(server) || !mcp.ToolName.MatchString(tool) || (status != "ok" && status != "denied" && status != "error") {
		return false, mcp.Invalid
	}
	tx, e := r.DB.BeginTx(ctx, nil)
	if e != nil {
		return false, MCPUnavailable
	}
	defer tx.Rollback()
	var id, revision int64
	var connected bool
	var key sql.NullString
	var env mcp.Envelope
	// Resolve without a lock, then lock the primary record just like DELETE.
	// Locking the secondary name index first can deadlock with deletion's PK lock.
	var lookupID int64
	e = tx.QueryRowContext(ctx, "SELECT id FROM mcp_servers WHERE class_id=? AND name=?", p.ClassID(), server).Scan(&lookupID)
	if e == sql.ErrNoRows {
		if expected != nil {
			expected.Call.Status = "denied"
			expected.Call.Text = mcp.DeniedText
		}
		return false, nil
	}
	if e != nil {
		return false, MCPUnavailable
	}
	e = tx.QueryRowContext(ctx, "SELECT id,connected,token_cipher,token_nonce,key_id,revision FROM mcp_servers FORCE INDEX(PRIMARY) WHERE id=? AND class_id=? AND name=? FOR SHARE", lookupID, p.ClassID(), server).Scan(&id, &connected, &env.Cipher, &env.Nonce, &key, &revision)
	if e == sql.ErrNoRows {
		if expected != nil {
			expected.Call.Status = "denied"
			expected.Call.Text = mcp.DeniedText
		}
		return false, nil
	}
	if e != nil {
		return false, MCPUnavailable
	}
	if expected != nil && expected.ID != id {
		expected.Call.Status = "denied"
		expected.Call.Text = mcp.DeniedText
		return false, nil
	}
	if expected != nil && expected.Revision != revision {
		status = "denied"
	}
	if !connected {
		status = "denied"
	}
	if expected != nil && expected.Call.Status != status {
		expected.Call.Status = status
		expected.Call.Text = mcp.DeniedText
	}
	detail := "工具调用失败。"
	if expected != nil && status == "error" {
		detail = expected.Call.Text
	}
	if status == "denied" {
		detail = "工具调用未获允许。"
	} else if status == "ok" {
		if !utf8.ValidString(text) {
			return false, mcp.Invalid
		}
		env.KeyID = key.String
		if key.Valid {
			token, e := r.Vault.Open(env, credentialBinding(p.ClassID(), server))
			if e != nil {
				return false, e
			}
			if token != "" {
				text = strings.ReplaceAll(text, token, "[redacted]")
			}
		}
		text = sensitiveMCP.ReplaceAllString(text, "[redacted]")
		runes := []rune(text)
		detail = string(runes[:min(len(runes), 200)])
	}
	if _, e = tx.ExecContext(ctx, "INSERT INTO mcp_tool_calls(class_id,server_id,user_id,conversation_id,tool_ref,tool_name,server_name,status,detail,created_at) VALUES(?,?,?,?,?,?,?,?,?,?)", p.ClassID(), id, p.UserID(), conversationID, tool+"@"+server, tool, server, status, detail, mcpNow()); e != nil {
		return false, MCPUnavailable
	}
	if tx.Commit() != nil {
		return false, MCPUnavailable
	}
	return true, nil
}
func (r MCP) Calls(ctx context.Context, p auth.Principal, limit int) ([]MCPCall, error) {
	if e := mcpTeacher(p); e != nil {
		return nil, e
	}
	if limit <= 0 {
		return nil, mcp.Invalid
	}
	limit = min(limit, 100)
	rows, e := r.DB.QueryContext(ctx, "SELECT id,tool_ref,server_name,tool_name,status,detail,user_id,conversation_id,created_at FROM mcp_tool_calls WHERE class_id=? ORDER BY created_at DESC,id DESC LIMIT ?", p.ClassID(), limit)
	if e != nil {
		return nil, MCPUnavailable
	}
	defer rows.Close()
	out := []MCPCall{}
	for rows.Next() {
		var v MCPCall
		var cid sql.NullInt64
		if rows.Scan(&v.ID, &v.Ref, &v.ServerName, &v.ToolName, &v.Status, &v.Detail, &v.UserID, &cid, &v.CreatedAt) != nil {
			return nil, MCPUnavailable
		}
		if cid.Valid {
			v.ConversationID = &cid.Int64
		}
		out = append(out, v)
	}
	if rows.Err() != nil {
		return nil, MCPUnavailable
	}
	return out, nil
}
func (r MCP) RecentCalls(ctx context.Context, p auth.Principal) ([]MCPCall, error) {
	return r.Calls(ctx, p, 50)
}
