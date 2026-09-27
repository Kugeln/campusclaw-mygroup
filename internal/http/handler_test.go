package httpapi

import (
	"campusclaw/internal/health"
	"campusclaw/internal/storage"
	"context"
	"errors"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

type database struct{ err error }

func (d database) PingContext(context.Context) error { return d.err }
func TestHealth(t *testing.T) {
	root := t.TempDir()
	for _, d := range []string{"tmp", "files"} {
		if err := os.Mkdir(filepath.Join(root, d), 0700); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		name  string
		dbErr error
		root  string
		code  int
		body  string
	}{{"healthy", nil, root, 200, `{"status":"ok"}`}, {"mysql unavailable", errors.New("private-credential"), root, 503, `{"status":"unhealthy"}`}, {"storage unavailable", nil, filepath.Join(root, "missing"), 503, `{"status":"unhealthy"}`}} {
		t.Run(tc.name, func(t *testing.T) {
			service := health.Service{DB: database{tc.dbErr}, Probe: func() error { return storage.Probe(tc.root) }}
			r := httptest.NewRequest("GET", "/health", nil)
			r.Header.Set("Cookie", "session=untrusted")
			w := httptest.NewRecorder()
			Handler(service).ServeHTTP(w, r)
			if w.Code != tc.code || w.Body.String() != tc.body || w.Header().Get("Set-Cookie") != "" {
				t.Fatalf("unexpected response: %d %s", w.Code, w.Body.String())
			}
		})
	}
	for _, d := range []string{root, filepath.Join(root, "tmp"), filepath.Join(root, "files")} {
		entries, _ := os.ReadDir(d)
		for _, e := range entries {
			if !e.IsDir() {
				t.Fatal("probe leaked file")
			}
		}
	}
	w := httptest.NewRecorder()
	Handler(health.Service{}).ServeHTTP(w, httptest.NewRequest("GET", "/api/materials", nil))
	if w.Code != 404 {
		t.Fatal("unexpected business route")
	}
}
