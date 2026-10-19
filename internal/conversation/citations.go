package conversation

import (
	"campusclaw/internal/rag"
	"strings"
	"unicode/utf8"
)

// CitationFilter holds only an ambiguous [digits prefix, plus an incomplete
// UTF-8 character. It never buffers a complete answer. ASCII numeric citations
// match the existing evidenced-answer grammar; ordinary brackets pass through.
type CitationFilter struct {
	pending []byte
	tail    []byte
}

func (f *CitationFilter) Feed(fragment string) string {
	out := append([]byte{}, f.tail...)
	f.tail = nil
	for _, b := range []byte(fragment) {
		if len(f.pending) > 0 {
			if b >= '0' && b <= '9' {
				f.pending = append(f.pending, b)
				continue
			}
			if b == ']' && len(f.pending) > 1 {
				f.pending = nil
				continue
			}
			out = append(out, f.pending...)
			f.pending = nil
		}
		if b == '[' {
			f.pending = []byte{'['}
		} else {
			out = append(out, b)
		}
	}
	// A controlled/custom stream may split a UTF-8 character between fragments.
	for i := 0; i < len(out); {
		if !utf8.FullRune(out[i:]) {
			f.tail = append([]byte{}, out[i:]...)
			out = out[:i]
			break
		}
		_, n := utf8.DecodeRune(out[i:])
		i += n
	}
	return string(out)
}

func (f *CitationFilter) End() string {
	out := string(f.tail) + string(f.pending)
	f.tail = nil
	f.pending = nil
	return out
}

func ValidateAnswer(text string, evidence []rag.Evidence) ([]rag.Source, error) {
	if len(evidence) > 0 {
		return rag.ValidateSources(text, evidence)
	}
	if !utf8.ValidString(text) || strings.TrimSpace(text) == "" || strings.TrimSpace(text) == rag.NoEvidence {
		return nil, rag.InvalidAnswer
	}
	return []rag.Source{}, nil
}
