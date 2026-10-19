package tutor

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"
)

func skillSource(name, trigger, subject, tools, enabled, body string) string {
	return "名称：" + name + "\n适用条件: " + trigger + "\n资料：" + subject + "\n工具：" + tools + "\n启用：" + enabled + "\n\n" + body
}

func TestParseSkillSourceRoundTrip(t *testing.T) {
	body := "  # 正文注释保持\r\n说明：包含:冒号\r\n\r\n尾行  \r\n"
	source := " \r\n # 教师备注\r\n\r\n名称: 解题引导\r\n适用条件：方程, A:B\r\n# 元数据注释\r\n资料: 数学\r\n工具：knowledge_search\r\n启用: 是\r\n \t\r\n" + body
	got, err := ParseSkillSource(source)
	want := SkillText{Source: source, Name: "解题引导", Trigger: "方程, A:B", Subject: "数学", Tools: []string{"knowledge_search"}, Description: body, Enabled: true}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("round trip: got %#v, error %v", got, err)
	}
	for _, source := range []string{
		"名称：简答\n适用条件：问题\n启用：否\n\n说明",
		"名称：简答\n适用条件：问题\n资料：\n工具：\n启用：否\n\n说明",
	} {
		got, err := ParseSkillSource(source)
		if err != nil || got.Enabled || got.Subject != "" || got.Tools == nil || len(got.Tools) != 0 {
			t.Fatalf("optional fields/disabled: %#v, %v", got, err)
		}
	}
}

func TestParseSkillSourceLimits(t *testing.T) {
	for _, tc := range []struct {
		field string
		max   int
	}{
		{"name", 32}, {"trigger", 120}, {"subject", 32}, {"body", 2000},
	} {
		for _, extra := range []int{0, 1} {
			t.Run(tc.field+strings.Repeat("+", extra), func(t *testing.T) {
				parts := map[string]string{"name": "技", "trigger": "题", "subject": "数", "body": "文"}
				// Supplementary-plane characters count as one rune, not bytes or UTF-16 units.
				parts[tc.field] = strings.Repeat("𠮷", tc.max+extra)
				source := skillSource(parts["name"], parts["trigger"], parts["subject"], "", "是", parts["body"])
				got, err := ParseSkillSource(source)
				if (err == nil) != (extra == 0) {
					t.Fatalf("boundary %s/%d: %v", tc.field, tc.max+extra, err)
				}
				if err == nil && got.Source != source {
					t.Fatal("source changed")
				}
			})
		}
	}
	base := skillSource("技", "题", "", "", "是", "文")
	for _, length := range []int{8000, 8001} {
		// Metadata comments allow testing the file limit independently of the body limit.
		source := "#" + strings.Repeat("字", length-utf8.RuneCountInString(base)-2) + "\n" + base
		if utf8.RuneCountInString(source) != length {
			t.Fatal("invalid test input length")
		}
		_, err := ParseSkillSource(source)
		if (err == nil) != (length == 8000) {
			t.Fatalf("source length %d: %v", length, err)
		}
	}
}

func TestParseSkillSourceRejectsMalformed(t *testing.T) {
	valid := skillSource("技", "题", "数学", "knowledge_search", "是", "正文")
	cases := map[string]string{
		"empty":                "",
		"comments_only":        "\n# 注释\n",
		"missing_name":         strings.Replace(valid, "名称：技\n", "", 1),
		"blank_name":           strings.Replace(valid, "名称：技", "名称： \t", 1),
		"missing_trigger":      strings.Replace(valid, "适用条件: 题\n", "", 1),
		"blank_trigger":        strings.Replace(valid, "适用条件: 题", "适用条件:", 1),
		"missing_enabled":      strings.Replace(valid, "启用：是\n", "", 1),
		"invalid_enabled":      strings.Replace(valid, "启用：是", "启用：true", 1),
		"unknown":              "脚本：run\n" + valid,
		"duplicate":            "名称: 重复\n" + valid,
		"duplicate_optional":   "资料：\n" + valid,
		"invalid_tool":         strings.Replace(valid, "knowledge_search", "shell", 1),
		"invalid_tool_in_list": strings.Replace(valid, "knowledge_search", "knowledge_search,shell", 1),
		"no_separator":         strings.Replace(valid, "\n\n", "\n", 1),
		"no_body":              strings.TrimSuffix(valid, "正文"),
		"blank_body":           strings.Replace(valid, "正文", " \r\n\t", 1),
		"yaml_frontmatter":     "---\n" + valid,
		"invalid_utf8":         valid + string([]byte{0xff}),
		"invalid_header_utf8":  string([]byte{0xff}) + valid,
	}
	for name, source := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := ParseSkillSource(source)
			if !errors.Is(err, Invalid) || !reflect.DeepEqual(got, SkillText{}) {
				t.Fatalf("expected readable Invalid with no partial result, got %#v / %v", got, err)
			}
		})
	}
}
