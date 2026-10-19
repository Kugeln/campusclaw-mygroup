package httpapi

import (
	"campusclaw/internal/auth"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"
)

// loginHandler documents this registered handler.
// @Summary 登录并获取8小时Bearer JWT
// @Description 接受JSON或表单username/password。无Cookie，角色/班级由数据库确定；退出撤销旧Token。
// @Tags auth
// @Accept json,x-www-form-urlencoded,mpfd
// @Produce json
// @Param body body LoginInput true "登录凭据（JSON或同名表单字段）"
// @Success 200 {object} auth.LoginResponse
// @Failure 401 {object} APIError
// @Failure 500 {object} APIError
// @Router /api/auth/login [post]
func loginHandler(login *auth.Login) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "application/json")
		username, password, valid := credentials(r)
		if !valid {
			w.WriteHeader(401)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "Invalid credentials"})
			return
		}
		result, err := login.Authenticate(r.Context(), username, password)
		if err != nil {
			code, message := 500, "Authentication storage unavailable"
			if errors.Is(err, auth.ErrInvalidCredentials) {
				code, message = 401, "Invalid credentials"
			}
			w.WriteHeader(code)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
			return
		}
		_ = json.NewEncoder(w).Encode(result)
	}
}

func credentials(r *http.Request) (string, string, bool) {
	media, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if media == "application/json" || strings.HasSuffix(media, "+json") {
		var data map[string]any
		d := json.NewDecoder(r.Body)
		if d.Decode(&data) != nil {
			return "", "", false
		}
		var extra any
		if d.Decode(&extra) != io.EOF {
			return "", "", false
		}
		u, ok1 := data["username"].(string)
		p, ok2 := data["password"].(string)
		return u, p, ok1 && ok2
	}
	if media == "multipart/form-data" {
		if r.ParseMultipartForm(1<<20) != nil {
			return "", "", false
		}
		defer r.MultipartForm.RemoveAll()
	} else if r.ParseForm() != nil {
		return "", "", false
	}
	// PostForm excludes query parameters, matching Flask request.form.
	u, p := r.PostForm["username"], r.PostForm["password"]
	if len(u) == 0 || len(p) == 0 {
		return "", "", false
	}
	return u[0], p[0], true
}
