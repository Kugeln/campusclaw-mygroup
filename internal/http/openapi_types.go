package httpapi

import (
	"campusclaw/internal/conversation"
	"campusclaw/internal/material"
	"campusclaw/internal/mcp"
	"campusclaw/internal/rag"
	"campusclaw/internal/tutor"
)

// Request models belong to the handler contract; annotations reference these
// compiled Go types. No separate JSON/Python schema source is maintained.
type APIError struct {
	Error string `json:"error" binding:"required"`
}
type LoginInput struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required" format:"password"`
}
type IdentityOutput struct {
	UserID    int64  `json:"user_id" binding:"required"`
	Role      string `json:"role" binding:"required" enums:"teacher,student"`
	ClassID   int64  `json:"class_id" binding:"required"`
	ClassName string `json:"class_name" binding:"required"`
}

// AgentInput is a complete update, including two non-null arrays. Lengths are
// Unicode characters, not UTF-8 bytes. Empty prompt uses the frozen default role.
type AgentInput struct {
	Name        string   `json:"name" binding:"required" minLength:"1" maxLength:"64"`
	Prompt      string   `json:"prompt" binding:"required" maxLength:"2000"`
	Enabled     bool     `json:"enabled" binding:"required"`
	SkillIDs    []int64  `json:"skill_ids" binding:"required"`
	MaterialIDs []string `json:"material_ids" binding:"required"`
}
type SkillSourceInput struct {
	Source string `json:"source" binding:"required" minLength:"1" maxLength:"8000"`
}

// SkillUpdateInput is represented as oneOf via the handler's x-request-oneof
// annotation, preserving enabled=false and rejecting mixed/unknown fields.
type SkillUpdateInput struct {
	Source  string `json:"source,omitempty" minLength:"1" maxLength:"8000"`
	Enabled bool   `json:"enabled,omitempty"`
}
type CreateConversationInput struct {
	AgentID int64 `json:"agent_id" binding:"required" minimum:"1"`
}
type SendMessageInput struct {
	Content string `json:"content" binding:"required" minLength:"2" maxLength:"500"`
}
type AskInput struct {
	Question string `json:"question" binding:"required" minLength:"1"`
	Mode     string `json:"mode,omitempty" enums:"keyword,vector,hybrid" default:"hybrid"`
}
type ReindexInput struct {
	Strategy  string `json:"strategy,omitempty" enums:"fixed,markdown,delimiter" default:"fixed"`
	Size      int    `json:"size,omitempty" minimum:"1" maximum:"20971520" default:"800"`
	Overlap   int    `json:"overlap,omitempty" minimum:"0"`
	Delimiter string `json:"delimiter,omitempty"`
}
type UploadOutput struct {
	MaterialID  string `json:"material_id" binding:"required" format:"uuid"`
	IndexStatus string `json:"index_status,omitempty" enums:"ready,failed"`
}
type IndexOutput struct {
	MaterialID  string `json:"material_id" binding:"required" format:"uuid"`
	IndexStatus string `json:"index_status" binding:"required"`
}
type AgentList struct {
	Agents []tutor.Agent `json:"agents" binding:"required"`
}
type SkillList struct {
	Skills []tutor.Skill `json:"skills" binding:"required"`
}
type MaterialList struct {
	Items []material.Item `json:"items" binding:"required"`
}
type SearchOutput struct {
	Mode  string         `json:"mode" binding:"required" enums:"keyword,vector,hybrid"`
	Items []rag.Evidence `json:"items" binding:"required"`
}
type ConversationList struct {
	Conversations []conversation.Conversation `json:"conversations" binding:"required"`
}
type ConversationOutput struct {
	Conversation conversation.Conversation `json:"conversation" binding:"required"`
	Messages     []conversation.Message    `json:"messages" binding:"required"`
}

// Event shapes are compiled here and explicitly referenced by the SSE operation
// annotation. The converter only resolves those references; it knows no routes.
type SSEMeta struct {
	ToolCalls []mcp.ToolCall `json:"tool_calls,omitempty"`
	Status    string         `json:"status" binding:"required" enums:"ok"`
	Citations []rag.Source   `json:"citations" binding:"required"`
	SkillName string         `json:"skill_name" binding:"required"`
	MessageID int64          `json:"message_id" binding:"required"`
}
type SSEDelta struct {
	Text string `json:"text" binding:"required"`
}
type SSEDone struct {
	MessageID int64        `json:"message_id" binding:"required"`
	State     string       `json:"state" binding:"required" enums:"completed"`
	Citations []rag.Source `json:"citations" binding:"required"`
	SkillName string       `json:"skill_name" binding:"required"`
}
type SSEError struct {
	Error  string `json:"error" binding:"required"`
	Reason string `json:"reason" binding:"required"`
}
type SSEEnvelope struct {
	Meta  SSEMeta  `json:"meta"`
	Delta SSEDelta `json:"delta"`
	Done  SSEDone  `json:"done"`
	Error SSEError `json:"error"`
}
