// bootstrap is a one-shot deployment gate, never an HTTP or migration service.
package main

import (
	"campusclaw/internal/config"
	"campusclaw/internal/provision"
	repository "campusclaw/internal/repository/mysql"
	"campusclaw/internal/transfer"
	"context"
	"fmt"
	"os"
	"time"
)

func validate(get func(string) string) error {
	if _, err := config.SigningSecret(get); err != nil {
		return err
	}
	switch get("INITIALIZATION_MODE") {
	case "fresh", "migrate":
		return nil
	default:
		return fmt.Errorf("INITIALIZATION_MODE must explicitly be fresh or migrate")
	}
}

func run() error {
	if err := validate(os.Getenv); err != nil {
		return err
	}
	c, err := config.Load(os.Getenv)
	if err != nil {
		return err
	}
	db, err := repository.Open(c)
	if err != nil {
		return fmt.Errorf("initialization database unavailable")
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	if os.Getenv("RAG_ENABLED") == "true" {
		if receipt, receiptErr := transfer.Receipt(c.UploadRoot); receiptErr == nil {
			if receipt.Mode != os.Getenv("INITIALIZATION_MODE") {
				return fmt.Errorf("initialization mode differs from receipt")
			}
			if err = transfer.PrepareCurrentSchema(ctx, db, c.UploadRoot); err != nil {
				return err
			}
		} else if !os.IsNotExist(receiptErr) {
			return receiptErr
		}
	}
	if os.Getenv("INITIALIZATION_MODE") == "fresh" {
		if err = repository.PrepareSchema(ctx, db); err != nil {
			return err
		}
		// Fresh checks the receipt before reading seed credentials. Existing data
		// are verified, never reseeded; unknown nonempty targets fail closed.
		if _, err = provision.Fresh(ctx, db, c.UploadRoot, os.Getenv); err != nil {
			return err
		}
	} else {
		receipt, err := transfer.Receipt(c.UploadRoot)
		if err != nil || receipt.Mode != "migrate" {
			return fmt.Errorf("verified migration receipt required; run offline import and verification first")
		}
	}
	if err = transfer.Ready(ctx, db, c.UploadRoot); err != nil {
		return err
	}
	if os.Getenv("RAG_ENABLED") == "true" {
		if err = repository.PrepareRAGSchema(ctx, db); err != nil {
			return err
		}
		// v3 migration preserves existing skills; never automatically recreate
		// a teacher-deleted skill when initialization is repeated.
		if err = transfer.Ready(ctx, db, c.UploadRoot); err != nil {
			return err
		}
		if err = transfer.RecordMCPSchema(ctx, db, c.UploadRoot); err != nil {
			return err
		}
	}
	fmt.Println("initialization verified; application may start")
	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
