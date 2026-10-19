package rag

import (
	"context"
	"math"
	"strings"
	"testing"
)

func TestChunkingAndRRF(t *testing.T) {
	s, e := ParseStrategy(nil)
	if e != nil {
		t.Fatal(e)
	}
	text := strings.Repeat("😀", 1601)
	c := Chunks(text, s)
	if len(c) != 3 || len([]rune(c[0])) != 800 || len([]rune(c[2])) != 161 {
		t.Fatal("Unicode windows")
	}
	if string([]rune(c[0])[720:]) != string([]rune(c[1])[:80]) {
		t.Fatal("overlap")
	}
	s, _ = ParseStrategy(map[string]any{"strategy": "delimiter", "delimiter": ".*"})
	c = Chunks("a.*b.*尾", s)
	if strings.Join(c, "") != "a.*b.*尾" || len(c) != 3 {
		t.Fatal(c)
	}
	s, _ = ParseStrategy(map[string]any{"strategy": "markdown"})
	c = Chunks("# 一\n正文\n```md\n# 围栏内\n```\n## 二\n尾", s)
	if len(c) != 2 || !strings.Contains(c[0], "围栏内") {
		t.Fatal(c)
	}
	for _, m := range []map[string]any{{"size": true}, {"size": 0}, {"size": 8, "overlap": 8}, {"overlap": -1}, {"strategy": "bad"}, {"strategy": "delimiter", "delimiter": ""}} {
		if _, e := ParseStrategy(m); e == nil {
			t.Fatal("invalid strategy accepted", m)
		}
	}
	ranks := RRF([]string{"a", "a", "b"}, []string{"b", "c", "a"})
	values := map[string]float64{}
	for _, v := range ranks {
		values[v.ID] = v.Score
	}
	if math.Abs(values["a"]-(1./61+1./63)) > 1e-12 || values["c"] != 1./62 {
		t.Fatal(values)
	}
}

type memoryStore struct{ items []Evidence }

func (s memoryStore) Keyword(context.Context, int64, string, []string, int) ([]Evidence, error) {
	return append([]Evidence{}, s.items...), nil
}
func (s memoryStore) Hydrate(context.Context, int64, []string, []string) (map[string]Evidence, error) {
	r := map[string]Evidence{}
	for _, e := range s.items {
		r[e.ChunkID] = e
	}
	return r, nil
}

type countingDialogue struct {
	calls int
	text  string
}

func (d *countingDialogue) Answer(context.Context, string, []Evidence) (string, error) {
	d.calls++
	return d.text, nil
}
func TestEvidenceAnswers(t *testing.T) {
	s := Scope{ClassID: 1}
	d := &countingDialogue{text: "说明 [1]"}
	r := Retrieval{Store: memoryStore{}}
	a, e := Ask(context.Background(), r, d, s, "问题", "keyword")
	if e != nil || a.Text != NoEvidence || len(a.Sources) != 0 || d.calls != 0 {
		t.Fatal(a, e)
	}
	r.Store = memoryStore{[]Evidence{{ChunkID: "a", MaterialID: "m", Title: "教材", Index: 1, Text: "正文", Score: 1}}}
	a, e = Ask(context.Background(), r, d, s, "问题", "keyword")
	if e != nil || len(a.Sources) != 1 || a.Sources[0].Text != "正文" {
		t.Fatal(a, e)
	}
	for _, text := range []string{"", "无引用", "[0]", "[2]"} {
		d.text = text
		if _, e = Ask(context.Background(), r, d, s, "问题", "keyword"); e != InvalidAnswer {
			t.Fatal(text, e)
		}
	}
}
