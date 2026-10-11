package httpapi

import (
	"campusclaw/internal/auth"
	"campusclaw/internal/material"
	"campusclaw/internal/storage"
	"context"
	"encoding/json"
	"mime"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type readRepo struct {
	detail material.Detail
	fail   bool
	seen   int64
}

func (r *readRepo) List(_ context.Context, class int64) ([]material.Item, error) {
	r.seen = class
	if r.fail {
		return nil, material.ErrUnavailable
	}
	if class != 2 {
		return []material.Item{}, nil
	}
	return []material.Item{r.detail.Item}, nil
}
func (r *readRepo) Find(_ context.Context, class int64, id string) (material.Detail, error) {
	r.seen = class
	if r.fail {
		return material.Detail{}, material.ErrUnavailable
	}
	if class != 2 || id != r.detail.ID {
		return material.Detail{}, material.ErrNotFound
	}
	return r.detail, nil
}
func TestMaterialReadHTTP(t *testing.T) {
	root := t.TempDir()
	files, e := storage.OpenPrivate(root)
	if e != nil {
		t.Fatal(e)
	}
	defer files.Close()
	raw := "\ufeff# 中文\n<script>raw</script>"
	u, e := files.Prepare("a.txt", strings.NewReader(raw), 100)
	if e != nil || u.Publish() != nil {
		t.Fatal("fixture")
	}
	repo := &readRepo{detail: material.Detail{Item: material.Item{ID: u.Info().ID, OriginalName: "中文材料.md", Size: int64(len(raw)), CreatedAt: "2026-10-09T00:00:00.000Z", Uploader: "teacher_a"}, PlainText: strings.TrimPrefix(raw, "\ufeff"), StorageKey: u.Info().ID}}
	tokens, _ := auth.NewTokens("read-test-secret-at-least-32bytes-only", time.Now)
	token, _ := tokens.Issue(1, 0)
	users := &identityStore{found: true, user: auth.CurrentUser{ID: 1, ClassID: 2, Role: "student"}}
	h := WithMaterials(healthy{}, nil, &auth.Identity{Store: users, Tokens: tokens}, nil, &MaterialReads{Reader: material.Reader{Repository: repo}, Store: files})
	get := func(path string, anonymous bool) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", path, nil)
		if !anonymous {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		r.Header.Set("X-Class-Id", "2")
		r.Header.Set("Cookie", "role=teacher; class_id=2")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	base := "/api/materials/" + u.Info().ID
	for _, role := range []string{"teacher", "student"} {
		users.user.Role = role
		w := get("/api/materials", false)
		var listing struct {
			Items []map[string]any `json:"items"`
		}
		json.Unmarshal(w.Body.Bytes(), &listing)
		if w.Code != 200 || len(listing.Items) != 1 || len(listing.Items[0]) != 5 || repo.seen != 2 {
			t.Fatal("list contract")
		}
		w = get(base, false)
		var detail map[string]any
		json.Unmarshal(w.Body.Bytes(), &detail)
		if w.Code != 200 || len(detail) != 6 || detail["plain_text"] != repo.detail.PlainText || detail["storage_key"] != nil {
			t.Fatal("detail contract")
		}
		w = get(base+"/file", false)
		disposition, params, e := mime.ParseMediaType(w.Header().Get("Content-Disposition"))
		if w.Code != 200 || w.Body.String() != raw || e != nil || disposition != "attachment" || params["filename"] != "中文材料.md" || w.Header().Get("X-Content-Type-Options") != "nosniff" || w.Header().Get("Content-Type") != "text/plain; charset=utf-8" {
			t.Fatal("attachment contract")
		}
	}
	if get("/api/materials?class_id=2", false).Code != 400 {
		t.Fatal("query scope allowed")
	}
	for _, path := range []string{"/api/materials", base, base + "/file"} {
		if get(path, true).Code != 401 {
			t.Fatal("anonymous read")
		}
	}
	users.user.ClassID = 3
	if w := get("/api/materials", false); w.Code != 200 || strings.TrimSpace(w.Body.String()) != `{"items":[]}` {
		t.Fatal("empty list")
	}
	for _, suffix := range []string{"", "/file"} {
		foreign, missing := get(base+suffix+"?class_id=2", false), get("/api/materials/absent"+suffix, false)
		if foreign.Code != 404 || missing.Code != 404 || foreign.Body.String() != missing.Body.String() {
			t.Fatal("scope leak")
		}
	}
	users.user.ClassID = 2
	repo.fail = true
	for _, path := range []string{"/api/materials", base, base + "/file"} {
		w := get(path, false)
		if w.Code != 500 || !strings.HasPrefix(w.Header().Get("Content-Type"), "text/html") {
			t.Fatal("read failure contract")
		}
	}
	repo.fail = false
	for _, key := range []string{"../secret", "00000000-0000-0000-0000-000000000000"} {
		repo.detail.StorageKey = key
		if get(base+"/file", false).Code != 500 {
			t.Fatal("invalid storage access")
		}
	}
	repo.detail.StorageKey = u.Info().ID
	os.Remove(filepath.Join(root, "files", u.Info().ID))
	if get(base+"/file", false).Code != 500 {
		t.Fatal("missing file success")
	}
	for _, path := range []string{"/files/", "/uploads/files/", "/static/uploads/files/"} {
		if get(path+u.Info().ID, false).Code != 404 {
			t.Fatal("private file exposed")
		}
	}
}
