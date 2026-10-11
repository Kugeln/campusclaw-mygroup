package storage

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

func testStore(t *testing.T) (*PrivateStore, string) {
	t.Helper()
	root := t.TempDir()
	s, e := OpenPrivate(root)
	if e != nil {
		t.Fatal("store open failed")
	}
	t.Cleanup(func() { s.Close() })
	return s, root
}
func TestUploadParsing(t *testing.T) {
	s, root := testStore(t)
	for _, tc := range []struct {
		name string
		data []byte
		want string
		err  error
	}{
		{"a.TXT", []byte("hello"), "hello", nil}, {"b.Md", []byte("\xef\xbb\xbf# 中\n<script>x</script>"), "# 中\n<script>x</script>", nil},
		{"bom.txt", []byte("\xef\xbb\xbf"), "", nil}, {"double.txt", []byte("\xef\xbb\xbf\xef\xbb\xbf"), "\ufeff", nil},
		{"empty.txt", nil, "", ErrText}, {"nul.txt", []byte("x\x00y"), "", ErrText}, {"bad.txt", []byte{0xc0, 0xaf}, "", ErrText},
		{"truncated.txt", []byte{0xe4, 0xb8}, "", ErrText}, {"binary.pdf", []byte("x"), "", ErrFileType}, {".txt", []byte("x"), "", ErrFileType},
		{".TXT", []byte("x"), "", ErrFileType},
	} {
		t.Run(tc.name, func(t *testing.T) {
			u, e := s.Prepare(tc.name, bytes.NewReader(tc.data), DefaultUploadLimit)
			if !errors.Is(e, tc.err) {
				t.Fatal("parse result mismatch")
			}
			if e != nil {
				return
			}
			defer u.Close()
			if u.Info().Text != tc.want || u.Info().Size != int64(len(tc.data)) {
				t.Fatal("text/size mismatch")
			}
			raw, e := os.ReadFile(filepath.Join(root, "tmp", u.Info().ID))
			if e != nil || !bytes.Equal(raw, tc.data) {
				t.Fatal("raw bytes changed")
			}
		})
	}
	// Force a multibyte rune across the streaming buffer boundary.
	data := strings.Repeat("x", 65535) + "界"
	u, e := s.Prepare("boundary.txt", strings.NewReader(data), DefaultUploadLimit)
	if e != nil || u.Info().Text != data {
		t.Fatal("UTF-8 boundary failed")
	}
	u.Close()
	entries, _ := os.ReadDir(filepath.Join(root, "tmp"))
	if len(entries) != 0 {
		t.Fatal("temporary files leaked")
	}
}

type infiniteText struct{ read int64 }

func (r *infiniteText) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 'x'
	}
	r.read += int64(len(p))
	return len(p), nil
}
func TestUploadLimit(t *testing.T) {
	s, _ := testStore(t)
	u, e := s.Prepare("exact.txt", io.LimitReader(&infiniteText{}, DefaultUploadLimit), DefaultUploadLimit)
	if e != nil || u.Info().Size != DefaultUploadLimit {
		t.Fatal("20 MiB boundary rejected")
	}
	u.Close()
	r := &infiniteText{}
	if _, e = s.Prepare("large.txt", r, DefaultUploadLimit); e != ErrLimit || r.read > DefaultUploadLimit+1 {
		t.Fatal("stream not bounded")
	}
}
func TestSafeNamesAndConcurrentUploads(t *testing.T) {
	for input, want := range map[string]string{"../../outside.txt": "outside.txt", "My cool file.txt": "My_cool_file.txt", "i contain cool ümläuts.txt": "i_contain_cool_umlauts.txt", "中文.txt": "txt", "...": "material.txt", "a\x00b.txt": "ab.txt"} {
		if SafeName(input) != want {
			t.Fatal("filename normalization mismatch")
		}
	}
	s, root := testStore(t)
	var wg sync.WaitGroup
	ids := make(chan string, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			u, e := s.Prepare("same.txt", strings.NewReader("text"), 100)
			if e != nil {
				t.Error("prepare failed")
				return
			}
			defer u.Close()
			if u.Publish() != nil || u.Verify() != nil {
				t.Error("publish failed")
				return
			}
			ids <- u.Info().ID
		}()
	}
	wg.Wait()
	close(ids)
	seen := map[string]bool{}
	for id := range ids {
		if seen[id] {
			t.Fatal("UUID reused")
		}
		seen[id] = true
	}
	if len(seen) != 8 {
		t.Fatal("concurrent upload lost")
	}
	// Deliberate UUID destination collision must not overwrite/delete old bytes.
	u, e := s.Prepare("same.txt", strings.NewReader("new"), 100)
	if e != nil {
		t.Fatal(e)
	}
	defer u.Close()
	dest := filepath.Join(root, "files", u.Info().ID)
	if os.WriteFile(dest, []byte("old"), 0600) != nil {
		t.Fatal("fixture failed")
	}
	if u.Publish() == nil {
		t.Fatal("collision overwritten")
	}
	u.Discard()
	b, _ := os.ReadFile(dest)
	if string(b) != "old" {
		t.Fatal("collision cleanup deleted existing file")
	}
}
func TestStorageFailures(t *testing.T) {
	s, root := testStore(t)
	u, e := s.Prepare("a.txt", strings.NewReader("text"), 100)
	if e != nil {
		t.Fatal(e)
	}
	// Simulate source disappearance before rename; cleanup owns only its reservation.
	u.Close()
	if u.Publish() == nil {
		t.Fatal("missing source moved")
	}
	if u.Discard() != nil {
		t.Fatal("reservation cleanup failed")
	}
	if runtime.GOOS != "windows" {
		if e := os.Symlink(t.TempDir(), filepath.Join(root, "link")); e != nil {
			t.Fatal(e)
		}
		if other, e := OpenPrivate(filepath.Join(root, "link")); e == nil {
			other.Close()
			t.Fatal("symlink root accepted")
		}
	}
	// Closed root makes staging file creation fail, without writing elsewhere.
	s.Close()
	if _, e := s.Prepare("a.txt", strings.NewReader("text"), 100); e != ErrUploadStorage {
		t.Fatal("storage write failure hidden")
	}
}
