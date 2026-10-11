package storage

import (
	"bufio"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"runtime"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

const DefaultUploadLimit int64 = 20 * 1024 * 1024

var ErrFileType = errors.New("A .txt or .md file is required")
var ErrText = errors.New("Invalid UTF-8 text file")

// Preserve the existing public text error while distinguishing invalid encoding
// from empty/NUL input for narrowly scoped HTTP precedence handling.
var ErrEncoding = fmt.Errorf("%w", ErrText)
var ErrLimit = errors.New("File exceeds upload limit")
var ErrUploadStorage = errors.New("Material storage unavailable")

// ErrUploadInput is a caller-classified input stream failure, not a disk error.
var ErrUploadInput = errors.New("Invalid upload input")

// PrivateStore has no HTTP file-server handler. All names used for storage are
// random server UUIDs, never user-provided paths. Root confines filesystem calls.
type PrivateStore struct{ root *os.Root }

func OpenPrivate(root string) (*PrivateStore, error) {
	info, err := os.Lstat(root)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, ErrUploadStorage
	}
	r, err := os.OpenRoot(root)
	if err != nil {
		return nil, ErrUploadStorage
	}
	for _, dir := range []string{"tmp", "files"} {
		if err = r.Mkdir(dir, 0700); err != nil && !errors.Is(err, os.ErrExist) {
			r.Close()
			return nil, ErrUploadStorage
		}
		info, err = r.Lstat(dir)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			r.Close()
			return nil, ErrUploadStorage
		}
	}
	return &PrivateStore{r}, nil
}
func (s *PrivateStore) Close() error { return s.root.Close() }

// SafeName follows Werkzeug's NFKD -> ASCII -> whitespace/character stripping
// behavior on the current platform. This is display metadata, never a disk path.
func SafeName(name string) string {
	var ascii strings.Builder
	for _, r := range norm.NFKD.String(name) {
		if r < 128 {
			ascii.WriteRune(r)
		}
	}
	name = strings.ReplaceAll(ascii.String(), "/", " ")
	if runtime.GOOS == "windows" {
		name = strings.ReplaceAll(name, "\\", " ")
	}
	name = strings.Join(strings.Fields(name), "_")
	var safe strings.Builder
	for _, r := range name {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '.' || r == '_' || r == '-' {
			safe.WriteRune(r)
		}
	}
	name = strings.Trim(safe.String(), "._")
	if runtime.GOOS == "windows" {
		base := strings.ToUpper(strings.SplitN(name, ".", 2)[0])
		if base == "CON" || base == "PRN" || base == "AUX" || base == "NUL" || base == "CONIN$" || base == "CONOUT$" || len(base) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) && base[3] >= '0' && base[3] <= '9' {
			name = "_" + name
		}
	}
	if name == "" {
		return "material.txt"
	}
	return name
}

type UploadInfo struct {
	ID, Name, Text, SHA256 string
	Size                   int64
}
type Upload struct {
	store     *PrivateStore
	info      UploadInfo
	ownsFinal bool
}

func (u *Upload) Info() UploadInfo { return u.info }
func uuid() (string, error) {
	b := make([]byte, 16)
	if _, e := rand.Read(b); e != nil {
		return "", e
	}
	b[6] = (b[6] & 15) | 64
	b[8] = (b[8] & 63) | 128
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[:4], b[4:6], b[6:8], b[8:10], b[10:]), nil
}
func (s *PrivateStore) Prepare(name string, reader io.Reader, limit int64) (*Upload, error) {
	ext := strings.ToLower(path.Ext(name))
	// A basename consisting solely of .txt/.md has no suffix in pathlib.
	base := path.Base(name)
	if runtime.GOOS == "windows" {
		base = path.Base(strings.ReplaceAll(name, "\\", "/"))
	}
	if name == "" || (ext != ".txt" && ext != ".md") || strings.ToLower(base) == ext {
		return nil, ErrFileType
	}
	if limit <= 0 || limit == int64(^uint64(0)>>1) {
		return nil, ErrUploadStorage
	}
	id, err := uuid()
	if err != nil {
		return nil, ErrUploadStorage
	}
	u := &Upload{store: s, info: UploadInfo{ID: id, Name: SafeName(name)}}
	f, err := s.root.OpenFile("tmp/"+id, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return nil, ErrUploadStorage
	}
	ok := false
	defer func() {
		f.Close()
		if !ok {
			u.Close()
		}
	}()
	h := sha256.New()
	size, err := io.CopyBuffer(io.MultiWriter(f, h), io.LimitReader(reader, limit+1), make([]byte, 65536))
	if size > limit {
		return nil, ErrLimit
	}
	if err != nil {
		if errors.Is(err, ErrUploadInput) {
			return nil, ErrUploadInput
		}
		return nil, ErrUploadStorage
	}
	if size == 0 {
		return nil, ErrText
	}
	if f.Sync() != nil || f.Close() != nil {
		return nil, ErrUploadStorage
	}
	input, err := s.root.Open("tmp/" + id)
	if err != nil {
		return nil, ErrUploadStorage
	}
	defer input.Close()
	text, err := parseText(input, limit)
	if err != nil {
		return nil, err
	}
	u.info.Size = size
	u.info.Text = text
	u.info.SHA256 = hex.EncodeToString(h.Sum(nil))
	ok = true
	return u, nil
}

// A bounded builder is needed for the LONGTEXT parameter. The raw request is
// streamed to disk; there is no unbounded multipart/body ReadAll.
func parseText(input io.Reader, limit int64) (string, error) {
	r := bufio.NewReaderSize(io.LimitReader(input, limit+1), 65536)
	var b strings.Builder
	var count int64
	for first := true; ; first = false {
		ch, n, err := r.ReadRune()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", ErrUploadStorage
		}
		count += int64(n)
		if count > limit {
			return "", ErrLimit
		}
		if ch == utf8.RuneError && n == 1 {
			return "", ErrEncoding
		}
		if ch == 0 {
			return "", ErrText
		}
		if first && ch == '\ufeff' {
			continue
		}
		b.WriteRune(ch)
	}
	return b.String(), nil
}
func (u *Upload) Publish() error {
	// Reserve atomically before rename so even a UUID collision cannot overwrite.
	name := "files/" + u.info.ID
	f, err := u.store.root.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return ErrUploadStorage
	}
	u.ownsFinal = true
	if f.Close() != nil {
		return ErrUploadStorage
	}
	if u.store.root.Rename("tmp/"+u.info.ID, name) != nil {
		return ErrUploadStorage
	}
	if u.store.syncDir("files") != nil || u.store.syncDir("tmp") != nil {
		return ErrUploadStorage
	}
	return nil
}
func (s *PrivateStore) syncDir(name string) error {
	// Directory fsync is supported by the Linux deployment filesystem, not Windows.
	if runtime.GOOS == "windows" {
		return nil
	}
	f, e := s.root.Open(name)
	if e != nil {
		return e
	}
	defer f.Close()
	return f.Sync()
}
func (u *Upload) Verify() error {
	name := "files/" + u.info.ID
	i, e := u.store.root.Lstat(name)
	if e != nil || !i.Mode().IsRegular() || i.Size() != u.info.Size {
		return ErrUploadStorage
	}
	f, e := u.store.root.Open(name)
	if e != nil {
		return ErrUploadStorage
	}
	defer f.Close()
	h := sha256.New()
	n, e := io.Copy(h, io.LimitReader(f, u.info.Size+1))
	if e != nil || n != u.info.Size || hex.EncodeToString(h.Sum(nil)) != u.info.SHA256 {
		return ErrUploadStorage
	}
	return nil
}

// Close only removes staging data. Never removes the published file, including
// when the request is canceled or COMMIT has an unknown outcome.
func (u *Upload) Close() error {
	e := u.store.root.Remove("tmp/" + u.info.ID)
	if errors.Is(e, os.ErrNotExist) {
		return nil
	}
	return e
}
func (u *Upload) Discard() error {
	if u.ownsFinal {
		if e := u.store.root.Remove("files/" + u.info.ID); e != nil && !errors.Is(e, os.ErrNotExist) {
			return ErrUploadStorage
		}
		u.ownsFinal = false
	}
	return u.Close()
}
