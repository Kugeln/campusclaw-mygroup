package auth

import (
	"campusclaw/internal/security"
	"context"
	"errors"
	"math"
)

var ErrInvalidCredentials = errors.New("Invalid credentials")
var ErrStorageUnavailable = errors.New("Authentication storage unavailable")

type LoginUser struct {
	ID, ClassID, Version int64
	Role, PasswordHash   string
}
type Users interface {
	FindLoginUser(context.Context, string) (LoginUser, bool, error)
}
type Login struct {
	Users  Users
	Tokens *Tokens
}
type LoginResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int64  `json:"expires_in"`
	Redirect    string `json:"redirect"`
}

func (l Login) Authenticate(ctx context.Context, username, password string) (LoginResponse, error) {
	u, found, err := l.Users.FindLoginUser(ctx, username)
	if err != nil {
		return LoginResponse{}, ErrStorageUnavailable
	}
	if !found || u.ID <= 0 || u.ClassID <= 0 || (u.Role != "teacher" && u.Role != "student") || u.Version < 0 || u.Version == math.MaxInt64 || !security.VerifyPassword(u.PasswordHash, password) {
		return LoginResponse{}, ErrInvalidCredentials
	}
	token, err := l.Tokens.Issue(u.ID, u.Version)
	if err != nil {
		return LoginResponse{}, errors.New("Authentication unavailable")
	}
	return LoginResponse{token, "Bearer", LifetimeSeconds, "/materials"}, nil
}
