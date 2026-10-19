package httpapi

import (
	"campusclaw/internal/auth"
	"campusclaw/internal/mcp"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// MCPHandler reuses the class-scoped v4 store and the existing identity guards.
type MCPStore interface {
	Servers(context.Context, auth.Principal) ([]mcp.Server, error)
	Get(context.Context, auth.Principal, int64) (mcp.Server, error)
	Create(context.Context, auth.Principal, string, string, string) (mcp.Server, error)
	Update(context.Context, auth.Principal, int64, string, string, string) (mcp.Server, error)
	Delete(context.Context, auth.Principal, int64) error
	Connect(context.Context, auth.Principal, int64) ([]mcp.Tool, error)
	Disconnect(context.Context, auth.Principal, int64) error
	Tools(context.Context, auth.Principal, int64) ([]mcp.Tool, error)
	Calls(context.Context, auth.Principal, int) ([]mcp.AuditCall, error)
}
type MCPHandler struct{ Store MCPStore }
type MCPCreateInput struct {
	// Name is unique within the current class; renaming does not rewrite skill references.
	Name string `json:"name" binding:"required" maxLength:"64" example:"math-demo"`
	// BaseURL is the service root, without credentials, query or fragment; the server appends /mcp and enforces its outbound allowlist.
	BaseURL string `json:"base_url" binding:"required" maxLength:"512" example:"http://mcp-demo:8090"`
	// AuthToken is a write-only service Bearer credential; only has_auth is returned. Empty is valid for services without authentication.
	AuthToken string `json:"auth_token" binding:"required" maxLength:"4096"`
}

// MCPUpdateInput updates configuration and clears connected state and cached tools.
// Omitted or empty auth_token preserves the previous credential; never refill it from a response.
type MCPUpdateInput struct {
	Name      string `json:"name" binding:"required" maxLength:"64"`
	BaseURL   string `json:"base_url" binding:"required" maxLength:"512"`
	AuthToken string `json:"auth_token,omitempty" maxLength:"4096"`
}
type MCPServerList struct {
	Servers []mcp.Server `json:"servers" binding:"required"`
}
type MCPToolList struct {
	Tools []mcp.Tool `json:"tools" binding:"required"`
}
type MCPConnected struct {
	Connected bool       `json:"connected" binding:"required"`
	Tools     []mcp.Tool `json:"tools" binding:"required"`
}
type MCPDisconnected struct {
	Connected bool `json:"connected" binding:"required"`
}
type MCPCallList struct {
	Calls []mcp.AuditCall `json:"calls" binding:"required"`
}

// WithMCP adds exactly the eight official operations to the existing handler.
func WithMCP(base http.Handler, identity *auth.Identity, h *MCPHandler) http.Handler {
	if h == nil || identity == nil {
		return base
	}
	mux := http.NewServeMux()
	protect := func(f http.HandlerFunc) http.Handler { return Authenticate(identity, TeacherRequired(f)) }
	mux.Handle("GET /api/mcp/servers", protect(h.Servers))
	mux.Handle("POST /api/mcp/servers", protect(h.Create))
	mux.Handle("PUT /api/mcp/servers/{id}", protect(h.Update))
	mux.Handle("DELETE /api/mcp/servers/{id}", protect(h.Delete))
	mux.Handle("POST /api/mcp/servers/{id}/connect", protect(h.Connect))
	mux.Handle("POST /api/mcp/servers/{id}/disconnect", protect(h.Disconnect))
	mux.Handle("GET /api/mcp/servers/{id}/tools", protect(h.Tools))
	mux.Handle("GET /api/mcp/tool-calls", protect(h.Calls))
	mux.Handle("/", base)
	return browserHeaders(mux)
}
func mcpHTTPError(w http.ResponseWriter, e error, remote bool) {
	code, message := 503, "MCP service unavailable"
	switch {
	case errors.Is(e, mcp.NotFound):
		code, message = 404, "MCP resource not found"
	case errors.Is(e, mcp.Forbidden):
		code, message = 403, "Teacher role required"
	case errors.Is(e, mcp.Unavailable), errors.Is(e, mcp.CredentialError):
	case remote:
		code, message = 502, "MCP connection failed"
	case errors.Is(e, mcp.Invalid), errors.Is(e, mcp.Denied), errors.Is(e, mcp.Timeout):
		code, message = 400, "Invalid MCP request"
	}
	authError(w, code, message)
}
func mcpQuery(w http.ResponseWriter, r *http.Request, limit bool) bool {
	// ParseQuery rejects malformed percent escapes instead of silently ignoring them.
	q, e := url.ParseQuery(r.URL.RawQuery)
	if e != nil {
		mcpHTTPError(w, mcp.Invalid, false)
		return false
	}
	for k, v := range q {
		if !limit || k != "limit" || len(v) != 1 {
			mcpHTTPError(w, mcp.Invalid, false)
			return false
		}
	}
	return true
}
func mcpID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, e := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if e != nil || id <= 0 {
		mcpHTTPError(w, mcp.Invalid, false)
		return 0, false
	}
	return id, true
}
func mcpInput(w http.ResponseWriter, r *http.Request, create bool) (name, base, token string, ok bool) {
	v, ok := tutorFields(w, r, "name", "base_url", "auth_token")
	if !ok {
		return "", "", "", false
	}
	if v["name"] == nil || v["base_url"] == nil || (create && v["auth_token"] == nil) {
		mcpHTTPError(w, mcp.Invalid, false)
		return "", "", "", false
	}
	for k, target := range map[string]*string{"name": &name, "base_url": &base, "auth_token": &token} {
		if raw := v[k]; raw != nil && json.Unmarshal(raw, target) != nil {
			mcpHTTPError(w, mcp.Invalid, false)
			return "", "", "", false
		}
	}
	return name, base, token, true
}

// Servers documents this registered MCP handler.
// @Summary List current class MCP servers
// @Description Teacher only; class comes from current verified Principal. Legacy authentication storage failure remains 500; MCP database failure is 503.
// @Description Lists only the current class. has_auth is a boolean, never the credential; connected and tool_count reflect persisted connection/cache state.
// @x-success-example {"servers":[{"id":1,"name":"math-demo","base_url":"http://mcp-demo:8090","connected":true,"has_auth":true,"tool_count":2}]}
// @Tags mcp
// @Produce json
// @Security bearerAuth
// @x-error-examples {"400":{"error":"Invalid MCP request"},"401":{"error":"Authentication required"},"403":{"error":"Teacher role required"},"500":{"error":"Authentication storage unavailable"},"503":{"error":"MCP service unavailable"}}
// @Success 200 {object} MCPServerList
// @Failure 400 {object} APIError
// @Failure 401 {object} APIError
// @Failure 403 {object} APIError
// @Failure 500 {object} APIError
// @Failure 503 {object} APIError
// @Router /api/mcp/servers [get]
func (h *MCPHandler) Servers(w http.ResponseWriter, r *http.Request) {
	if !mcpQuery(w, r, false) {
		return
	}
	p, _ := auth.FromContext(r.Context())
	v, e := h.Store.Servers(r.Context(), p)
	if e != nil {
		mcpHTTPError(w, e, false)
		return
	}
	tutorJSON(w, 200, MCPServerList{v})
}

// Create documents this registered MCP handler.
// @Summary Register current class MCP server
// @Description Teacher only; class comes from current verified Principal. Legacy authentication storage failure remains 500; MCP database failure is 503.
// @Description Registers configuration without connecting or calling tools. auth_token is write-only; base_url is a service root and must pass the configured server-side allowlist. Response contains has_auth only.
// @x-success-example {"id":1,"name":"math-demo","base_url":"http://mcp-demo:8090","connected":false,"has_auth":true,"tool_count":0}
// @Tags mcp
// @Produce json
// @Security bearerAuth
// @x-error-examples {"400":{"error":"Invalid MCP request"},"401":{"error":"Authentication required"},"403":{"error":"Teacher role required"},"500":{"error":"Authentication storage unavailable"},"503":{"error":"MCP service unavailable"}}
// @x-strict-request true
// @Accept json
// @Param body body MCPCreateInput true "Configuration; credentials never returned"
// @Success 201 {object} mcp.Server
// @Failure 400 {object} APIError
// @Failure 401 {object} APIError
// @Failure 403 {object} APIError
// @Failure 500 {object} APIError
// @Failure 503 {object} APIError
// @Router /api/mcp/servers [post]
func (h *MCPHandler) Create(w http.ResponseWriter, r *http.Request) {
	if !mcpQuery(w, r, false) {
		return
	}
	p, _ := auth.FromContext(r.Context())
	name, base, token, ok := mcpInput(w, r, true)
	if !ok {
		return
	}
	v, e := h.Store.Create(r.Context(), p, name, base, token)
	if e != nil {
		mcpHTTPError(w, e, false)
		return
	}
	tutorJSON(w, 201, v)
}

// Update documents this registered MCP handler.
// @Summary Update configuration; empty token retains; clears cache
// @Description Teacher only; class comes from current verified Principal. Legacy authentication storage failure remains 500; MCP database failure is 503.
// @Description Omitted or empty auth_token retains the previous credential. Every update disconnects and clears cached tools. Rename does not rewrite tool@server skill references; update and resave skills explicitly.
// @x-success-example {"id":1,"name":"math-demo-renamed","base_url":"http://mcp-demo:8090","connected":false,"has_auth":true,"tool_count":0}
// @Tags mcp
// @Produce json
// @Security bearerAuth
// @x-error-examples {"400":{"error":"Invalid MCP request"},"401":{"error":"Authentication required"},"403":{"error":"Teacher role required"},"404":{"error":"MCP resource not found"},"500":{"error":"Authentication storage unavailable"},"503":{"error":"MCP service unavailable"}}
// @Param id path int true "Current class server ID" minimum(1)
// @x-strict-request true
// @Accept json
// @Param body body MCPUpdateInput true "Configuration; credentials never returned"
// @Success 200 {object} mcp.Server
// @Failure 400 {object} APIError
// @Failure 401 {object} APIError
// @Failure 403 {object} APIError
// @Failure 404 {object} APIError
// @Failure 500 {object} APIError
// @Failure 503 {object} APIError
// @Router /api/mcp/servers/{id} [put]
func (h *MCPHandler) Update(w http.ResponseWriter, r *http.Request) {
	if !mcpQuery(w, r, false) {
		return
	}
	p, _ := auth.FromContext(r.Context())
	id, ok := mcpID(w, r)
	if !ok {
		return
	}
	if _, e := h.Store.Get(r.Context(), p, id); e != nil {
		mcpHTTPError(w, e, false)
		return
	}
	name, base, token, ok := mcpInput(w, r, false)
	if !ok {
		return
	}
	v, e := h.Store.Update(r.Context(), p, id, name, base, token)
	if e != nil {
		mcpHTTPError(w, e, false)
		return
	}
	tutorJSON(w, 200, v)
}

// Delete documents this registered MCP handler.
// @Summary Delete unaudited server; audited deletion returns 503
// @Description Teacher only; class comes from current verified Principal. Legacy authentication storage failure remains 500; MCP database failure is 503.
// @Description Unaudited servers are physically deleted with 204. Existing append-only audits prohibit deletion and return 503; no force-delete or audit purge exists.
// @Tags mcp
// @Produce json
// @Security bearerAuth
// @x-error-examples {"400":{"error":"Invalid MCP request"},"401":{"error":"Authentication required"},"403":{"error":"Teacher role required"},"404":{"error":"MCP resource not found"},"500":{"error":"Authentication storage unavailable"},"503":{"error":"MCP service unavailable"}}
// @Param id path int true "Current class server ID" minimum(1)
// @Success 204 "No Content"
// @Failure 400 {object} APIError
// @Failure 401 {object} APIError
// @Failure 403 {object} APIError
// @Failure 404 {object} APIError
// @Failure 500 {object} APIError
// @Failure 503 {object} APIError
// @Router /api/mcp/servers/{id} [delete]
func (h *MCPHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if !mcpQuery(w, r, false) {
		return
	}
	p, _ := auth.FromContext(r.Context())
	id, ok := mcpID(w, r)
	if !ok {
		return
	}
	if e := h.Store.Delete(r.Context(), p, id); e != nil {
		mcpHTTPError(w, e, false)
		return
	}
	w.WriteHeader(204)
}

// Connect documents this registered MCP handler.
// @Summary Initialize and discover tools; remote failures 502
// @Description Teacher only; class comes from current verified Principal. Legacy authentication storage failure remains 500; MCP database failure is 503.
// @Description Uses stored service Bearer credentials for initialize then tools/list; does not call tools/call. Successful discovery atomically replaces cached tools. Remote/protocol/authentication failure is 502, not browser JWT revocation.
// @x-success-example {"connected":true,"tools":[{"name":"formula_hint","description":"Formula hints","inputSchema":{"type":"object","required":["query"],"properties":{"query":{"type":"string"}}}},{"name":"unit_convert","description":"Unit conversion","inputSchema":{"type":"object","required":["value","from","to"],"properties":{"value":{"type":"number"},"from":{"type":"string"},"to":{"type":"string"}}}}]}
// @Tags mcp
// @Produce json
// @Security bearerAuth
// @x-error-examples {"400":{"error":"Invalid MCP request"},"401":{"error":"Authentication required"},"403":{"error":"Teacher role required"},"404":{"error":"MCP resource not found"},"502":{"error":"MCP connection failed"},"500":{"error":"Authentication storage unavailable"},"503":{"error":"MCP service unavailable"}}
// @Param id path int true "Current class server ID" minimum(1)
// @Success 200 {object} MCPConnected
// @Failure 400 {object} APIError
// @Failure 401 {object} APIError
// @Failure 403 {object} APIError
// @Failure 404 {object} APIError
// @Failure 502 {object} APIError
// @Failure 500 {object} APIError
// @Failure 503 {object} APIError
// @Router /api/mcp/servers/{id}/connect [post]
func (h *MCPHandler) Connect(w http.ResponseWriter, r *http.Request) {
	if !mcpQuery(w, r, false) {
		return
	}
	p, _ := auth.FromContext(r.Context())
	id, ok := mcpID(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	v, e := h.Store.Connect(ctx, p, id)
	if e != nil {
		mcpHTTPError(w, e, true)
		return
	}
	tutorJSON(w, 200, MCPConnected{true, v})
}

// Disconnect documents this registered MCP handler.
// @Summary Idempotently disconnect and clear tools
// @Description Teacher only; class comes from current verified Principal. Legacy authentication storage failure remains 500; MCP database failure is 503.
// @Description Idempotently marks disconnected and removes cached tools; does not delete audits or skill source. Subsequent resolved calls are denied and append terminal audit records.
// @x-success-example {"connected":false}
// @Tags mcp
// @Produce json
// @Security bearerAuth
// @x-error-examples {"400":{"error":"Invalid MCP request"},"401":{"error":"Authentication required"},"403":{"error":"Teacher role required"},"404":{"error":"MCP resource not found"},"500":{"error":"Authentication storage unavailable"},"503":{"error":"MCP service unavailable"}}
// @Param id path int true "Current class server ID" minimum(1)
// @Success 200 {object} MCPDisconnected
// @Failure 400 {object} APIError
// @Failure 401 {object} APIError
// @Failure 403 {object} APIError
// @Failure 404 {object} APIError
// @Failure 500 {object} APIError
// @Failure 503 {object} APIError
// @Router /api/mcp/servers/{id}/disconnect [post]
func (h *MCPHandler) Disconnect(w http.ResponseWriter, r *http.Request) {
	if !mcpQuery(w, r, false) {
		return
	}
	p, _ := auth.FromContext(r.Context())
	id, ok := mcpID(w, r)
	if !ok {
		return
	}
	if e := h.Store.Disconnect(r.Context(), p, id); e != nil {
		mcpHTTPError(w, e, false)
		return
	}
	tutorJSON(w, 200, MCPDisconnected{false})
}

// Tools documents this registered MCP handler.
// @Summary Read cached tools without network I/O
// @Description Teacher only; class comes from current verified Principal. Legacy authentication storage failure remains 500; MCP database failure is 503.
// @Description Returns only cached name/description/inputSchema fields. Does not discover or invoke tools, and does not return credentials or internal revision.
// @x-success-example {"tools":[]}
// @Tags mcp
// @Produce json
// @Security bearerAuth
// @x-error-examples {"400":{"error":"Invalid MCP request"},"401":{"error":"Authentication required"},"403":{"error":"Teacher role required"},"404":{"error":"MCP resource not found"},"500":{"error":"Authentication storage unavailable"},"503":{"error":"MCP service unavailable"}}
// @Param id path int true "Current class server ID" minimum(1)
// @Success 200 {object} MCPToolList
// @Failure 400 {object} APIError
// @Failure 401 {object} APIError
// @Failure 403 {object} APIError
// @Failure 404 {object} APIError
// @Failure 500 {object} APIError
// @Failure 503 {object} APIError
// @Router /api/mcp/servers/{id}/tools [get]
func (h *MCPHandler) Tools(w http.ResponseWriter, r *http.Request) {
	if !mcpQuery(w, r, false) {
		return
	}
	p, _ := auth.FromContext(r.Context())
	id, ok := mcpID(w, r)
	if !ok {
		return
	}
	v, e := h.Store.Tools(r.Context(), p, id)
	if e != nil {
		mcpHTTPError(w, e, false)
		return
	}
	tutorJSON(w, 200, MCPToolList{v})
}

// Calls documents this registered MCP handler.
// @Summary Read current class terminal audit records newest first
// @Description Teacher only; class comes from current verified Principal. Legacy authentication storage failure remains 500; MCP database failure is 503.
// @Description Returns only the current class, newest terminal records first. Default 50, maximum 100 (larger positive limits clamp); detail is a safe summary, not raw arguments or remote errors.
// @x-success-example {"calls":[{"id":1,"tool_ref":"formula_hint@math-demo","server_name":"math-demo","tool_name":"formula_hint","status":"ok","detail":"Safe summary","user_id":3,"conversation_id":1,"created_at":"2026-10-19T00:00:00Z"}]}
// @Tags mcp
// @Produce json
// @Security bearerAuth
// @x-error-examples {"400":{"error":"Invalid MCP request"},"401":{"error":"Authentication required"},"403":{"error":"Teacher role required"},"500":{"error":"Authentication storage unavailable"},"503":{"error":"MCP service unavailable"}}
// @Param limit query int false "Default 50; values above 100 clamp to 100" minimum(1) default(50)
// @Success 200 {object} MCPCallList
// @Failure 400 {object} APIError
// @Failure 401 {object} APIError
// @Failure 403 {object} APIError
// @Failure 500 {object} APIError
// @Failure 503 {object} APIError
// @Router /api/mcp/tool-calls [get]
func (h *MCPHandler) Calls(w http.ResponseWriter, r *http.Request) {
	if !mcpQuery(w, r, true) {
		return
	}
	p, _ := auth.FromContext(r.Context())
	limit := 50
	if raw, exists := r.URL.Query()["limit"]; exists {
		n, e := strconv.Atoi(raw[0])
		if e != nil || n <= 0 {
			mcpHTTPError(w, mcp.Invalid, false)
			return
		}
		limit = n
	}
	v, e := h.Store.Calls(r.Context(), p, limit)
	if e != nil {
		mcpHTTPError(w, e, false)
		return
	}
	tutorJSON(w, 200, MCPCallList{v})
}
