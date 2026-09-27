package storage

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
)

// Probe only uses temporary files; it never scans or recovers material files.
func Probe(root string) error {
	for _, dir := range []string{root, filepath.Join(root, "tmp"), filepath.Join(root, "files")} {
		if err := probeDir(dir); err != nil {
			return err
		}
	}
	return nil
}
func probeDir(dir string) error {
	info, err := os.Lstat(dir)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("invalid storage directory")
	}
	f, err := os.CreateTemp(dir, ".health-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	defer f.Close()
	data := []byte("campusclaw-storage-probe")
	if _, err = f.Write(data); err != nil {
		return err
	}
	if err = f.Sync(); err != nil {
		return err
	}
	if _, err = f.Seek(0, 0); err != nil {
		return err
	}
	got := make([]byte, len(data))
	if _, err = f.Read(got); err != nil {
		return err
	}
	if !bytes.Equal(got, data) {
		return fmt.Errorf("storage probe mismatch")
	}
	return nil
}
