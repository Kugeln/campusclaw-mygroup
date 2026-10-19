// Package rag implements the fourth-course algorithms without HTTP or SQL.
package rag

import (
	"context"
	"errors"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var Invalid = errors.New("A question and valid mode are required")
var Unavailable = errors.New("Retrieval dependency unavailable")
var InvalidAnswer = errors.New("Answer or citations unavailable")
var NotFound = errors.New("Material not found")
var Busy = errors.New("Material indexing in progress")

const NoEvidence = "资料中未找到相关内容"

type Scope struct {
	UserID, ClassID int64
	Role            string
}
type Evidence struct {
	ChunkID    string  `json:"chunk_id"`
	MaterialID string  `json:"material_id"`
	EntryID    int64   `json:"-"`
	Title      string  `json:"title"`
	Index      int     `json:"chunk_index"`
	Text       string  `json:"chunk_text"`
	Score      float64 `json:"score"`
}
type Strategy struct {
	Kind      string `json:"strategy"`
	Size      int    `json:"size"`
	Overlap   int    `json:"overlap"`
	Delimiter string `json:"delimiter,omitempty"`
}

func ParseStrategy(m map[string]any) (Strategy, error) {
	s := Strategy{Kind: "fixed", Size: 800}
	if m == nil {
		m = map[string]any{}
	}
	integer := func(v any) (int, bool) {
		switch n := v.(type) {
		case int:
			return n, true
		case float64:
			if !math.IsNaN(n) && !math.IsInf(n, 0) && n == math.Trunc(n) && n >= 0 && n <= 20*1024*1024 {
				return int(n), true
			}
		}
		return 0, false
	}
	if v, ok := m["strategy"]; ok {
		s.Kind, _ = v.(string)
	}
	if v, ok := m["size"]; ok {
		n, ok := integer(v)
		if !ok {
			return s, Invalid
		}
		s.Size = n
	}
	s.Overlap = s.Size / 10
	if v, ok := m["overlap"]; ok {
		n, ok := integer(v)
		if !ok {
			return s, Invalid
		}
		s.Overlap = n
	}
	s.Delimiter, _ = m["delimiter"].(string)
	if s.Size <= 0 || s.Overlap < 0 || s.Overlap >= s.Size || (s.Kind != "fixed" && s.Kind != "delimiter" && s.Kind != "markdown") || (s.Kind == "delimiter" && s.Delimiter == "") {
		return s, Invalid
	}
	return s, nil
}

var fenceStart = regexp.MustCompile("^ {0,3}(`{3,}|~{3,})")
var heading = regexp.MustCompile(`^#{1,6}(?:\s|$)`)

func Chunks(text string, s Strategy) []string {
	sections := []string{text}
	if s.Kind == "delimiter" {
		sections = strings.Split(text, s.Delimiter)
		for i := 0; i < len(sections)-1; i++ {
			sections[i] += s.Delimiter
		}
	}
	if s.Kind == "markdown" {
		sections = nil
		var b strings.Builder
		fence := ""
		for _, line := range strings.SplitAfter(text, "\n") {
			if fence != "" {
				closing := regexp.MustCompile("^ {0,3}" + regexp.QuoteMeta(string(fence[0])) + "{" + strconv.Itoa(len(fence)) + ",}\\s*$")
				if closing.MatchString(line) {
					fence = ""
				}
			} else if m := fenceStart.FindStringSubmatch(line); m != nil {
				fence = m[1]
			} else if heading.MatchString(line) && b.Len() > 0 {
				sections = append(sections, b.String())
				b.Reset()
			}
			b.WriteString(line)
		}
		sections = append(sections, b.String())
	}
	result := []string{}
	for _, section := range sections {
		r := []rune(section)
		for start := 0; start < len(r); start += s.Size - s.Overlap {
			end := min(start+s.Size, len(r))
			piece := string(r[start:end])
			if strings.TrimSpace(piece) != "" {
				result = append(result, piece)
			}
			if end == len(r) {
				break
			}
		}
	}
	return result
}

type Ranked struct {
	ID    string
	Score float64
}

func RRF(lists ...[]string) []Ranked {
	scores := map[string]float64{}
	for _, list := range lists {
		seen := map[string]bool{}
		rank := 0
		for _, id := range list {
			if seen[id] {
				continue
			}
			seen[id] = true
			rank++
			scores[id] += 1 / float64(60+rank)
		}
	}
	out := []Ranked{}
	for id, s := range scores {
		out = append(out, Ranked{id, s})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Score == out[j].Score {
			return out[i].ID < out[j].ID
		}
		return out[i].Score > out[j].Score
	})
	return out
}

type Store interface {
	Keyword(context.Context, int64, string, []string, int) ([]Evidence, error)
	Hydrate(context.Context, int64, []string, []string) (map[string]Evidence, error)
}
type Embedder interface {
	Embed(context.Context, string) ([]float64, error)
}
type Hit struct {
	ID      string         `json:"id"`
	Score   float64        `json:"score"`
	Payload map[string]any `json:"payload"`
}
type VectorSearch interface {
	Search(context.Context, int64, []float64, []string, int) ([]Hit, error)
}
type Retrieval struct {
	Store     Store
	Embedding Embedder
	Vectors   VectorSearch
}

func (r Retrieval) Search(ctx context.Context, s Scope, q, mode string, materials []string, limit int) ([]Evidence, error) {
	q = strings.TrimSpace(q)
	if q == "" || s.ClassID <= 0 || (mode != "keyword" && mode != "vector" && mode != "hybrid") {
		return nil, Invalid
	}
	keyword, vector := []Evidence{}, []Evidence{}
	var e error
	if mode != "vector" {
		keyword, e = r.Store.Keyword(ctx, s.ClassID, q, materials, 20)
		if e != nil {
			return nil, Unavailable
		}
	}
	if mode != "keyword" {
		v, e := r.Embedding.Embed(ctx, q)
		if e != nil {
			return nil, Unavailable
		}
		hits, e := r.Vectors.Search(ctx, s.ClassID, v, materials, 20)
		if e != nil {
			return nil, Unavailable
		}
		ids := []string{}
		for _, h := range hits {
			ids = append(ids, h.ID)
		}
		resolved, e := r.Store.Hydrate(ctx, s.ClassID, ids, materials)
		if e != nil {
			return nil, Unavailable
		}
		for _, h := range hits {
			entry, ok := resolved[h.ID]
			if !ok || strings.TrimSpace(entry.Text) == "" || math.IsNaN(h.Score) || math.IsInf(h.Score, 0) || h.Score < .35 {
				continue
			}
			p := h.Payload
			number := func(key string, want int64) bool { n, ok := p[key].(float64); return ok && n == float64(want) }
			if !number("class_id", s.ClassID) || !number("knowledge_entry_id", entry.EntryID) || !number("chunk_index", int64(entry.Index)) || p["material_id"] != entry.MaterialID || p["chunk_id"] != entry.ChunkID {
				continue
			}
			entry.Score = h.Score
			vector = append(vector, entry)
		}
	}
	ordered := func(entries []Evidence) []Evidence {
		sort.Slice(entries, func(i, j int) bool {
			if entries[i].Score == entries[j].Score {
				return entries[i].ChunkID < entries[j].ChunkID
			}
			return entries[i].Score > entries[j].Score
		})
		seen := map[string]bool{}
		out := []Evidence{}
		for _, v := range entries {
			if !seen[v.ChunkID] && strings.TrimSpace(v.Text) != "" && v.Score > 0 {
				seen[v.ChunkID] = true
				out = append(out, v)
			}
		}
		return out
	}
	keyword, vector = ordered(keyword), ordered(vector)
	if mode == "keyword" {
		return keyword[:min(limit, len(keyword))], nil
	}
	if mode == "vector" {
		return vector[:min(limit, len(vector))], nil
	}
	evidence := map[string]Evidence{}
	kl, vl := []string{}, []string{}
	for _, v := range keyword {
		kl = append(kl, v.ChunkID)
		evidence[v.ChunkID] = v
	}
	for _, v := range vector {
		vl = append(vl, v.ChunkID)
		evidence[v.ChunkID] = v
	}
	out := []Evidence{}
	for _, rank := range RRF(kl, vl) {
		v := evidence[rank.ID]
		v.Score = rank.Score
		out = append(out, v)
	}
	return out[:min(limit, len(out))], nil
}

type Dialogue interface {
	Answer(context.Context, string, []Evidence) (string, error)
}
type Source struct {
	Number     int    `json:"number"`
	ChunkID    string `json:"chunk_id"`
	MaterialID string `json:"material_id"`
	Title      string `json:"title"`
	Index      int    `json:"chunk_index"`
	Text       string `json:"chunk_text"`
}
type Answer struct {
	Text    string   `json:"answer"`
	Sources []Source `json:"sources"`
}

var citations = regexp.MustCompile(`\[(\d+)\]`)

// ValidateSources is shared by the legacy one-shot answer and streamed turns.
// It only publishes identifiers from this exact, already-authorized evidence.
func ValidateSources(text string, evidence []Evidence) ([]Source, error) {
	if strings.TrimSpace(text) == "" {
		return nil, InvalidAnswer
	}
	used := map[int]bool{}
	for _, match := range citations.FindAllStringSubmatch(text, -1) {
		n, e := strconv.Atoi(match[1])
		if e != nil || n < 1 || n > len(evidence) {
			return nil, InvalidAnswer
		}
		used[n] = true
	}
	if len(used) == 0 {
		return nil, InvalidAnswer
	}
	out := []Source{}
	for i, v := range evidence {
		if used[i+1] {
			out = append(out, Source{i + 1, v.ChunkID, v.MaterialID, v.Title, v.Index, v.Text})
		}
	}
	return out, nil
}

func Ask(ctx context.Context, r Retrieval, d Dialogue, s Scope, q, mode string) (Answer, error) {
	result := Answer{Sources: []Source{}}
	evidence, e := r.Search(ctx, s, q, mode, nil, 4)
	if e != nil {
		return result, e
	}
	if len(evidence) == 0 {
		result.Text = NoEvidence
		return result, nil
	}
	text, e := d.Answer(ctx, strings.TrimSpace(q), evidence)
	if e != nil || strings.TrimSpace(text) == "" {
		return result, InvalidAnswer
	}
	sources, e := ValidateSources(text, evidence)
	if e != nil {
		return result, e
	}
	result.Text = strings.TrimSpace(text)
	result.Sources = sources
	return result, nil
}
