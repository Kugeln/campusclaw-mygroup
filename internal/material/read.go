package material

import (
	"campusclaw/internal/auth"
	"context"
	"errors"
)

var ErrNotFound = errors.New("Material not found")

type Item struct {
	ID           string `json:"id"`
	OriginalName string `json:"original_name"`
	Size         int64  `json:"size"`
	CreatedAt    string `json:"created_at"`
	Uploader     string `json:"uploader"`
}
type Detail struct {
	Item
	PlainText  string `json:"plain_text"`
	StorageKey string `json:"-"`
}
type ReadRepository interface {
	List(context.Context, int64) ([]Item, error)
	Find(context.Context, int64, string) (Detail, error)
}
type Reader struct{ Repository ReadRepository }

func (s Reader) List(ctx context.Context, p auth.Principal) ([]Item, error) {
	if p.UserID() <= 0 || p.ClassID() <= 0 {
		return nil, ErrForbidden
	}
	return s.Repository.List(ctx, p.ClassID())
}
func (s Reader) Find(ctx context.Context, p auth.Principal, id string) (Detail, error) {
	if p.UserID() <= 0 || p.ClassID() <= 0 {
		return Detail{}, ErrForbidden
	}
	return s.Repository.Find(ctx, p.ClassID(), id)
}
