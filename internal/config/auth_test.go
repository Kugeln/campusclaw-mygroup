package config

import (
	"strings"
	"testing"
)

func TestSigningSecret(t *testing.T) {
	for _, secret := range []string{"", "short-test-value", strings.Repeat(" ", 32), strings.Repeat("a", 31)} {
		value, err := SigningSecret(func(k string) string {
			if k == "SECRET_KEY" {
				return strings.Repeat("fallback", 8)
			}
			return secret
		})
		if err == nil || value != "" || (secret != "" && strings.Contains(err.Error(), secret)) {
			t.Fatal("invalid secret not safely rejected")
		}
	}
	for _, secret := range []string{strings.Repeat("a", 32), strings.Repeat("界", 11)} {
		value, err := SigningSecret(func(k string) string {
			if k != "JWT_SIGNING_SECRET" {
				t.Fatal("unexpected configuration fallback")
			}
			return secret
		})
		if err != nil || value != secret {
			t.Fatal("valid byte length rejected")
		}
	}
}
