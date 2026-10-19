package conversation

import (
	"campusclaw/internal/rag"
	"campusclaw/internal/tutor"
	"encoding/json"
	"strings"
	"testing"
)

// Exercise the actual Week5 prompt -> model JSON -> shared validation boundary.
// Candidate numbers intentionally differ from material chunk indexes.
func TestWeekFiveEvidenceCitationPromptAndIdentities(t *testing.T) {
	const material = "00000000-0000-4000-8000-000000000030"
	snapshot := tutor.Snapshot{ClassID: 1, Agent: tutor.Agent{ID: 10, MaterialIDs: []string{material}}}
	evidence := []rag.Evidence{
		{ChunkID: "00000000-0000-4000-8000-000000000031", MaterialID: material, Title: "equation-course.md", Index: 7, Text: "等式两边进行相同运算。"},
		{ChunkID: "00000000-0000-4000-8000-000000000032", MaterialID: material, Title: "equation-course.md", Index: 23, Text: "进一步说明。"},
	}
	prepared, err := tutor.Prepare(snapshot, evidence)
	if err != nil {
		t.Fatal(err)
	}
	messages, err := ModelMessages(prepared, nil, "请解释等式性质")
	if err != nil || len(messages) != 2 || messages[0].Role != "system" {
		t.Fatal("unexpected model messages", err)
	}
	for _, rule := range []string{
		"必须在相关结论或说明后标注对应的数字引用",
		"等式两边进行相同运算仍保持相等[1]",
		"编号必须取自本轮evidence中的number字段；number=2对应[2]，不是chunk_index",
		"仅写材料名称不能代替数字引用",
		"只引用实际使用的证据，不得编造、重编号或为凑格式引用未使用的资料",
	} {
		if !strings.Contains(messages[0].Content, rule) {
			t.Fatalf("missing evidenced citation rule: %s", rule)
		}
	}
	var payload struct {
		Question string           `json:"question"`
		Evidence []map[string]any `json:"evidence"`
	}
	if json.Unmarshal([]byte(messages[1].Content), &payload) != nil || len(payload.Evidence) != 2 {
		t.Fatal("evidence JSON missing")
	}
	if payload.Question != "请解释等式性质" {
		t.Fatal("question changed")
	}
	for i, item := range payload.Evidence {
		if len(item) != 4 || item["number"] != float64(i+1) || item["chunk_index"] != float64(evidence[i].Index) || item["title"] != evidence[i].Title || item["chunk_text"] != evidence[i].Text {
			t.Fatal("candidate number/chunk index or evidence fields changed", item)
		}
	}
	for _, tc := range []struct {
		text          string
		number, index int
		valid         bool
	}{
		{"来源为equation-course.md，答案x=4。", 0, 0, false},
		{"等式两边进行相同运算[1]。", 1, 7, true},
		{"只用第二条说明[2]。", 2, 23, true},
		{"把切片序号当编号[7]。", 0, 0, false},
		{"把切片序号当编号[23]。", 0, 0, false},
		{"非法[0]", 0, 0, false},
		{"越界[3]", 0, 0, false},
		{"虚构[999]", 0, 0, false},
		{"非法字面引用[x]", 0, 0, false},
		{"非约定全角引用［1］", 0, 0, false},
		{"合法与虚构混合[1][999]", 0, 0, false},
	} {
		t.Run(tc.text, func(t *testing.T) {
			sources, err := ValidateAnswer(tc.text, prepared.Evidence)
			if !tc.valid {
				if err == nil {
					t.Fatal("missing/invalid citation accepted")
				}
				return
			}
			if err != nil || len(sources) != 1 || sources[0].Number != tc.number || sources[0].Index != tc.index {
				t.Fatal("valid citation identity changed", sources, err)
			}
		})
	}
	empty, err := tutor.Prepare(snapshot, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"必须在相关结论或说明后标注", "number=2对应[2]", "仅写材料名称不能代替数字引用"} {
		if strings.Contains(empty.System, forbidden) {
			t.Fatal("evidenced requirement leaked into empty branch")
		}
	}
	if !strings.Contains(empty.System, "不得编造出处或使用[数字]引用") || empty.Answer != "" {
		t.Fatal("no-evidence generation instructions changed")
	}
	if sources, err := ValidateAnswer("没有本轮材料依据，下面按一般知识解释。", empty.Evidence); err != nil || len(sources) != 0 {
		t.Fatal("no-evidence answer requires citations", err)
	}
}
