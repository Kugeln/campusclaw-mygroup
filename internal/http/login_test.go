package httpapi

import (
	"bytes"
	"campusclaw/internal/auth"
	"context"
	"encoding/json"
	"errors"
	"log"
	"mime/multipart"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type loginUsers struct{}

func (loginUsers) FindLoginUser(_ context.Context, name string) (auth.LoginUser, bool, error) {
	if name == "storage-error" {
		return auth.LoginUser{}, false, errors.New("sensitive-driver-detail")
	}
	role := "teacher"
	if name == "student" {
		role = "student"
	}
	return auth.LoginUser{ID: 1, ClassID: 1, Version: 7, Role: role, PasswordHash: "pbkdf2:sha256:1000000$legacySalt123$549739d9241dd3cf2e5167ed3a129d9d1981d7204e8006090b57217ebaab46ab"}, name == "teacher" || name == "student", nil
}

func TestLoginHTTPContract(t *testing.T) {
	var logs bytes.Buffer
	previous := log.Writer()
	log.SetOutput(&logs)
	defer log.SetOutput(previous)
	tokens, _ := auth.NewTokens("http-test-only-signing-material-32-bytes", func() time.Time { return time.Unix(1700000000, 0) })
	h := WithLogin(nil, &auth.Login{Users: loginUsers{}, Tokens: tokens})
	var multipartBody bytes.Buffer
	m := multipart.NewWriter(&multipartBody)
	_ = m.WriteField("username", "teacher")
	_ = m.WriteField("password", "fixture-password")
	_ = m.Close()
	for _, tc := range []struct {
		name, media, body string
		code              int
	}{
		{"teacher-json", "application/json", `{"username":"teacher","password":"fixture-password","role":"student"}`, 200},
		{"student-form", "application/x-www-form-urlencoded", "username=student&password=fixture-password", 200},
		{"browser-multipart", m.FormDataContentType(), multipartBody.String(), 200},
		{"wrong-password", "application/json", `{"username":"teacher","password":"wrong"}`, 401},
		{"unknown", "application/json", `{"username":"unknown","password":"fixture-password"}`, 401},
		{"missing", "application/json", `{}`, 401}, {"bad-types", "application/json", `{"username":true,"password":123}`, 401},
		{"bad-json", "application/json", `{`, 401}, {"array-json", "application/json", `[]`, 401}, {"trailing-json", "application/json", `{} {}`, 401},
		{"query-only", "application/x-www-form-urlencoded", "", 401},
		{"storage", "application/json", `{"username":"storage-error","password":"fixture-password"}`, 500},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest("POST", "/api/auth/login?username=teacher&password=fixture-password", strings.NewReader(tc.body))
			r.Header.Set("Content-Type", tc.media)
			r.Header.Set("Cookie", "session=ignored")
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != tc.code || w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("Set-Cookie") != "" {
				t.Fatal("login status/cache/cookie mismatch")
			}
			if tc.code == 200 {
				var response auth.LoginResponse
				if json.Unmarshal(w.Body.Bytes(), &response) != nil || response.TokenType != "Bearer" || response.ExpiresIn != 28800 || response.Redirect != "/materials" {
					t.Fatal("login response mismatch")
				}
				verified, err := tokens.Verify(response.AccessToken)
				if err != nil || verified.UserID != 1 || verified.Version != 7 {
					t.Fatal("login token mismatch")
				}
				var fields map[string]any
				_ = json.Unmarshal(w.Body.Bytes(), &fields)
				if len(fields) != 4 {
					t.Fatal("unexpected response fields")
				}
			} else {
				want := "Invalid credentials"
				if tc.code == 500 {
					want = "Authentication storage unavailable"
				}
				var response map[string]string
				_ = json.Unmarshal(w.Body.Bytes(), &response)
				if len(response) != 1 || response["error"] != want {
					t.Fatal("unsafe or incompatible error response")
				}
			}
		})
	}
	if logs.Len() != 0 {
		t.Fatal("authentication emitted unexpected logs")
	}
	for _, path := range []string{"/api/auth/logout", "/api/auth/me", "/api/materials"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("POST", path, nil))
		if w.Code != 404 {
			t.Fatal("out-of-scope route implemented")
		}
	}
}
