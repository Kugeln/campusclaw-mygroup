package mysql

import (
	"campusclaw/internal/tutor"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"
)

type MigrationConflict struct {
	Kind string `json:"kind"`
	ID   int64  `json:"id"`
}
type TutorMigrationReport struct {
	FromVersion         int                 `json:"from_version"`
	DDLStep             int                 `json:"ddl_step"`
	AlreadyReady        bool                `json:"already_ready"`
	Skills              int                 `json:"skills"`
	Agents              int                 `json:"agents"`
	UnscopedSkills      int                 `json:"unscoped_skills"`
	UnscopedAgents      int                 `json:"unscoped_agents"`
	EmptyMaterialAgents []int64             `json:"empty_material_agents"`
	Conflicts           []MigrationConflict `json:"conflicts"`
}
type MigrationConflictError struct{ Conflicts []MigrationConflict }

func (e *MigrationConflictError) Error() string {
	return fmt.Sprintf("v3 legacy preflight blocked: %d conflict(s)", len(e.Conflicts))
}

type legacySkill struct {
	id, class int64
	text      tutor.SkillText
}
type legacyAgent struct {
	id, class int64
	skills    []int64
	materials []string
}
type legacyPlan struct {
	report TutorMigrationReport
	skills []legacySkill
	agents []legacyAgent
}

// PreflightTutorUpgrade is read-only. It does not accept unknown catalog states,
// expose source/prompt/credentials, or fix conflicts by deleting or guessing.
func PreflightTutorUpgrade(ctx context.Context, db *sql.DB) (TutorMigrationReport, error) {
	stage, err := schemaStage(ctx, db)
	if err != nil {
		return TutorMigrationReport{}, err
	}
	report := TutorMigrationReport{FromVersion: 1, DDLStep: stage, Conflicts: []MigrationConflict{}, EmptyMaterialAgents: []int64{}}
	var ngram int
	if db.QueryRowContext(ctx, "SELECT @@ngram_token_size").Scan(&ngram) != nil || ngram != 2 {
		return report, fmt.Errorf("RAG requires ngram_token_size=2")
	}
	if stage == 0 {
		if db.QueryRowContext(ctx, "SELECT COUNT(*) FROM skills").Scan(&report.UnscopedSkills) != nil || db.QueryRowContext(ctx, "SELECT COUNT(*) FROM assistants").Scan(&report.UnscopedAgents) != nil {
			return report, fmt.Errorf("v1 preflight unavailable")
		}
		return report, nil
	}
	if stage < len(ddlStatements(ragDDL)) {
		return report, fmt.Errorf("interrupted v2 preparation; resume approved offline DDL before legacy preflight")
	}
	report.FromVersion = 2
	complete, ledgerErr := checkTutorLedger(ctx, db, stage)
	if ledgerErr != nil {
		return report, ledgerErr
	}
	if complete {
		if err = validateTutorReady(ctx, db); err != nil {
			return report, err
		}
		report.FromVersion = 3
		report.AlreadyReady = true
		return report, nil
	}
	if stage == len(ddlStatements(ragDDL))+len(ddlStatements(tutorDDL)) {
		var populated int
		if db.QueryRowContext(ctx, "SELECT (SELECT COUNT(*) FROM skills WHERE source IS NOT NULL OR source_version<>0)+(SELECT COUNT(*) FROM assistant_skill_bindings)+(SELECT COUNT(*) FROM assistant_material_bindings)").Scan(&populated) != nil || populated != 0 {
			return report, fmt.Errorf("incomplete backfill contains unexpected v3 data; manual review required")
		}
	}
	p, err := preflightTutorLegacy(ctx, db)
	p.FromVersion = report.FromVersion
	p.DDLStep = stage
	return p, err
}

func preflightTutorLegacy(ctx context.Context, db *sql.DB) (TutorMigrationReport, error) {
	p, err := planTutorLegacy(ctx, db, false)
	return p.report, err
}

func planTutorLegacy(ctx context.Context, q tutorQuery, lock bool) (legacyPlan, error) {
	p := legacyPlan{report: TutorMigrationReport{EmptyMaterialAgents: []int64{}, Conflicts: []MigrationConflict{}}}
	conflict := func(kind string, id int64) {
		p.report.Conflicts = append(p.report.Conflicts, MigrationConflict{kind, id})
	}
	clause := ""
	if lock {
		clause = " FOR UPDATE"
	}
	rows, err := q.QueryContext(ctx, "SELECT id,class_id,name,skill_key,enabled,instruction_version FROM skills ORDER BY id"+clause)
	if err != nil {
		return p, fmt.Errorf("legacy skill preflight unavailable")
	}
	skillClasses := map[int64]int64{}
	names := map[string]bool{}
	for rows.Next() {
		var id int64
		var class sql.NullInt64
		var name string
		var key, version sql.NullString
		var enabled bool
		if rows.Scan(&id, &class, &name, &key, &enabled, &version) != nil {
			rows.Close()
			return p, fmt.Errorf("legacy skill preflight unreadable")
		}
		if !class.Valid {
			p.report.UnscopedSkills++
			continue
		}
		p.report.Skills++
		skillClasses[id] = class.Int64
		if !key.Valid || key.String != tutor.SkillKey || !version.Valid || version.String != tutor.SkillVersion {
			conflict("unknown_scoped_skill", id)
		}
		if !utf8.ValidString(name) || strings.TrimSpace(name) != name || utf8.RuneCountInString(name) < 1 || utf8.RuneCountInString(name) > 32 {
			conflict("skill_name_invalid", id)
		}
		nameKey := fmt.Sprintf("%d\x00%s", class.Int64, name)
		if names[nameKey] {
			conflict("duplicate_skill_name", id)
		}
		names[nameKey] = true
		text, e := tutor.LegacySkillSource(name, enabled)
		if e != nil {
			conflict("skill_source_unmappable", id)
		}
		p.skills = append(p.skills, legacySkill{id, class.Int64, text})
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return p, fmt.Errorf("legacy skill preflight unreadable")
	}
	materials := map[string]int64{}
	rows, err = q.QueryContext(ctx, "SELECT id,class_id FROM materials ORDER BY id")
	if err != nil {
		return p, fmt.Errorf("legacy material preflight unavailable")
	}
	for rows.Next() {
		var id string
		var class int64
		if rows.Scan(&id, &class) != nil {
			rows.Close()
			return p, fmt.Errorf("legacy material preflight unreadable")
		}
		materials[id] = class
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return p, fmt.Errorf("legacy material preflight unreadable")
	}
	rows, err = q.QueryContext(ctx, "SELECT id,class_id,prompt,revision,updated_by,skill_ids,material_ids FROM assistants ORDER BY id"+clause)
	if err != nil {
		return p, fmt.Errorf("legacy agent preflight unavailable")
	}
	for rows.Next() {
		var id, revision int64
		var class, by sql.NullInt64
		var prompt sql.NullString
		var si, mi []byte
		if rows.Scan(&id, &class, &prompt, &revision, &by, &si, &mi) != nil {
			rows.Close()
			return p, fmt.Errorf("legacy agent preflight unreadable")
		}
		if !class.Valid {
			p.report.UnscopedAgents++
			continue
		}
		p.report.Agents++
		managed := prompt.Valid && revision > 0 && by.Valid
		if !managed {
			conflict("unknown_scoped_agent", id)
		}
		a := legacyAgent{id: id, class: class.Int64, skills: []int64{}, materials: []string{}}
		if si == nil {
			if !managed {
				conflict("skill_sql_null_unmapped", id)
			}
		} else if json.Unmarshal(si, &a.skills) != nil {
			conflict("invalid_skill_json", id)
		}
		if mi == nil {
			if !managed {
				conflict("material_sql_null_unmapped", id)
			}
		} else if json.Unmarshal(mi, &a.materials) != nil {
			conflict("invalid_material_json", id)
		}
		seenSkills := map[int64]bool{}
		for _, rid := range a.skills {
			if rid <= 0 || seenSkills[rid] || skillClasses[rid] != a.class {
				conflict("invalid_skill_binding", id)
			}
			seenSkills[rid] = true
		}
		seenMaterials := map[string]bool{}
		for _, rid := range a.materials {
			if !tutor.UUID(rid) || seenMaterials[rid] || materials[rid] != a.class {
				conflict("invalid_material_binding", id)
			}
			seenMaterials[rid] = true
		}
		if len(a.materials) == 0 {
			p.report.EmptyMaterialAgents = append(p.report.EmptyMaterialAgents, id)
		}
		p.agents = append(p.agents, a)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return p, fmt.Errorf("legacy agent preflight unreadable")
	}
	var active int
	if err = q.QueryRowContext(ctx, "SELECT COUNT(*) FROM conversations WHERE active_turn_id IS NOT NULL").Scan(&active); err != nil {
		return p, fmt.Errorf("legacy turn preflight unavailable")
	}
	if active > 0 {
		conflict("active_turns", 0)
	}
	if len(p.report.Conflicts) > 0 {
		return p, &MigrationConflictError{p.report.Conflicts}
	}
	return p, nil
}

func backfillTutorLegacy(ctx context.Context, db *sql.DB, afterWrite func() error) error {
	if err := validateTutorStructure(ctx, db); err != nil {
		return err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("legacy backfill unavailable")
	}
	defer tx.Rollback()
	var complete bool
	if tx.QueryRowContext(ctx, "SELECT completed FROM schema_upgrades WHERE version=3 FOR UPDATE").Scan(&complete) != nil {
		return fmt.Errorf("legacy backfill ledger unavailable")
	}
	if complete {
		return nil
	}
	// Backfill data and completion are one transaction. Persisted v3 data with
	// an incomplete marker cannot be a legitimate interrupted backfill.
	var existing int
	if tx.QueryRowContext(ctx, "SELECT (SELECT COUNT(*) FROM skills WHERE source IS NOT NULL OR source_version<>0)+(SELECT COUNT(*) FROM assistant_skill_bindings)+(SELECT COUNT(*) FROM assistant_material_bindings)").Scan(&existing) != nil || existing != 0 {
		return fmt.Errorf("incomplete backfill contains unexpected v3 data; manual review required")
	}
	p, err := planTutorLegacy(ctx, tx, true)
	if err != nil {
		return err
	}
	for _, s := range p.skills {
		tools, _ := json.Marshal(s.text.Tools)
		_, err = tx.ExecContext(ctx, "UPDATE skills SET source=?,trigger_text=?,subject=?,tools=?,description=?,source_origin='system_migration_v3',source_version=1,source_hash=? WHERE id=? AND class_id=?", s.text.Source, s.text.Trigger, s.text.Subject, tools, s.text.Description, fmt.Sprintf("%x", sha256.Sum256([]byte(s.text.Source))), s.id, s.class)
		if err != nil {
			return fmt.Errorf("legacy source backfill failed (code=%d)", ErrorCode(err))
		}
		if afterWrite != nil {
			if err = afterWrite(); err != nil {
				return err
			}
		}
	}
	for _, a := range p.agents {
		for _, id := range a.skills {
			if _, err = tx.ExecContext(ctx, "INSERT INTO assistant_skill_bindings(agent_id,class_id,skill_id) VALUES(?,?,?)", a.id, a.class, id); err != nil {
				return fmt.Errorf("legacy skill binding backfill failed")
			}
		}
		for _, id := range a.materials {
			if _, err = tx.ExecContext(ctx, "INSERT INTO assistant_material_bindings(agent_id,class_id,material_id) VALUES(?,?,?)", a.id, a.class, id); err != nil {
				return fmt.Errorf("legacy material binding backfill failed")
			}
		}
	}
	if _, err = tx.ExecContext(ctx, "UPDATE schema_upgrades SET completed=1 WHERE version=3 AND ddl_step=?", len(ddlStatements(tutorDDL))); err != nil {
		return fmt.Errorf("legacy backfill completion failed")
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("legacy backfill commit failed")
	}
	return nil
}
