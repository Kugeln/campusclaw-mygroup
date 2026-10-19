package httpapi

import (
	"campusclaw/internal/auth"
	"campusclaw/internal/rag"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"
)

type RAGHandler struct {
	Retrieval rag.Retrieval
	Dialogue  rag.Dialogue
	Indexer   *rag.Indexer
}

func scope(r *http.Request) rag.Scope {
	p, _ := auth.FromContext(r.Context())
	return rag.Scope{UserID: p.UserID(), ClassID: p.ClassID(), Role: p.Role()}
}
func ragError(w http.ResponseWriter, e error) {
	code := 503
	switch {
	case errors.Is(e, rag.Invalid):
		code = 400
	case errors.Is(e, rag.NotFound):
		code = 404
	case errors.Is(e, rag.Busy):
		code = 409
	case errors.Is(e, rag.InvalidAnswer):
		code = 502
	}
	authError(w, code, e.Error())
}
func readObject(w http.ResponseWriter, r *http.Request) (map[string]any, bool) {
	media, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if media != "application/json" && !strings.HasSuffix(media, "+json") {
		authError(w, 400, "JSON object required")
		return nil, false
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	d := json.NewDecoder(r.Body)
	var data map[string]any
	var extra any
	if d.Decode(&data) != nil || data == nil || d.Decode(&extra) != io.EOF {
		authError(w, 400, "JSON object required")
		return nil, false
	}
	return data, true
}

// Search documents this registered handler.
// @Summary Week4本班三模式检索
// @Description 固定top4；keyword零Embedding，向量阈值0.35及RRF保持；依赖故障503不降级。本接口nil材料范围为本班，不采用Week5空绑定语义。
// @Tags rag
// @Produce json
// @Security bearerAuth
// @Param q query string true "非空问题"
// @Param mode query string false "检索模式" Enums(keyword,vector,hybrid) default(hybrid)
// @Success 200 {object} SearchOutput
// @Failure 400 {object} APIError
// @Failure 401 {object} APIError
// @Failure 500 {object} APIError
// @Failure 503 {object} APIError
// @Router /api/search [get]
func (h *RAGHandler) Search(w http.ResponseWriter, r *http.Request) {
	mode := "hybrid"
	if values, exists := r.URL.Query()["mode"]; exists {
		mode = values[0]
	}
	items, e := h.Retrieval.Search(r.Context(), scope(r), r.URL.Query().Get("q"), mode, nil, 4)
	if e != nil {
		ragError(w, e)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"mode": mode, "items": items})
}

// Ask documents this registered handler.
// @Summary Week4非流式单次问答
// @Description question/mode输入，answer/sources输出；不写会话或注入教师技能。无命中固定“资料中未找到相关内容”、sources=[]、对话调用0次。有据非法引用502，依赖故障503。
// @Tags rag
// @Accept json
// @Produce json
// @Security bearerAuth
// @Param body body AskInput true "单次问题"
// @Success 200 {object} rag.Answer
// @Failure 400 {object} APIError
// @Failure 401 {object} APIError
// @Failure 500 {object} APIError
// @Failure 502 {object} APIError
// @Failure 503 {object} APIError
// @Router /api/ask [post]
func (h *RAGHandler) Ask(w http.ResponseWriter, r *http.Request) {
	data, ok := readObject(w, r)
	if !ok {
		return
	}
	q, _ := data["question"].(string)
	mode := "hybrid"
	if v, ok := data["mode"]; ok {
		mode, _ = v.(string)
	}
	answer, e := rag.Ask(r.Context(), h.Retrieval, h.Dialogue, scope(r), q, mode)
	if e != nil {
		ragError(w, e)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(answer)
}

// Reindex documents this registered handler.
// @Summary 教师重建本班材料索引
// @Description 空对象使用默认策略；size默认800，overlap默认size/10且必须小于size，delimiter策略需要非空delimiter。失败不删除原文或改历史引用。
// @Tags materials
// @Accept json
// @Produce json
// @Security bearerAuth
// @Param id path string true "材料UUID" format(uuid)
// @Param body body ReindexInput true "切片策略"
// @Success 200 {object} IndexOutput
// @Failure 400 {object} APIError
// @Failure 401 {object} APIError
// @Failure 403 {object} APIError
// @Failure 404 {object} APIError
// @Failure 409 {object} APIError
// @Failure 500 {object} APIError
// @Failure 503 {object} APIError
// @Router /api/materials/{id}/reindex [post]
func (h *RAGHandler) Reindex(w http.ResponseWriter, r *http.Request) {
	data, ok := readObject(w, r)
	if !ok {
		return
	}
	status, e := h.Indexer.Run(r.Context(), scope(r), r.PathValue("id"), data, false)
	if e != nil {
		ragError(w, e)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"material_id": r.PathValue("id"), "index_status": status})
}
