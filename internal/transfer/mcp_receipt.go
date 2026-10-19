package transfer

import (
	repository "campusclaw/internal/repository/mysql"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const MCPSchemaReceiptName = ".campusclaw-schema-v4.json"

// PrepareCurrentSchema is offline-only. Check old receipt/data before resuming
// a catalog-proven partial upgrade; never bypass checks for an unknown target.
func PrepareCurrentSchema(ctx context.Context, db *sql.DB, root string) error {
	if err := repository.PreflightMCPSchema(ctx, db); err != nil {
		return err
	}
	if err := readyInitializedData(ctx, db, root); err != nil {
		return err
	}
	if err := repository.PrepareRAGSchema(ctx, db); err != nil {
		return err
	}
	return RecordMCPSchema(ctx, db, root)
}

type mcpSchemaReceipt struct {
	Version            int    `json:"version"`
	DDLHash            string `json:"ddl_hash"`
	InitializationHash string `json:"initialization_hash"`
}

func expectedMCPReceipt(ctx context.Context, db *sql.DB, root string) (mcpSchemaReceipt, error) {
	if e := repository.ValidateMCPSchema(ctx, db); e != nil {
		return mcpSchemaReceipt{}, e
	}
	if e := Ready(ctx, db, root); e != nil {
		return mcpSchemaReceipt{}, e
	}
	b, e := os.ReadFile(filepath.Join(root, ReceiptName))
	if e != nil {
		return mcpSchemaReceipt{}, fmt.Errorf("initialization receipt unavailable")
	}
	return mcpSchemaReceipt{Version: 4, DDLHash: repository.MCPSchemaFingerprint(), InitializationHash: fmt.Sprintf("%x", sha256.Sum256(b))}, nil
}

// RecordMCPSchema is an explicit initializer action; the old receipt is retained.
func RecordMCPSchema(ctx context.Context, db *sql.DB, root string) error {
	want, e := expectedMCPReceipt(ctx, db, root)
	if e != nil {
		return e
	}
	path := filepath.Join(root, MCPSchemaReceiptName)
	if _, e = os.Lstat(path); e == nil {
		return VerifyMCPSchemaReceipt(ctx, db, root)
	} else if !os.IsNotExist(e) {
		return fmt.Errorf("schema receipt unavailable")
	}
	b, _ := json.MarshalIndent(want, "", "  ")
	f, e := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if e != nil {
		return fmt.Errorf("schema receipt creation failed")
	}
	_, e = f.Write(b)
	syncErr := f.Sync()
	closeErr := f.Close()
	if e != nil || syncErr != nil || closeErr != nil {
		return fmt.Errorf("schema receipt write failed; preserve file for review")
	}
	return nil
}
func VerifyMCPSchemaReceipt(ctx context.Context, db *sql.DB, root string) error {
	want, e := expectedMCPReceipt(ctx, db, root)
	if e != nil {
		return e
	}
	path := filepath.Join(root, MCPSchemaReceiptName)
	info, e := os.Lstat(path)
	if e != nil || !info.Mode().IsRegular() {
		return fmt.Errorf("schema receipt required")
	}
	b, e := os.ReadFile(path)
	var actual mcpSchemaReceipt
	if e != nil || json.Unmarshal(b, &actual) != nil || actual != want {
		return fmt.Errorf("schema receipt mismatch")
	}
	return nil
}
