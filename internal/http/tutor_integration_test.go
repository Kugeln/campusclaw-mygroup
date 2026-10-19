//go:build tutorintegration

package httpapi

import (
	"bytes"
	"campusclaw/internal/auth"
	"campusclaw/internal/rag"
	repository "campusclaw/internal/repository/mysql"
	"campusclaw/internal/tutor"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"
)

type tutorHealth struct{}

func (tutorHealth) Check(context.Context) error { return nil }
func TestTutorConfigurationRealMySQL(t *testing.T) {
	ctx := context.Background()
	db, e := sql.Open("mysql", os.Getenv("WEEK5_TEST_DSN"))
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	var name string
	if db.QueryRow("SELECT DATABASE()").Scan(&name) != nil || name != "week5_rag_test" {
		t.Fatal("existing isolated fixture required")
	}
	if e = repository.ValidateRAGSchema(ctx, db); e != nil {
		t.Fatal(e)
	}
	store := repository.Tutor{DB: db}
	if e = store.ProvisionSkills(ctx); e != nil {
		t.Fatal(e)
	}
	if e = store.ProvisionSkills(ctx); e != nil {
		t.Fatal("idempotent skill metadata", e)
	}
	secret := make([]byte, 32)
	rand.Read(secret)
	tokens, _ := auth.NewTokens(hex.EncodeToString(secret), time.Now)
	identity := &auth.Identity{Store: repository.LoginUsers{DB: db}, Tokens: tokens}
	login := &auth.Login{Users: repository.LoginUsers{DB: db}, Tokens: tokens}
	server := httptest.NewServer(WithTutor(tutorHealth{}, login, identity, nil, nil, nil, &TutorHandler{Store: store}))
	defer server.Close()
	request := func(method, path, token string, input any) (int, map[string]any) {
		b, _ := json.Marshal(input)
		req, _ := http.NewRequest(method, server.URL+path, bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		res, e := http.DefaultClient.Do(req)
		if e != nil {
			t.Fatal(e)
		}
		defer res.Body.Close()
		v := map[string]any{}
		json.NewDecoder(res.Body).Decode(&v)
		return res.StatusCode, v
	}
	creds := map[string]string{}
	for _, n := range []string{"teacher_a", "student_a1", "teacher_b", "student_b1"} {
		code, v := request("POST", "/api/auth/login", "", map[string]any{"username": n, "password": os.Getenv("WEEK5_ACCOUNT_PASSWORD")})
		if code != 200 {
			t.Fatal("fixture login failed", n)
		}
		creds[n], _ = v["access_token"].(string)
	}
	for _, path := range []string{"/api/agents", "/api/skills"} {
		if code, _ := request("GET", path, "", nil); code != 401 {
			t.Fatal("anonymous", path)
		}
	}
	sA := rag.Scope{UserID: 1, ClassID: 1, Role: "teacher"}
	sB := rag.Scope{UserID: 3, ClassID: 2, Role: "teacher"}
	skillsA, e := store.Skills(ctx, sA)
	if e != nil || len(skillsA) != 1 {
		t.Fatal("A skill metadata", e)
	}
	skillsB, e := store.Skills(ctx, sB)
	if e != nil || len(skillsB) != 1 || skillsA[0].ID == skillsB[0].ID {
		t.Fatal("B skill scope", e)
	}
	skillID := skillsA[0].ID
	skillPath := "/api/skills/" + strconv.FormatInt(skillID, 10)
	if code, _ := request("PUT", skillPath, creds["teacher_a"], map[string]any{"enabled": true}); code != 200 {
		t.Fatal("teacher enables")
	}
	after, _ := store.Skills(ctx, sA)
	revision := after[0].Revision
	for _, bad := range []struct {
		token string
		body  any
		code  int
	}{{creds["student_a1"], map[string]any{"enabled": false, "role": "teacher"}, 403}, {creds["teacher_b"], map[string]any{"enabled": false}, 404}, {creds["teacher_a"], map[string]any{"enabled": "true"}, 400}, {creds["teacher_a"], map[string]any{"enabled": true, "source": "instructions"}, 400}} {
		if code, _ := request("PUT", skillPath, bad.token, bad.body); code != bad.code {
			t.Fatal("skill write guard", code, bad.code)
		}
	}
	after, _ = store.Skills(ctx, sA)
	if after[0].Revision != revision || !after[0].Enabled {
		t.Fatal("rejected write changed skill")
	}
	var materialA, materialB string
	if db.QueryRow("SELECT id FROM materials WHERE class_id=1 LIMIT 1").Scan(&materialA) != nil || db.QueryRow("SELECT id FROM materials WHERE class_id=2 LIMIT 1").Scan(&materialB) != nil {
		t.Fatal("phase1 materials required")
	}
	agentName := "数学助手-" + time.Now().Format("150405.000000")
	input := map[string]any{"name": agentName, "prompt": "用耐心、简洁的风格引导学习", "enabled": true, "skill_ids": []int64{skillID}, "material_ids": []string{materialA}}
	if code, _ := request("POST", "/api/agents", creds["student_a1"], input); code != 403 {
		t.Fatal("student create")
	}
	code, v := request("POST", "/api/agents", creds["teacher_a"], input)
	if code != 201 {
		t.Fatal("teacher create", code)
	}
	id := int64(v["id"].(float64))
	path := "/api/agents/" + strconv.FormatInt(id, 10)
	if code, _ := request("POST", "/api/agents", creds["teacher_a"], input); code != 400 {
		t.Fatal("duplicate name")
	}
	for _, token := range []string{creds["teacher_b"], creds["student_b1"]} {
		if code, _ := request("GET", path, token, nil); code != 404 {
			t.Fatal("cross-class read")
		}
		if token == creds["teacher_b"] {
			if code, _ := request("PUT", path, token, input); code != 404 {
				t.Fatal("cross-class edit")
			}
		}
	}
	if code, v := request("GET", path, creds["student_a1"], nil); code != 200 || v["prompt"] != input["prompt"] {
		t.Fatal("student use")
	}
	for _, role := range []string{"teacher_b", "student_b1"} {
		if code, v := request("GET", "/api/agents?class_id=1", creds[role], nil); code != 200 {
			t.Fatal("list")
		} else {
			for _, row := range v["agents"].([]any) {
				if int64(row.(map[string]any)["id"].(float64)) == id {
					t.Fatal("list class leak")
				}
			}
		}
	}
	old, e := tutor.Capture(ctx, store, sA, id)
	if e != nil || old.Selected == nil {
		t.Fatal("atomic snapshot", e)
	}
	serialized, _ := json.Marshal(old)
	input["prompt"] = "下一轮使用的新教学原则"
	if code, _ := request("PUT", path, creds["teacher_a"], input); code != 200 {
		t.Fatal("prompt update")
	}
	if code, _ := request("PUT", skillPath, creds["teacher_a"], map[string]any{"enabled": false}); code != 200 {
		t.Fatal("disable")
	}
	fresh, e := tutor.Capture(ctx, repository.Tutor{DB: db}, sA, id)
	if e != nil || fresh.Agent.Revision != old.Agent.Revision+1 || fresh.Selected != nil || fresh.Agent.Prompt != input["prompt"] {
		t.Fatal("next snapshot", e)
	}
	unchanged, _ := json.Marshal(old)
	if string(serialized) != string(unchanged) {
		t.Fatal("inflight snapshot changed")
	}
	// New repository connection verifies actual durable configuration, not cache.
	reopened, e := sql.Open("mysql", os.Getenv("WEEK5_TEST_DSN"))
	if e != nil {
		t.Fatal(e)
	}
	defer reopened.Close()
	persisted, e := (repository.Tutor{DB: reopened}).Agent(ctx, sA, id)
	if e != nil || persisted.Prompt != fresh.Agent.Prompt || persisted.Revision != fresh.Agent.Revision {
		t.Fatal("durable config", e)
	}
	input["material_ids"] = []string{materialB}
	if code, _ := request("PUT", path, creds["teacher_a"], input); code != 404 {
		t.Fatal("foreign material binding")
	}
	input["material_ids"] = []string{materialA}
	input["skill_ids"] = []int64{skillsB[0].ID}
	if code, _ := request("PUT", path, creds["teacher_a"], input); code != 404 {
		t.Fatal("foreign skill binding")
	}
	input["skill_ids"] = []int64{skillID}
	input["class_id"] = 2
	if code, _ := request("PUT", path, creds["teacher_a"], input); code != 400 {
		t.Fatal("forged scope")
	}
	delete(input, "class_id")
	if code, _ := request("PUT", path, creds["student_a1"], input); code != 403 {
		t.Fatal("student prompt edit")
	}
	unchangedAgent, _ := store.Agent(ctx, sA, id)
	if unchangedAgent.Revision != fresh.Agent.Revision {
		t.Fatal("rejected writes changed revision")
	}
	input["enabled"] = false
	if code, _ := request("PUT", path, creds["teacher_a"], input); code != 200 {
		t.Fatal("disable assistant")
	}
	if code, _ := request("GET", path, creds["student_a1"], nil); code != 404 {
		t.Fatal("disabled student read")
	}
	if code, _ := request("GET", path, creds["teacher_a"], nil); code != 200 {
		t.Fatal("teacher disabled read")
	}
	t.Log("PASS existing isolated MySQL: Bearer, teacher/student, A/B ownership, binding validation, revisions, durable prompt/skill state, immutable snapshots; no migrations/conversations/SSE")
}
