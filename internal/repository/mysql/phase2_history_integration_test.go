//go:build alignstorage

package mysql

import (
	"campusclaw/internal/conversation"
	"campusclaw/internal/rag"
	"campusclaw/internal/tutor"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestAlignPhase2HistoryAcceptedTurnWindow(t *testing.T) {
	db, repo := currentFixture(t)
	ctx := context.Background()
	store := Conversations{DB: db}
	for seq := 3; seq <= 10; seq++ {
		role := "user"
		if seq%2 == 0 {
			role = "assistant"
		}
		content := fmt.Sprintf("消息%d", seq)
		result := "ok"
		if seq == 10 {
			content = strings.Repeat("𠮷", 801)
			result = "no_evidence"
		}
		mustSQL(t, db, "INSERT INTO conversation_messages(conversation_id,class_id,turn_id,sequence_number,role,content,status,result_status,created_at) VALUES(40,1,?,?,?,?,'completed',?,'2026-10-01')", fmt.Sprintf("00000000-0000-4000-8000-%012d", seq), seq, role, content, result)
	}
	for i, state := range []string{"completed", "failed", "interrupted", "generating"} {
		role, content := "assistant", "draft "+state
		if i == 0 {
			role = "user"
			content = " \n\t"
		}
		mustSQL(t, db, "INSERT INTO conversation_messages(conversation_id,class_id,turn_id,sequence_number,role,content,status,created_at) VALUES(40,1,?,?,?,?,?,'2026-10-01')", alignMaterialA, 11+i, role, content, state)
	}
	c, _, err := store.Get(ctx, alignStudent, 40)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := repo.Snapshot(ctx, alignStudent, 10)
	if err != nil {
		t.Fatal(err)
	}
	// Exercise the accepted-user window independently of HTTP transport.
	turn, err := store.Start(ctx, alignStudent, c, "当前问题", tutor.Prepared{Snapshot: snapshot})
	if err != nil {
		t.Fatal(err)
	}
	history, err := store.History(ctx, alignStudent, 40)
	if err != nil || len(history) != 6 || history[5].Content != "当前问题" || !history[5].Current {
		t.Fatal("accepted window", history, err)
	}
	current := 0
	noEvidence := false
	for _, m := range history {
		if m.Current {
			current++
		}
		if utf8.RuneCountInString(m.Content) > 500 || strings.Contains(m.Content, "draft") {
			t.Fatal("invalid history fed")
		}
		if m.Content == strings.Repeat("𠮷", 500) {
			noEvidence = true
		}
	}
	if current != 1 || !noEvidence {
		t.Fatal("current or completed no-evidence missing")
	}
	modelMessages, err := conversation.ModelMessages(tutor.Prepared{System: "role"}, history, "当前问题")
	if err != nil || len(modelMessages) != 7 {
		t.Fatal("model window", err)
	}
	var payload struct {
		Question string `json:"question"`
	}
	if json.Unmarshal([]byte(modelMessages[6].Content), &payload) != nil || payload.Question != "当前问题" {
		t.Fatal("current question payload")
	}
	if scalar(t, db, "SELECT CHAR_LENGTH(content) FROM conversation_messages WHERE conversation_id=40 AND sequence_number=10") != 801 {
		t.Fatal("stored original truncated")
	}
	for _, s := range []rag.Scope{{UserID: 5, ClassID: 1}, {UserID: 1, ClassID: 2}, {UserID: 3, ClassID: 2}} {
		if _, err = store.History(ctx, s, 40); !errors.Is(err, conversation.NotFound) {
			t.Fatal("history scope", err)
		}
	}
	_, all, err := store.Get(ctx, alignStudent, 40)
	if err != nil {
		t.Fatal(err)
	}
	draftSeen := false
	for _, m := range all {
		if m.State == "failed" && strings.Contains(m.Content, "draft") {
			draftSeen = true
		}
	}
	if !draftSeen {
		t.Fatal("failure prefix lost from authorized history")
	}
	if err = store.Finish(ctx, alignStudent, 40, turn, "interrupted", nil, "controlled_phase2"); err != nil {
		t.Fatal(err)
	}
}

func TestAlignPhase2HistoryUnicodeTitleAndOrdering(t *testing.T) {
	db, repo := currentFixture(t)
	ctx := context.Background()
	store := Conversations{DB: db}
	var latest int64
	acceptAndComplete := func(c conversation.Conversation, q string, snapshot tutor.Snapshot) {
		t.Helper()
		turn, err := store.Start(ctx, alignStudent, c, q, tutor.Prepared{Snapshot: snapshot})
		if err != nil {
			t.Fatal(err)
		}
		if err = store.Append(ctx, alignStudent, c.ID, turn, "受控回答"); err != nil {
			t.Fatal(err)
		}
		if err = store.Finish(ctx, alignStudent, c.ID, turn, "completed", nil, ""); err != nil {
			t.Fatal(err)
		}
	}
	for _, n := range []int{24, 25} {
		c, err := store.Create(ctx, alignStudent, 10)
		if err != nil {
			t.Fatal(err)
		}
		snapshot, err := repo.Snapshot(ctx, alignStudent, 10)
		if err != nil {
			t.Fatal(err)
		}
		q := " \t" + strings.Repeat("𠮷", n) + "\n "
		acceptAndComplete(c, q, snapshot)
		first, _, err := store.Get(ctx, alignStudent, c.ID)
		if err != nil || first.Title != strings.Repeat("𠮷", 24) {
			t.Fatal("first title", first.Title, err)
		}
		acceptAndComplete(first, "后续问题", snapshot)
		second, _, err := store.Get(ctx, alignStudent, c.ID)
		if err != nil || second.Title != first.Title || second.UpdatedAt == first.UpdatedAt || second.AgentID != 10 {
			t.Fatal("title/updated/agent changed", err)
		}
		var original string
		db.QueryRow("SELECT content FROM conversation_messages WHERE conversation_id=? AND sequence_number=1", c.ID).Scan(&original)
		if original != q {
			t.Fatal("original question rewritten")
		}
		latest = c.ID
	}
	list, err := store.List(ctx, alignStudent)
	if err != nil || list[0].ID != latest {
		t.Fatal("recent ordering", err)
	}
}
