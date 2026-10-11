package auth

import (
	"context"
	"math"
)

type CurrentUser struct {
	ID, ClassID, Version int64
	Role, ClassName      string
}
type IdentityStore interface {
	FindCurrentUser(context.Context, int64) (CurrentUser, bool, error)
	Revoke(context.Context, int64) error
}

// Principal has no exported mutable fields. It is produced only after verification.
type Principal struct {
	userID, classID int64
	role, className string
}

func (p Principal) UserID() int64     { return p.userID }
func (p Principal) ClassID() int64    { return p.classID }
func (p Principal) Role() string      { return p.role }
func (p Principal) ClassName() string { return p.className }

type Identity struct {
	Store  IdentityStore
	Tokens *Tokens
}

func (a Identity) Authenticate(ctx context.Context, encoded string) (Principal, error) {
	c, err := a.Tokens.Verify(encoded)
	if err != nil {
		return Principal{}, ErrInvalidToken
	}
	u, found, err := a.Store.FindCurrentUser(ctx, c.UserID)
	if err != nil {
		return Principal{}, ErrStorageUnavailable
	}
	if !found || u.ID != c.UserID || u.ClassID <= 0 || (u.Role != "teacher" && u.Role != "student") || u.Version < 0 || u.Version == math.MaxInt64 || u.Version != c.Version {
		return Principal{}, ErrInvalidToken
	}
	return Principal{u.ID, u.ClassID, u.Role, u.ClassName}, nil
}
func (a Identity) Logout(ctx context.Context, p Principal) error {
	if p.userID <= 0 {
		return ErrInvalidToken
	}
	if err := a.Store.Revoke(ctx, p.userID); err != nil {
		return ErrStorageUnavailable
	}
	return nil
}

type principalKey struct{}

func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, principalKey{}, p)
}
func FromContext(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(principalKey{}).(Principal)
	return p, ok && p.userID > 0
}
