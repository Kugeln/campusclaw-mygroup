package storage

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestRecoveryPreflightAndCleanup(t *testing.T) {
	for _, mode := range []string{"success", "missing-file", "missing-body", "wrong-text", "bad-key", "database-failure", "unexpected-dir", "symlink", "replaced-files-dir"} {
		t.Run(mode, func(t *testing.T) {
			if (mode == "symlink" || mode == "replaced-files-dir") && runtime.GOOS == "windows" {
				t.Skip("symlink coverage runs in Linux integration runner")
			}
			s, root := testStore(t)
			u, e := s.Prepare("a.txt", strings.NewReader("\ufeff原文"), 100)
			if e != nil || u.Publish() != nil {
				t.Fatal("fixture")
			}
			ref := Reference{Key: u.Info().ID, Text: "原文", Size: u.Info().Size}
			marker := filepath.Join(root, "tmp", "interrupted")
			orphan := filepath.Join(root, "files", "orphan")
			os.WriteFile(marker, []byte("evidence"), 0600)
			os.WriteFile(orphan, []byte("uncommitted"), 0600)
			confirm := func() error { return nil }
			switch mode {
			case "missing-file":
				os.Remove(filepath.Join(root, "files", ref.Key))
			case "missing-body":
				ref.Text = ""
			case "wrong-text":
				ref.Text = "wrong"
			case "bad-key":
				ref.Key = "../outside"
			case "database-failure":
				confirm = func() error { return errors.New("database down") }
			case "unexpected-dir":
				os.Mkdir(filepath.Join(root, "files", "directory"), 0700)
			case "symlink":
				os.Symlink(marker, filepath.Join(root, "files", "link"))
			case "replaced-files-dir":
				os.Rename(filepath.Join(root, "files"), filepath.Join(root, "saved"))
				os.Symlink(filepath.Join(root, "saved"), filepath.Join(root, "files"))
			}
			e = s.Recover([]Reference{ref}, confirm)
			if mode == "success" {
				if e != nil {
					t.Fatal("recovery failed")
				}
				if _, e = os.Stat(marker); !os.IsNotExist(e) {
					t.Fatal("temporary file retained")
				}
				if _, e = os.Stat(orphan); !os.IsNotExist(e) {
					t.Fatal("orphan retained")
				}
				f, e := s.OpenMaterial(ref.Key, ref.Size)
				if e != nil {
					t.Fatal("committed file removed")
				}
				f.Close()
			} else {
				if e == nil {
					t.Fatal("unsafe recovery allowed")
				}
				if b, e := os.ReadFile(marker); e != nil || string(b) != "evidence" {
					t.Fatal("cleaned before complete verification")
				}
				if _, e = os.Stat(orphan); e != nil {
					t.Fatal("orphan evidence deleted")
				}
			}
		})
	}
}

func TestMaterialOpenRejectsUnsafeKeys(t *testing.T) {
	s, _ := testStore(t)
	for _, key := range []string{"../secret", "/absolute", "..\\secret", "ABCDEF00-0000-0000-0000-000000000000", "00000000-0000-0000-0000-000000000000"} {
		if f, e := s.OpenMaterial(key, 1); e == nil {
			f.Close()
			t.Fatal("unsafe or missing file opened")
		}
	}
}
