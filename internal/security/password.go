package security

import (
	"crypto/pbkdf2"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"strings"
)

// VerifyPassword accepts exactly the format supported by the migration preflight.
// Salt is the stored string, not its hex-decoded representation.
func VerifyPassword(encoded, password string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 3 || parts[0] != "pbkdf2:sha256:1000000" || parts[1] == "" || len(parts[2]) != 64 {
		return false
	}
	want, err := hex.DecodeString(parts[2])
	if err != nil {
		return false
	}
	got, err := pbkdf2.Key(sha256.New, password, []byte(parts[1]), 1000000, 32)
	return err == nil && subtle.ConstantTimeCompare(got, want) == 1
}
