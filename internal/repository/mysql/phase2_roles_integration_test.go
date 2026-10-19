//go:build alignstorage

package mysql

import (
	"campusclaw/internal/conversation"
	httpapi "campusclaw/internal/http"
	"campusclaw/internal/tutor"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"testing"

	"campusclaw/internal/rag"
)

type phase2Model struct {
	opened   int
	prepared tutor.Prepared
	history  []conversation.ModelMessage
	question string
}

func (m *phase2Model) Open(_ context.Context, p tutor.Prepared, h []conversation.ModelMessage, q string) (conversation.Stream, error) {
	m.opened++
	m.prepared = p
	m.history = append([]conversation.ModelMessage{}, h...)
	m.question = q
	return &phase2Stream{}, nil
}

type phase2Stream struct{ sent bool }

func (s *phase2Stream) Next() (string, error) {
	if s.sent {
		return "", io.EOF
	}
	s.sent = true
	return "按角色解释[1]", nil
}
func (s *phase2Stream) Close() error { return nil }

func agentInput(name, prompt string, enabled bool, skills []int64, materials []string) map[string]any {
	return map[string]any{"name": name, "prompt": prompt, "enabled": enabled, "skill_ids": skills, "material_ids": materials}
}

func TestAlignPhase2RolesDefaultAndDisabledContinuation(t *testing.T) {
	f := newPhase2HTTP(t)
	body := agentInput("空角色", " \t\n", true, []int64{20}, []string{alignMaterialA})
	w := f.request("POST", "/api/agents", "ta", body)
	var a tutor.Agent
	if w.Code != 201 || json.Unmarshal(w.Body.Bytes(), &a) != nil || a.Prompt != "" {
		t.Fatal("blank role", w.Code, w.Body.String())
	}
	create := f.request("POST", "/api/conversations", "sa", map[string]any{"agent_id": a.ID})
	var c conversation.Conversation
	if create.Code != 201 || json.Unmarshal(create.Body.Bytes(), &c) != nil {
		t.Fatal("create", create.Code)
	}
	body["enabled"] = false
	if w = f.request("PUT", fmt.Sprintf("/api/agents/%d", a.ID), "ta", body); w.Code != 200 {
		t.Fatal(w.Code)
	}
	if w = f.request("POST", "/api/conversations", "sa", map[string]any{"agent_id": a.ID}); w.Code != 404 {
		t.Fatal("disabled new selection", w.Code)
	}
	if w = f.request("GET", fmt.Sprintf("/api/agents/%d", a.ID), "sa", nil); w.Code != 404 {
		t.Fatal("student direct disabled", w.Code)
	}
	if w = f.request("GET", fmt.Sprintf("/api/agents/%d", a.ID), "ta", nil); w.Code != 200 {
		t.Fatal("teacher disabled management", w.Code)
	}
	var listing struct {
		Agents []tutor.Agent `json:"agents"`
	}
	w = f.request("GET", "/api/agents", "sa", nil)
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &listing) != nil {
		t.Fatal("student list")
	}
	for _, item := range listing.Agents {
		if item.ID == a.ID || !item.Enabled {
			t.Fatal("disabled selection exposed")
		}
	}
	r := &phase2Retriever{data: []rag.Evidence{{ChunkID: alignMaterialA, MaterialID: alignMaterialA, Title: "数学", Index: 1, Text: "资料"}}}
	model := &phase2Model{}
	f.handler = httpapi.WithConversations(phase2Health{}, nil, f.identity, nil, nil, nil, &httpapi.TutorHandler{Store: f.repo}, &httpapi.ConversationHandler{Store: Conversations{DB: f.db}, Tutor: f.repo, Retrieval: r, Model: model})
	path := fmt.Sprintf("/api/conversations/%d/messages", c.ID)
	w = f.request("POST", path, "sa", map[string]any{"content": "方程问题"})
	if w.Code != 200 || !strings.Contains(w.Body.String(), "event: done") || model.opened != 1 || !strings.Contains(model.prepared.System, tutor.DefaultRole) || model.prepared.Snapshot.Agent.Enabled || model.prepared.Snapshot.Agent.ID != a.ID {
		t.Fatal("continuation/fallback", w.Code, w.Body.String())
	}
	for _, who := range []string{"sa2", "sb"} {
		before := phase2Fingerprint(t, f.db)
		w = f.request("POST", path, who, map[string]any{"content": "方程问题"})
		if w.Code != 404 || phase2Fingerprint(t, f.db) != before {
			t.Fatal("owner/class bypass", who, w.Code)
		}
	}
	before := phase2Fingerprint(t, f.db)
	w = f.request("POST", path, "sa", map[string]any{"content": "方程问题", "agent_id": 10})
	if w.Code != 400 || phase2Fingerprint(t, f.db) != before {
		t.Fatal("agent override allowed")
	}
	if scalar(t, f.db, "SELECT agent_id FROM conversations WHERE id=?", c.ID) != int(a.ID) {
		t.Fatal("conversation re-bound")
	}
	mustSQL(t, f.db, "UPDATE assistants SET enabled=0 WHERE class_id=1")
	if w = f.request("POST", "/api/conversations", "sa", map[string]any{"agent_id": a.ID}); w.Code != 400 {
		t.Fatal("no enabled agent", w.Code)
	}
	mustSQL(t, f.db, "UPDATE users SET class_id=2 WHERE id=1")
	if w = f.request("POST", path, "sa", map[string]any{"content": "方程问题"}); w.Code != 404 {
		t.Fatal("transfer bypass", w.Code)
	}
}

func TestAlignPhase2RolesConfigurationBoundsAndGuards(t *testing.T) {
	f := newPhase2HTTP(t)
	for _, tc := range []struct {
		name, prompt string
		status       int
	}{{strings.Repeat("字", 64), strings.Repeat("𠮷", 2000), 201}, {strings.Repeat("字", 65), "role", 400}, {"长提示词", strings.Repeat("𠮷", 2001), 400}} {
		before := phase2Fingerprint(t, f.db)
		w := f.request("POST", "/api/agents", "ta", agentInput(tc.name, tc.prompt, true, []int64{}, []string{}))
		if w.Code != tc.status {
			t.Fatal("bounds", w.Code)
		}
		if tc.status == 400 && phase2Fingerprint(t, f.db) != before {
			t.Fatal("bounds partial mutation")
		}
	}
	for _, body := range []map[string]any{agentInput("missing", "", true, nil, []string{}), agentInput("missing", "", true, []int64{}, nil)} {
		if w := f.request("POST", "/api/agents", "ta", body); w.Code != 400 {
			t.Fatal("null binding allowed", w.Code)
		}
	}
	for _, tc := range []struct {
		body map[string]any
		code int
	}{
		{agentInput("A tutor", "", true, []int64{}, []string{}), 400},
		{agentInput("foreign skill", "", true, []int64{20, 21}, []string{alignMaterialA}), 404},
		{agentInput("foreign material", "", true, []int64{20}, []string{alignMaterialA, alignMaterialB}), 404},
		{agentInput("duplicate skill", "", true, []int64{20, 20}, []string{}), 400},
	} {
		before := phase2Fingerprint(t, f.db)
		w := f.request("POST", "/api/agents", "ta", tc.body)
		if w.Code != tc.code || phase2Fingerprint(t, f.db) != before {
			t.Fatal("configuration rejection not atomic", w.Code, tc.code)
		}
	}
	missing := agentInput("missing arrays", "", true, []int64{}, []string{})
	delete(missing, "skill_ids")
	if w := f.request("POST", "/api/agents", "ta", missing); w.Code != 400 {
		t.Fatal("missing array accepted")
	}
	for _, who := range []string{"", "sa"} {
		want := 401
		if who == "sa" {
			want = 403
		}
		before := phase2Fingerprint(t, f.db)
		if w := f.request("POST", "/api/agents", who, agentInput("forged", "", true, []int64{}, []string{})); w.Code != want || phase2Fingerprint(t, f.db) != before {
			t.Fatal("write guard", w.Code)
		}
	}
	before := phase2Fingerprint(t, f.db)
	if w := f.request("PUT", "/api/agents/10", "tb", agentInput("foreign", "", true, []int64{}, []string{})); w.Code != 404 || phase2Fingerprint(t, f.db) != before {
		t.Fatal("foreign config mutation")
	}
}
