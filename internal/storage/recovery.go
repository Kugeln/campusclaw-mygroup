package storage

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"errors"
	"io"
	"os"
	"strings"
	"unicode/utf8"
)

var ErrRecovery = errors.New("Material storage inconsistent; restore data before startup")

type Reference struct {
	Key, Text string
	Size      int64
}

// Recover runs only before the sole writer starts. The caller holds a verified,
// locking database view throughout. It must not be used on a live upload volume.
// All directory entries and referenced contents are checked before any cleanup.
func (s *PrivateStore) Recover(refs []Reference, confirm func() error) error {
	referenced := map[string]bool{}
	for _, v := range refs {
		if referenced[v.Key] || !utf8.ValidString(v.Text) || strings.ContainsRune(v.Text, 0) {
			return ErrRecovery
		}
		f, e := s.OpenMaterial(v.Key, v.Size)
		if e != nil {
			return ErrRecovery
		}
		r := bufio.NewReader(f)
		prefix, _ := r.Peek(3)
		if bytes.Equal(prefix, []byte{0xef, 0xbb, 0xbf}) {
			_, _ = r.Discard(3)
		}
		h := sha256.New()
		_, e = io.Copy(h, r)
		closeErr := f.Close()
		want := sha256.Sum256([]byte(v.Text))
		if e != nil || closeErr != nil || !bytes.Equal(h.Sum(nil), want[:]) {
			return ErrRecovery
		}
		referenced[v.Key] = true
	}
	type candidate struct {
		path string
		info os.FileInfo
	}
	var pending []candidate
	for _, dir := range []string{"tmp", "files"} {
		info, e := s.root.Lstat(dir)
		if e != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return ErrRecovery
		}
		f, e := s.root.Open(dir)
		if e != nil {
			return ErrRecovery
		}
		entries, e := f.ReadDir(-1)
		f.Close()
		if e != nil {
			return ErrRecovery
		}
		for _, entry := range entries {
			path := dir + "/" + entry.Name()
			info, e := s.root.Lstat(path)
			if e != nil || !info.Mode().IsRegular() {
				return ErrRecovery
			}
			if dir == "tmp" || !referenced[entry.Name()] {
				pending = append(pending, candidate{path, info})
			}
		}
	}
	if confirm == nil || confirm() != nil {
		return ErrRecovery
	}
	for _, v := range pending {
		// Recheck both authority and inode immediately before unlinking. Never follow
		// a replaced symlink, recurse into a directory, or clean with a failed query.
		if confirm() != nil {
			return ErrRecovery
		}
		current, e := s.root.Lstat(v.path)
		if e != nil || !current.Mode().IsRegular() || !os.SameFile(v.info, current) {
			return ErrRecovery
		}
		if s.root.Remove(v.path) != nil {
			return ErrRecovery
		}
	}
	if s.syncDir("tmp") != nil || s.syncDir("files") != nil {
		return ErrRecovery
	}
	return nil
}
