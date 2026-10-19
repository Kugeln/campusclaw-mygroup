//go:build alignstorage

package mysql

import (
	"campusclaw/internal/auth"
	httpapi "campusclaw/internal/http"
	"campusclaw/internal/tutor"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
)

type phase2Health struct{}

func (phase2Health) Check(context.Context) error { return nil }

type phase2HTTP struct {
	db       *sql.DB
	repo     Tutor
	handler  http.Handler
	tokens   map[string]string
	identity *auth.Identity
}

func newPhase2HTTP(t *testing.T) *phase2HTTP {
	t.Helper()
	db, repo := currentFixture(t)
	mustSQL(t, db, "INSERT INTO users(id,username,password_hash,role,class_id,session_version) VALUES(5,'sa2','test','student',1,0)")
	tokens, err := auth.NewTokens("align-phase2-test-only-signing-secret-32-bytes", nil)
	if err != nil {
		t.Fatal(err)
	}
	f := &phase2HTTP{db: db, repo: repo, tokens: map[string]string{}}
	for name, account := range map[string][2]int64{"sa": {1, 9}, "ta": {2, 4}, "sb": {3, 8}, "tb": {4, 5}, "sa2": {5, 0}} {
		token, e := tokens.Issue(account[0], account[1])
		if e != nil {
			t.Fatal(e)
		}
		f.tokens[name] = token
	}
	identity := &auth.Identity{Store: LoginUsers{DB: db}, Tokens: tokens}
	f.identity = identity
	f.handler = httpapi.WithConversations(phase2Health{}, nil, identity, nil, nil, nil, &httpapi.TutorHandler{Store: repo}, &httpapi.ConversationHandler{Store: Conversations{DB: db}, Tutor: repo})
	return f
}
func (f *phase2HTTP) raw(method, path, who, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	if who != "" {
		r.Header.Set("Authorization", "Bearer "+f.tokens[who])
	}
	r.Header.Set("X-Role", "teacher")
	r.Header.Set("X-Class-Id", "2")
	w := httptest.NewRecorder()
	f.handler.ServeHTTP(w, r)
	return w
}
func (f *phase2HTTP) request(method, path, who string, data any) *httptest.ResponseRecorder {
	b, _ := json.Marshal(data)
	return f.raw(method, path, who, string(b))
}
func decodePhase2Skill(t *testing.T, w *httptest.ResponseRecorder, status int) tutor.Skill {
	t.Helper()
	var v tutor.Skill
	if w.Code != status || json.Unmarshal(w.Body.Bytes(), &v) != nil {
		t.Fatalf("skill HTTP expected %d got %d: %s", status, w.Code, w.Body.String())
	}
	return v
}
func phase2Fingerprint(t *testing.T, db *sql.DB) string {
	t.Helper()
	h := sha256.New()
	for _, table := range []string{"skills", "assistants", "assistant_skill_bindings", "assistant_material_bindings", "conversation_messages", "conversations", "users"} {
		rows, err := db.Query("SELECT * FROM " + table)
		if err != nil {
			t.Fatal(err)
		}
		cols, _ := rows.Columns()
		encoded := []string{}
		for rows.Next() {
			v := make([]sql.NullString, len(cols))
			args := make([]any, len(cols))
			for i := range v {
				args[i] = &v[i]
			}
			if rows.Scan(args...) != nil {
				t.Fatal("fingerprint scan")
			}
			b, _ := json.Marshal(v)
			encoded = append(encoded, string(b))
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			t.Fatal(err)
		}
		sort.Strings(encoded)
		fmt.Fprint(h, table, encoded)
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

func TestAlignPhase2APISkillCRUDAndGuards(t *testing.T) {
	f := newPhase2HTTP(t)
	ctx := context.Background()
	source := textSkill("教师自定义", "# 正文原样\r\n说明：A:B")
	for _, tc := range []struct {
		method, path, who string
		data              any
		code              int
	}{
		{"GET", "/api/skills", "", nil, 401}, {"GET", "/api/skills/20", "", nil, 401},
		{"POST", "/api/skills", "sa", map[string]any{"source": source}, 403},
		{"PUT", "/api/skills/20", "sa", map[string]any{"enabled": false}, 403},
		{"DELETE", "/api/skills/20", "sa", nil, 403},
		{"GET", "/api/skills/21", "ta", nil, 404}, {"PUT", "/api/skills/21", "ta", map[string]any{"source": source}, 404},
		{"DELETE", "/api/skills/21", "ta", nil, 404},
	} {
		before := phase2Fingerprint(t, f.db)
		w := f.request(tc.method, tc.path, tc.who, tc.data)
		if w.Code != tc.code {
			t.Fatalf("%s %s: %d %s", tc.method, tc.path, w.Code, w.Body.String())
		}
		if phase2Fingerprint(t, f.db) != before {
			t.Fatal("rejected request changed DB")
		}
	}
	foreign := f.request("GET", "/api/skills/21", "ta", nil)
	missing := f.request("GET", "/api/skills/99999", "ta", nil)
	if foreign.Body.String() != missing.Body.String() {
		t.Fatal("foreign existence leaked")
	}
	v := decodePhase2Skill(t, f.request("POST", "/api/skills", "ta", map[string]any{"source": source}), 201)
	if v.Source != source || v.SourceVersion != 1 || v.Name != "教师自定义" {
		t.Fatal("source round trip")
	}
	path := fmt.Sprintf("/api/skills/%d", v.ID)
	got := decodePhase2Skill(t, f.request("GET", path, "sa", nil), 200)
	if got.Source != source {
		t.Fatal("student same-class read")
	}
	before := phase2Fingerprint(t, f.db)
	if w := f.request("POST", "/api/skills", "ta", map[string]any{"source": source}); w.Code != 400 || phase2Fingerprint(t, f.db) != before {
		t.Fatal("duplicate not atomic")
	}
	decodePhase2Skill(t, f.request("POST", "/api/skills", "tb", map[string]any{"source": source}), 201)
	for name, body := range map[string]string{
		"empty": "{}", "mixed": `{"source":"x","enabled":true}`, "null": `{"source":null}`, "wrong_type": `{"source":true}`, "unknown": `{"source":"x","class_id":2}`, "duplicate_json": `{"source":"x","source":"y"}`, "duplicate_metadata": `{"source":"名称：a\n名称：b\n适用条件：题\n启用：是\n\n说明"}`, "invalid_utf8": "{\"source\":\"" + string([]byte{0xff}) + "\"}", "trailing": "{} {}", "not_object": "[]", "malformed": "{",
	} {
		t.Run(name, func(t *testing.T) {
			before := phase2Fingerprint(t, f.db)
			w := f.raw("PUT", path, "ta", body)
			if w.Code != 400 || phase2Fingerprint(t, f.db) != before {
				t.Fatalf("invalid request mutated DB: %d", w.Code)
			}
		})
	}
	updated := decodePhase2Skill(t, f.request("PUT", path, "ta", map[string]any{"source": textSkill("已修改", "新的说明")}), 200)
	if updated.Revision != 2 || updated.SourceVersion != 2 {
		t.Fatal("update versions")
	}
	off := decodePhase2Skill(t, f.request("PUT", path, "ta", map[string]any{"enabled": false}), 200)
	parsed, err := tutor.ParseSkillSource(off.Source)
	if err != nil || off.Enabled || parsed.Enabled || off.Revision != 3 || parsed.Description != "新的说明" {
		t.Fatal("legacy toggle divergence", err)
	}
	if _, err = f.repo.SaveCurrentAgent(ctx, alignTeacher, 10, tutor.Config{Name: "A tutor", Prompt: "role", Enabled: true, SkillIDs: []int64{20, v.ID}, MaterialIDs: []string{alignMaterialA}}); err != nil {
		t.Fatal(err)
	}
	var oldMessage string
	f.db.QueryRow("SELECT CAST(configuration_snapshot AS CHAR) FROM conversation_messages WHERE id=51").Scan(&oldMessage)
	if w := f.request("DELETE", path, "ta", nil); w.Code != 204 || w.Body.Len() != 0 {
		t.Fatal("delete", w.Code)
	}
	if scalar(t, f.db, "SELECT COUNT(*) FROM assistant_skill_bindings WHERE skill_id=?", v.ID) != 0 {
		t.Fatal("dangling binding")
	}
	if w := f.request("GET", path, "ta", nil); w.Code != 404 {
		t.Fatal("deleted resource readable")
	}
	var afterMessage string
	f.db.QueryRow("SELECT CAST(configuration_snapshot AS CHAR) FROM conversation_messages WHERE id=51").Scan(&afterMessage)
	if oldMessage != afterMessage {
		t.Fatal("delete changed history")
	}
}

func TestAlignPhase2APIDatabaseFailureAtomic(t *testing.T) {
	f := newPhase2HTTP(t)
	before := phase2Fingerprint(t, f.db)
	mustSQL(t, f.db, "CREATE TRIGGER reject_source_update BEFORE UPDATE ON skills FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='controlled source rejection'")
	w := f.request("PUT", "/api/skills/20", "ta", map[string]any{"source": textSkill("修改尝试", "说明")})
	if w.Code != 503 || phase2Fingerprint(t, f.db) != before {
		t.Fatal("source failure partial update", w.Code)
	}
	w = f.request("PUT", "/api/skills/20", "ta", map[string]any{"enabled": false})
	if w.Code != 503 || phase2Fingerprint(t, f.db) != before {
		t.Fatal("toggle failure partial update", w.Code)
	}
}
