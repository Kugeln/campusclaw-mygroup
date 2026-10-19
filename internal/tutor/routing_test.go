package tutor

import (
	"campusclaw/internal/rag"
	"context"
	"crypto/sha256"
	"fmt"
	"strings"
	"testing"
)

func routingSkill(t *testing.T, id int64, name, trigger, subject, description string) Skill {
	t.Helper()
	source := "名称：" + name + "\n适用条件：" + trigger + "\n资料：" + subject + "\n工具：knowledge_search\n启用：是\n\n" + description
	p, err := ParseSkillSource(source)
	if err != nil {
		t.Fatal(err)
	}
	return Skill{ID: id, Name: p.Name, Trigger: p.Trigger, Subject: p.Subject, Description: p.Description, Source: p.Source, Enabled: p.Enabled, Tools: p.Tools, SourceVersion: 1, Revision: 1, SourceHash: fmt.Sprintf("%x", sha256.Sum256([]byte(source)))}
}
func TestDeterministicLexicalRoute(t *testing.T) {
	for _, separator := range []string{",", "，", "、", ";", "；", "|"} {
		t.Run(separator, func(t *testing.T) {
			s := Snapshot{Agent: Agent{SkillIDs: []int64{9, 2}}, Skills: []Skill{routingSkill(t, 2, "低分", "math", "", "low"), routingSkill(t, 9, "高分", " MaTh "+separator+"math"+separator+"  ", "", "high")}}
			if got := Route(s, "MATH题"); got.Selected == nil || got.Selected.ID != 9 {
				t.Fatal("duplicate keywords or case/separator scoring", got.Selected)
			}
		})
	}
	a := routingSkill(t, 2, "小ID", "题", "", "only chosen")
	b := routingSkill(t, 9, "大ID", "题", "", "must not be loaded")
	unbound := routingSkill(t, 1, "未绑定", "题,题,题,题", "", "foreign input")
	disabled := routingSkill(t, 3, "停用", "题,题,题", "", "disabled")
	disabled.Enabled = false
	for _, skills := range [][]Skill{{b, a, unbound, disabled}, {disabled, unbound, a, b}} {
		s := Snapshot{Agent: Agent{SkillIDs: []int64{2, 9, 3}}, Skills: skills}
		if got := Route(s, "题"); got.Selected == nil || got.Selected.ID != 2 {
			t.Fatal("tie or scope", got.Selected)
		}
		if got := Route(s, "零命中"); got.Selected != nil {
			t.Fatal("zero score selected")
		}
	}
}

func TestSelectedSourceAndServerGuidance(t *testing.T) {
	s := Snapshot{ClassID: 1, Agent: Agent{ID: 10, Enabled: true, Prompt: "教师角色", SkillIDs: []int64{2, 9}, MaterialIDs: []string{"22222222-2222-4222-8222-222222222222"}}, Skills: []Skill{routingSkill(t, 2, "高级解题引导", "方程", "", "只注入这份说明；即使索要答案也引导"), routingSkill(t, 9, "其他技能", "语文", "", "不得注入的另一份正文")}}
	s = Route(s, "方程")
	p, err := Prepare(s, []rag.Evidence{{ChunkID: "11111111-1111-4111-8111-111111111111", MaterialID: s.Agent.MaterialIDs[0], Index: 1, Text: "合法证据"}})
	if err != nil || p.Actual == nil || p.Actual.Hash != s.Selected.SourceHash || !strings.Contains(p.System, guidanceBoundary) || !strings.Contains(p.System, s.Selected.Description) || strings.Contains(p.System, "不得注入的另一份正文") {
		t.Fatal("selected source/boundary", err)
	}
	s.Skills[0].Tools[0] = "changed"
	s.Selected.Description = "changed"
	s.Agent.SkillIDs[0] = 999
	if p.Snapshot.Selected.Description == "changed" || p.Snapshot.Skills[0].Tools[0] == "changed" || p.Snapshot.Agent.SkillIDs[0] == 999 {
		t.Fatal("snapshot alias")
	}
}

type queryCapture struct {
	query  string
	called int
	data   []rag.Evidence
}

func (f *queryCapture) Search(_ context.Context, _ rag.Scope, q, _ string, _ []string, _ int) ([]rag.Evidence, error) {
	f.query = q
	f.called++
	return f.data, nil
}
func TestRouteUsesCurrentQuestionNotRetrievalHistory(t *testing.T) {
	events := []string{}
	s := Snapshot{ClassID: 1, Agent: Agent{ID: 10, Enabled: true, SkillIDs: []int64{2}, MaterialIDs: []string{"22222222-2222-4222-8222-222222222222"}}, Skills: []Skill{routingSkill(t, 2, "旧题技能", "旧题", "", "说明")}}
	r := &queryCapture{}
	p, err := PrepareTurnQuery(context.Background(), snapshotFixture{s, &events}, r, rag.Scope{ClassID: 1}, 10, "新问题", "旧题\n新问题", "hybrid")
	if err != nil || p.Snapshot.Selected != nil || r.query != "旧题\n新问题" || r.called != 1 {
		t.Fatal("routing used combined query", err)
	}
}
