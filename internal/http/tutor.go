package httpapi

import (
	"campusclaw/internal/rag"
	"campusclaw/internal/tutor"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"
)

type TutorStore interface {
	Agents(context.Context, rag.Scope) ([]tutor.Agent, error)
	Agent(context.Context, rag.Scope, int64) (tutor.Agent, error)
	SaveAgent(context.Context, rag.Scope, int64, tutor.Config) (tutor.Agent, error)
	Skills(context.Context, rag.Scope) ([]tutor.Skill, error)
	Toggle(context.Context, rag.Scope, int64, bool) (tutor.Skill, error)
	Skill(context.Context, rag.Scope, int64) (tutor.Skill, error)
	SaveSkill(context.Context, rag.Scope, int64, string) (tutor.Skill, error)
	DeleteSkill(context.Context, rag.Scope, int64) error
}
type TutorHandler struct{ Store TutorStore }

func tutorError(w http.ResponseWriter, e error) {
	code := 503
	switch {
	case errors.Is(e, tutor.Invalid):
		code = 400
	case errors.Is(e, tutor.Forbidden):
		code = 403
	case errors.Is(e, tutor.NotFound):
		code = 404
	}
	authError(w, code, e.Error())
}
func tutorID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, e := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if e != nil || id <= 0 {
		tutorError(w, tutor.Invalid)
		return 0, false
	}
	return id, true
}
func tutorObject(w http.ResponseWriter, r *http.Request, allowed ...string) (map[string]json.RawMessage, bool) {
	v, ok := tutorFields(w, r, allowed...)
	if !ok {
		return nil, false
	}
	if len(v) != len(allowed) {
		tutorError(w, tutor.Invalid)
		return nil, false
	}
	return v, true
}
func tutorFields(w http.ResponseWriter, r *http.Request, allowed ...string) (map[string]json.RawMessage, bool) {
	media, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if media != "application/json" && !strings.HasSuffix(media, "+json") {
		tutorError(w, tutor.Invalid)
		return nil, false
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	body, err := io.ReadAll(r.Body)
	if err != nil || !utf8.Valid(body) {
		tutorError(w, tutor.Invalid)
		return nil, false
	}
	d := json.NewDecoder(strings.NewReader(string(body)))
	start, err := d.Token()
	if err != nil || start != json.Delim('{') {
		tutorError(w, tutor.Invalid)
		return nil, false
	}
	v := map[string]json.RawMessage{}
	for d.More() {
		token, e := d.Token()
		key, valid := token.(string)
		if e != nil || !valid {
			tutorError(w, tutor.Invalid)
			return nil, false
		}
		if _, duplicate := v[key]; duplicate {
			tutorError(w, tutor.Invalid)
			return nil, false
		}
		var value json.RawMessage
		if d.Decode(&value) != nil {
			tutorError(w, tutor.Invalid)
			return nil, false
		}
		v[key] = value
	}
	end, err := d.Token()
	var extra any
	if err != nil || end != json.Delim('}') || d.Decode(&extra) != io.EOF {
		tutorError(w, tutor.Invalid)
		return nil, false
	}
	keys := map[string]bool{}
	for _, k := range allowed {
		keys[k] = true
	}
	for k, b := range v {
		if !keys[k] || string(b) == "null" {
			tutorError(w, tutor.Invalid)
			return nil, false
		}
	}
	return v, true
}
func tutorJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// Agents documents this registered handler.
// @Summary 本班智能体列表
// @Description 教师可看本班全部；学生只看启用项。班级来自Bearer身份。
// @Tags agents
// @Produce json
// @Security bearerAuth
// @Success 200 {object} AgentList
// @Failure 401 {object} APIError
// @Failure 500 {object} APIError
// @Failure 503 {object} APIError
// @Router /api/agents [get]
func (h *TutorHandler) Agents(w http.ResponseWriter, r *http.Request) {
	v, e := h.Store.Agents(r.Context(), scope(r))
	if e != nil {
		tutorError(w, e)
		return
	}
	tutorJSON(w, 200, map[string]any{"agents": v})
}

// Agent documents this registered handler.
// @Summary 读取本班智能体
// @Description 学生读取停用、外班或不存在项均404；教师可管理本班停用项。
// @Tags agents
// @Produce json
// @Security bearerAuth
// @Param id path int true "本班ID" minimum(1)
// @Success 200 {object} tutor.Agent
// @Failure 400 {object} APIError
// @Failure 401 {object} APIError
// @Failure 404 {object} APIError
// @Failure 500 {object} APIError
// @Failure 503 {object} APIError
// @Router /api/agents/{id} [get]
func (h *TutorHandler) Agent(w http.ResponseWriter, r *http.Request) {
	id, ok := tutorID(w, r)
	if !ok {
		return
	}
	v, e := h.Store.Agent(r.Context(), scope(r), id)
	if e != nil {
		tutorError(w, e)
		return
	}
	tutorJSON(w, 200, v)
}
func (h *TutorHandler) Save(w http.ResponseWriter, r *http.Request) {
	id := int64(0)
	if r.Method == "PUT" {
		var ok bool
		id, ok = tutorID(w, r)
		if !ok {
			return
		}
	}
	v, ok := tutorObject(w, r, "name", "prompt", "enabled", "skill_ids", "material_ids")
	if !ok {
		return
	}
	var c tutor.Config
	if json.Unmarshal(v["name"], &c.Name) != nil || json.Unmarshal(v["prompt"], &c.Prompt) != nil || json.Unmarshal(v["enabled"], &c.Enabled) != nil || json.Unmarshal(v["skill_ids"], &c.SkillIDs) != nil || json.Unmarshal(v["material_ids"], &c.MaterialIDs) != nil {
		tutorError(w, tutor.Invalid)
		return
	}
	a, e := h.Store.SaveAgent(r.Context(), scope(r), id, c)
	if e != nil {
		tutorError(w, e)
		return
	}
	status := 200
	if r.Method == "POST" {
		status = 201
	}
	tutorJSON(w, status, a)
}

// CreateAgent creates a scoped teacher configuration.
// @Summary 教师创建本班智能体
// @Description 完整字段必填，两个绑定数组不可为null。空prompt使用默认助教角色；空材料不检索，仍生成。绑定仅允许本班资源。
// @Tags agents
// @Accept json
// @Produce json
// @Security bearerAuth
// @Param body body AgentInput true "完整配置，名称1–64字、prompt≤2000字"
// @Success 201 {object} tutor.Agent
// @Failure 400 {object} APIError "重名、非法或缺失字段"
// @Failure 401 {object} APIError
// @Failure 403 {object} APIError "仅教师"
// @Failure 404 {object} APIError "外班或不存在的绑定"
// @Failure 500 {object} APIError "身份存储故障"
// @Failure 503 {object} APIError
// @x-strict-request true
// @Router /api/agents [post]
func (h *TutorHandler) CreateAgent(w http.ResponseWriter, r *http.Request) { h.Save(w, r) }

// UpdateAgent updates the complete configuration for the next turn.
// @Summary 教师更新本班智能体
// @Description 完整字段必填。修改/停用/解绑下一轮生效，正在生成及历史快照不变；停用只禁止新建会话。
// @Tags agents
// @Accept json
// @Produce json
// @Security bearerAuth
// @Param id path int true "本班智能体ID" minimum(1)
// @Param body body AgentInput true "完整配置，空数组合法，null或省略拒绝"
// @Success 200 {object} tutor.Agent
// @Failure 400 {object} APIError
// @Failure 401 {object} APIError
// @Failure 403 {object} APIError
// @Failure 404 {object} APIError
// @Failure 500 {object} APIError
// @Failure 503 {object} APIError
// @x-strict-request true
// @Router /api/agents/{id} [put]
func (h *TutorHandler) UpdateAgent(w http.ResponseWriter, r *http.Request) { h.Save(w, r) }

// Skills documents this registered handler.
// @Summary 本班文本技能列表
// @Description 已登录用户可读本班技能原文、解析字段、启用状态和版本；写操作仅教师。
// @Tags skills
// @Produce json
// @Security bearerAuth
// @Success 200 {object} SkillList
// @Failure 401 {object} APIError
// @Failure 500 {object} APIError
// @Failure 503 {object} APIError
// @Router /api/skills [get]
func (h *TutorHandler) Skills(w http.ResponseWriter, r *http.Request) {
	v, e := h.Store.Skills(r.Context(), scope(r))
	if e != nil {
		tutorError(w, e)
		return
	}
	tutorJSON(w, 200, map[string]any{"skills": v})
}
func (h *TutorHandler) Toggle(w http.ResponseWriter, r *http.Request) {
	id, ok := tutorID(w, r)
	if !ok {
		return
	}
	v, ok := tutorObject(w, r, "enabled")
	if !ok {
		return
	}
	var enabled bool
	if json.Unmarshal(v["enabled"], &enabled) != nil {
		tutorError(w, tutor.Invalid)
		return
	}
	skill, e := h.Store.Toggle(r.Context(), scope(r), id, enabled)
	if e != nil {
		tutorError(w, e)
		return
	}
	tutorJSON(w, 200, skill)
}

// Skill documents this registered handler.
// @Summary 读取本班技能原文与元数据
// @Tags skills
// @Produce json
// @Security bearerAuth
// @Param id path int true "本班技能ID" minimum(1)
// @Success 200 {object} tutor.Skill
// @Failure 400 {object} APIError
// @Failure 401 {object} APIError
// @Failure 404 {object} APIError
// @Failure 500 {object} APIError
// @Failure 503 {object} APIError
// @Router /api/skills/{id} [get]
func (h *TutorHandler) Skill(w http.ResponseWriter, r *http.Request) {
	id, ok := tutorID(w, r)
	if !ok {
		return
	}
	v, err := h.Store.Skill(r.Context(), scope(r), id)
	if err != nil {
		tutorError(w, err)
		return
	}
	tutorJSON(w, 200, v)
}
func (h *TutorHandler) SaveSkill(w http.ResponseWriter, r *http.Request) {
	id := int64(0)
	if r.Method == "PUT" {
		var ok bool
		id, ok = tutorID(w, r)
		if !ok {
			return
		}
	}
	allowed := []string{"source"}
	if r.Method == "PUT" {
		allowed = append(allowed, "enabled")
	}
	v, ok := tutorFields(w, r, allowed...)
	if !ok {
		return
	}
	if len(v) != 1 {
		tutorError(w, tutor.Invalid)
		return
	}
	var skill tutor.Skill
	var err error
	if raw, exists := v["source"]; exists {
		var source string
		if json.Unmarshal(raw, &source) != nil {
			tutorError(w, tutor.Invalid)
			return
		}
		skill, err = h.Store.SaveSkill(r.Context(), scope(r), id, source)
	} else {
		var enabled bool
		if json.Unmarshal(v["enabled"], &enabled) != nil {
			tutorError(w, tutor.Invalid)
			return
		}
		skill, err = h.Store.Toggle(r.Context(), scope(r), id, enabled)
	}
	if err != nil {
		tutorError(w, err)
		return
	}
	status := 200
	if r.Method == "POST" {
		status = 201
	}
	tutorJSON(w, status, skill)
}

// DeleteSkill documents this registered handler.
// @Summary 教师删除本班技能
// @Description 清理当前绑定，历史技能名/配置/证据快照不重写；外班或不存在统一404。
// @Tags skills
// @Security bearerAuth
// @Param id path int true "本班技能ID" minimum(1)
// @Success 204 "No Content"
// @Failure 400 {object} APIError
// @Failure 401 {object} APIError
// @Failure 403 {object} APIError
// @Failure 404 {object} APIError
// @Failure 500 {object} APIError
// @Failure 503 {object} APIError
// @Router /api/skills/{id} [delete]
func (h *TutorHandler) DeleteSkill(w http.ResponseWriter, r *http.Request) {
	id, ok := tutorID(w, r)
	if !ok {
		return
	}
	if err := h.Store.DeleteSkill(r.Context(), scope(r), id); err != nil {
		tutorError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// CreateSkill creates a parsed UTF-8 skill source.
// @Summary 教师创建文本技能
// @Description 名称/适用条件/启用必填，空行后为正文；source≤8000、名称1–32、条件1–120、资料≤32、正文1–2000个Unicode字符。保留原文；班内重名400，非法UTF-8/字段/工具拒绝。
// @Tags skills
// @Accept json
// @Produce json
// @Security bearerAuth
// @Param body body SkillSourceInput true "source完整文本"
// @Success 201 {object} tutor.Skill
// @Failure 400 {object} APIError
// @Failure 401 {object} APIError
// @Failure 403 {object} APIError
// @Failure 500 {object} APIError
// @Failure 503 {object} APIError
// @x-strict-request true
// @Router /api/skills [post]
func (h *TutorHandler) CreateSkill(w http.ResponseWriter, r *http.Request) { h.SaveSkill(w, r) }

// UpdateSkill accepts exactly one of source or the legacy enabled toggle.
// @Summary 教师更新技能或兼容启停
// @Description 仅接受source完整文本或enabled-only二选一；混合/null/未知字段400。开关原子同步source与解析元数据。下一轮生效，历史不改。
// @Tags skills
// @Accept json
// @Produce json
// @Security bearerAuth
// @Param id path int true "本班技能ID" minimum(1)
// @Param body body SkillUpdateInput true "source或enabled-only，禁止混合"
// @Success 200 {object} tutor.Skill
// @Failure 400 {object} APIError
// @Failure 401 {object} APIError
// @Failure 403 {object} APIError
// @Failure 404 {object} APIError
// @Failure 500 {object} APIError
// @Failure 503 {object} APIError
// @x-request-oneof [["source"],["enabled"]]
// @x-strict-request true
// @Router /api/skills/{id} [put]
func (h *TutorHandler) UpdateSkill(w http.ResponseWriter, r *http.Request) { h.SaveSkill(w, r) }
