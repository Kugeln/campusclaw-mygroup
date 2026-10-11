package config

import (
	"errors"
	"strings"
)

// SigningSecret is separate from offline database/admin configuration.
// There is deliberately no SECRET_KEY fallback.
func SigningSecret(get func(string) string) (string, error) {
	secret := get("JWT_SIGNING_SECRET")
	if err := ValidateSigningSecret(secret); err != nil {
		return "", err
	}
	return secret, nil
}

func ValidateSigningSecret(secret string) error {
	if strings.TrimSpace(secret) == "" || len([]byte(secret)) < 32 {
		return errors.New("JWT_SIGNING_SECRET must contain at least 32 bytes of random secret material")
	}
	return nil
}
