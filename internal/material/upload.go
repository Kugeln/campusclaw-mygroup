package material

import (
	"campusclaw/internal/auth"
	"campusclaw/internal/storage"
	"context"
	"errors"
	"time"
)

var ErrUnavailable = errors.New("Material storage unavailable")
var ErrForbidden = errors.New("Teacher role required")

type Record struct {
	storage.UploadInfo
	ClassID, UploaderID int64
	CreatedAt           string
}
type StagedFile interface {
	Info() storage.UploadInfo
	Publish() error
	Verify() error
	Discard() error
}
type Transaction interface {
	Insert(Record) error
	Commit() error
	Rollback() error
}
type Outcome int

const (
	Unknown Outcome = iota
	Absent
	Committed
)

type Repository interface {
	Begin(context.Context) (Transaction, error)
	Resolve(context.Context, Record) (Outcome, error)
}
type Uploader struct {
	Repository Repository
	Now        func() time.Time
}

func (u Uploader) Save(ctx context.Context, p auth.Principal, file StagedFile) (string, error) {
	if p.UserID() <= 0 || p.ClassID() <= 0 || p.Role() != "teacher" {
		return "", ErrForbidden
	}
	now := time.Now()
	if u.Now != nil {
		now = u.Now()
	}
	r := Record{UploadInfo: file.Info(), ClassID: p.ClassID(), UploaderID: p.UserID(), CreatedAt: now.UTC().Format("2006-01-02T15:04:05.000Z")}
	tx, err := u.Repository.Begin(ctx)
	if err != nil {
		return "", ErrUnavailable
	}
	defer tx.Rollback()
	if tx.Insert(r) != nil {
		_ = tx.Rollback()
		_ = file.Discard()
		return "", ErrUnavailable
	}
	if file.Publish() != nil {
		_ = tx.Rollback()
		_ = file.Discard()
		return "", ErrUnavailable
	}
	if tx.Commit() == nil {
		if file.Verify() != nil {
			return "", ErrUnavailable
		} // committed: never compensate by deleting
		return r.ID, nil
	}
	// The client/request context may already be canceled. Reconcile with a fresh,
	// bounded connection/context, waiting for any in-flight insert to settle.
	check, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	state, err := u.Repository.Resolve(check, r)
	if err != nil {
		return "", ErrUnavailable
	} // preserve UUID file as recovery evidence
	switch state {
	case Committed:
		if file.Verify() == nil {
			return r.ID, nil
		}
	case Absent:
		_ = file.Discard()
	}
	return "", ErrUnavailable
}
