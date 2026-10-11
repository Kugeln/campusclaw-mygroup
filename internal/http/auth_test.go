package httpapi

import (
	"bytes"
	"campusclaw/internal/auth"
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type identityStore struct {
	user                auth.CurrentUser
	found               bool
	queryErr, revokeErr error
	reads, writes       int
}

func (s *identityStore) FindCurrentUser(context.Context, int64) (auth.CurrentUser, bool, error) {
	s.reads++
	return s.user, s.found, s.queryErr
}
func (s *identityStore) Revoke(context.Context, int64) error {
	s.writes++
	if s.revokeErr != nil {
		return s.revokeErr
	}
	s.user.Version++
	return nil
}

type healthy struct{}

func (healthy) Check(context.Context) error { return nil }

func TestBearerIdentityAndRevocation(t *testing.T) {
	var logs bytes.Buffer
	old := log.Writer()
	log.SetOutput(&logs)
	defer log.SetOutput(old)
	now := time.Unix(1700000000, 0)
	tokens, _ := auth.NewTokens("unit-only-auth-signing-material-32-bytes", func() time.Time { return now })
	s := &identityStore{user: auth.CurrentUser{ID: 1, ClassID: 1, Version: 0, Role: "student", ClassName: "A"}, found: true}
	a := &auth.Identity{Store: s, Tokens: tokens}
	h := WithAuth(healthy{}, nil, a)
	token, _ := tokens.Issue(1, 0)
	request := func(method, path string, headers []string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(`{"role":"teacher","class_id":2}`))
		r.Header.Set("Cookie", "session=ignored")
		r.Header.Set("X-Role", "teacher")
		r.Header.Set("X-Class-Id", "2")
		for _, v := range headers {
			r.Header.Add("Authorization", v)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	for _, headers := range [][]string{nil, {"Basic x"}, {"Bearer"}, {"Bearer "}, {"Bearer x y"}, {"Bearer " + token + ", Bearer " + token}, {"Bearer " + token, "Bearer " + token}, {"Bearer malformed"}, {"Bearer\t" + token}} {
		w := request("GET", "/api/auth/me?access_token=ignored", headers)
		if w.Code != 401 || w.Header().Get("WWW-Authenticate") != "Bearer" || w.Body.String() != "{\"error\":\"Authentication required\"}\n" {
			t.Fatal("invalid header contract")
		}
	}
	if s.reads != 0 {
		t.Fatal("invalid credentials reached database")
	}
	for _, role := range []string{"student", "teacher"} {
		for _, class := range []int64{1, 2} {
			s.user.Role = role
			s.user.ClassID = class
			w := request("GET", "/api/auth/me?role=teacher&class_id=999", []string{"bEaReR  " + token})
			var body map[string]any
			_ = json.Unmarshal(w.Body.Bytes(), &body)
			if w.Code != 200 || len(body) != 4 || body["role"] != role || body["class_id"] != float64(class) || w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("Set-Cookie") != "" {
				t.Fatal("current identity or response mismatch")
			}
		}
	}
	called := false
	guard := Authenticate(a, TeacherRequired(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		p, ok := auth.FromContext(r.Context())
		if !ok || p.ClassID() != s.user.ClassID {
			t.Fatal("untrusted scope")
		}
		w.WriteHeader(201)
	})))
	for _, role := range []string{"student", "teacher"} {
		s.user.Role = role
		called = false
		r := httptest.NewRequest("POST", "/test-only?role=teacher&class_id=999", nil)
		r.Header.Set("Authorization", "Bearer "+token)
		r.Header.Set("X-Role", "teacher")
		w := httptest.NewRecorder()
		guard.ServeHTTP(w, r)
		if role == "student" && (w.Code != 403 || called || w.Body.String() != "{\"error\":\"Teacher role required\"}\n") {
			t.Fatal("student elevated")
		}
		if role == "teacher" && (w.Code != 201 || !called) {
			t.Fatal("teacher requires unexpected CSRF")
		}
	}
	s.queryErr = errors.New("private-query-detail")
	if w := request("GET", "/api/auth/me", []string{"Bearer " + token}); w.Code != 500 || strings.Contains(w.Body.String(), "private") {
		t.Fatal("query failure not safe")
	}
	if w := request("GET", "/health", []string{"Bearer malformed"}); w.Code != 200 {
		t.Fatal("health depends on identity")
	}
	s.queryErr = nil
	for _, mutate := range []func(){func() { s.found = false }, func() { s.user.Role = "disabled" }, func() { s.user.ClassID = 0 }, func() { s.user.Version = 1 }} {
		s.user = auth.CurrentUser{ID: 1, Role: "teacher", ClassID: 1}
		s.found = true
		mutate()
		if request("GET", "/api/auth/me", []string{"Bearer " + token}).Code != 401 {
			t.Fatal("invalid user accepted")
		}
	}
	s.user = auth.CurrentUser{ID: 1, Role: "teacher", ClassID: 1}
	s.found = true
	s.revokeErr = errors.New("private-commit-detail")
	w := request("POST", "/api/auth/logout", []string{"Bearer " + token})
	if w.Code != 500 || s.user.Version != 0 || strings.Contains(w.Body.String(), "private") {
		t.Fatal("failed logout succeeded")
	}
	s.revokeErr = nil
	second, _ := tokens.Issue(1, 0)
	w = request("POST", "/api/auth/logout", []string{"Bearer " + token})
	if w.Code != 204 || w.Body.Len() != 0 {
		t.Fatal("logout contract")
	}
	for _, v := range []string{token, second} {
		if request("GET", "/api/auth/me", []string{"Bearer " + v}).Code != 401 {
			t.Fatal("old version survived")
		}
	}
	fresh, _ := tokens.Issue(1, 1)
	if request("GET", "/api/auth/me", []string{"Bearer " + fresh}).Code != 200 {
		t.Fatal("new version rejected")
	}
	now = now.Add(28800 * time.Second)
	if request("GET", "/api/auth/me", []string{"Bearer " + fresh}).Code != 401 {
		t.Fatal("expired token accepted")
	}
	if logs.Len() != 0 {
		t.Fatal("authentication emitted unexpected logs")
	}
}
