package auth

import (
	"encoding/json"
	"math"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const testSecret = "unit-test-only-signing-material-32-bytes"

func TestFlaskIssuedToken(t *testing.T) {
	// Generated with the existing app/tokens.py issue_token/verify_token using
	// a frozen clock and testSecret. This is synthetic, never a Demo credential.
	data, err := os.ReadFile("testdata/flask-token.json")
	if err != nil {
		t.Fatal("missing Flask fixture")
	}
	var fixture struct {
		Token    string `json:"token"`
		IssuedAt int64  `json:"issued_at"`
		UserID   int64  `json:"user_id"`
		Version  int64  `json:"version"`
	}
	if json.Unmarshal(data, &fixture) != nil {
		t.Fatal("invalid Flask fixture")
	}
	tokens, _ := NewTokens(testSecret, func() time.Time { return time.Unix(fixture.IssuedAt, 0) })
	claims, err := tokens.Verify(fixture.Token)
	if err != nil || claims.UserID != fixture.UserID || claims.Version != fixture.Version || claims.ExpiresAt != fixture.IssuedAt+28800 {
		t.Fatal("Flask-issued token incompatible")
	}
}

func TestTokenExpiryAndIssuance(t *testing.T) {
	now := time.Unix(1700000000, 0)
	tokens, err := NewTokens(testSecret, func() time.Time { return now })
	if err != nil {
		t.Fatal("token initialization failed")
	}
	a, err := tokens.Issue(math.MaxInt64, math.MaxInt64-1)
	if err != nil {
		t.Fatal("token issue failed")
	}
	c, err := tokens.Verify(a)
	if err != nil || c.UserID != math.MaxInt64 || c.Version != math.MaxInt64-1 || c.ExpiresAt-c.IssuedAt != 28800 {
		t.Fatal("claims or precision mismatch")
	}
	for _, offset := range []int64{1, 14400, 28799} {
		now = time.Unix(1700000000+offset, 0)
		if got, err := tokens.Verify(a); err != nil || got.ExpiresAt != c.ExpiresAt {
			t.Fatal("access changed expiry")
		}
	}
	if _, err := tokens.Issue(1, 0); err != nil {
		t.Fatal("repeat login failed")
	}
	if _, err := tokens.Verify(a); err != nil {
		t.Fatal("new issuance revoked prior token")
	}
	now = time.Unix(c.ExpiresAt, 0)
	if _, err := tokens.Verify(a); err != ErrInvalidToken {
		t.Fatal("expiry boundary accepted")
	}
	now = now.Add(time.Second)
	if _, err := tokens.Verify(a); err != ErrInvalidToken {
		t.Fatal("expired token accepted")
	}
}

func TestInvalidTokenMatrix(t *testing.T) {
	now := time.Unix(1700000000, 0)
	tokens, _ := NewTokens(testSecret, func() time.Time { return now })
	base := func() jwt.MapClaims {
		return jwt.MapClaims{"sub": "1", "ver": 0, "iat": now.Unix(), "exp": now.Unix() + 28800}
	}
	sign := func(c jwt.MapClaims, method jwt.SigningMethod, key any) string {
		s, err := jwt.NewWithClaims(method, c).SignedString(key)
		if err != nil {
			t.Fatal("test fixture signing failed")
		}
		return s
	}
	for _, key := range []string{"sub", "ver", "iat", "exp"} {
		t.Run("missing_"+key, func(t *testing.T) {
			c := base()
			delete(c, key)
			if _, err := tokens.Verify(sign(c, jwt.SigningMethodHS256, []byte(testSecret))); err == nil {
				t.Fatal("missing claim accepted")
			}
		})
	}
	for _, tc := range []struct {
		name, key string
		value     any
	}{
		{"numeric-sub", "sub", 1}, {"zero-sub", "sub", "0"}, {"negative-sub", "sub", "-1"}, {"padded-sub", "sub", "01"}, {"overflow-sub", "sub", "9223372036854775808"},
		{"negative-version", "ver", -1}, {"max-version", "ver", json.Number("9223372036854775807")}, {"overflow-version", "ver", json.Number("9223372036854775808")},
		{"bool-version", "ver", true}, {"float-version", "ver", json.Number("0.0")}, {"exponent-version", "ver", json.Number("0e0")}, {"string-version", "ver", "0"}, {"null-version", "ver", nil},
		{"float-iat", "iat", json.Number("1700000000.0")}, {"string-iat", "iat", "1700000000"}, {"future-iat", "iat", now.Unix() + 1}, {"negative-iat", "iat", -1},
		{"float-exp", "exp", json.Number("1700028800.0")}, {"string-exp", "exp", "1700028800"}, {"wrong-lifetime", "exp", now.Unix() + 28801}, {"expired", "exp", now.Unix()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := base()
			c[tc.key] = tc.value
			if _, err := tokens.Verify(sign(c, jwt.SigningMethodHS256, []byte(testSecret))); err != ErrInvalidToken {
				t.Fatal("invalid claims not safely rejected")
			}
		})
	}
	good := sign(base(), jwt.SigningMethodHS256, []byte(testSecret))
	parts := strings.Split(good, ".")
	parts[2] = "A" + parts[2][1:]
	if parts[2] == strings.Split(good, ".")[2] {
		parts[2] = "B" + parts[2][1:]
	}
	for _, bad := range []string{"", "garbage", "a.b.c", good + "=", good + "\n", strings.Join(parts, "."), sign(base(), jwt.SigningMethodHS384, []byte(testSecret)), sign(base(), jwt.SigningMethodNone, jwt.UnsafeAllowNoneSignatureType), sign(base(), jwt.SigningMethodHS256, []byte("another-test-key"))} {
		if _, err := tokens.Verify(bad); err != ErrInvalidToken {
			t.Fatal("invalid format/signature/algorithm accepted")
		}
	}
	if _, err := NewTokens("", nil); err == nil {
		t.Fatal("missing secret accepted")
	}
	for _, pair := range [][2]int64{{0, 0}, {1, -1}, {1, math.MaxInt64}} {
		if _, err := tokens.Issue(pair[0], pair[1]); err == nil {
			t.Fatal("invalid issuance accepted")
		}
	}
}
