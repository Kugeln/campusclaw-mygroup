package transfer

import (
	repository "campusclaw/internal/repository/mysql"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const ReceiptName = ".campusclaw-initialization.json"
const PendingName = ".campusclaw-pending.json"

func Receipt(root string) (Report, error) {
	var r Report
	b, e := os.ReadFile(filepath.Join(root, ReceiptName))
	if e != nil {
		return r, e
	}
	e = json.Unmarshal(b, &r)
	if e != nil || r.Version != 1 || !r.Verified || (r.Mode != "migrate" && r.Mode != "fresh") {
		return r, fmt.Errorf("invalid initialization receipt")
	}
	return r, nil
}
func WriteReport(path string, r Report) error {
	b, e := json.MarshalIndent(r, "", "  ")
	if e != nil {
		return e
	}
	f, e := os.CreateTemp(filepath.Dir(path), ".report-*")
	if e != nil {
		return fmt.Errorf("report creation failed")
	}
	name := f.Name()
	defer os.Remove(name)
	if e = f.Chmod(0600); e == nil {
		_, e = f.Write(b)
	}
	if e == nil {
		e = f.Sync()
	}
	closeErr := f.Close()
	if e != nil || closeErr != nil {
		return fmt.Errorf("report write failed")
	}
	if e = os.Rename(name, path); e != nil {
		return fmt.Errorf("report publication failed")
	}
	return nil
}
func sessionsEmpty(ctx context.Context, db *sql.DB) error {
	var n int
	if e := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sessions").Scan(&n); e != nil || n != 0 {
		return fmt.Errorf("target sessions must be empty; refusing active or incomplete target")
	}
	return nil
}

// Verify compares every field and ID, relationships, body and file hashes.
func Verify(ctx context.Context, db *sql.DB, want Snapshot, root string, mode string) (Report, error) {
	if e := repository.ValidateSchema(ctx, db); e != nil {
		return Report{}, e
	}
	if e := sessionsEmpty(ctx, db); e != nil {
		return Report{}, e
	}
	got, e := Read(ctx, db)
	if e != nil {
		return Report{}, e
	}
	if e = got.Validate(root); e != nil {
		return Report{}, e
	}
	expected := want.Report(mode, true)
	actual := got.Report(mode, true)
	actual.SourceSHA256 = want.SourceSHA256
	if actual.Fingerprint != expected.Fingerprint {
		return Report{}, fmt.Errorf("target data or file fingerprint differs")
	}
	return actual, nil
}

// Publish requires exclusive offline access. It never overwrites target rows/files.
// afterInsert is an internal test hook, not configurable by environment or CLI.
func Publish(ctx context.Context, db *sql.DB, s Snapshot, sourceRoot, targetRoot, mode string, afterInsert func() error) (Report, error) {
	return publish(ctx, db, s, sourceRoot, targetRoot, mode, afterInsert, func(tx *sql.Tx) error { return tx.Commit() })
}

func publish(ctx context.Context, db *sql.DB, s Snapshot, sourceRoot, targetRoot, mode string, afterInsert func() error, commit func(*sql.Tx) error) (Report, error) {
	if e := repository.ValidateSchema(ctx, db); e != nil {
		return Report{}, e
	}
	if mode != "migrate" && mode != "fresh" {
		return Report{}, fmt.Errorf("invalid mode")
	}
	if e := s.Validate(sourceRoot); e != nil {
		return Report{}, e
	}
	sourceAbs, _ := filepath.Abs(sourceRoot)
	targetAbs, _ := filepath.Abs(targetRoot)
	if sourceAbs == targetAbs || strings.HasPrefix(targetAbs, sourceAbs+string(os.PathSeparator)) || strings.HasPrefix(sourceAbs, targetAbs+string(os.PathSeparator)) {
		return Report{}, fmt.Errorf("source and target directories must be separate")
	}
	if e := SafeDirectory(sourceAbs); e != nil {
		return Report{}, e
	}
	if e := sessionsEmpty(ctx, db); e != nil {
		return Report{}, e
	}
	existing, e := Read(ctx, db)
	if e != nil {
		return Report{}, e
	}
	receipt, re := Receipt(targetRoot)
	if re == nil && receipt.Mode != mode {
		return Report{}, fmt.Errorf("initialization mode conflict")
	}
	if re != nil && !os.IsNotExist(re) {
		return Report{}, re
	}
	if !existing.Empty() {
		report, e := Verify(ctx, db, s, targetRoot, mode)
		if e != nil {
			return Report{}, fmt.Errorf("nonempty target differs; no overwrite")
		}
		if re == nil && receipt.Fingerprint != report.Fingerprint {
			return Report{}, fmt.Errorf("target changed since initialization")
		}
		if e = WriteReport(filepath.Join(targetRoot, ReceiptName), report); e != nil {
			return Report{}, e
		}
		_ = os.Remove(filepath.Join(targetRoot, PendingName))
		return report, nil
	}
	// Only create missing directories; existing paths must not be symlinks.
	for _, dir := range []string{targetRoot, filepath.Join(targetRoot, "files"), filepath.Join(targetRoot, "tmp")} {
		if e = os.MkdirAll(dir, 0700); e != nil {
			return Report{}, fmt.Errorf("target directory preparation failed")
		}
		if e = SafeDirectory(dir); e != nil {
			return Report{}, e
		}
	}
	for _, dir := range []string{"files", "tmp"} {
		items, err := os.ReadDir(filepath.Join(targetRoot, dir))
		if err != nil {
			return Report{}, err
		}
		if len(items) != 0 {
			return Report{}, fmt.Errorf("empty target database requires empty file directories")
		}
	}
	// Preserve the expected seed fingerprint before COMMIT, so a lost receipt
	// can be recovered without generating new passwords or material IDs.
	if e = WriteReport(filepath.Join(targetRoot, PendingName), s.Report(mode, false)); e != nil {
		return Report{}, e
	}
	tx, e := db.BeginTx(ctx, nil)
	if e != nil {
		return Report{}, fmt.Errorf("target transaction failed")
	}
	defer tx.Rollback()
	created := []string{}
	committing := false
	defer func() {
		if !committing {
			for _, p := range created {
				_ = os.Remove(p)
			}
		}
	}()
	for _, t := range Tables {
		query := "INSERT INTO " + t.Name + " (" + strings.Join(t.Columns, ",") + ") VALUES (" + strings.TrimSuffix(strings.Repeat("?,", len(t.Columns)), ",") + ")"
		for _, r := range s.Rows[t.Name] {
			args := make([]any, len(t.Columns))
			for i, c := range t.Columns {
				args[i] = r[c]
			}
			if _, e = tx.ExecContext(ctx, query, args...); e != nil {
				return Report{}, fmt.Errorf("insert failed in %s", t.Name)
			}
		}
	}
	if afterInsert != nil {
		if e = afterInsert(); e != nil {
			return Report{}, e
		}
	}
	for key, digest := range s.Files {
		b, err := os.ReadFile(filepath.Join(sourceRoot, "files", key))
		if err != nil || Hash(b) != digest {
			return Report{}, fmt.Errorf("source changed during copy")
		}
		path := filepath.Join(targetRoot, "files", key)
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			return Report{}, fmt.Errorf("file publication failed")
		}
		created = append(created, path)
		_, err = f.Write(b)
		if err == nil {
			err = f.Sync()
		}
		ce := f.Close()
		if err != nil || ce != nil {
			return Report{}, fmt.Errorf("file publication failed")
		}
	}
	inside, e := Read(ctx, tx)
	if e != nil {
		return Report{}, e
	}
	if e = inside.Validate(targetRoot); e != nil {
		return Report{}, e
	}
	if inside.Report(mode, true).Fingerprint != s.Report(mode, true).Fingerprint {
		return Report{}, fmt.Errorf("transaction verification failed")
	}
	committing = true
	commitError := commit(tx)
	// Reconciliation has its own deadline even when the original context expired.
	checkCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	report, verifyError := Verify(checkCtx, db, s, targetRoot, mode)
	if verifyError != nil {
		// A failed COMMIT can have succeeded remotely. Only clean after confirming empty.
		if commitError != nil {
			check, readErr := Read(checkCtx, db)
			if readErr == nil && check.Empty() {
				for _, p := range created {
					_ = os.Remove(p)
				}
			}
		}
		return Report{}, fmt.Errorf("commit or post-commit verification failed; preserve target and reverify")
	}
	if e = WriteReport(filepath.Join(targetRoot, ReceiptName), report); e != nil {
		return Report{}, e
	}
	_ = os.Remove(filepath.Join(targetRoot, PendingName))
	return report, nil
}
