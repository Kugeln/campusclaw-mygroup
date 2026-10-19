package transfer

import (
	repository "campusclaw/internal/repository/mysql"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Ready checks the initialization evidence and current integrity without writes.
// It does not clear sessions or demand that mutable account data remain at seed values.
func Ready(ctx context.Context, db *sql.DB, root string) error {
	if err := repository.ValidateSchema(ctx, db); err != nil {
		return err
	}
	return readyInitializedData(ctx, db, root)
}

func readyInitializedData(ctx context.Context, db *sql.DB, root string) error {
	if err := SafeDirectory(root); err != nil {
		return err
	}
	r, err := Receipt(root)
	if err != nil || len(r.Fingerprint) != 64 || len(r.Tables) != 7 {
		return fmt.Errorf("verified initialization receipt required")
	}
	if r.Mode == "migrate" && len(r.SourceSHA256) != 64 {
		return fmt.Errorf("source snapshot association missing")
	}
	current, err := Read(ctx, db)
	if err != nil {
		return err
	}
	if err = current.Validate(root); err != nil {
		return err
	}
	actual := current.Report(r.Mode, true)
	// Existing material/placeholder data have no update/delete API. Bind the
	// receipt to those original records while allowing new rows and account changes.
	for _, table := range Tables {
		previous, ok := r.Tables[table.Name]
		if !ok || previous.Count != len(previous.IDs) || previous.Count != len(previous.RowsSHA256) {
			return fmt.Errorf("incomplete initialization evidence")
		}
		if table.Name == "users" {
			continue
		}
		for _, id := range previous.IDs {
			if actual.Tables[table.Name].RowsSHA256[id] != previous.RowsSHA256[id] {
				return fmt.Errorf("initialized data differs: %s", table.Name)
			}
		}
	}
	for key, hash := range r.FilesSHA256 {
		if current.Files[key] != hash {
			return fmt.Errorf("initialized file differs")
		}
	}
	return nil
}

// RecoverFreshReceipt accepts only the exact pre-commit seed fingerprint.
func RecoverFreshReceipt(ctx context.Context, db *sql.DB, root string) (Report, error) {
	var pending Report
	b, err := os.ReadFile(filepath.Join(root, PendingName))
	if err != nil {
		return Report{}, fmt.Errorf("fresh requires empty target or matching pending seed evidence")
	}
	if json.Unmarshal(b, &pending) != nil || pending.Mode != "fresh" || pending.Version != 1 || pending.Verified {
		return Report{}, fmt.Errorf("invalid pending seed evidence")
	}
	if err = repository.ValidateSchema(ctx, db); err != nil {
		return Report{}, err
	}
	if err = sessionsEmpty(ctx, db); err != nil {
		return Report{}, err
	}
	current, err := Read(ctx, db)
	if err != nil {
		return Report{}, err
	}
	if err = current.Validate(root); err != nil {
		return Report{}, err
	}
	actual := current.Report("fresh", true)
	if actual.Fingerprint != pending.Fingerprint {
		return Report{}, fmt.Errorf("pending seed fingerprint differs; preserve target")
	}
	if err = WriteReport(filepath.Join(root, ReceiptName), actual); err != nil {
		return Report{}, err
	}
	_ = os.Remove(filepath.Join(root, PendingName))
	return actual, nil
}
