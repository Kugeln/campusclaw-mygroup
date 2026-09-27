package config

import (
	"strings"
	"testing"
)

func TestConfiguration(t *testing.T) {
	values := map[string]string{"DB_HOST": "mysql", "DB_NAME": "campusclaw", "DB_USER": "app", "DB_PASSWORD": "test-only-secret", "UPLOAD_ROOT": "/uploads"}
	get := func(k string) string { return values[k] }
	if _, err := Load(get); err != nil {
		t.Fatal(err)
	}
	for key, value := range values {
		delete(values, key)
		_, err := Load(get)
		if err == nil || !strings.Contains(err.Error(), key) || strings.Contains(err.Error(), "test-only-secret") {
			t.Fatalf("missing %s not safely rejected", key)
		}
		values[key] = value
	}
	for _, port := range []string{"0", "65536", "secret-value"} {
		values["DB_PORT"] = port
		if _, err := Load(get); err == nil || strings.Contains(err.Error(), port) {
			t.Fatal("invalid port not safely rejected")
		}
	}
}
