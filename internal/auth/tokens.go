// Package auth implements the login and token boundary, independent of HTTP.
package auth

import (
	"campusclaw/internal/config"
	"encoding/json"
	"errors"
	"math"
	"regexp"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const LifetimeSeconds int64 = 28800

var ErrInvalidToken = errors.New("Invalid credential")
var compactToken = regexp.MustCompile(`^[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+$`)
var subjectPattern = regexp.MustCompile(`^[1-9][0-9]{0,18}$`)

type Tokens struct {
	secret []byte
	now    func() time.Time
}

// VerifiedToken is not an authorization principal. Current user/version checks
// belong to the later authentication middleware.
type VerifiedToken struct{ UserID, Version, IssuedAt, ExpiresAt int64 }

func NewTokens(secret string, now func() time.Time) (*Tokens, error) {
	if err := config.ValidateSigningSecret(secret); err != nil {
		return nil, err
	}
	if now == nil {
		now = time.Now
	}
	return &Tokens{[]byte(secret), now}, nil
}

func (t *Tokens) Issue(userID, version int64) (string, error) {
	now := t.now().Unix()
	if userID <= 0 || version < 0 || version == math.MaxInt64 || now < 0 || now > math.MaxInt64-LifetimeSeconds {
		return "", ErrInvalidToken
	}
	claims := jwt.MapClaims{"sub": strconv.FormatInt(userID, 10), "ver": version, "iat": now, "exp": now + LifetimeSeconds}
	encoded, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(t.secret)
	if err != nil {
		return "", ErrInvalidToken
	}
	return encoded, nil
}

func integer(claims jwt.MapClaims, key string) (int64, bool) {
	n, ok := claims[key].(json.Number)
	if !ok {
		return 0, false
	}
	v, err := n.Int64()
	return v, err == nil
}

func (t *Tokens) Verify(encoded string) (VerifiedToken, error) {
	fail := func() (VerifiedToken, error) { return VerifiedToken{}, ErrInvalidToken }
	if !compactToken.MatchString(encoded) {
		return fail()
	}
	// JSON numbers preserve integer precision and permit strict type checks below.
	parsed, err := jwt.Parse(encoded, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, ErrInvalidToken
		}
		return t.secret, nil
	}, jwt.WithValidMethods([]string{"HS256"}), jwt.WithJSONNumber(), jwt.WithTimeFunc(t.now), jwt.WithIssuedAt(), jwt.WithExpirationRequired())
	if err != nil || !parsed.Valid {
		return fail()
	}
	claims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok {
		return fail()
	}
	sub, ok := claims["sub"].(string)
	if !ok || !subjectPattern.MatchString(sub) {
		return fail()
	}
	id, err := strconv.ParseInt(sub, 10, 64)
	ver, vOK := integer(claims, "ver")
	iat, iOK := integer(claims, "iat")
	exp, eOK := integer(claims, "exp")
	now := t.now().Unix()
	if err != nil || !vOK || !iOK || !eOK || ver < 0 || ver == math.MaxInt64 || iat < 0 || iat > math.MaxInt64-LifetimeSeconds || exp != iat+LifetimeSeconds || now < iat || now >= exp {
		return fail()
	}
	return VerifiedToken{id, ver, iat, exp}, nil
}
