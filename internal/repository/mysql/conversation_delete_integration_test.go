//go:build alignstorage

package mysql

import (
	"campusclaw/internal/conversation"
	"campusclaw/internal/rag"
	"context"
	"errors"
	"os"
	"strings"
	"testing"
)

func TestAlignDeleteScopedTransaction(t *testing.T) {
	db, _ := currentFixture(t)
	repo := Conversations{DB: db}
	ctx := context.Background()
	mustSQL(t, db, "INSERT INTO conversations(id,user_id,class_id,agent_id,title,created_at,updated_at) VALUES(41,1,1,10,'other','2026-10-01','2026-10-01')")
	mustSQL(t, db, "INSERT INTO conversation_messages(id,conversation_id,class_id,turn_id,sequence_number,role,content,status,created_at) VALUES(52,41,1,?,1,'user','other content','completed','2026-10-01')", alignMaterialA)
	for _, s := range []rag.Scope{{UserID: 2, ClassID: 1}, {UserID: 1, ClassID: 2}, {UserID: 3, ClassID: 2}} {
		if err := repo.Delete(ctx, s, 40); !errors.Is(err, conversation.NotFound) {
			t.Fatal("foreign delete", err)
		}
	}
	if err := repo.Delete(ctx, alignStudent, 999); !errors.Is(err, conversation.NotFound) {
		t.Fatal("missing delete", err)
	}
	if scalar(t, db, "SELECT COUNT(*) FROM conversation_messages WHERE conversation_id=40") != 2 {
		t.Fatal("denial changed messages")
	}
	mustSQL(t, db, "UPDATE conversations SET active_turn_id='active' WHERE id=40")
	if err := repo.Delete(ctx, alignStudent, 40); !errors.Is(err, conversation.Busy) {
		t.Fatal("active accepted", err)
	}
	mustSQL(t, db, "UPDATE conversations SET active_turn_id=NULL WHERE id=40")
	before := legacyFingerprint(t, db)
	if err := repo.deleteConversation(ctx, alignStudent, 40, func() error { return errors.New("controlled interruption") }); !errors.Is(err, conversation.Unavailable) {
		t.Fatal(err)
	}
	if before != legacyFingerprint(t, db) {
		t.Fatal("mid-delete failure committed")
	}
	if err := repo.Delete(ctx, alignStudent, 40); err != nil {
		t.Fatal(err)
	}
	if scalar(t, db, "SELECT COUNT(*) FROM conversations WHERE id=40") != 0 || scalar(t, db, "SELECT COUNT(*) FROM conversation_messages WHERE conversation_id=40") != 0 {
		t.Fatal("orphan left")
	}
	if scalar(t, db, "SELECT COUNT(*) FROM conversation_messages WHERE id=52 AND content='other content'") != 1 || scalar(t, db, "SELECT COUNT(*) FROM conversations WHERE id=41") != 1 {
		t.Fatal("other conversation changed")
	}
}

func TestAlignDeleteRealDatabaseFailure(t *testing.T) {
	db, _ := currentFixture(t)
	repo := Conversations{DB: db}
	before := legacyFingerprint(t, db)
	mustSQL(t, db, "CREATE TRIGGER reject_conversation_delete BEFORE DELETE ON conversations FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='controlled delete failure'")
	if err := repo.Delete(context.Background(), alignStudent, 40); !errors.Is(err, conversation.Unavailable) {
		t.Fatal(err)
	}
	if legacyFingerprint(t, db) != before {
		t.Fatal("messages not rolled back after parent delete rejection")
	}
}

func TestAlignDeleteNoPublicRoute(t *testing.T) {
	content, err := os.ReadFile("../../http/handler.go")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(content), "DELETE /api/conversations") {
		t.Fatal("public deletion route added")
	}
	server, err := os.ReadFile("../../../cmd/server/main.go")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(server), "PrepareRAGSchema") || strings.Contains(string(server), "PrepareSchema") {
		t.Fatal("HTTP startup runs DDL")
	}
}
