package mysql

import (
	"campusclaw/internal/auth"
	"campusclaw/internal/mcp"
	"campusclaw/internal/rag"
	"campusclaw/internal/tutor"
	"context"
	"database/sql"
	"errors"
	"sort"
	"time"
)

// The locks cover the entire skill write, but never cross a network request.
func validateSkillTools(ctx context.Context, tx *sql.Tx, s rag.Scope, refs []string) error {
	ordered := append([]string{}, refs...)
	sort.Strings(ordered)
	for _, ref := range ordered {
		if ref == "knowledge_search" {
			continue
		}
		tool, server, ok := mcp.Reference(ref)
		if !ok {
			return tutor.Invalid
		}
		var connected bool
		var schema []byte
		var name sql.NullString
		e := tx.QueryRowContext(ctx, "SELECT s.connected,t.tool_name,t.schema_json FROM mcp_servers s LEFT JOIN mcp_tools t ON t.server_id=s.id AND t.class_id=s.class_id AND t.tool_name=? WHERE s.class_id=? AND s.name=? FOR SHARE", tool, s.ClassID, server).Scan(&connected, &name, &schema)
		if e == sql.ErrNoRows {
			return tutor.Invalid
		}
		if e != nil {
			return tutor.Unavailable
		}
		if !connected || !name.Valid || mcp.ValidateTools([]mcp.Tool{{Name: tool, InputSchema: schema}}) != nil {
			return tutor.Invalid
		}
	}
	return nil
}

type runtimeStamp struct {
	ID, Revision int64
	Call         *mcp.ToolCall
}

// ExecuteTool resolves current class/name and cache anew for each call. It
// releases all database reads before RPC, and revalidates under the audit lock.
func (r MCP) ExecuteTool(ctx context.Context, p auth.Principal, ref string, args map[string]any, cid int64) (mcp.ToolCall, error) {
	out := mcp.ToolCall{Ref: ref, Status: "denied", Text: mcp.DeniedText}
	if e := mcpScope(p); e != nil {
		return out, e
	}
	tool, server, ok := mcp.Reference(ref)
	if !ok {
		return out, mcp.Invalid
	}
	var id, revision int64
	var connected bool
	var base string
	var schema []byte
	var name, key sql.NullString
	var env mcp.Envelope
	e := r.DB.QueryRowContext(ctx, "SELECT s.id,s.revision,s.connected,s.base_url,s.token_cipher,s.token_nonce,s.key_id,t.tool_name,t.schema_json FROM mcp_servers s LEFT JOIN mcp_tools t ON t.server_id=s.id AND t.class_id=s.class_id AND t.tool_name=? WHERE s.class_id=? AND s.name=?", tool, p.ClassID(), server).Scan(&id, &revision, &connected, &base, &env.Cipher, &env.Nonce, &key, &name, &schema)
	if e == sql.ErrNoRows {
		return out, nil
	}
	if e != nil {
		return out, MCPUnavailable
	}
	if connected && name.Valid && mcp.SchemaArguments(schema, args) && ctx.Err() == nil {
		token := ""
		env.KeyID = key.String
		if key.Valid {
			token, e = r.Vault.Open(env, credentialBinding(p.ClassID(), server))
		}
		if e == nil {
			var client *mcp.Client
			client, e = mcp.NewClient(base, token, r.Policy)
			if e == nil {
				var result mcp.CallResult
				result, e = client.Call(ctx, tool, args)
				if e == nil {
					out = mcp.SafeResult(result, append(r.Secrets, token)...)
					out.Ref = ref
				}
			}
		}
		if e != nil {
			out.Status, out.Text = "error", mcp.ErrorText
			if errors.Is(e, mcp.AuthFailed) {
				out.Text = "工具服务鉴权失败，已继续原问答。"
			}
			if errors.Is(e, mcp.Timeout) {
				out.Text = "工具调用超时，已继续原问答。"
			}
		}
	}
	// Terminal audit survives explicit request cancellation, with a bounded recovery write.
	// An exhausted stage/turn deadline is never extended: failed audit returns 503.
	// Normal calls remain on the same stage/turn deadline.
	auditCtx := ctx
	cancel := func() {}
	if errors.Is(ctx.Err(), context.Canceled) {
		auditCtx, cancel = context.WithTimeout(context.WithoutCancel(ctx), time.Second)
	}
	defer cancel()
	var conversation *int64
	if cid > 0 {
		conversation = &cid
	}
	_, e = r.appendTerminal(auditCtx, p, server, tool, out.Status, out.Text, conversation, &runtimeStamp{id, revision, &out})
	if e != nil {
		return out, e
	}
	return out, nil
}
