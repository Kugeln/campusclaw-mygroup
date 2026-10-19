package httpapi

import (
	"campusclaw/internal/auth"
	"campusclaw/internal/material"
	"campusclaw/internal/storage"
	"encoding/json"
	"errors"
	"mime"
	"net/http"
	"strings"
	"time"
	"unicode"
)

type MaterialReads struct {
	Reader material.Reader
	Store  *storage.PrivateStore
}

// List documents this registered handler.
// @Summary 本班材料列表
// @Description 不接受客户端scope查询参数；使用可信当前班级。
// @Tags materials
// @Produce json
// @Security bearerAuth
// @Success 200 {object} MaterialList
// @Failure 400 {object} APIError
// @Failure 401 {object} APIError
// @Failure 500 {string} string "存储故障为HTML500，身份故障为JSON500"
// @x-response-media {"500":{"text/html":{"type":"string"},"application/json":{"$ref":"#/components/schemas/httpapi.APIError"}}}
// @Router /api/materials [get]
func (h *MaterialReads) List(w http.ResponseWriter, r *http.Request) {
	if r.URL.RawQuery != "" {
		authError(w, 400, "Client scope parameters are not accepted")
		return
	}
	p, _ := auth.FromContext(r.Context())
	items, e := h.Reader.List(r.Context(), p)
	if e != nil {
		materialReadError(w, e)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(struct {
		Items []material.Item `json:"items"`
	}{items})
}
func materialReadError(w http.ResponseWriter, e error) {
	if errors.Is(e, material.ErrNotFound) {
		authError(w, 404, "Material not found")
		return
	}
	// Flask's read routes let storage exceptions reach the generic HTML 500
	// handler. Keep that status/media contract without exposing exception text.
	uploadInputError(w, 500, "The server encountered an internal error and was unable to complete your request.")
}

// Detail documents this registered handler.
// @Summary 本班材料原文
// @Description 不存在/外班统一404；正文是数据，不提升为系统权限。
// @Tags materials
// @Produce json
// @Security bearerAuth
// @Param id path string true "材料UUID" format(uuid)
// @Success 200 {object} material.Detail
// @Failure 401 {object} APIError
// @Failure 404 {object} APIError
// @Failure 500 {string} string "存储故障为HTML500"
// @x-response-media {"500":{"text/html":{"type":"string"},"application/json":{"$ref":"#/components/schemas/httpapi.APIError"}}}
// @Router /api/materials/{id} [get]
func (h *MaterialReads) Detail(w http.ResponseWriter, r *http.Request) {
	p, _ := auth.FromContext(r.Context())
	v, e := h.Reader.Find(r.Context(), p, r.PathValue("id"))
	if e != nil {
		materialReadError(w, e)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

// Download documents this registered handler.
// @Summary 授权后下载材料
// @Description 校验当前班级及文件完整性，附件名称清理路径/控制字符；支持Range。
// @Tags materials
// @Produce plain
// @Security bearerAuth
// @Param id path string true "材料UUID" format(uuid)
// @Success 200 {file} file
// @Success 206 {file} file
// @Failure 401 {object} APIError
// @Failure 404 {object} APIError
// @Failure 500 {string} string "存储故障为HTML500"
// @x-response-media {"200":"text/plain; charset=utf-8","206":"text/plain; charset=utf-8","401":"application/json","404":"application/json","500":{"text/html":{"type":"string"},"application/json":{"$ref":"#/components/schemas/httpapi.APIError"}}}
// @Router /api/materials/{id}/file [get]
func (h *MaterialReads) Download(w http.ResponseWriter, r *http.Request) {
	p, _ := auth.FromContext(r.Context())
	v, e := h.Reader.Find(r.Context(), p, r.PathValue("id"))
	if e != nil {
		materialReadError(w, e)
		return
	}
	f, e := h.Store.OpenMaterial(v.StorageKey, v.Size)
	if e != nil {
		materialReadError(w, e)
		return
	}
	defer f.Close()
	// Keep Unicode metadata (including migrated Chinese names); strip path and
	// control characters before RFC 2231 encoding. Never use this as a disk path.
	name := strings.Map(func(ch rune) rune {
		if unicode.IsControl(ch) || ch == '/' || ch == '\\' {
			return '_'
		}
		return ch
	}, v.OriginalName)
	if name == "" {
		name = "material.txt"
	}
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": name}))
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "private, no-store")
	// ServeContent streams the already-open descriptor and supports HEAD/ranges.
	http.ServeContent(w, r, name, time.Time{}, f)
}
