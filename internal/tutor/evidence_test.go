package tutor

import (
	"campusclaw/internal/rag"
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestWeekFiveEmptyBindingShortCircuits(t *testing.T) {
	for _, ids := range [][]string{nil, {}} {
		events := []string{}
		s := Snapshot{ClassID: 1, Agent: Agent{ID: 10, Enabled: true, MaterialIDs: ids}}
		r := &queryCapture{}
		p, err := PrepareTurn(context.Background(), snapshotFixture{s, &events}, r, rag.Scope{ClassID: 1}, 10, "问题", "hybrid")
		if err != nil || r.called != 0 || len(p.Evidence) != 0 || p.Answer != "" || p.System == "" {
			t.Fatal("empty binding expanded or generation input missing", err)
		}
	}
}
func TestWeekFiveTopFourThenSubjectAndNumbering(t *testing.T) {
	material := "22222222-2222-4222-8222-222222222222"
	s := Snapshot{ClassID: 1, Agent: Agent{ID: 10, Enabled: true, SkillIDs: []int64{2}, MaterialIDs: []string{material}}, Skills: []Skill{routingSkill(t, 2, "技能", "题", "数学", "说明")}}
	s = Route(s, "题")
	evidence := []rag.Evidence{}
	for i, title := range []string{"数学讲义", "语文", "数学空文", "数学复习", "数学不应补入"} {
		text := "事实"
		if i == 2 {
			text = " \n"
		}
		evidence = append(evidence, rag.Evidence{ChunkID: fmt.Sprintf("00000000-0000-4000-8000-%012d", i+1), MaterialID: material, Title: title, Index: i + 1, Text: text})
	}
	p, err := Prepare(s, evidence)
	if err != nil || len(p.Evidence) != 2 || p.Evidence[0].Index != 1 || p.Evidence[1].Index != 4 {
		t.Fatal("filter/top4", p.Evidence, err)
	}
	sources, err := rag.ValidateSources("事实[1]和[2]", p.Evidence)
	if err != nil || len(sources) != 2 || sources[1].Number != 2 || sources[1].Index != 4 {
		t.Fatal("continuous identities", sources, err)
	}
	if strings.Contains(p.System, "事实") || strings.Contains(p.System, "数学复习") {
		t.Fatal("evidence elevated into system")
	}
	foreign := evidence[0]
	foreign.MaterialID = "33333333-3333-4333-8333-333333333333"
	if _, err = Prepare(s, []rag.Evidence{foreign}); !errors.Is(err, NotFound) {
		t.Fatal("foreign/unbound evidence accepted", err)
	}
	s.Agent.MaterialIDs = nil
	if _, err = Prepare(s, evidence); !errors.Is(err, NotFound) {
		t.Fatal("direct core bypassed empty scope", err)
	}
}
