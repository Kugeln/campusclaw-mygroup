package security_test

import (
	"campusclaw/internal/provision"
	"campusclaw/internal/security"
	"strings"
	"testing"
)

func TestPasswordCompatibility(t *testing.T) {
	// Independent Python hashlib/Werkzeug-format fixture; no real credentials.
	const hash = "pbkdf2:sha256:1000000$legacySalt123$549739d9241dd3cf2e5167ed3a129d9d1981d7204e8006090b57217ebaab46ab"
	if !security.VerifyPassword(hash, "fixture-password") || security.VerifyPassword(hash, "wrong") {
		t.Fatal("legacy password vector mismatch")
	}
	for _, bad := range []string{"", "scrypt$salt$value", strings.Replace(hash, "1000000", "1", 1), strings.Replace(hash, "legacySalt123", "", 1), hash + "$extra", hash[:len(hash)-1], strings.Replace(hash, "549739", "zzzzzz", 1)} {
		if security.VerifyPassword(bad, "fixture-password") {
			t.Fatal("invalid password format accepted")
		}
	}
	a, err := provision.PasswordHash("test-only-password")
	if err != nil {
		t.Fatal("password creation failed")
	}
	b, err := provision.PasswordHash("test-only-password")
	if err != nil || a == b || strings.Split(a, "$")[1] == strings.Split(b, "$")[1] {
		t.Fatal("salt not randomized")
	}
	if !security.VerifyPassword(a, "test-only-password") || !security.VerifyPassword(b, "test-only-password") {
		t.Fatal("Phase 2 hash incompatible")
	}
}
