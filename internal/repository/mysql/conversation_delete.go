package mysql

import (
	"campusclaw/internal/conversation"
	"campusclaw/internal/rag"
	"context"
)

// Delete is an internal storage operation. No HTTP route, scheduler or bulk
// cleanup invokes it. Locking serializes deletion against Start/Finish.
func (r Conversations) Delete(ctx context.Context, s rag.Scope, id int64) error {
	return r.deleteConversation(ctx, s, id, nil)
}

func (r Conversations) deleteConversation(ctx context.Context, s rag.Scope, id int64, afterMessages func() error) error {
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return conversation.Unavailable
	}
	defer tx.Rollback()
	c, err := scanConversation(tx.QueryRowContext(ctx, "SELECT "+conversationColumns+" FROM conversations WHERE id=? AND user_id=? AND class_id=? FOR UPDATE", id, s.UserID, s.ClassID))
	if err != nil {
		return err
	}
	if c.Active != "" {
		return conversation.Busy
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM conversation_messages WHERE conversation_id=? AND class_id=?", id, s.ClassID); err != nil {
		return conversation.Unavailable
	}
	if afterMessages != nil {
		if err = afterMessages(); err != nil {
			return conversation.Unavailable
		}
	}
	result, err := tx.ExecContext(ctx, "DELETE FROM conversations WHERE id=? AND user_id=? AND class_id=?", id, s.UserID, s.ClassID)
	if err != nil {
		return conversation.Unavailable
	}
	n, err := result.RowsAffected()
	if err != nil || n != 1 {
		return conversation.Unavailable
	}
	if tx.Commit() != nil {
		return conversation.Unavailable
	}
	return nil
}
