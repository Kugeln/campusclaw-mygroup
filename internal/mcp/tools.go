package mcp

import (
	"campusclaw/internal/auth"
	"context"
	"encoding/json"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const StageBudget = 10 * time.Second
const MaxReferences = 3
const UnsafeText = "工具返回内容未通过安全检查，已忽略。"
const DeniedText = "工具调用未获允许。"
const ErrorText = "工具调用失败，已继续原问答。"

type ToolCall struct {
	Ref    string `json:"ref" binding:"required"`
	Status string `json:"status" binding:"required" enums:"ok,denied,error"`
	Text   string `json:"text" binding:"required"`
}

var reference = regexp.MustCompile(`^([a-z][a-z0-9_]{0,31})@([a-z][a-z0-9_-]{0,31})$`)

func Reference(ref string) (tool, server string, ok bool) {
	m := reference.FindStringSubmatch(ref)
	if m == nil {
		return "", "", false
	}
	return m[1], m[2], true
}
func ParseReferences(value string) ([]string, error) {
	out := []string{}
	seen := map[string]bool{}
	count := 0
	for _, part := range strings.FieldsFunc(value, func(r rune) bool { return strings.ContainsRune(",，、;；|", r) }) {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if seen[part] {
			return nil, Invalid
		}
		seen[part] = true
		if part != "knowledge_search" {
			if _, _, ok := Reference(part); !ok {
				return nil, Invalid
			}
			count++
			if count > MaxReferences {
				return nil, Invalid
			}
		}
		out = append(out, part)
	}
	return out, nil
}

var convertPattern = regexp.MustCompile(`([+-]?(?:[0-9]+(?:\.[0-9]+)?|\.[0-9]+)(?:[eE][+-]?[0-9]+)?)\s*(km|cm|mm|kg|mg|min|m|g|s|h)\s*(?:转换为|换算为|转为|换成|转|to|=)\s*(km|cm|mm|kg|mg|min|m|g|s|h)`)
var numeric = regexp.MustCompile(`[+-]?(?:[0-9]+(?:\.[0-9]+)?|\.[0-9]+)(?:[eE][+-]?[0-9]+)?`)
var dimensions = map[string]int{"m": 1, "cm": 1, "mm": 1, "km": 1, "g": 2, "kg": 2, "mg": 2, "s": 3, "min": 3, "h": 3}

func Arguments(tool, question string) map[string]any {
	switch tool {
	case "formula_hint":
		if utf8.ValidString(question) && utf8.RuneCountInString(question) >= 2 && utf8.RuneCountInString(question) <= 500 {
			return map[string]any{"query": question}
		}
	case "unit_convert":
		matches := convertPattern.FindAllStringSubmatch(question, -1)
		if len(matches) != 1 || len(numeric.FindAllString(question, -1)) != 1 {
			return nil
		}
		m := matches[0]
		value, e := strconv.ParseFloat(m[1], 64)
		if e != nil || math.IsNaN(value) || math.IsInf(value, 0) || math.Abs(value) > 1e12 || dimensions[m[2]] != dimensions[m[3]] {
			return nil
		}
		// Unit suffixes must be complete: "cmX" is ambiguous, not "cm".
		loc := convertPattern.FindStringIndex(question)
		if loc[1] < len(question) {
			r, _ := utf8.DecodeRuneInString(question[loc[1]:])
			if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' {
				return nil
			}
		}
		return map[string]any{"value": value, "from": m[2], "to": m[3]}
	}
	return nil
}

// SchemaArguments validates the supported deterministic adapter schema subset.
// Unknown validation keywords fail closed; remote descriptions never grant access.
func SchemaArguments(schema json.RawMessage, args map[string]any) bool {
	if args == nil || !strictJSON(schema) {
		return false
	}
	var root map[string]any
	if json.Unmarshal(schema, &root) != nil {
		return false
	}
	var validate func(map[string]any, any) bool
	validate = func(s map[string]any, value any) bool {
		for k := range s {
			switch k {
			case "type", "properties", "required", "additionalProperties", "enum", "minimum", "maximum", "minLength", "maxLength", "description", "title", "$schema", "default", "examples":
			default:
				return false
			}
		}
		if enum, ok := s["enum"].([]any); ok {
			found := false
			for _, v := range enum {
				a, _ := json.Marshal(v)
				b, _ := json.Marshal(value)
				found = found || string(a) == string(b)
			}
			if !found {
				return false
			}
		}
		switch s["type"] {
		case "object":
			v, ok := value.(map[string]any)
			if !ok {
				return false
			}
			properties, _ := s["properties"].(map[string]any)
			if required, exists := s["required"]; exists {
				list, ok := required.([]any)
				if !ok {
					return false
				}
				for _, r := range list {
					k, ok := r.(string)
					if !ok || v[k] == nil {
						return false
					}
				}
			}
			for key, item := range v {
				raw, exists := properties[key]
				if !exists {
					if s["additionalProperties"] == false {
						return false
					}
					continue
				}
				child, ok := raw.(map[string]any)
				if !ok || !validate(child, item) {
					return false
				}
			}
			return true
		case "string":
			v, ok := value.(string)
			if !ok || !utf8.ValidString(v) {
				return false
			}
			n := float64(utf8.RuneCountInString(v))
			if min, ok := s["minLength"].(float64); ok && n < min {
				return false
			}
			if max, ok := s["maxLength"].(float64); ok && n > max {
				return false
			}
			return true
		case "number", "integer":
			v, ok := value.(float64)
			if !ok || math.IsNaN(v) || math.IsInf(v, 0) {
				return false
			}
			if s["type"] == "integer" && math.Trunc(v) != v {
				return false
			}
			if min, ok := s["minimum"].(float64); ok && v < min {
				return false
			}
			if max, ok := s["maximum"].(float64); ok && v > max {
				return false
			}
			return true
		}
		return false
	}
	return validate(root, args)
}

var dangerous = regexp.MustCompile(`(?i)忽略前述|ignore\s+previous\s+instructions|你现在是|system\s*:`)
var credentialText = regexp.MustCompile(`(?i)(authorization["']?\s*[:=]\s*["']?(?:bearer\s+)?[^\s,;]+|(?:bearer|password|api[_-]?key|auth[_-]?token|secret)["']?\s*[:= ]\s*["']?[^\s,;]+)`)
var fakeCitation = regexp.MustCompile(`\[\s*[0-9]+\s*\]`)

func SafeResult(result CallResult, secrets ...string) ToolCall {
	out := ToolCall{Status: "error", Text: ErrorText}
	if result.IsError || len(result.Content) == 0 {
		return out
	}
	text := ""
	for _, c := range result.Content {
		text += c.Text
	}
	if !utf8.ValidString(text) || strings.TrimSpace(text) == "" || utf8.RuneCountInString(text) > 1200 || dangerous.MatchString(text) || fakeCitation.MatchString(text) {
		out.Text = UnsafeText
		return out
	}
	for _, secret := range secrets {
		if secret != "" {
			text = strings.ReplaceAll(text, secret, "[redacted]")
		}
	}
	text = credentialText.ReplaceAllString(text, "[redacted]")
	if utf8.RuneCountInString(text) > 1200 {
		out.Text = UnsafeText
		return out
	}
	out.Status, out.Text = "ok", text
	return out
}

type RuntimeStore interface {
	ExecuteTool(context.Context, auth.Principal, string, map[string]any, int64) (ToolCall, error)
}
type Runner struct{ Store RuntimeStore }

func (r Runner) Run(ctx context.Context, p auth.Principal, refs []string, question string, conversation int64) ([]ToolCall, error) {
	if len(refs) > MaxReferences {
		return nil, Invalid
	}
	stage, cancel := context.WithTimeout(ctx, StageBudget)
	defer cancel()
	out := []ToolCall{}
	for _, ref := range refs {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		tool, _, ok := Reference(ref)
		if !ok {
			return nil, Invalid
		}
		args := Arguments(tool, question)
		if stage.Err() != nil {
			args = nil
		}
		call, e := r.Store.ExecuteTool(stage, p, ref, args, conversation)
		if e != nil {
			return nil, e
		}
		out = append(out, call)
	}
	return out, nil
}
