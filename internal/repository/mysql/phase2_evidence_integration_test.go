//go:build alignstorage

package mysql

import (
	"campusclaw/internal/rag"
	"campusclaw/internal/tutor"
	"context"
	"errors"
	"testing"
)

func TestAlignPhase2EvidenceCurrentMaterialSet(t *testing.T) {
	_, repo := currentFixture(t)
	ctx := context.Background()
	r := &phase2Retriever{data: []rag.Evidence{{ChunkID: alignMaterialA, MaterialID: alignMaterialA, Title: "数学", Index: 1, Text: "资料"}}}
	_, err := tutor.PrepareTurn(ctx, repo, r, alignStudent, 10, "方程", "hybrid")
	if err != nil || r.calls != 1 || len(r.materials) != 1 || r.materials[0] != alignMaterialA || r.scope.ClassID != 1 {
		t.Fatal("binding scope", err)
	}
	cfg := tutor.Config{Name: "A tutor", Prompt: "role", Enabled: true, SkillIDs: []int64{20}, MaterialIDs: []string{}}
	if _, err = repo.SaveAgent(ctx, alignTeacher, 10, cfg); err != nil {
		t.Fatal(err)
	}
	p, err := tutor.PrepareTurn(ctx, repo, r, alignStudent, 10, "方程", "hybrid")
	if err != nil || r.calls != 1 || len(p.Evidence) != 0 || p.Answer != "" || p.System == "" {
		t.Fatal("last unbind expanded or generation input missing", err)
	}
	cfg.MaterialIDs = []string{alignMaterialA}
	if _, err = repo.SaveAgent(ctx, alignTeacher, 10, cfg); err != nil {
		t.Fatal(err)
	}
	r.err = rag.Unavailable
	if _, err = tutor.PrepareTurn(ctx, repo, r, alignStudent, 10, "方程", "hybrid"); !errors.Is(err, rag.Unavailable) {
		t.Fatal("outage treated as empty", err)
	}
	r.err = nil
	r.data[0].MaterialID = alignMaterialB
	if _, err = tutor.PrepareTurn(ctx, repo, r, alignStudent, 10, "方程", "hybrid"); !errors.Is(err, tutor.NotFound) {
		t.Fatal("foreign material accepted", err)
	}
}
