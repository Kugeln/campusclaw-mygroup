package conversation

import (
	"campusclaw/internal/rag"
	"strings"
	"testing"
)

func TestNoEvidenceCrossFragmentCitations(t *testing.T) {
	for _, tc := range []struct{ in, out string }{
		{"解释[1]继续[203]。", "解释继续。"},
		{"普通[文字]（说明）和[]以及[12x]。", "普通[文字]（说明）和[]以及[12x]。"},
		{"[1][02][999]𠮷末尾[123", "𠮷末尾[123"},
		{"[[1]] [ ] [１２] (123)", "[] [ ] [１２] (123)"},
		{"开头[", "开头["},
	} {
		for width := 1; width <= len(tc.in)+1; width++ {
			f := CitationFilter{}
			out := ""
			for i := 0; i < len(tc.in); i += width {
				out += f.Feed(tc.in[i:min(i+width, len(tc.in))])
			}
			out += f.End()
			if out != tc.out {
				t.Fatalf("width %d: %q != %q", width, out, tc.out)
			}
		}
	}
	f := CitationFilter{}
	if got := f.Feed("已经输出文字"); got != "已经输出文字" {
		t.Fatal("whole-answer buffering")
	}
}

func TestAnswerCompletionValidation(t *testing.T) {
	for _, text := range []string{"", " \n", rag.NoEvidence, "  " + rag.NoEvidence + "\n"} {
		if _, err := ValidateAnswer(text, nil); err == nil {
			t.Fatalf("accepted %q", text)
		}
	}
	if sources, err := ValidateAnswer("按角色说明", nil); err != nil || sources == nil || len(sources) != 0 {
		t.Fatal(sources, err)
	}
	evidence := []rag.Evidence{{Title: "一"}, {Title: "二"}}
	for _, text := range []string{"无引用", "错误[0]", "错误[3]"} {
		if _, err := ValidateAnswer(text, evidence); err == nil {
			t.Fatal(text)
		}
	}
	if sources, err := ValidateAnswer("只用[2]", evidence); err != nil || len(sources) != 1 || sources[0].Number != 2 {
		t.Fatal(sources, err)
	}
	f := CitationFilter{}
	text := f.Feed("[1]") + f.End()
	if strings.TrimSpace(text) != "" {
		t.Fatal("numeric-only answer survived")
	}
}
