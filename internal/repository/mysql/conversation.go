package mysql

import (
	"campusclaw/internal/conversation"
	"campusclaw/internal/rag"
	"campusclaw/internal/tutor"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type Conversations struct{ DB *sql.DB }

const conversationColumns = "id,agent_id,user_id,class_id,title,created_at,updated_at,COALESCE(active_turn_id,'')"

func scanConversation(row interface{ Scan(...any) error }) (conversation.Conversation, error) {
	var v conversation.Conversation
	e := row.Scan(&v.ID, &v.AgentID, &v.UserID, &v.ClassID, &v.Title, &v.CreatedAt, &v.UpdatedAt, &v.Active)
	if e == sql.ErrNoRows {
		return v, conversation.NotFound
	}
	if e != nil {
		return v, conversation.Unavailable
	}
	return v, nil
}
func (r Conversations) Create(ctx context.Context, s rag.Scope, agent int64) (conversation.Conversation, error) {
	tx, e := r.DB.BeginTx(ctx, nil)
	if e != nil {
		return conversation.Conversation{}, conversation.Unavailable
	}
	defer tx.Rollback()
	var available int
	if tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM assistants WHERE class_id=? AND enabled=1", s.ClassID).Scan(&available) != nil {
		return conversation.Conversation{}, conversation.Unavailable
	}
	if available == 0 {
		return conversation.Conversation{}, conversation.Invalid
	}
	var id int64
	if e = tx.QueryRowContext(ctx, "SELECT id FROM assistants WHERE id=? AND class_id=? AND enabled=1 FOR SHARE", agent, s.ClassID).Scan(&id); e != nil {
		if e == sql.ErrNoRows {
			return conversation.Conversation{}, conversation.NotFound
		}
		return conversation.Conversation{}, conversation.Unavailable
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	result, e := tx.ExecContext(ctx, "INSERT INTO conversations(agent_id,user_id,class_id,title,created_at,updated_at) VALUES (?,?,?,'新的提问',?,?)", agent, s.UserID, s.ClassID, now, now)
	if e != nil {
		return conversation.Conversation{}, conversation.Unavailable
	}
	id, e = result.LastInsertId()
	if e != nil || tx.Commit() != nil {
		return conversation.Conversation{}, conversation.Unavailable
	}
	return conversation.Conversation{ID: id, AgentID: agent, UserID: s.UserID, ClassID: s.ClassID, Title: "新的提问", CreatedAt: now, UpdatedAt: now}, nil
}
func (r Conversations) List(ctx context.Context, s rag.Scope) ([]conversation.Conversation, error) {
	rows, e := r.DB.QueryContext(ctx, "SELECT "+conversationColumns+" FROM conversations WHERE user_id=? AND class_id=? ORDER BY updated_at DESC,id DESC", s.UserID, s.ClassID)
	if e != nil {
		return nil, conversation.Unavailable
	}
	defer rows.Close()
	out := []conversation.Conversation{}
	for rows.Next() {
		v, e := scanConversation(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	if rows.Err() != nil {
		return nil, conversation.Unavailable
	}
	return out, nil
}
func (r Conversations) Get(ctx context.Context, s rag.Scope, id int64) (conversation.Conversation, []conversation.Message, error) {
	c, e := scanConversation(r.DB.QueryRowContext(ctx, "SELECT "+conversationColumns+" FROM conversations WHERE id=? AND user_id=? AND class_id=?", id, s.UserID, s.ClassID))
	if e != nil {
		return c, nil, e
	}
	rows, e := r.DB.QueryContext(ctx, `SELECT id,role,content,status,COALESCE(result_status,''),created_at,COALESCE(citations,JSON_ARRAY()),skill_name,COALESCE(prompt_revision,0),COALESCE(skill_version,''),configuration_snapshot,evidence_snapshot,COALESCE(skill_hash,'') FROM conversation_messages WHERE conversation_id=? AND class_id=? ORDER BY sequence_number`, id, s.ClassID)
	if e != nil {
		return c, nil, conversation.Unavailable
	}
	defer rows.Close()
	messages := []conversation.Message{}
	for rows.Next() {
		var m conversation.Message
		var citations, snapshot, evidence []byte
		if rows.Scan(&m.ID, &m.Role, &m.Content, &m.State, &m.ResultStatus, &m.CreatedAt, &citations, &m.SkillName, &m.PromptRevision, &m.SkillVersion, &snapshot, &evidence, &m.SkillHash) != nil {
			return c, nil, conversation.Unavailable
		}
		if json.Unmarshal(citations, &m.Citations) != nil {
			return c, nil, conversation.Unavailable
		}
		if len(snapshot) > 0 {
			if json.Unmarshal(snapshot, &m.Snapshot) != nil {
				return c, nil, conversation.Unavailable
			}
		}
		if len(evidence) > 0 {
			if json.Unmarshal(evidence, &m.Evidence) != nil {
				return c, nil, conversation.Unavailable
			}
		}
		messages = append(messages, m)
	}
	if rows.Err() != nil {
		return c, nil, conversation.Unavailable
	}
	return c, messages, nil
}
func (r Conversations) History(ctx context.Context, s rag.Scope, id int64) ([]conversation.ModelMessage, error) {
	if _, err := scanConversation(r.DB.QueryRowContext(ctx, "SELECT "+conversationColumns+" FROM conversations WHERE id=? AND user_id=? AND class_id=?", id, s.UserID, s.ClassID)); err != nil {
		return nil, err
	}
	// Ownership is repeated in the query; callers cannot use history as a bypass.
	rows, e := r.DB.QueryContext(ctx, `SELECT m.role,m.content,m.turn_id,COALESCE(c.active_turn_id,'') FROM conversation_messages m JOIN conversations c ON c.id=m.conversation_id AND c.class_id=m.class_id WHERE c.id=? AND c.user_id=? AND c.class_id=? AND m.role IN ('user','assistant') AND (m.role='user' OR m.status='completed') ORDER BY m.sequence_number DESC`, id, s.UserID, s.ClassID)
	if e != nil {
		return nil, conversation.Unavailable
	}
	defer rows.Close()
	out := []conversation.ModelMessage{}
	for rows.Next() {
		var m conversation.ModelMessage
		var turn, active string
		if rows.Scan(&m.Role, &m.Content, &turn, &active) != nil {
			return nil, conversation.Unavailable
		}
		m.Current = m.Role == "user" && active != "" && turn == active
		valid := conversation.Window([]conversation.ModelMessage{m})
		if len(valid) > 0 {
			out = append(out, valid[0])
		}
		if len(out) == 6 {
			break
		}
	}
	if rows.Err() != nil {
		return nil, conversation.Unavailable
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, nil
}
func newTurnID() (string, error) {
	var b [16]byte
	if _, e := rand.Read(b[:]); e != nil {
		return "", e
	}
	b[6] = (b[6] & 15) | 64
	b[8] = (b[8] & 63) | 128
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[:4], b[4:6], b[6:8], b[8:10], b[10:]), nil
}
func (r Conversations) Start(ctx context.Context, s rag.Scope, c conversation.Conversation, q string, p tutor.Prepared) (conversation.Turn, error) {
	tx, e := r.DB.BeginTx(ctx, nil)
	if e != nil {
		return conversation.Turn{}, conversation.Unavailable
	}
	defer tx.Rollback()
	locked, e := scanConversation(tx.QueryRowContext(ctx, "SELECT "+conversationColumns+" FROM conversations WHERE id=? AND user_id=? AND class_id=? FOR UPDATE", c.ID, s.UserID, s.ClassID))
	if e != nil {
		return conversation.Turn{}, e
	}
	if locked.Active != "" {
		return conversation.Turn{}, conversation.Busy
	}
	if p.Snapshot.ClassID != s.ClassID || p.Snapshot.Agent.ID != locked.AgentID {
		return conversation.Turn{}, conversation.NotFound
	}
	token, e := newTurnID()
	if e != nil {
		return conversation.Turn{}, conversation.Unavailable
	}
	var sequence int64
	if tx.QueryRowContext(ctx, "SELECT COALESCE(MAX(sequence_number),0)+1 FROM conversation_messages WHERE conversation_id=?", c.ID).Scan(&sequence) != nil {
		return conversation.Turn{}, conversation.Unavailable
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, e = tx.ExecContext(ctx, "INSERT INTO conversation_messages(conversation_id,class_id,turn_id,sequence_number,role,content,status,created_at,citations) VALUES (?,?,?,?,'user',?,'completed',?,JSON_ARRAY())", c.ID, s.ClassID, token, sequence, q, now); e != nil {
		return conversation.Turn{}, conversation.Unavailable
	}
	config, _ := json.Marshal(p.Snapshot)
	evidence, _ := json.Marshal(p.Evidence)
	var selected any
	if p.Snapshot.Selected != nil {
		selected = p.Snapshot.Selected.ID
	}
	// Start accepts the question only. A selected skill is not a model call.
	res, e := tx.ExecContext(ctx, `INSERT INTO conversation_messages(conversation_id,class_id,turn_id,sequence_number,role,content,status,result_status,created_at,prompt_snapshot,prompt_revision,configuration_snapshot,selected_skill_id,skill_id,skill_name,skill_version,skill_hash,selection_reason,evidence_snapshot,citations) VALUES (?,?,?,?,'assistant','','generating','ok',?,?,?,?,?,NULL,'','','','preparing',?,JSON_ARRAY())`, c.ID, s.ClassID, token, sequence+1, now, p.Snapshot.Agent.Prompt, p.Snapshot.Agent.Revision, config, selected, evidence)
	if e != nil {
		return conversation.Turn{}, conversation.Unavailable
	}
	id, e := res.LastInsertId()
	if e != nil {
		return conversation.Turn{}, conversation.Unavailable
	}
	title := locked.Title
	if sequence == 1 {
		first := []rune(strings.TrimSpace(q))
		if len(first) > 0 {
			title = string(first[:min(24, len(first))])
		}
	}
	if _, e = tx.ExecContext(ctx, "UPDATE conversations SET active_turn_id=?,updated_at=?,title=? WHERE id=?", token, now, title, c.ID); e != nil || tx.Commit() != nil {
		return conversation.Turn{}, conversation.Unavailable
	}
	return conversation.Turn{ID: id, Token: token, Completed: false}, nil
}

// Prepare and ModelStarted are short autocommit writes, scoped to the active
// owner/class/turn token. No transaction crosses retrieval or the model gateway.
func (r Conversations) Prepare(ctx context.Context, s rag.Scope, id int64, t conversation.Turn, p tutor.Prepared) error {
	if p.Snapshot.ClassID != s.ClassID {
		return conversation.NotFound
	}
	config, configErr := json.Marshal(p.Snapshot)
	if configErr != nil {
		return conversation.Invalid
	}
	evidence, err := json.Marshal(p.Evidence)
	if err != nil {
		return conversation.Invalid
	}
	res, err := r.DB.ExecContext(ctx, `UPDATE conversation_messages m JOIN conversations c ON c.id=m.conversation_id AND c.class_id=m.class_id SET m.configuration_snapshot=?,m.evidence_snapshot=?,m.selection_reason='prepared' WHERE m.id=? AND m.turn_id=? AND m.status='generating' AND c.id=? AND c.user_id=? AND c.class_id=? AND c.agent_id=? AND c.active_turn_id=?`, config, evidence, t.ID, t.Token, id, s.UserID, s.ClassID, p.Snapshot.Agent.ID, t.Token)
	return turnWrite(res, err)
}
func (r Conversations) ModelStarted(ctx context.Context, s rag.Scope, id int64, t conversation.Turn, p tutor.Prepared) error {
	var actual any
	name, version, hash := "", "", ""
	if p.Actual != nil {
		actual = p.Actual.ID
		name = p.Actual.Name
		version = p.Actual.Version
		hash = p.Actual.Hash
	}
	res, err := r.DB.ExecContext(ctx, `UPDATE conversation_messages m JOIN conversations c ON c.id=m.conversation_id AND c.class_id=m.class_id SET m.skill_id=?,m.skill_name=?,m.skill_version=?,m.skill_hash=?,m.selection_reason='model_started' WHERE m.id=? AND m.turn_id=? AND m.status='generating' AND m.selection_reason='prepared' AND c.id=? AND c.user_id=? AND c.class_id=? AND c.active_turn_id=?`, actual, name, version, hash, t.ID, t.Token, id, s.UserID, s.ClassID, t.Token)
	return turnWrite(res, err)
}
func turnWrite(res sql.Result, err error) error {
	if err != nil {
		return conversation.Unavailable
	}
	n, err := res.RowsAffected()
	if err != nil || n != 1 {
		return conversation.Unavailable
	}
	return nil
}
func (r Conversations) Append(ctx context.Context, s rag.Scope, id int64, t conversation.Turn, text string) error {
	res, e := r.DB.ExecContext(ctx, `UPDATE conversation_messages m JOIN conversations c ON c.id=m.conversation_id SET m.content=CONCAT(m.content,?) WHERE m.id=? AND m.turn_id=? AND m.status='generating' AND c.id=? AND c.user_id=? AND c.class_id=? AND c.active_turn_id=?`, text, t.ID, t.Token, id, s.UserID, s.ClassID, t.Token)
	if e != nil {
		return conversation.Unavailable
	}
	n, _ := res.RowsAffected()
	if n != 1 {
		return conversation.Unavailable
	}
	return nil
}
func (r Conversations) Finish(ctx context.Context, s rag.Scope, id int64, t conversation.Turn, state string, sources []rag.Source, reason string) error {
	if state != "completed" && state != "failed" && state != "interrupted" {
		return conversation.Invalid
	}
	tx, e := r.DB.BeginTx(ctx, nil)
	if e != nil {
		return conversation.Unavailable
	}
	defer tx.Rollback()
	c, e := scanConversation(tx.QueryRowContext(ctx, "SELECT "+conversationColumns+" FROM conversations WHERE id=? AND user_id=? AND class_id=? FOR UPDATE", id, s.UserID, s.ClassID))
	if e != nil {
		return e
	}
	if c.Active != t.Token {
		return conversation.Unavailable
	}
	if sources == nil {
		sources = []rag.Source{}
	}
	b, _ := json.Marshal(sources)
	res, e := tx.ExecContext(ctx, "UPDATE conversation_messages SET status=?,citations=?,failure_reason=? WHERE id=? AND turn_id=? AND status='generating'", state, b, reason, t.ID, t.Token)
	if e != nil {
		return conversation.Unavailable
	}
	n, _ := res.RowsAffected()
	if n != 1 {
		return conversation.Unavailable
	}
	if _, e = tx.ExecContext(ctx, "UPDATE conversations SET active_turn_id=NULL,updated_at=? WHERE id=?", time.Now().UTC().Format(time.RFC3339Nano), id); e != nil || tx.Commit() != nil {
		return conversation.Unavailable
	}
	return nil
}

// Only called at startup of the course's single writer, never during live turns.
func (r Conversations) Recover(ctx context.Context) error {
	tx, e := r.DB.BeginTx(ctx, nil)
	if e != nil {
		return conversation.Unavailable
	}
	defer tx.Rollback()
	if _, e = tx.ExecContext(ctx, "UPDATE conversation_messages SET status='interrupted',failure_reason='process_interrupted',citations=JSON_ARRAY() WHERE status='generating'"); e != nil {
		return conversation.Unavailable
	}
	if _, e = tx.ExecContext(ctx, "UPDATE conversations SET active_turn_id=NULL WHERE active_turn_id IS NOT NULL"); e != nil || tx.Commit() != nil {
		return conversation.Unavailable
	}
	return nil
}
