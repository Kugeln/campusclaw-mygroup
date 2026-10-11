package httpapi

import (
	"campusclaw/internal/auth"
	"context"
	"net/http"
	"time"
)

type Checker interface{ Check(context.Context) error }

func Handler(check Checker) http.Handler {
	return WithLogin(check, nil)
}

// WithLogin adds only login; protected routes and logout are separate tasks.
func WithLogin(check Checker, login *auth.Login) http.Handler {
	return WithAuth(check, login, nil)
}

func WithAuth(check Checker, login *auth.Login, identity *auth.Identity) http.Handler {
	return WithUpload(check, login, identity, nil)
}

func WithUpload(check Checker, login *auth.Login, identity *auth.Identity, upload *UploadHandler) http.Handler {
	return WithMaterials(check, login, identity, upload, nil)
}

func WithMaterials(check Checker, login *auth.Login, identity *auth.Identity, upload *UploadHandler, reads *MaterialReads) http.Handler {
	mux := http.NewServeMux()
	if identity != nil && reads != nil {
		registerPages(mux)
		mux.Handle("GET /api/materials", Authenticate(identity, http.HandlerFunc(reads.List)))
		mux.Handle("GET /api/materials/{id}", Authenticate(identity, http.HandlerFunc(reads.Detail)))
		mux.Handle("GET /api/materials/{id}/file", Authenticate(identity, http.HandlerFunc(reads.Download)))
	}
	if identity != nil && upload != nil {
		mux.Handle("POST /api/materials", Authenticate(identity, TeacherRequired(upload)))
	}
	if identity != nil {
		mux.Handle("GET /api/auth/me", Authenticate(identity, http.HandlerFunc(me)))
		mux.Handle("POST /api/auth/logout", Authenticate(identity, logout(identity)))
	}
	if login != nil {
		mux.HandleFunc("POST /api/auth/login", loginHandler(login))
	}
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 4*time.Second)
		defer cancel()
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		if check.Check(ctx) != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"status":"unhealthy"}`))
			return
		}
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	return browserHeaders(mux)
}
