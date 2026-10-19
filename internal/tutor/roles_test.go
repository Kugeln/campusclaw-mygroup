package tutor

import (
	"campusclaw/internal/rag"
	"strings"
	"testing"
)

func TestBlankRoleAndDisabledExistingPreparation(t *testing.T) {
	for _, role := range []string{"", " \n\t"} {
		cfg := Config{Name: strings.Repeat("字", 64), Prompt: role, SkillIDs: []int64{1, 2}}
		if err := cfg.Validate(); err != nil || cfg.Prompt != "" {
			t.Fatal("blank role rejected", err)
		}
		s := Snapshot{ClassID: 1, Agent: Agent{ID: 10, Enabled: false, Prompt: cfg.Prompt, MaterialIDs: []string{"22222222-2222-4222-8222-222222222222"}}}
		p, err := Prepare(s, []rag.Evidence{{ChunkID: "11111111-1111-4111-8111-111111111111", MaterialID: s.Agent.MaterialIDs[0], Index: 1, Text: "资料"}})
		if err != nil || !strings.Contains(p.System, DefaultRole) || p.Snapshot.Agent.Prompt != "" || p.Snapshot.Agent.Enabled {
			t.Fatal("fallback or disabled eligibility", err)
		}
	}
	for _, n := range []int{2000, 2001} {
		c := Config{Name: "边界", Prompt: strings.Repeat("𠮷", n)}
		if (c.Validate() == nil) != (n == 2000) {
			t.Fatal("role boundary", n)
		}
	}
}
