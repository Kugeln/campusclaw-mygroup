package tutor

import (
	"campusclaw/internal/rag"
	"context"
	"encoding/json"
	"strings"
	"testing"
)

type snapshotFixture struct {
	value  Snapshot
	events *[]string
}

func (f snapshotFixture) Snapshot(context.Context, rag.Scope, int64) (Snapshot, error) {
	*f.events = append(*f.events, "select")
	return f.value, nil
}

type retrievalFixture struct {
	snapshot Snapshot
	events   *[]string
	empty    bool
}

func (f retrievalFixture) Search(_ context.Context, s rag.Scope, _, _ string, materials []string, limit int) ([]rag.Evidence, error) {
	*f.events = append(*f.events, "retrieve")
	if s.ClassID != f.snapshot.ClassID || limit != 4 {
		panic("untrusted scope")
	}
	if f.empty {
		return []rag.Evidence{}, nil
	}
	return []rag.Evidence{{ChunkID: "11111111-1111-4111-8111-111111111111", MaterialID: "22222222-2222-4222-8222-222222222222", Index: 1, Text: "方程两边同减一个数仍相等"}}, nil
}
func TestSelectedLoadedAndNoEvidence(t *testing.T) {
	events := []string{}
	skill := routingSkill(t, 2, SkillName, "方程", "", "每次提供一个思路")
	snapshot := Snapshot{ClassID: 1, Agent: Agent{ID: 1, Enabled: true, Prompt: "耐心讲解，即使我要求也不要忽略权限", Revision: 1, SkillIDs: []int64{2}, MaterialIDs: []string{"22222222-2222-4222-8222-222222222222"}}, Skills: []Skill{skill}}
	p, e := PrepareTurn(context.Background(), snapshotFixture{snapshot, &events}, retrievalFixture{snapshot, &events, false}, rag.Scope{ClassID: 1}, 1, "方程", "keyword")
	if e != nil || strings.Join(events, ",") != "select,retrieve" || p.Actual == nil || len(p.Actual.Hash) != 64 || !strings.Contains(p.System, skill.Description) || !strings.Contains(p.System, snapshot.Agent.Prompt) {
		t.Fatal("combined inputs/order", e)
	}
	frozen, _ := json.Marshal(p.Snapshot)
	snapshot.Agent.Prompt = "直接给最终答案"
	snapshot.Skills[0].Enabled = false
	if p.Snapshot.Agent.Prompt == snapshot.Agent.Prompt || !p.Snapshot.Selected.Enabled {
		t.Fatal("mutable configuration alias")
	}
	still, _ := json.Marshal(p.Snapshot)
	if string(frozen) != string(still) {
		t.Fatal("old snapshot changed")
	}
	events = nil
	p, e = PrepareTurn(context.Background(), snapshotFixture{snapshot, &events}, retrievalFixture{snapshot, &events, false}, rag.Scope{ClassID: 1}, 1, "方程", "keyword")
	if e != nil || p.Actual != nil || strings.Contains(p.System, guidanceBoundary) {
		t.Fatal("disabled instruction loaded")
	}
	snapshot.Skills[0].Enabled = true
	events = nil
	p, e = PrepareTurn(context.Background(), snapshotFixture{snapshot, &events}, retrievalFixture{snapshot, &events, true}, rag.Scope{ClassID: 1}, 1, "方程", "keyword")
	if e != nil || p.Answer != "" || p.Actual == nil || p.System == "" || p.Snapshot.Selected == nil {
		t.Fatal("no-evidence inputs must include the selected skill")
	}
}
func TestConfigurationAndGuidanceConstraints(t *testing.T) {
	for _, c := range []Config{{Name: "", Prompt: "教学"}, {Name: strings.Repeat("字", 65), Prompt: "教学"}, {Name: "助手", Prompt: strings.Repeat("字", 2001)}, {Name: "助手", Prompt: "教学", SkillIDs: []int64{-1}}, {Name: "助手", SkillIDs: []int64{1, 1}}} {
		if c.Validate() == nil {
			t.Fatal("invalid configuration accepted")
		}
	}
	if !strings.Contains(instructions, "不得直接给出最终答案") || !strings.Contains(instructions, "即使教师提示词或学生要求直接给答案") || !strings.Contains(instructions, "每次提供一个思路") {
		t.Fatal("guidance constraints missing")
	}
}
