//go:build alignstorage

package mysql

import (
	"campusclaw/internal/rag"
	"campusclaw/internal/tutor"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
)

var alignTeacher = rag.Scope{UserID: 2, ClassID: 1, Role: "teacher"}
var alignOtherTeacher = rag.Scope{UserID: 4, ClassID: 2, Role: "teacher"}
var alignStudent = rag.Scope{UserID: 1, ClassID: 1, Role: "student"}

func currentFixture(t *testing.T) (*sql.DB, Tutor) {
	t.Helper()
	db := legacyFixture(t)
	if err := PrepareRAGSchema(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	return db, Tutor{DB: db}
}
func textSkill(name, body string) string {
	return "# teacher source\r\n名称：" + name + "\r\n适用条件：方程,问题\r\n工具：knowledge_search\r\n启用：是\r\n\r\n" + body
}

func TestAlignStoreSkillCRUD(t *testing.T) {
	db, repo := currentFixture(t)
	ctx := context.Background()
	var oldHistory string
	db.QueryRow("SELECT CAST(JSON_OBJECT('content',content,'skill',skill_name,'hash',skill_hash,'config',configuration_snapshot,'citations',citations) AS CHAR) FROM conversation_messages WHERE id=51").Scan(&oldHistory)
	source := textSkill("教师技能", "# 保留正文\r\n耐心解释：方程")
	skill, err := repo.SaveSkill(ctx, alignTeacher, 0, source)
	if err != nil {
		t.Fatal(err)
	}
	got, err := repo.Skill(ctx, alignTeacher, skill.ID)
	if err != nil || got.Source != source || got.SourceVersion != 1 || got.Revision != 1 || got.SourceOrigin != "teacher" {
		t.Fatal(got, err)
	}
	if _, err = repo.SaveSkill(ctx, alignTeacher, 0, source); !errors.Is(err, tutor.Invalid) {
		t.Fatal("duplicate", err)
	}
	other, err := repo.SaveSkill(ctx, alignOtherTeacher, 0, source)
	if err != nil {
		t.Fatal("cross-class same name", err)
	}
	if _, err = repo.Skill(ctx, alignTeacher, other.ID); !errors.Is(err, tutor.NotFound) {
		t.Fatal("foreign read", err)
	}
	if _, err = repo.SaveSkill(ctx, alignTeacher, other.ID, source); !errors.Is(err, tutor.NotFound) {
		t.Fatal("foreign write", err)
	}
	if err = repo.DeleteSkill(ctx, alignTeacher, other.ID); !errors.Is(err, tutor.NotFound) {
		t.Fatal("foreign delete", err)
	}
	for _, write := range []func() error{
		func() error { _, e := repo.SaveSkill(ctx, alignStudent, 0, source); return e },
		func() error { _, e := repo.Toggle(ctx, alignStudent, skill.ID, false); return e },
		func() error { return repo.DeleteSkill(ctx, alignStudent, skill.ID) },
	} {
		if err = write(); !errors.Is(err, tutor.Forbidden) {
			t.Fatal("student write", err)
		}
	}
	if _, err = repo.SaveSkill(ctx, alignTeacher, skill.ID, "invalid"); !errors.Is(err, tutor.Invalid) {
		t.Fatal(err)
	}
	got, err = repo.Skill(ctx, alignTeacher, skill.ID)
	if err != nil || got.Source != source || got.Revision != 1 {
		t.Fatal("rejected mutation")
	}
	got, err = repo.Toggle(ctx, alignTeacher, skill.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := tutor.ParseSkillSource(got.Source)
	if err != nil || parsed.Enabled || got.Enabled || got.Revision != 2 || got.SourceVersion != 2 || parsed.Description != skill.Description || !strings.Contains(got.Source, "启用：否\r\n") {
		t.Fatal("toggle inconsistency", got, err)
	}
	got, err = repo.SaveSkill(ctx, alignTeacher, skill.ID, textSkill("已编辑技能", "edited body"))
	if err != nil || got.Revision != 3 || got.SourceVersion != 3 || got.SourceHash != sourceHash(got.Source) {
		t.Fatal(got, err)
	}
	if _, err = repo.SaveSkill(ctx, alignTeacher, skill.ID, textSkill(tutor.SkillName, "conflict")); !errors.Is(err, tutor.Invalid) {
		t.Fatal("rename duplicate accepted", err)
	}
	got, err = repo.Skill(ctx, alignTeacher, skill.ID)
	if err != nil || got.Revision != 3 || got.Name != "已编辑技能" {
		t.Fatal("duplicate rename changed source", got, err)
	}
	a, err := repo.SaveCurrentAgent(ctx, alignTeacher, 10, tutor.Config{Name: "A tutor", Prompt: "role", Enabled: true, SkillIDs: []int64{20, skill.ID}, MaterialIDs: []string{alignMaterialA}})
	if err != nil || len(a.SkillIDs) != 2 {
		t.Fatal(a, err)
	}
	snapshot, err := repo.Snapshot(ctx, alignStudent, 10)
	if err != nil || len(snapshot.Skills) != 2 {
		t.Fatal(snapshot, err)
	}
	if err = repo.DeleteSkill(ctx, alignTeacher, 20); err != nil {
		t.Fatal(err)
	}
	if scalar(t, db, "SELECT COUNT(*) FROM assistant_skill_bindings WHERE skill_id=20") != 0 {
		t.Fatal("delete left bindings")
	}
	if len(snapshot.Skills) != 2 || snapshot.Skills[0].ID != 20 {
		t.Fatal("delete rewrote old snapshot")
	}
	var afterHistory string
	db.QueryRow("SELECT CAST(JSON_OBJECT('content',content,'skill',skill_name,'hash',skill_hash,'config',configuration_snapshot,'citations',citations) AS CHAR) FROM conversation_messages WHERE id=51").Scan(&afterHistory)
	if oldHistory != afterHistory {
		t.Fatal("history changed")
	}
	if err = PrepareRAGSchema(ctx, db); err != nil {
		t.Fatal(err)
	}
	if scalar(t, db, "SELECT COUNT(*) FROM skills WHERE id=20") != 0 {
		t.Fatal("upgrade resurrected deleted skill")
	}
	if err = repo.DeleteSkill(ctx, alignTeacher, skill.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = repo.Skill(ctx, alignTeacher, skill.ID); !errors.Is(err, tutor.NotFound) {
		t.Fatal(err)
	}
}

func TestAlignStoreBindingRollback(t *testing.T) {
	db, repo := currentFixture(t)
	ctx := context.Background()
	before, err := repo.Agent(ctx, alignTeacher, 10)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []tutor.Config{
		{Name: "changed", Prompt: "changed", Enabled: true, SkillIDs: []int64{20, 21}, MaterialIDs: []string{alignMaterialA}},
		{Name: "changed", Prompt: "changed", Enabled: true, SkillIDs: []int64{20}, MaterialIDs: []string{alignMaterialA, alignMaterialB}},
	} {
		if _, err = repo.SaveCurrentAgent(ctx, alignTeacher, 10, c); !errors.Is(err, tutor.NotFound) {
			t.Fatal("foreign binding", err)
		}
		after, e := repo.Agent(ctx, alignTeacher, 10)
		if e != nil || !reflect.DeepEqual(before, after) {
			t.Fatal("partial change", before, after, e)
		}
	}
	if _, err = repo.SaveCurrentAgent(ctx, alignOtherTeacher, 10, tutor.Config{Name: "x", Prompt: "r"}); !errors.Is(err, tutor.NotFound) {
		t.Fatal(err)
	}
	// A real MySQL trigger rejects a relation insert after the agent update and
	// deletes have run. The entire transaction must roll back, including revision.
	mustSQL(t, db, "CREATE TRIGGER reject_binding BEFORE INSERT ON assistant_material_bindings FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='controlled test failure'")
	_, err = repo.SaveCurrentAgent(ctx, alignTeacher, 10, tutor.Config{Name: "changed", Prompt: "new", Enabled: true, SkillIDs: []int64{20}, MaterialIDs: []string{alignMaterialA}})
	if !errors.Is(err, tutor.Unavailable) {
		t.Fatal("DB failure not reached", err)
	}
	after, err := repo.Agent(ctx, alignTeacher, 10)
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatal("DB failure partial commit", after, err)
	}
	// The trigger is confined to this newly created database, retained for audit.
	if scalar(t, db, "SELECT COUNT(*) FROM assistant_material_bindings WHERE agent_id=10") != 1 {
		t.Fatal("lost old relation")
	}
}

func TestAlignStoreCurrentRelationsAndSnapshot(t *testing.T) {
	db, repo := currentFixture(t)
	ctx := context.Background()
	var legacyJSON string
	db.QueryRow("SELECT CAST(skill_ids AS CHAR) FROM assistants WHERE id=10").Scan(&legacyJSON)
	a, err := repo.SaveCurrentAgent(ctx, alignTeacher, 10, tutor.Config{Name: "A tutor", Prompt: "role", Enabled: true, SkillIDs: []int64{}, MaterialIDs: []string{}})
	if err != nil {
		t.Fatal(err)
	}
	if len(a.SkillIDs) != 0 || len(a.MaterialIDs) != 0 {
		t.Fatal(a)
	}
	if err = PrepareRAGSchema(ctx, db); err != nil {
		t.Fatal(err)
	}
	a, err = repo.Agent(ctx, alignTeacher, 10)
	if err != nil || len(a.SkillIDs) != 0 || len(a.MaterialIDs) != 0 {
		t.Fatal("obsolete JSON used", a, err)
	}
	var afterJSON string
	db.QueryRow("SELECT CAST(skill_ids AS CHAR) FROM assistants WHERE id=10").Scan(&afterJSON)
	if legacyJSON != afterJSON {
		t.Fatal("legacy JSON dual written")
	}
	// Writers atomically pair a role version with bound skill and evidence set;
	// concurrent repeatable-read snapshots may see either state, never a mixture.
	second, err := repo.SaveSkill(ctx, alignTeacher, 0, textSkill("第二技能", "说明2"))
	if err != nil {
		t.Fatal(err)
	}
	config := func(i int) tutor.Config {
		ids := []int64{20}
		if i%2 != 0 {
			ids = []int64{second.ID}
		}
		return tutor.Config{Name: "A tutor", Prompt: fmt.Sprint(i % 2), Enabled: true, SkillIDs: ids, MaterialIDs: []string{alignMaterialA}}
	}
	if _, err = repo.SaveCurrentAgent(ctx, alignTeacher, 10, config(0)); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	failures := make(chan error, 2)
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 1; i <= 30; i++ {
			if _, e := repo.SaveCurrentAgent(ctx, alignTeacher, 10, config(i)); e != nil {
				failures <- e
				return
			}
		}
	}()
	for i := 0; i < 60; i++ {
		v, e := repo.Snapshot(ctx, alignStudent, 10)
		if e != nil {
			t.Fatal(e)
		}
		want := int64(20)
		if v.Agent.Prompt == "1" {
			want = second.ID
		}
		if len(v.Skills) != 1 || v.Skills[0].ID != want || len(v.Agent.SkillIDs) != 1 || v.Agent.SkillIDs[0] != want || len(v.Agent.MaterialIDs) != 1 {
			t.Fatalf("mixed snapshot %#v", v)
		}
	}
	wg.Wait()
	close(failures)
	for e := range failures {
		t.Fatal(e)
	}
	old, err := repo.Snapshot(ctx, alignStudent, 10)
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(old)
	if _, err = repo.Toggle(ctx, alignTeacher, second.ID, false); err != nil {
		t.Fatal(err)
	}
	current, err := repo.Snapshot(ctx, alignStudent, 10)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range current.Skills {
		if s.ID == second.ID {
			t.Fatal("disabled candidate returned")
		}
	}
	still, _ := json.Marshal(old)
	if string(encoded) != string(still) {
		t.Fatal("prior snapshot changed")
	}
}

func TestAlignStoreConcurrentSourceEdits(t *testing.T) {
	_, repo := currentFixture(t)
	ctx := context.Background()
	skill, err := repo.SaveSkill(ctx, alignTeacher, 0, textSkill("原子技能", "0"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = repo.SaveCurrentAgent(ctx, alignTeacher, 10, tutor.Config{Name: "A tutor", Prompt: "role", Enabled: true, SkillIDs: []int64{skill.ID}, MaterialIDs: []string{alignMaterialA}}); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	failures := make(chan error, 1)
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 1; i <= 20; i++ {
			if _, e := repo.SaveSkill(ctx, alignTeacher, skill.ID, textSkill("原子技能", fmt.Sprint(i))); e != nil {
				failures <- e
				return
			}
		}
	}()
	for i := 0; i < 40; i++ {
		snapshot, e := repo.Snapshot(ctx, alignStudent, 10)
		if e != nil {
			t.Fatal(e)
		}
		if len(snapshot.Skills) != 1 {
			t.Fatal(snapshot)
		}
		s := snapshot.Skills[0]
		if s.Description != fmt.Sprint(s.SourceVersion-1) || s.Revision != s.SourceVersion || s.SourceHash != sourceHash(s.Source) {
			t.Fatal("mixed source/metadata/revision", s)
		}
	}
	wg.Wait()
	close(failures)
	for e := range failures {
		t.Fatal(e)
	}
}
