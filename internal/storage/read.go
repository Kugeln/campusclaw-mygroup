package storage

import (
	"os"
	"regexp"
)

var canonicalKey = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// OpenMaterial accepts only canonical server storage keys and regular files.
// The descriptor is validated before any HTTP success headers are written.
func (s *PrivateStore) OpenMaterial(key string, size int64) (*os.File, error) {
	if !canonicalKey.MatchString(key) || size <= 0 {
		return nil, ErrUploadStorage
	}
	dir, err := s.root.Lstat("files")
	if err != nil || !dir.IsDir() || dir.Mode()&os.ModeSymlink != 0 {
		return nil, ErrUploadStorage
	}
	name := "files/" + key
	info, err := s.root.Lstat(name)
	if err != nil || !info.Mode().IsRegular() || info.Size() != size {
		return nil, ErrUploadStorage
	}
	f, err := s.root.Open(name)
	if err != nil {
		return nil, ErrUploadStorage
	}
	opened, err := f.Stat()
	if err != nil || !opened.Mode().IsRegular() || !os.SameFile(info, opened) || opened.Size() != size {
		f.Close()
		return nil, ErrUploadStorage
	}
	return f, nil
}
