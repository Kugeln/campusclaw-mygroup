package material

import (
	"campusclaw/internal/auth"
	"campusclaw/internal/storage"
	"context"
	"errors"
	"testing"
	"time"
)

type identity struct{}

func (identity) FindCurrentUser(context.Context, int64) (auth.CurrentUser, bool, error) {
	return auth.CurrentUser{ID: 1, ClassID: 2, Role: "teacher"}, true, nil
}
func (identity) Revoke(context.Context, int64) error { return nil }
func principal(t *testing.T) auth.Principal {
	tokens, _ := auth.NewTokens("test-only-material-signing-key-32bytes", time.Now)
	token, _ := tokens.Issue(1, 0)
	p, e := (auth.Identity{Store: identity{}, Tokens: tokens}).Authenticate(context.Background(), token)
	if e != nil {
		t.Fatal("identity fixture")
	}
	return p
}

type txFake struct {
	insertErr, commitErr error
	record               Record
	committed            bool
	events               *[]string
}

func (t *txFake) Insert(r Record) error {
	*t.events = append(*t.events, "insert")
	t.record = r
	return t.insertErr
}
func (t *txFake) Commit() error {
	*t.events = append(*t.events, "commit")
	t.committed = t.commitErr == nil
	return t.commitErr
}
func (t *txFake) Rollback() error { *t.events = append(*t.events, "rollback"); return nil }

type repoFake struct {
	tx               *txFake
	out              Outcome
	err              error
	resolveContextOK bool
}

func (r *repoFake) Begin(context.Context) (Transaction, error) { return r.tx, nil }
func (r *repoFake) Resolve(ctx context.Context, _ Record) (Outcome, error) {
	r.resolveContextOK = ctx.Err() == nil
	return r.out, r.err
}

type fileFake struct {
	publishErr, verifyErr error
	discarded             bool
	events                *[]string
}

func (f *fileFake) Info() storage.UploadInfo {
	return storage.UploadInfo{ID: "uuid", Name: "name", Text: "body", Size: 4}
}
func (f *fileFake) Publish() error { *f.events = append(*f.events, "publish"); return f.publishErr }
func (f *fileFake) Verify() error  { return f.verifyErr }
func (f *fileFake) Discard() error { f.discarded = true; return nil }
func TestUploadTransactionOutcomes(t *testing.T) {
	p := principal(t)
	fault := errors.New("private failure")
	for _, tc := range []struct {
		name                                     string
		insert, publish, commit, resolve, verify error
		out                                      Outcome
		success, discard                         bool
	}{
		{"success", nil, nil, nil, nil, nil, Unknown, true, false},
		{"insert-failure", fault, nil, nil, nil, nil, Unknown, false, true},
		{"move-failure", nil, fault, nil, nil, nil, Unknown, false, true},
		{"confirmed-rollback", nil, nil, fault, nil, nil, Absent, false, true},
		{"lost-commit-response", nil, nil, fault, nil, nil, Committed, true, false},
		{"unknown-preserve", nil, nil, fault, fault, nil, Unknown, false, false},
		{"incomplete-preserve", nil, nil, fault, nil, nil, Unknown, false, false},
		{"committed-file-damaged", nil, nil, nil, nil, fault, Unknown, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			events := []string{}
			tx := &txFake{insertErr: tc.insert, commitErr: tc.commit, events: &events}
			repo := &repoFake{tx: tx, out: tc.out, err: tc.resolve}
			file := &fileFake{publishErr: tc.publish, verifyErr: tc.verify, events: &events}
			ctx, cancel := context.WithCancel(context.Background())
			cancel() // reconciliation must not inherit cancellation
			id, e := (Uploader{Repository: repo}).Save(ctx, p, file)
			if (e == nil) != tc.success || (id != "") != tc.success || file.discarded != tc.discard {
				t.Fatal("unsafe transaction outcome")
			}
			if tx.record.ClassID != 2 || tx.record.UploaderID != 1 {
				t.Fatal("identity scope lost")
			}
			if tc.commit != nil && !repo.resolveContextOK {
				t.Fatal("canceled recovery context")
			}
			if tc.success && (len(events) < 3 || events[0] != "insert" || events[1] != "publish" || events[2] != "commit") {
				t.Fatal("wrong publish/commit sequence")
			}
		})
	}
}
