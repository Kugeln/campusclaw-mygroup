package tutor

import (
	"campusclaw/internal/mcp"
	"fmt"
	"strings"
	"unicode/utf8"
)

// SkillText is the parsed course text format, not YAML or executable code.
// Source and Description retain the original bytes, including line endings.
type SkillText struct {
	Source      string   `json:"source"`
	Name        string   `json:"name"`
	Trigger     string   `json:"trigger"`
	Subject     string   `json:"subject"`
	Tools       []string `json:"tools"`
	Description string   `json:"description"`
	Enabled     bool     `json:"enabled"`
}

// ParseSkillSource validates a single UTF-8 skill without side effects. A blank
// line after the first metadata field starts the body. Leading blank/comment
// lines are ignored for parsing, but remain in Source for an exact round trip.
func ParseSkillSource(source string) (SkillText, error) {
	fail := func(message string) (SkillText, error) {
		return SkillText{}, fmt.Errorf("%w: %s", Invalid, message)
	}
	if !utf8.ValidString(source) {
		return fail("技能文件必须是有效的 UTF-8 文本")
	}
	if utf8.RuneCountInString(source) > 8000 {
		return fail("技能文件不能超过8000字")
	}
	fields := make(map[string]string)
	body := ""
	for offset := 0; offset < len(source); {
		end := strings.IndexByte(source[offset:], '\n')
		next := len(source)
		if end < 0 {
			end = len(source)
		} else {
			end += offset
			next = end + 1
		}
		line := strings.TrimSpace(source[offset:end])
		offset = next
		if line == "" {
			if len(fields) > 0 {
				body = source[next:]
				break
			}
			continue
		}
		if strings.HasPrefix(line, "#") {
			continue
		}
		separator := strings.IndexAny(line, ":：")
		if separator < 0 {
			return fail("元数据必须使用“字段：值”格式")
		}
		key := strings.TrimSpace(line[:separator])
		_, width := utf8.DecodeRuneInString(line[separator:])
		value := strings.TrimSpace(line[separator+width:])
		switch key {
		case "名称", "适用条件", "资料", "工具", "启用":
		default:
			return fail("未知元数据字段：“" + key + "”")
		}
		if _, exists := fields[key]; exists {
			return fail("重复元数据字段：“" + key + "”")
		}
		fields[key] = value
	}
	for _, limit := range []struct {
		key      string
		min, max int
	}{
		{"名称", 1, 32}, {"适用条件", 1, 120}, {"资料", 0, 32},
	} {
		length := utf8.RuneCountInString(fields[limit.key])
		if length < limit.min || length > limit.max {
			return fail(fmt.Sprintf("%s须为%d至%d字", limit.key, limit.min, limit.max))
		}
	}
	if fields["启用"] != "是" && fields["启用"] != "否" {
		return fail("启用字段必填，且只能为“是”或“否”")
	}
	if strings.TrimSpace(body) == "" || utf8.RuneCountInString(body) > 2000 {
		return fail("空行以下须有1至2000字的非空正文")
	}
	tools, err := mcp.ParseReferences(fields["工具"])
	if err != nil {
		return fail("工具引用格式、重复或数量不合法")
	}
	return SkillText{
		Source: source, Name: fields["名称"], Trigger: fields["适用条件"],
		Subject: fields["资料"], Tools: tools, Description: body, Enabled: fields["启用"] == "是",
	}, nil
}
