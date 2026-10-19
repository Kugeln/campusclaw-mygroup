package httpapi

import (
	"campusclaw/internal/auth"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
)

var bearerHeader = regexp.MustCompile(`(?i)^Bearer +([^\s,]+)$`)

func authError(w http.ResponseWriter, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	if code == 401 {
		w.Header().Set("WWW-Authenticate", "Bearer")
	}
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
func Authenticate(identity *auth.Identity, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		values := r.Header.Values("Authorization")
		var match []string
		if len(values) == 1 {
			match = bearerHeader.FindStringSubmatch(values[0])
		}
		if match == nil {
			authError(w, 401, "Authentication required")
			return
		}
		p, err := identity.Authenticate(r.Context(), match[1])
		if err != nil {
			if errors.Is(err, auth.ErrStorageUnavailable) {
				authError(w, 500, "Authentication storage unavailable")
			} else {
				authError(w, 401, "Authentication required")
			}
			return
		}
		next.ServeHTTP(w, r.WithContext(auth.WithPrincipal(r.Context(), p)))
	})
}
func TeacherRequired(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, ok := auth.FromContext(r.Context())
		if !ok {
			authError(w, 401, "Authentication required")
			return
		}
		if p.Role() != "teacher" {
			authError(w, 403, "Teacher role required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// me documents this registered handler.
// @Summary 当前可信身份
// @Tags auth
// @Produce json
// @Security bearerAuth
// @Success 200 {object} IdentityOutput
// @Failure 401 {object} APIError
// @Failure 500 {object} APIError
// @Router /api/auth/me [get]
func me(w http.ResponseWriter, r *http.Request) {
	p, _ := auth.FromContext(r.Context())
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(struct {
		UserID    int64  `json:"user_id"`
		Role      string `json:"role"`
		ClassID   int64  `json:"class_id"`
		ClassName string `json:"class_name"`
	}{p.UserID(), p.Role(), p.ClassID(), p.ClassName()})
}

// logout documents this registered handler.
// @Summary 退出并撤销账号旧Token
// @Tags auth
// @Security bearerAuth
// @Success 204 "No Content"
// @Failure 401 {object} APIError
// @Failure 500 {object} APIError
// @Router /api/auth/logout [post]
func logout(identity *auth.Identity) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, _ := auth.FromContext(r.Context())
		if identity.Logout(r.Context(), p) != nil {
			authError(w, 500, "Authentication storage unavailable")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
