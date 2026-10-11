package httpapi

import (
	"bytes"
	"campusclaw/internal/auth"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestMaterialPagesAndStaticBoundary(t *testing.T) {
	h := WithMaterials(healthy{}, nil, &auth.Identity{}, nil, &MaterialReads{})
	for _, path := range []string{"/login", "/materials"} {
		r := httptest.NewRequest("GET", path+"?role=teacher&class_id=999&name=%3Cscript%3E", nil)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 200 || w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("Set-Cookie") != "" || !strings.Contains(w.Body.String(), `lang="zh-CN"`) || strings.Contains(w.Body.String(), "<script>") || strings.Contains(w.Body.String(), "/knowledge") || strings.Contains(w.Body.String(), "999") {
			t.Fatal("public page shell contract")
		}
		if !strings.Contains(w.Header().Get("Content-Security-Policy"), "script-src 'self'") || w.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Fatal("browser safety headers")
		}
	}
	for _, name := range []string{"api.js", "login.js", "styles.css", "logo.png"} {
		// Byte-for-byte reuse prevents unnoticed reimplementation/drift at migration.
		original, e := os.ReadFile("../../app/static/" + name)
		if e != nil {
			t.Fatal(e)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", "/static/"+name, nil))
		if w.Code != 200 || !bytes.Equal(original, w.Body.Bytes()) {
			t.Fatal("reused asset differs: " + name)
		}
	}
	for _, path := range []string{"/static/", "/static/knowledge.js", "/knowledge", "/static/uploads/files/secret", "/uploads/files/secret", "/files/secret", "/static/.env", "/static/logo.png/extra"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 404 {
			t.Fatal("private/unrelated path exposed")
		}
	}
	for _, path := range []string{"/login", "/materials", "/static/logo.png"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("POST", path, nil))
		if w.Code != 405 {
			t.Fatal("unsupported method")
		}
	}
	script, _ := webAssets.ReadFile("web/static/materials.js")
	if strings.Contains(string(script), "innerHTML") || strings.Contains(string(script), "index_status") || !strings.Contains(string(script), "textContent = material.plain_text") || !strings.Contains(string(script), "URL.revokeObjectURL") {
		t.Fatal("material-only text and download flow")
	}
}
