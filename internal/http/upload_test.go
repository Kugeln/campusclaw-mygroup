package httpapi

import (
	"bytes"
	"campusclaw/internal/auth"
	"campusclaw/internal/material"
	"campusclaw/internal/storage"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type uploadRepo struct{ records []material.Record }
type uploadTx struct {
	repo   *uploadRepo
	record material.Record
}

func (r *uploadRepo) Begin(context.Context) (material.Transaction, error) {
	return &uploadTx{repo: r}, nil
}
func (r *uploadRepo) Resolve(context.Context, material.Record) (material.Outcome, error) {
	return material.Unknown, nil
}
func (t *uploadTx) Insert(r material.Record) error { t.record = r; return nil }
func (t *uploadTx) Commit() error                  { t.repo.records = append(t.repo.records, t.record); return nil }
func (t *uploadTx) Rollback() error                { return nil }

func TestUploadHTTPContract(t *testing.T) {
	root := t.TempDir()
	files, e := storage.OpenPrivate(root)
	if e != nil {
		t.Fatal(e)
	}
	defer files.Close()
	tokens, _ := auth.NewTokens("unit-upload-only-secret-more-than-32bytes", time.Now)
	token, _ := tokens.Issue(1, 0)
	users := &identityStore{found: true, user: auth.CurrentUser{ID: 1, ClassID: 2, Role: "teacher"}}
	repo := &uploadRepo{}
	h := WithUpload(healthy{}, nil, &auth.Identity{Store: users, Tokens: tokens}, &UploadHandler{Store: files, Uploader: material.Uploader{Repository: repo}, Limit: 8})
	for _, tc := range []struct {
		name, filename, body, field, extra, query, role string
		count                                           int
		anonymous                                       bool
		code                                            int
		message                                         string
	}{
		{name: "ok", filename: "../../same.MD", body: "\xef\xbb\xbfhello", count: 1, code: 201},
		{name: "exact-limit", filename: "same.txt", body: "12345678", count: 1, code: 201},
		{name: "over-limit", filename: "same.txt", body: "123456789", count: 1, code: 413},
		{name: "bad-text", filename: "same.txt", body: "a\x00b", count: 1, code: 400},
		{name: "empty", filename: "same.txt", count: 1, code: 400},
		{name: "bad-extension", filename: "same.pdf", body: "hello", count: 1, code: 400},
		{name: "no-file", count: 0, code: 400, message: "Exactly one file is required"},
		{name: "two-files", filename: "same.txt", body: "hello", count: 2, code: 400, message: "Exactly one file is required"},
		{name: "extra-file-field", filename: "same.txt", body: "hello", field: "other", count: 1, code: 400, message: "Client identity fields are not accepted"},
		{name: "identity-before-file", filename: "same.txt", body: "hello", extra: "role", count: 1, code: 400, message: "Client identity fields are not accepted"},
		{name: "query-identity", filename: "same.txt", body: "hello", query: "?class_id=1", count: 1, code: 400, message: "Client identity fields are not accepted"},
		{name: "student-forged", filename: "same.txt", body: "hello", extra: "role", role: "student", count: 1, code: 403, message: "Teacher role required"},
		{name: "anonymous", filename: "same.txt", body: "hello", count: 1, anonymous: true, code: 401, message: "Authentication required"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			users.user.Role = "teacher"
			if tc.role != "" {
				users.user.Role = tc.role
			}
			var body bytes.Buffer
			mw := multipart.NewWriter(&body)
			if tc.extra != "" {
				mw.WriteField(tc.extra, "teacher")
			}
			field := tc.field
			if field == "" {
				field = "file"
			}
			for i := 0; i < tc.count; i++ {
				part, _ := mw.CreateFormFile(field, tc.filename)
				part.Write([]byte(tc.body))
			}
			mw.Close()
			r := httptest.NewRequest("POST", "/api/materials"+tc.query, &body)
			r.Header.Set("Content-Type", mw.FormDataContentType())
			r.Header.Set("X-Class-Id", "999")
			r.Header.Set("X-Role", "teacher")
			r.Header.Set("Cookie", "class_id=999")
			if !tc.anonymous {
				r.Header.Set("Authorization", "Bearer "+token)
			}
			before := len(repo.records)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != tc.code || w.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("unexpected status/cache: %d", w.Code)
			}
			if tc.code == 201 {
				var response map[string]string
				if json.Unmarshal(w.Body.Bytes(), &response) != nil || len(response) != 1 || response["material_id"] == "" {
					t.Fatal("201 schema changed")
				}
				m := repo.records[len(repo.records)-1]
				if m.ClassID != 2 || m.UploaderID != 1 {
					t.Fatal("client identity trusted")
				}
				raw, e := os.ReadFile(filepath.Join(root, "files", m.ID))
				if e != nil || string(raw) != tc.body {
					t.Fatal("201 before file persistence")
				}
			} else if len(repo.records) != before {
				t.Fatal("rejected request wrote database")
			}
			if tc.message != "" {
				var v map[string]string
				if json.Unmarshal(w.Body.Bytes(), &v) != nil || v["error"] != tc.message {
					t.Fatal("Flask JSON error contract changed")
				}
			} else if tc.code != 201 && !strings.HasPrefix(w.Header().Get("Content-Type"), "text/html") {
				t.Fatal("Flask validation media type changed")
			}
			tmp, _ := os.ReadDir(filepath.Join(root, "tmp"))
			if len(tmp) != 0 {
				t.Fatal("staging residue")
			}
		})
	}
	for _, path := range []string{"/uploads/files/private", "/static/uploads/files/private", "/files/private", "/api/materials/id", "/api/materials/id/file"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 404 {
			t.Fatal("private/read route exposed")
		}
	}
}
