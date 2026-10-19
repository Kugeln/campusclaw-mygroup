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
	return WithRAG(check, login, identity, upload, reads, nil)
}
func WithRAG(check Checker, login *auth.Login, identity *auth.Identity, upload *UploadHandler, reads *MaterialReads, rag *RAGHandler) http.Handler {
	return WithTutor(check, login, identity, upload, reads, rag, nil)
}
func WithTutor(check Checker, login *auth.Login, identity *auth.Identity, upload *UploadHandler, reads *MaterialReads, rag *RAGHandler, tutor *TutorHandler) http.Handler {
	return WithConversations(check, login, identity, upload, reads, rag, tutor, nil)
}
func WithConversations(check Checker, login *auth.Login, identity *auth.Identity, upload *UploadHandler, reads *MaterialReads, rag *RAGHandler, tutor *TutorHandler, conversations *ConversationHandler) http.Handler {
	mux := http.NewServeMux()
	registerDocumentation(mux)
	if identity != nil && conversations != nil {
		mux.Handle("GET /api/conversations", Authenticate(identity, studentRequired(http.HandlerFunc(conversations.List))))
		mux.Handle("POST /api/conversations", Authenticate(identity, studentRequired(http.HandlerFunc(conversations.Create))))
		mux.Handle("GET /api/conversations/{id}", Authenticate(identity, studentRequired(http.HandlerFunc(conversations.Get))))
		mux.Handle("POST /api/conversations/{id}/messages", Authenticate(identity, studentRequired(http.HandlerFunc(conversations.Messages))))
	}
	if identity != nil && tutor != nil {
		mux.Handle("GET /api/agents", Authenticate(identity, http.HandlerFunc(tutor.Agents)))
		mux.Handle("POST /api/agents", Authenticate(identity, TeacherRequired(http.HandlerFunc(tutor.CreateAgent))))
		mux.Handle("GET /api/agents/{id}", Authenticate(identity, http.HandlerFunc(tutor.Agent)))
		mux.Handle("PUT /api/agents/{id}", Authenticate(identity, TeacherRequired(http.HandlerFunc(tutor.UpdateAgent))))
		mux.Handle("GET /api/skills", Authenticate(identity, http.HandlerFunc(tutor.Skills)))
		mux.Handle("GET /api/skills/{id}", Authenticate(identity, http.HandlerFunc(tutor.Skill)))
		mux.Handle("POST /api/skills", Authenticate(identity, TeacherRequired(http.HandlerFunc(tutor.CreateSkill))))
		mux.Handle("PUT /api/skills/{id}", Authenticate(identity, TeacherRequired(http.HandlerFunc(tutor.UpdateSkill))))
		mux.Handle("DELETE /api/skills/{id}", Authenticate(identity, TeacherRequired(http.HandlerFunc(tutor.DeleteSkill))))
	}
	if identity != nil && rag != nil {
		mux.Handle("GET /api/search", Authenticate(identity, http.HandlerFunc(rag.Search)))
		mux.Handle("POST /api/ask", Authenticate(identity, http.HandlerFunc(rag.Ask)))
		mux.Handle("POST /api/materials/{id}/reindex", Authenticate(identity, TeacherRequired(http.HandlerFunc(rag.Reindex))))
	}
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
