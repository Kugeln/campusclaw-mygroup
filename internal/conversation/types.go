package conversation

import (
	"campusclaw/internal/rag"
	"campusclaw/internal/tutor"
	"context"
	"errors"
	"strings"
	"unicode/utf8"
)

var Invalid = errors.New("Invalid conversation request")
var NotFound = errors.New("Resource not found")
var Busy = errors.New("Conversation generation in progress")
var Unavailable = errors.New("Conversation storage unavailable")

type Conversation struct {
	ID        int64  `json:"id"`
	AgentID   int64  `json:"agent_id"`
	UserID    int64  `json:"user_id"`
	ClassID   int64  `json:"class_id"`
	Title     string `json:"title"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
	Active    string `json:"-"`
}
type Message struct {
	ID             int64           `json:"id"`
	Role           string          `json:"role"`
	Content        string          `json:"content"`
	State          string          `json:"state"`
	ResultStatus   string          `json:"result_status"`
	CreatedAt      string          `json:"created_at"`
	Citations      []rag.Source    `json:"citations"`
	SkillName      string          `json:"skill_name"`
	PromptRevision int64           `json:"prompt_revision"`
	SkillVersion   string          `json:"skill_version"`
	Snapshot       *tutor.Snapshot `json:"configuration_snapshot,omitempty"`
	Evidence       []rag.Evidence  `json:"evidence_snapshot,omitempty"`
	SkillHash      string          `json:"skill_hash,omitempty"`
}
type ModelMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
	// Set only from the server's active turn, never accepted from a client.
	Current bool `json:"-"`
}
type Turn struct {
	ID        int64
	Token     string
	Completed bool
}
type Store interface {
	Create(context.Context, rag.Scope, int64) (Conversation, error)
	List(context.Context, rag.Scope) ([]Conversation, error)
	Get(context.Context, rag.Scope, int64) (Conversation, []Message, error)
	History(context.Context, rag.Scope, int64) ([]ModelMessage, error)
	Start(context.Context, rag.Scope, Conversation, string, tutor.Prepared) (Turn, error)
	Prepare(context.Context, rag.Scope, int64, Turn, tutor.Prepared) error
	ModelStarted(context.Context, rag.Scope, int64, Turn, tutor.Prepared) error
	Append(context.Context, rag.Scope, int64, Turn, string) error
	Finish(context.Context, rag.Scope, int64, Turn, string, []rag.Source, string) error
	Recover(context.Context) error
	Delete(context.Context, rag.Scope, int64) error
}

func Question(text string) (string, error) {
	text = strings.TrimSpace(text)
	if !utf8.ValidString(text) || utf8.RuneCountInString(text) < 2 || utf8.RuneCountInString(text) > 500 {
		return "", Invalid
	}
	return text, nil
}
func Window(history []ModelMessage) []ModelMessage {
	out := []ModelMessage{}
	for _, m := range history {
		if (m.Role != "user" && m.Role != "assistant") || !utf8.ValidString(m.Content) || strings.TrimSpace(m.Content) == "" {
			continue
		}
		runes := []rune(m.Content)
		m.Content = string(runes[:min(500, len(runes))])
		if strings.TrimSpace(m.Content) == "" {
			continue
		}
		out = append(out, m)
	}
	if len(out) > 6 {
		out = out[len(out)-6:]
	}
	return out
}
func Context(history []ModelMessage) ([]ModelMessage, string) {
	out := Window(history)
	previous := ""
	for i := len(out) - 1; i >= 0; i-- {
		if out[i].Role == "user" && !out[i].Current {
			previous = out[i].Content
			break
		}
	}
	return out, previous
}

// WithCurrent includes the accepted current user exactly once. Pure model
// assembly callers may also supply a window without the current input.
// Equal text in a prior turn is legitimate history, not a duplicate turn.
func WithCurrent(history []ModelMessage, q string) []ModelMessage {
	out := []ModelMessage{}
	for _, m := range history {
		if !m.Current {
			out = append(out, m)
		}
	}
	out = append(out, ModelMessage{Role: "user", Content: q, Current: true})
	return Window(out)
}
