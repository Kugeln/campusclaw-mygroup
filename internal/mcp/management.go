package mcp

import "errors"

// Shared safe views keep HTTP independent from the MySQL implementation.
var Unavailable = errors.New("MCP storage unavailable")
var NotFound = errors.New("MCP resource not found")
var Forbidden = errors.New("MCP teacher role required")
var Stale = errors.New("MCP configuration changed")

type Server struct {
	ID        int64  `json:"id" binding:"required"`
	Name      string `json:"name" binding:"required"`
	BaseURL   string `json:"base_url" binding:"required"`
	Connected bool   `json:"connected" binding:"required"`
	HasAuth   bool   `json:"has_auth" binding:"required"`
	ToolCount int    `json:"tool_count" binding:"required"`
	Revision  int64  `json:"-"`
}
type AuditCall struct {
	ID             int64  `json:"id" binding:"required"`
	Ref            string `json:"tool_ref" binding:"required"`
	ServerName     string `json:"server_name" binding:"required"`
	ToolName       string `json:"tool_name" binding:"required"`
	Status         string `json:"status" binding:"required"`
	Detail         string `json:"detail" binding:"required"`
	UserID         int64  `json:"user_id" binding:"required"`
	ConversationID *int64 `json:"conversation_id" binding:"required" extensions:"x-nullable"`
	CreatedAt      string `json:"created_at" binding:"required"`
}
