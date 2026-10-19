package mysql

import (
	"campusclaw/internal/rag"
	"campusclaw/internal/tutor"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

type Tutor struct{ DB *sql.DB }
type tutorQuery interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}
type tutorScanner interface{ Scan(...any) error }

// ProvisionSkills is explicit offline seeding, never part of schema retries.
func (r Tutor) ProvisionSkills(ctx context.Context) error {
	if err := ValidateRAGSchema(ctx, r.DB); err != nil {
		return err
	}
	text, err := tutor.LegacySkillSource(tutor.SkillName, false)
	if err != nil {
		return err
	}
	tools, _ := json.Marshal(text.Tools)
	_, err = r.DB.ExecContext(ctx, `INSERT INTO skills(name,class_id,skill_key,enabled,instruction_version,revision,source,trigger_text,subject,tools,description,source_origin,source_version,source_hash) SELECT ?,c.id,?,0,?,1,?,?,?,?,?,'system_seed_v3',1,? FROM classes c WHERE NOT EXISTS(SELECT 1 FROM skills s WHERE s.class_id=c.id AND s.skill_key=?)`, text.Name, tutor.SkillKey, tutor.SkillVersion, text.Source, text.Trigger, text.Subject, tools, text.Description, sourceHash(text.Source), tutor.SkillKey)
	return tutorStorageError(err)
}

const agentColumns = `id,name,COALESCE(prompt,''),enabled,
 (SELECT COALESCE(JSON_ARRAYAGG(skill_id),JSON_ARRAY()) FROM assistant_skill_bindings b WHERE b.agent_id=assistants.id AND b.class_id=assistants.class_id),
 (SELECT COALESCE(JSON_ARRAYAGG(material_id),JSON_ARRAY()) FROM assistant_material_bindings b WHERE b.agent_id=assistants.id AND b.class_id=assistants.class_id),revision`

func readAgent(row tutorScanner) (tutor.Agent, error) {
	var a tutor.Agent
	var si, mi []byte
	if err := row.Scan(&a.ID, &a.Name, &a.Prompt, &a.Enabled, &si, &mi, &a.Revision); err != nil {
		return a, tutorStorageError(err)
	}
	if json.Unmarshal(si, &a.SkillIDs) != nil || json.Unmarshal(mi, &a.MaterialIDs) != nil {
		return a, tutor.Unavailable
	}
	sort.Slice(a.SkillIDs, func(i, j int) bool { return a.SkillIDs[i] < a.SkillIDs[j] })
	sort.Strings(a.MaterialIDs)
	return a, nil
}
func tutorStorageError(err error) error {
	if err == nil {
		return nil
	}
	if err == sql.ErrNoRows {
		return tutor.NotFound
	}
	if ErrorCode(err) == 1062 {
		return tutor.Invalid
	}
	return tutor.Unavailable
}
func (r Tutor) Agent(ctx context.Context, s rag.Scope, id int64) (tutor.Agent, error) {
	q := "SELECT " + agentColumns + " FROM assistants WHERE id=? AND class_id=?"
	if s.Role != "teacher" {
		q += " AND enabled=1"
	}
	return readAgent(r.DB.QueryRowContext(ctx, q, id, s.ClassID))
}
func (r Tutor) Agents(ctx context.Context, s rag.Scope) ([]tutor.Agent, error) {
	q := "SELECT " + agentColumns + " FROM assistants WHERE class_id=? AND prompt IS NOT NULL"
	if s.Role != "teacher" {
		q += " AND enabled=1"
	}
	rows, err := r.DB.QueryContext(ctx, q+" ORDER BY id", s.ClassID)
	if err != nil {
		return nil, tutor.Unavailable
	}
	defer rows.Close()
	out := []tutor.Agent{}
	for rows.Next() {
		a, e := readAgent(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, a)
	}
	if rows.Err() != nil {
		return nil, tutor.Unavailable
	}
	return out, nil
}

func bindings(ctx context.Context, tx *sql.Tx, s rag.Scope, c tutor.Config) error {
	for _, id := range c.SkillIDs {
		var found int64
		if e := tx.QueryRowContext(ctx, "SELECT id FROM skills WHERE id=? AND class_id=? AND source IS NOT NULL FOR SHARE", id, s.ClassID).Scan(&found); e != nil {
			return tutorStorageError(e)
		}
	}
	for _, id := range c.MaterialIDs {
		var found string
		if e := tx.QueryRowContext(ctx, "SELECT id FROM materials WHERE id=? AND class_id=? FOR SHARE", id, s.ClassID).Scan(&found); e != nil {
			return tutorStorageError(e)
		}
	}
	return nil
}

// SaveAgent validates a complete HTTP configuration before its atomic save.
func (r Tutor) SaveAgent(ctx context.Context, s rag.Scope, id int64, c tutor.Config) (tutor.Agent, error) {
	if s.Role != "teacher" {
		return tutor.Agent{}, tutor.Forbidden
	}
	if c.Validate() != nil {
		return tutor.Agent{}, tutor.Invalid
	}
	return r.SaveCurrentAgent(ctx, s, id, c)
}
func (r Tutor) SaveCurrentAgent(ctx context.Context, s rag.Scope, id int64, c tutor.Config) (tutor.Agent, error) {
	if s.Role != "teacher" {
		return tutor.Agent{}, tutor.Forbidden
	}
	check := c
	check.SkillIDs = []int64{}
	if check.Validate() != nil {
		return tutor.Agent{}, tutor.Invalid
	}
	c.Name, c.Prompt, c.MaterialIDs = check.Name, check.Prompt, check.MaterialIDs
	if c.SkillIDs == nil {
		c.SkillIDs = []int64{}
	}
	seen := map[int64]bool{}
	for _, sid := range c.SkillIDs {
		if sid <= 0 || seen[sid] {
			return tutor.Agent{}, tutor.Invalid
		}
		seen[sid] = true
	}
	c.SkillIDs = append([]int64{}, c.SkillIDs...)
	c.MaterialIDs = append([]string{}, c.MaterialIDs...)
	sort.Slice(c.SkillIDs, func(i, j int) bool { return c.SkillIDs[i] < c.SkillIDs[j] })
	sort.Strings(c.MaterialIDs)
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return tutor.Agent{}, tutor.Unavailable
	}
	defer tx.Rollback()
	var revision int64
	if id != 0 {
		if err = tx.QueryRowContext(ctx, "SELECT revision FROM assistants WHERE id=? AND class_id=? FOR UPDATE", id, s.ClassID).Scan(&revision); err != nil {
			return tutor.Agent{}, tutorStorageError(err)
		}
		if revision == math.MaxInt64 {
			return tutor.Agent{}, tutor.Unavailable
		}
	}
	if err = bindings(ctx, tx, s, c); err != nil {
		return tutor.Agent{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if id == 0 {
		var res sql.Result
		res, err = tx.ExecContext(ctx, "INSERT INTO assistants(name,class_id,prompt,enabled,revision,updated_by,updated_at) VALUES(?,?,?,?,1,?,?)", c.Name, s.ClassID, c.Prompt, c.Enabled, s.UserID, now)
		if err == nil {
			id, err = res.LastInsertId()
		}
	} else {
		_, err = tx.ExecContext(ctx, "UPDATE assistants SET name=?,prompt=?,enabled=?,revision=revision+1,updated_by=?,updated_at=? WHERE id=? AND class_id=?", c.Name, c.Prompt, c.Enabled, s.UserID, now, id, s.ClassID)
	}
	if err != nil {
		return tutor.Agent{}, tutorStorageError(err)
	}
	for _, table := range []string{"assistant_skill_bindings", "assistant_material_bindings"} {
		if _, err = tx.ExecContext(ctx, "DELETE FROM "+table+" WHERE agent_id=? AND class_id=?", id, s.ClassID); err != nil {
			return tutor.Agent{}, tutor.Unavailable
		}
	}
	for _, rid := range c.SkillIDs {
		if _, err = tx.ExecContext(ctx, "INSERT INTO assistant_skill_bindings(agent_id,class_id,skill_id) VALUES(?,?,?)", id, s.ClassID, rid); err != nil {
			return tutor.Agent{}, tutor.Unavailable
		}
	}
	for _, rid := range c.MaterialIDs {
		if _, err = tx.ExecContext(ctx, "INSERT INTO assistant_material_bindings(agent_id,class_id,material_id) VALUES(?,?,?)", id, s.ClassID, rid); err != nil {
			return tutor.Agent{}, tutor.Unavailable
		}
	}
	if tx.Commit() != nil {
		return tutor.Agent{}, tutor.Unavailable
	}
	return tutor.Agent{ID: id, Name: c.Name, Prompt: c.Prompt, Enabled: c.Enabled, SkillIDs: c.SkillIDs, MaterialIDs: c.MaterialIDs, Revision: revision + 1}, nil
}

const skillColumns = "id,name,enabled,COALESCE(instruction_version,''),revision,source,trigger_text,subject,tools,description,source_origin,source_version,source_hash"

func sourceHash(source string) string { return fmt.Sprintf("%x", sha256.Sum256([]byte(source))) }
func readSkill(row tutorScanner) (tutor.Skill, error) {
	var v tutor.Skill
	var tools []byte
	if err := row.Scan(&v.ID, &v.Name, &v.Enabled, &v.Version, &v.Revision, &v.Source, &v.Trigger, &v.Subject, &tools, &v.Description, &v.SourceOrigin, &v.SourceVersion, &v.SourceHash); err != nil {
		return v, tutorStorageError(err)
	}
	if json.Unmarshal(tools, &v.Tools) != nil {
		return v, tutor.Unavailable
	}
	parsed, err := tutor.ParseSkillSource(v.Source)
	if err != nil || parsed.Name != v.Name || parsed.Trigger != v.Trigger || parsed.Subject != v.Subject || parsed.Description != v.Description || parsed.Enabled != v.Enabled || sourceHash(v.Source) != v.SourceHash || strings.Join(parsed.Tools, "\x00") != strings.Join(v.Tools, "\x00") {
		return v, tutor.Unavailable
	}
	return v, nil
}
func (r Tutor) Skill(ctx context.Context, s rag.Scope, id int64) (tutor.Skill, error) {
	return readSkill(r.DB.QueryRowContext(ctx, "SELECT "+skillColumns+" FROM skills WHERE id=? AND class_id=?", id, s.ClassID))
}
func (r Tutor) Skills(ctx context.Context, s rag.Scope) ([]tutor.Skill, error) {
	rows, err := r.DB.QueryContext(ctx, "SELECT "+skillColumns+" FROM skills WHERE class_id=? ORDER BY id", s.ClassID)
	if err != nil {
		return nil, tutor.Unavailable
	}
	defer rows.Close()
	out := []tutor.Skill{}
	for rows.Next() {
		v, e := readSkill(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	if rows.Err() != nil {
		return nil, tutor.Unavailable
	}
	return out, nil
}
func saveSkillTx(ctx context.Context, tx *sql.Tx, s rag.Scope, id int64, text tutor.SkillText, origin string, previous *tutor.Skill, validate bool) (tutor.Skill, error) {
	if validate {
		if err := validateSkillTools(ctx, tx, s, text.Tools); err != nil {
			return tutor.Skill{}, err
		}
	}
	tools, _ := json.Marshal(text.Tools)
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if previous != nil && (previous.Revision == math.MaxInt64 || previous.SourceVersion == math.MaxInt64) {
		return tutor.Skill{}, tutor.Unavailable
	}
	var err error
	if id == 0 {
		var res sql.Result
		res, err = tx.ExecContext(ctx, "INSERT INTO skills(name,class_id,enabled,revision,updated_by,updated_at,source,trigger_text,subject,tools,description,source_origin,source_version,source_hash) VALUES(?,?,?,1,?,?,?,?,?,?,?,?,1,?)", text.Name, s.ClassID, text.Enabled, s.UserID, now, text.Source, text.Trigger, text.Subject, tools, text.Description, origin, sourceHash(text.Source))
		if err == nil {
			id, err = res.LastInsertId()
		}
	} else {
		_, err = tx.ExecContext(ctx, "UPDATE skills SET name=?,enabled=?,revision=revision+1,updated_by=?,updated_at=?,source=?,trigger_text=?,subject=?,tools=?,description=?,source_origin=?,source_version=source_version+1,source_hash=? WHERE id=? AND class_id=?", text.Name, text.Enabled, s.UserID, now, text.Source, text.Trigger, text.Subject, tools, text.Description, origin, sourceHash(text.Source), id, s.ClassID)
	}
	if err != nil {
		return tutor.Skill{}, tutorStorageError(err)
	}
	return readSkill(tx.QueryRowContext(ctx, "SELECT "+skillColumns+" FROM skills WHERE id=? AND class_id=?", id, s.ClassID))
}
func (r Tutor) SaveSkill(ctx context.Context, s rag.Scope, id int64, source string) (tutor.Skill, error) {
	if s.Role != "teacher" {
		return tutor.Skill{}, tutor.Forbidden
	}
	text, err := tutor.ParseSkillSource(source)
	if err != nil {
		return tutor.Skill{}, err
	}
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return tutor.Skill{}, tutor.Unavailable
	}
	defer tx.Rollback()
	var previous *tutor.Skill
	if id != 0 {
		v, e := readSkill(tx.QueryRowContext(ctx, "SELECT "+skillColumns+" FROM skills WHERE id=? AND class_id=? FOR UPDATE", id, s.ClassID))
		if e != nil {
			return tutor.Skill{}, e
		}
		previous = &v
	}
	v, err := saveSkillTx(ctx, tx, s, id, text, "teacher", previous, true)
	if err != nil {
		return v, err
	}
	if tx.Commit() != nil {
		return v, tutor.Unavailable
	}
	return v, nil
}
func (r Tutor) Toggle(ctx context.Context, s rag.Scope, id int64, enabled bool) (tutor.Skill, error) {
	if s.Role != "teacher" {
		return tutor.Skill{}, tutor.Forbidden
	}
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return tutor.Skill{}, tutor.Unavailable
	}
	defer tx.Rollback()
	v, err := readSkill(tx.QueryRowContext(ctx, "SELECT "+skillColumns+" FROM skills WHERE id=? AND class_id=? FOR UPDATE", id, s.ClassID))
	if err != nil {
		return v, err
	}
	lines := strings.SplitAfter(v.Source, "\n")
	started := false
	replaced := false
	for i, line := range lines {
		trim := strings.TrimSpace(line)
		if trim == "" {
			if started {
				break
			}
			continue
		}
		if strings.HasPrefix(trim, "#") {
			continue
		}
		started = true
		p := strings.IndexAny(line, ":：")
		if p < 0 || strings.TrimSpace(line[:p]) != "启用" {
			continue
		}
		prefix := line[:p]
		sep := ":"
		if strings.HasPrefix(line[p:], "：") {
			sep = "："
		}
		ending := ""
		if strings.HasSuffix(line, "\r\n") {
			ending = "\r\n"
		} else if strings.HasSuffix(line, "\n") {
			ending = "\n"
		}
		flag := "否"
		if enabled {
			flag = "是"
		}
		lines[i] = prefix + sep + flag + ending
		replaced = true
		break
	}
	if !replaced {
		return v, tutor.Unavailable
	}
	text, err := tutor.ParseSkillSource(strings.Join(lines, ""))
	if err != nil {
		return v, err
	}
	updated, err := saveSkillTx(ctx, tx, s, id, text, v.SourceOrigin, &v, enabled)
	if err != nil {
		return updated, err
	}
	if tx.Commit() != nil {
		return updated, tutor.Unavailable
	}
	return updated, nil
}
func (r Tutor) DeleteSkill(ctx context.Context, s rag.Scope, id int64) error {
	if s.Role != "teacher" {
		return tutor.Forbidden
	}
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return tutor.Unavailable
	}
	defer tx.Rollback()
	var found int64
	if err = tx.QueryRowContext(ctx, "SELECT id FROM skills WHERE id=? AND class_id=? FOR UPDATE", id, s.ClassID).Scan(&found); err != nil {
		return tutorStorageError(err)
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM skills WHERE id=? AND class_id=?", id, s.ClassID); err != nil {
		return tutor.Unavailable
	}
	return tutorStorageError(tx.Commit())
}
func (r Tutor) Snapshot(ctx context.Context, s rag.Scope, id int64) (tutor.Snapshot, error) {
	tx, err := r.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return tutor.Snapshot{}, tutor.Unavailable
	}
	defer tx.Rollback()
	a, err := readAgent(tx.QueryRowContext(ctx, "SELECT "+agentColumns+" FROM assistants WHERE id=? AND class_id=? AND prompt IS NOT NULL", id, s.ClassID))
	if err != nil {
		return tutor.Snapshot{}, err
	}
	v := tutor.Snapshot{Agent: a, ClassID: s.ClassID, Skills: []tutor.Skill{}}
	rows, err := tx.QueryContext(ctx, "SELECT "+skillColumns+" FROM skills WHERE class_id=? AND enabled=1 AND id IN(SELECT skill_id FROM assistant_skill_bindings WHERE agent_id=? AND class_id=?) ORDER BY id", s.ClassID, id, s.ClassID)
	if err != nil {
		return v, tutor.Unavailable
	}
	for rows.Next() {
		skill, e := readSkill(rows)
		if e != nil {
			rows.Close()
			return v, e
		}
		v.Skills = append(v.Skills, skill)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return v, tutor.Unavailable
	}
	// Selection is question-dependent and belongs to the pure lexical router.
	if tx.Commit() != nil {
		return v, tutor.Unavailable
	}
	return v, nil
}
