package httpapi

import (
	"bytes"
	"embed"
	"net/http"
	"time"
)

//go:embed web/swagger/* web/swagger/vendor/*
var documentation embed.FS

func registerDocumentation(mux *http.ServeMux) {
	mux.HandleFunc("GET /openapi.json", openAPIDocument)
	mux.HandleFunc("GET /swagger/index.html", swaggerDocument)
	mux.HandleFunc("GET /swagger/{$}", swaggerDocument)
	assets := map[string]struct{ file, kind string }{
		"/swagger/init.js":                     {"init.js", "text/javascript; charset=utf-8"},
		"/swagger/vendor/swagger-ui-bundle.js": {"vendor/swagger-ui-bundle.js", "text/javascript; charset=utf-8"},
		"/swagger/vendor/swagger-ui.css":       {"vendor/swagger-ui.css", "text/css; charset=utf-8"},
	}
	for path, asset := range assets {
		data, e := documentation.ReadFile("web/swagger/" + asset.file)
		if e != nil {
			panic("missing embedded documentation asset")
		}
		// {$} prevents the slash alias from becoming an arbitrary file server.
		pattern := path
		if path == "/swagger/" {
			pattern += "{$}"
		}
		mux.HandleFunc("GET "+pattern, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", asset.kind)
			w.Header().Set("Cache-Control", "no-store")
			w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; connect-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'")
			http.ServeContent(w, r, asset.file, time.Time{}, bytes.NewReader(data))
		})
	}
	for name, kind := range map[string]string{"sse.js": "text/javascript; charset=utf-8", "tutor.css": "text/css; charset=utf-8"} {
		data, e := webAssets.ReadFile("web/static/" + name)
		if e != nil {
			panic("missing documentation support asset")
		}
		mux.HandleFunc("GET /swagger/"+name, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", kind)
			http.ServeContent(w, r, name, time.Time{}, bytes.NewReader(data))
		})
	}
}

// openAPIDocument serves the reproducible annotation artifact.
// @Summary 注解生成的OpenAPI契约
// @Tags documentation
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /openapi.json [get]
func openAPIDocument(w http.ResponseWriter, r *http.Request) {
	serveDocument(w, r, "openapi.json", "application/json")
}

// swaggerDocument serves the anonymous Swagger UI and streaming debugger.
// @Summary Swagger UI与实时SSE调试
// @Tags documentation
// @Produce html
// @Success 200 {string} string
// @Router /swagger/index.html [get]
// @Router /swagger/ [get]
func swaggerDocument(w http.ResponseWriter, r *http.Request) {
	serveDocument(w, r, "index.html", "text/html; charset=utf-8")
}

func serveDocument(w http.ResponseWriter, r *http.Request, file, kind string) {
	data, e := documentation.ReadFile("web/swagger/" + file)
	if e != nil {
		panic("missing embedded documentation asset")
	}
	w.Header().Set("Content-Type", kind)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; connect-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'")
	http.ServeContent(w, r, file, time.Time{}, bytes.NewReader(data))
}
