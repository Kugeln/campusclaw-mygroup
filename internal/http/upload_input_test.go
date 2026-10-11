package httpapi

import (
	"bytes"
	"campusclaw/internal/auth"
	"campusclaw/internal/material"
	"campusclaw/internal/storage"
	"encoding/json"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestConfirmedUploadErrorCombinations(t *testing.T) {
	type part struct{ field, filename, text string }
	for _, tc := range []struct {
		name    string
		parts   []part
		status  int
		message string
	}{
		{"extension-identity", []part{{"file", "a.pdf", "x"}, {"role", "", "teacher"}}, 400, "Client identity fields are not accepted"},
		{"encoding-second-file", []part{{"file", "a.txt", "\xff"}, {"file", "b.txt", "x"}}, 400, "Exactly one file is required"},
		{"extension-only", []part{{"file", "a.pdf", "x"}}, 400, ""},
		{"encoding-only", []part{{"file", "a.txt", "\xff"}}, 400, ""},
		{"empty-second-file", []part{{"file", "a.txt", ""}, {"file", "b.txt", "x"}}, 400, ""},
		{"nul-second-file", []part{{"file", "a.txt", "x\x00"}, {"file", "b.txt", "x"}}, 400, ""},
		{"extension-second-file", []part{{"file", "a.pdf", "x"}, {"file", "b.txt", "x"}}, 400, ""},
		{"encoding-identity", []part{{"file", "a.txt", "\xff"}, {"role", "", "teacher"}}, 400, ""},
		{"oversize-identity", []part{{"file", "a.txt", strings.Repeat("x", 33)}, {"class_id", "", "2"}}, 413, ""},
		{"extension-large-form", []part{{"file", "a.pdf", "x"}, {"role", "", strings.Repeat("x", 500001)}}, 400, ""},
		{"encoding-third-part", []part{{"file", "a.txt", "\xff"}, {"file", "b.txt", "x"}, {"role", "", "teacher"}}, 400, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			store, err := storage.OpenPrivate(root)
			if err != nil {
				t.Fatal(err)
			}
			defer store.Close()
			tokens, _ := auth.NewTokens("unit-combination-secret-more-than-32bytes", time.Now)
			token, _ := tokens.Issue(1, 0)
			users := &identityStore{found: true, user: auth.CurrentUser{ID: 1, ClassID: 1, Role: "teacher"}}
			repo := &uploadRepo{}
			h := WithUpload(healthy{}, nil, &auth.Identity{Store: users, Tokens: tokens}, &UploadHandler{Store: store, Uploader: material.Uploader{Repository: repo}, Limit: 32})
			var body bytes.Buffer
			writer := multipart.NewWriter(&body)
			for _, p := range tc.parts {
				if p.filename == "" {
					writer.WriteField(p.field, p.text)
				} else {
					f, _ := writer.CreateFormFile(p.field, p.filename)
					f.Write([]byte(p.text))
				}
			}
			writer.Close()
			r := httptest.NewRequest("POST", "/api/materials", &body)
			r.Header.Set("Content-Type", writer.FormDataContentType())
			r.Header.Set("Authorization", "Bearer "+token)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != tc.status {
				t.Fatalf("status: got %d want %d", w.Code, tc.status)
			}
			if tc.message != "" {
				var got map[string]string
				if w.Header().Get("Content-Type") != "application/json" || json.Unmarshal(w.Body.Bytes(), &got) != nil || len(got) != 1 || got["error"] != tc.message {
					t.Fatal("JSON error contract mismatch")
				}
			} else if w.Header().Get("Content-Type") != "text/html; charset=utf-8" {
				t.Fatal("unconfirmed combination response changed")
			}
			if len(repo.records) != 0 || w.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("persistence/cache mismatch")
			}
			for _, dir := range []string{"tmp", "files"} {
				entries, e := os.ReadDir(filepath.Join(root, dir))
				if e != nil || len(entries) != 0 {
					t.Fatal("rejected upload left files")
				}
			}
		})
	}
}

func TestUploadTruncatedMultipartAndStorageFailure(t *testing.T) {
	for _, tc := range []struct {
		name                  string
		closeBody, closeStore bool
		text                  string
		status                int
		message               string
	}{
		{"truncated-content", false, false, "partial", 400, "Exactly one file is required"},
		{"truncated-empty", false, false, "", 400, "Exactly one file is required"},
		{"storage-failure", true, true, "valid", 500, "Material storage unavailable"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			store, err := storage.OpenPrivate(root)
			if err != nil {
				t.Fatal(err)
			}
			defer store.Close()
			if tc.closeStore {
				store.Close()
			}
			tokens, _ := auth.NewTokens("unit-input-only-secret-more-than-32bytes", time.Now)
			token, _ := tokens.Issue(1, 0)
			users := &identityStore{found: true, user: auth.CurrentUser{ID: 1, ClassID: 1, Role: "teacher"}}
			repo := &uploadRepo{}
			h := WithUpload(healthy{}, nil, &auth.Identity{Store: users, Tokens: tokens}, &UploadHandler{Store: store, Uploader: material.Uploader{Repository: repo}})
			var body bytes.Buffer
			writer := multipart.NewWriter(&body)
			part, _ := writer.CreateFormFile("file", "a.txt")
			part.Write([]byte(tc.text))
			if tc.closeBody {
				writer.Close()
			}
			request := httptest.NewRequest("POST", "/api/materials", &body)
			request.Header.Set("Content-Type", writer.FormDataContentType())
			request.Header.Set("Authorization", "Bearer "+token)
			response := httptest.NewRecorder()
			h.ServeHTTP(response, request)
			var got map[string]string
			if response.Code != tc.status || response.Header().Get("Content-Type") != "application/json" || json.Unmarshal(response.Body.Bytes(), &got) != nil || len(got) != 1 || got["error"] != tc.message {
				t.Fatalf("unexpected error contract: status=%d media=%s", response.Code, response.Header().Get("Content-Type"))
			}
			if response.Header().Get("Cache-Control") != "no-store" || len(repo.records) != 0 {
				t.Fatal("error persisted or cache contract changed")
			}
			for _, dir := range []string{"tmp", "files"} {
				entries, err := os.ReadDir(filepath.Join(root, dir))
				if err != nil || len(entries) != 0 {
					t.Fatal("error left files")
				}
			}
		})
	}
}
