package httpapi

import (
	"campusclaw/internal/auth"
	"campusclaw/internal/material"
	"campusclaw/internal/rag"
	"campusclaw/internal/storage"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
)

type UploadHandler struct {
	Store    *storage.PrivateStore
	Uploader material.Uploader
	Limit    int64
	Indexer  *rag.Indexer
}

// Tag only malformed multipart input at the HTTP boundary. Storage write/open
// failures retain their independent 500 classification.
type multipartInput struct{ io.Reader }

func (r multipartInput) Read(p []byte) (int, error) {
	n, err := r.Reader.Read(p)
	if errors.Is(err, io.ErrUnexpectedEOF) {
		return n, storage.ErrUploadInput
	}
	return n, err
}

func uploadInputError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	title := http.StatusText(status)
	_, _ = fmt.Fprintf(w, "<!doctype html>\n<html lang=en>\n<title>%d %s</title>\n<h1>%s</h1>\n<p>%s</p>\n", status, title, title, html.EscapeString(message))
}

// Only the two confirmed two-part combinations override the original error.
// Lookahead is bounded and must reach a clean multipart end; other errors,
// extra parts and oversized/truncated input keep their previous response.
func confirmedUploadError(mr *multipart.Reader, first *multipart.Part, original error, limit int64) string {
	if !errors.Is(original, storage.ErrFileType) && !errors.Is(original, storage.ErrEncoding) {
		return ""
	}
	drain := func(r io.Reader, bound int64) bool {
		n, err := io.Copy(io.Discard, io.LimitReader(r, bound+1))
		return err == nil && n <= bound
	}
	if !drain(first, limit) {
		return ""
	}
	next, err := mr.NextRawPart()
	if err != nil {
		return ""
	}
	_, params, err := mime.ParseMediaType(next.Header.Get("Content-Disposition"))
	if err != nil {
		return ""
	}
	_, file := params["filename"]
	message := ""
	bound := limit
	if errors.Is(original, storage.ErrFileType) && !file {
		switch params["name"] {
		case "role", "class_id", "uploader_id":
			message = "Client identity fields are not accepted"
		}
		// Do not add a new precedence rule for oversized form fields.
		bound = 500000 // Flask/Werkzeug default max_form_memory_size.
	} else if errors.Is(original, storage.ErrEncoding) && file && params["name"] == "file" {
		message = "Exactly one file is required"
	}
	if message == "" || !drain(next, bound) {
		return ""
	}
	if _, err = mr.NextRawPart(); err != io.EOF {
		return ""
	}
	return message
}

// ServeHTTP documents this registered handler.
// @Summary 教师上传本班UTF-8 TXT/MD
// @Description 仅一个file，最大20MiB，不允许身份覆盖字段。先提交原文；RAG启用时返回index_status，索引失败仍201且原文保留。
// @Tags materials
// @Accept mpfd
// @Produce json
// @Security bearerAuth
// @Param file formData file true "UTF-8 TXT/MD文件（≤20MiB）"
// @Success 201 {object} UploadOutput
// @Failure 400 {object} APIError
// @Failure 401 {object} APIError
// @Failure 403 {object} APIError
// @Failure 413 {string} string "文件过大，HTML错误"
// @Failure 500 {string} string "存储故障，HTML错误"
// @x-response-media {"413":"text/html","500":{"text/html":{"type":"string"},"application/json":{"$ref":"#/components/schemas/httpapi.APIError"}}}
// @Router /api/materials [post]
func (h *UploadHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if len(r.URL.Query()) != 0 {
		authError(w, 400, "Client identity fields are not accepted")
		return
	}
	limit := h.Limit
	if limit == 0 {
		limit = storage.DefaultUploadLimit
	}
	// Bound multipart framing too, independently of the exact file byte limit.
	r.Body = http.MaxBytesReader(w, r.Body, limit+(1<<20))
	mr, err := r.MultipartReader()
	if err != nil {
		media, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if media == "application/x-www-form-urlencoded" {
			if r.ParseForm() == nil && len(r.PostForm) > 0 {
				authError(w, 400, "Client identity fields are not accepted")
				return
			}
		}
		authError(w, 400, "Exactly one file is required")
		return
	}
	part, err := mr.NextRawPart()
	if err != nil {
		authError(w, 400, "Exactly one file is required")
		return
	}
	_, params, err := mime.ParseMediaType(part.Header.Get("Content-Disposition"))
	name, filename := params["filename"]
	if err != nil || params["name"] != "file" || !filename {
		authError(w, 400, "Client identity fields are not accepted")
		return
	}
	file, err := h.Store.Prepare(name, multipartInput{part}, limit)
	if err != nil {
		switch {
		case errors.Is(err, storage.ErrLimit):
			uploadInputError(w, 413, err.Error())
		case errors.Is(err, storage.ErrUploadInput):
			authError(w, 400, "Exactly one file is required")
		case errors.Is(err, storage.ErrText), errors.Is(err, storage.ErrFileType):
			if message := confirmedUploadError(mr, part, err, limit); message != "" {
				authError(w, 400, message)
				return
			}
			uploadInputError(w, 400, err.Error())
		default:
			authError(w, 500, "Material storage unavailable")
		}
		return
	}
	defer file.Close()
	next, err := mr.NextRawPart()
	if err != io.EOF {
		if err != nil {
			uploadInputError(w, 400, "Invalid multipart body")
			return
		}
		_, params, _ := mime.ParseMediaType(next.Header.Get("Content-Disposition"))
		if _, exists := params["filename"]; !exists || params["name"] != "file" {
			authError(w, 400, "Client identity fields are not accepted")
		} else {
			authError(w, 400, "Exactly one file is required")
		}
		return
	}
	p, _ := auth.FromContext(r.Context())
	id, err := h.Uploader.Save(r.Context(), p, file)
	if err != nil {
		if errors.Is(err, material.ErrForbidden) {
			authError(w, 403, "Teacher role required")
		} else {
			authError(w, 500, "Material storage unavailable")
		}
		return
	}
	response := map[string]string{"material_id": id}
	if h.Indexer != nil {
		status, e := h.Indexer.Run(r.Context(), rag.Scope{UserID: p.UserID(), ClassID: p.ClassID(), Role: p.Role()}, id, nil, true)
		if e != nil {
			status = "failed"
		}
		response["index_status"] = status
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(response)
}
