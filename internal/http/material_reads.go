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
