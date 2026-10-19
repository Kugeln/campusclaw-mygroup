//go:build alignstorage

package mysql

import (
	"campusclaw/internal/rag"
	"campusclaw/internal/tutor"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

type phase2Retriever struct {
	calls     int
	materials []string
	scope     rag.Scope
	query     string
	data      []rag.Evidence
	err       error
}

func (r *phase2Retriever) Search(_ context.Context, s rag.Scope, q, _ string, materials []string, limit int) ([]rag.Evidence, error) {
	r.calls++
	r.scope = s
	r.query = q
	r.materials = append([]string{}, materials...)
	if limit != 4 {
		panic("expected top4")
	}
	return r.data, r.err
}

func TestAlignPhase2RoutingCurrentBindings(t *testing.T) {
	f := newPhase2HTTP(t)
	ctx := context.Background()
	makeSkill := func(name, trigger, body string) tutor.Skill {
		source := "名称：" + name + "\n适用条件：" + trigger + "\n启用：是\n\n" + body
		v, e := f.repo.SaveSkill(ctx, alignTeacher, 0, source)
		if e != nil {
			t.Fatal(e)
		}
		return v
	}
	first := makeSkill("动态解题引导", "方程,数学", "first instruction")
	second := makeSkill("普通讲解", "方程", "second instruction")
	_ = makeSkill("未绑定高分", "方程,方程,方程,方程", "never inject")
	cfg := tutor.Config{Name: "A tutor", Prompt: "role", Enabled: true, SkillIDs: []int64{second.ID, first.ID}, MaterialIDs: []string{alignMaterialA}}
	if _, err := f.repo.SaveAgent(ctx, alignTeacher, 10, cfg); err != nil {
		t.Fatal(err)
	}
	r := &phase2Retriever{data: []rag.Evidence{{ChunkID: alignMaterialA, MaterialID: alignMaterialA, Title: "数学", Index: 1, Text: "资料"}}}
	prepare := func() tutor.Prepared {
		p, e := tutor.PrepareTurn(ctx, f.repo, r, alignStudent, 10, "数学方程", "hybrid")
		if e != nil {
			t.Fatal(e)
		}
		return p
	}
	old := prepare()
	if old.Actual == nil || old.Actual.ID != first.ID || !strings.Contains(old.System, "不得直接给出最终答案") || strings.Contains(old.System, "never inject") || strings.Contains(old.System, "second instruction") {
		t.Fatal("wrong route", old)
	}
	frozen, _ := json.Marshal(old)
	updated := strings.Replace(first.Source, "first instruction", "edited instruction", 1)
	if w := f.request("PUT", fmt.Sprintf("/api/skills/%d", first.ID), "ta", map[string]any{"source": updated}); w.Code != 200 {
		t.Fatal(w.Code)
	}
	next := prepare()
	if !strings.Contains(next.System, "edited instruction") || next.Actual.Hash == old.Actual.Hash {
		t.Fatal("edit not visible next turn")
	}
	if _, err := f.repo.Toggle(ctx, alignTeacher, first.ID, false); err != nil {
		t.Fatal(err)
	}
	next = prepare()
	if next.Actual == nil || next.Actual.ID != second.ID || strings.Contains(next.System, "不得直接给出最终答案") {
		t.Fatal("disabled route")
	}
	if _, err := f.repo.Toggle(ctx, alignTeacher, first.ID, true); err != nil {
		t.Fatal(err)
	}
	cfg.SkillIDs = []int64{second.ID}
	if _, err := f.repo.SaveAgent(ctx, alignTeacher, 10, cfg); err != nil {
		t.Fatal(err)
	}
	if next = prepare(); next.Actual == nil || next.Actual.ID != second.ID {
		t.Fatal("unbind not visible")
	}
	still, _ := json.Marshal(old)
	if string(frozen) != string(still) {
		t.Fatal("in-flight snapshot changed")
	}
	if r.scope.ClassID != 1 || r.scope.UserID != 1 {
		t.Fatal("untrusted scope")
	}
}
