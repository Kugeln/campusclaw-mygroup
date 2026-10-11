package storage

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

type failedUploadInput struct{ err error }

func (r failedUploadInput) Read(p []byte) (int, error) {
	n := copy(p, "partial")
	return n, r.err
}

func TestUploadInputFailureClassification(t *testing.T) {
	for _, tc := range []struct {
		name         string
		source, want error
	}{
		{"classified-input", ErrUploadInput, ErrUploadInput},
		{"unclassified-io", io.ErrUnexpectedEOF, ErrUploadStorage},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, root := testStore(t)
			_, err := s.Prepare("a.txt", failedUploadInput{tc.source}, 100)
			if !errors.Is(err, tc.want) {
				t.Fatal("input/storage classification lost")
			}
			entries, err := os.ReadDir(filepath.Join(root, "tmp"))
			if err != nil || len(entries) != 0 {
				t.Fatal("failed copy left staging data")
			}
		})
	}
}
