package server

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

const MaxTools = 100

var toolName = regexp.MustCompile(`^[a-z][a-z0-9_]{0,31}$`)
var invalidArguments = errors.New("invalid tool arguments")

type Text struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type CallResult struct {
	Content []Text `json:"content"`
	IsError bool   `json:"isError"`
}

type Tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
	validate    func(json.RawMessage) (any, error)
	execute     func(context.Context, any) CallResult
}

func result(text string, failed bool) CallResult {
	return CallResult{Content: []Text{{Type: "text", Text: text}}, IsError: failed}
}

// Registry is immutable after construction. Its order is also discovery order.
type Registry struct{ tools []Tool }

func NewRegistry(tools []Tool) (*Registry, error) {
	if len(tools) == 0 || len(tools) > MaxTools {
		return nil, errors.New("invalid tool registry size")
	}
	seen := map[string]bool{}
	for _, t := range tools {
		if !toolName.MatchString(t.Name) || seen[t.Name] || t.InputSchema == nil || t.validate == nil || t.execute == nil {
			return nil, errors.New("invalid or duplicate tool registration")
		}
		seen[t.Name] = true
	}
	return &Registry{tools: append([]Tool(nil), tools...)}, nil
}

type conversion struct {
	Value float64
	From  string
	To    string
}

type unit struct {
	dimension string
	factor    float64
}

var units = map[string]unit{
	"m": {"length", 1}, "cm": {"length", .01}, "mm": {"length", .001}, "km": {"length", 1000},
	"g": {"mass", 1}, "kg": {"mass", 1000}, "mg": {"mass", .001},
	"s": {"time", 1}, "min": {"time", 60}, "h": {"time", 3600},
}

func object(raw json.RawMessage, names ...string) (map[string]json.RawMessage, error) {
	var obj map[string]json.RawMessage
	if json.Unmarshal(raw, &obj) != nil || obj == nil || len(obj) != len(names) {
		return nil, invalidArguments
	}
	for _, name := range names {
		if v, ok := obj[name]; !ok || string(v) == "null" {
			return nil, invalidArguments
		}
	}
	return obj, nil
}

func Builtins() *Registry {
	querySchema := map[string]any{"type": "object", "required": []string{"query"}, "additionalProperties": false,
		"properties": map[string]any{"query": map[string]any{"type": "string", "minLength": 2, "maxLength": 500}}}
	convertSchema := map[string]any{"type": "object", "required": []string{"value", "from", "to"}, "additionalProperties": false,
		"properties": map[string]any{
			"value": map[string]any{"type": "number", "minimum": -1e12, "maximum": 1e12},
			"from":  map[string]any{"type": "string", "enum": []string{"m", "cm", "mm", "km", "g", "kg", "mg", "s", "min", "h"}},
			"to":    map[string]any{"type": "string", "enum": []string{"m", "cm", "mm", "km", "g", "kg", "mg", "s", "min", "h"}},
		}}
	registry, err := NewRegistry([]Tool{
		{Name: "formula_hint", Description: "提供去括号、圆面积及一般解题的公式或下一步提示，不代算最终答案。", InputSchema: querySchema,
			validate: func(raw json.RawMessage) (any, error) {
				obj, err := object(raw, "query")
				if err != nil {
					return nil, err
				}
				var q string
				if json.Unmarshal(obj["query"], &q) != nil || !utf8.ValidString(q) || utf8.RuneCountInString(q) < 2 || utf8.RuneCountInString(q) > 500 {
					return nil, invalidArguments
				}
				return q, nil
			},
			execute: func(ctx context.Context, value any) CallResult {
				if ctx.Err() != nil {
					return result("工具执行已取消。", true)
				}
				q := value.(string)
				switch {
				case strings.Contains(q, "括号"):
					return result("去括号可用分配律 a(b+c)=ab+ac。先检查括号前的系数是否乘到每一项，再检查负号引起的符号变化。", false)
				case strings.Contains(q, "圆") && strings.Contains(q, "面积"):
					return result("圆面积公式为 S=πr²。先确认已知量是半径还是直径；若是直径，先求半径，再检查面积单位。", false)
				default:
					return result("先列出已知条件与待求量，找到它们之间的等式或公式，再尝试只推进一步并检查单位和符号。", false)
				}
			}},
		{Name: "unit_convert", Description: "确定性换算同维度长度、质量或时间单位。", InputSchema: convertSchema,
			validate: func(raw json.RawMessage) (any, error) {
				obj, err := object(raw, "value", "from", "to")
				if err != nil {
					return nil, err
				}
				var a conversion
				// Unmarshal rejects quoted numbers, overflow, NaN and infinities.
				if json.Unmarshal(obj["value"], &a.Value) != nil || json.Unmarshal(obj["from"], &a.From) != nil || json.Unmarshal(obj["to"], &a.To) != nil || math.IsNaN(a.Value) || math.IsInf(a.Value, 0) || math.Abs(a.Value) > 1e12 {
					return nil, invalidArguments
				}
				if _, ok := units[a.From]; !ok {
					return nil, invalidArguments
				}
				if _, ok := units[a.To]; !ok {
					return nil, invalidArguments
				}
				return a, nil
			},
			execute: func(ctx context.Context, value any) CallResult {
				if ctx.Err() != nil {
					return result("工具执行已取消。", true)
				}
				a := value.(conversion)
				if units[a.From].dimension != units[a.To].dimension {
					return result("无法换算不同维度的单位。", true)
				}
				n := a.Value * units[a.From].factor / units[a.To].factor
				if math.IsNaN(n) || math.IsInf(n, 0) {
					return result("换算结果超出可用范围。", true)
				}
				return result(strconv.FormatFloat(a.Value, 'g', -1, 64)+" "+a.From+" = "+strconv.FormatFloat(n, 'g', -1, 64)+" "+a.To, false)
			}},
	})
	if err != nil {
		panic("invalid builtin registry")
	}
	return registry
}
