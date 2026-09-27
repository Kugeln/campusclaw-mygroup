package main

import (
	"campusclaw/internal/config"
	"campusclaw/internal/migration"
	repository "campusclaw/internal/repository/mysql"
	"campusclaw/internal/transfer"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func run() error {
	mode := flag.String("mode", "preflight", "preflight, import, or verify")
	source := flag.String("source", "", "offline SQLite snapshot")
	files := flag.String("source-uploads", "", "offline source uploads root")
	reportPath := flag.String("report", "", "optional report output")
	flag.Parse()
	if *source == "" || *files == "" || (*mode != "preflight" && *mode != "import" && *mode != "verify") {
		return fmt.Errorf("invalid migration arguments")
	}
	if *reportPath != "" {
		report, e := filepath.Abs(*reportPath)
		if e != nil {
			return fmt.Errorf("invalid report path")
		}
		snapshot, e := filepath.Abs(*source)
		if e != nil {
			return fmt.Errorf("invalid source path")
		}
		uploads, e := filepath.Abs(*files)
		if e != nil {
			return fmt.Errorf("invalid source uploads path")
		}
		// EqualFold is intentionally conservative on both Windows and Linux.
		if strings.EqualFold(report, snapshot) || strings.EqualFold(report, uploads) || strings.HasPrefix(strings.ToLower(report), strings.ToLower(uploads)+string(os.PathSeparator)) {
			return fmt.Errorf("report must be outside the source snapshot")
		}
		if _, e = os.Lstat(report); !os.IsNotExist(e) {
			return fmt.Errorf("report output must be a new file")
		}
		if e = transfer.SafeDirectory(filepath.Dir(report)); e != nil {
			return e
		}
	}
	ctx := context.Background()
	s, err := migration.Source(ctx, *source, *files)
	if err != nil {
		return err
	}
	report := s.Report("preflight", false)
	if *mode != "preflight" {
		c, e := config.Load(os.Getenv)
		if e != nil {
			return e
		}
		db, e := repository.Open(c)
		if e != nil {
			return fmt.Errorf("target connection failed")
		}
		defer db.Close()
		// Schema preparation is explicitly a separate admin operation.
		if *mode == "import" {
			report, err = transfer.Publish(ctx, db, s, *files, c.UploadRoot, "migrate", nil)
		} else {
			report, err = transfer.Verify(ctx, db, s, c.UploadRoot, "migrate")
		}
		if err != nil {
			return err
		}
	}
	if *reportPath != "" {
		if err = transfer.WriteReport(*reportPath, report); err != nil {
			return err
		}
	}
	return json.NewEncoder(os.Stdout).Encode(report)
}
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
