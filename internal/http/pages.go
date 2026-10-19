package httpapi

import (
	"bytes"
	"embed"
	"html/template"
	"net/http"
	"strings"
	"time"
)

// These are explicit build-time assets, never a filesystem root supplied by an
// environment variable. No upload directory or arbitrary static file is served.
//
//go:embed web/templates/*.html web/static/api.js web/static/login.js web/static/materials.js web/static/styles.css web/static/logo.png web/static/tutor.js web/static/tutor.css web/static/sse.js web/static/knowledge.js web/static/mcp.js
var webAssets embed.FS

var pages = template.Must(template.ParseFS(webAssets, "web/templates/*.html"))

func registerPages(mux *http.ServeMux) {
	for _, name := range []string{"login", "materials", "tutor", "knowledge"} {
		mux.HandleFunc("GET /"+name, func(w http.ResponseWriter, r *http.Request) {
			var b bytes.Buffer
			if pages.ExecuteTemplate(&b, name+".html", nil) != nil {
				http.Error(w, "Page unavailable", 500)
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("Cache-Control", "no-store")
			http.ServeContent(w, r, name+".html", time.Time{}, bytes.NewReader(b.Bytes()))
		})
	}
	for name, kind := range map[string]string{"api.js": "text/javascript; charset=utf-8", "login.js": "text/javascript; charset=utf-8", "materials.js": "text/javascript; charset=utf-8", "styles.css": "text/css; charset=utf-8", "logo.png": "image/png", "tutor.js": "text/javascript; charset=utf-8", "tutor.css": "text/css; charset=utf-8", "sse.js": "text/javascript; charset=utf-8", "knowledge.js": "text/javascript; charset=utf-8", "mcp.js": "text/javascript; charset=utf-8"} {
		data, err := webAssets.ReadFile("web/static/" + name)
		if err != nil {
			panic("missing embedded material page asset")
		}
		mux.HandleFunc("GET /static/"+name, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", kind)
			http.ServeContent(w, r, name, time.Time{}, bytes.NewReader(data))
		})
	}
}

func browserHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Cache-Control", "no-store")
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; connect-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'")
		next.ServeHTTP(w, r)
	})
}
