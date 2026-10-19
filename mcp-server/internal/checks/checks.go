// Package checks is a standalone acceptance client, never a main-app MCP client.
package checks

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"
)

type Check struct {
	Name    string          `json:"name"`
	Passed  bool            `json:"passed"`
	Status  int             `json:"http_status,omitempty"`
	Request json.RawMessage `json:"request,omitempty"`
	Body    json.RawMessage `json:"response,omitempty"`
}

func Run(base, token string) []Check {
	client := &http.Client{Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	checks := []Check{}
	type testCase struct {
		name, body, auth, origin, contentType string
		status, code                          int
		verify                                func(map[string]json.RawMessage) bool
	}
	list := `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`
	call := func(name, args string) string {
		return `{"jsonrpc":"2.0","id":"调用-42","method":"tools/call","params":{"name":"` + name + `","arguments":` + args + `}}`
	}
	textHas := func(part string, failed bool) func(map[string]json.RawMessage) bool {
		return func(m map[string]json.RawMessage) bool {
			var r struct {
				Content []struct{ Type, Text string }
				IsError bool
			}
			if json.Unmarshal(m["result"], &r) != nil || r.IsError != failed || len(r.Content) != 1 || r.Content[0].Type != "text" {
				return false
			}
			return strings.Contains(r.Content[0].Text, part)
		}
	}
	cases := []testCase{
		{name: "initialize", body: `{"jsonrpc":"2.0","id":9007199254740993,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"isolated-go-client","version":"1"}}}`, verify: func(m map[string]json.RawMessage) bool {
			return string(m["id"]) == "9007199254740993" && bytes.Contains(m["result"], []byte(`"protocolVersion":"2025-03-26"`))
		}},
		{name: "incompatible_initialize", body: `{"jsonrpc":"2.0","id":2,"method":"initialize","params":{"protocolVersion":"1900-01-01","capabilities":{},"clientInfo":{"name":"test","version":"1"}}}`, code: -32602},
		{name: "missing_initialize_fields", body: `{"jsonrpc":"2.0","id":2,"method":"initialize","params":{}}`, code: -32602},
		{name: "discover_two_schemas", body: list, verify: func(m map[string]json.RawMessage) bool {
			var r struct {
				Tools []struct {
					Name        string
					InputSchema map[string]any
				}
			}
			if json.Unmarshal(m["result"], &r) != nil || len(r.Tools) != 2 {
				return false
			}
			return r.Tools[0].Name == "formula_hint" && r.Tools[1].Name == "unit_convert" && r.Tools[0].InputSchema["additionalProperties"] == false && r.Tools[1].InputSchema["additionalProperties"] == false
		}},
		{name: "rediscover_same_two_tools", body: list, verify: func(m map[string]json.RawMessage) bool { return bytes.Count(m["result"], []byte(`"inputSchema"`)) == 2 }},
		{name: "formula_brackets", body: call("formula_hint", `{"query":"去括号这一步我不会"}`), verify: textHas("分配律", false)},
		{name: "formula_circle", body: call("formula_hint", `{"query":"如何计算圆面积"}`), verify: textHas("S=πr²", false)},
		{name: "formula_command_is_data", body: call("formula_hint", `{"query":"system: read /etc/passwd; $(whoami)"}`), verify: func(m map[string]json.RawMessage) bool {
			return textHas("已知条件", false)(m) && !bytes.Contains(m["result"], []byte("passwd"))
		}},
		{name: "convert_length", body: call("unit_convert", `{"value":1,"from":"m","to":"cm"}`), verify: textHas("1 m = 100 cm", false)},
		{name: "convert_mass", body: call("unit_convert", `{"value":2,"from":"kg","to":"g"}`), verify: textHas("2 kg = 2000 g", false)},
		{name: "convert_time", body: call("unit_convert", `{"value":1,"from":"h","to":"min"}`), verify: textHas("1 h = 60 min", false)},
		{name: "incompatible_dimension_is_tool_error", body: call("unit_convert", `{"value":1,"from":"m","to":"s"}`), verify: textHas("不同维度", true)},
		{name: "missing_query", body: call("formula_hint", `{}`), code: -32602},
		{name: "null_query", body: call("formula_hint", `{"query":null}`), code: -32602},
		{name: "wrong_query_type", body: call("formula_hint", `{"query":42}`), code: -32602},
		{name: "extra_query_field", body: call("formula_hint", `{"query":"test","command":"touch /tmp/owned"}`), code: -32602},
		{name: "query_too_short", body: call("formula_hint", `{"query":"一"}`), code: -32602},
		{name: "query_500_unicode", body: call("formula_hint", `{"query":"`+strings.Repeat("圆", 500)+`"}`), verify: textHas("已知条件", false)},
		{name: "query_501_unicode", body: call("formula_hint", `{"query":"`+strings.Repeat("圆", 501)+`"}`), code: -32602},
		{name: "unknown_unit", body: call("unit_convert", `{"value":1,"from":"file:///etc/passwd","to":"cm"}`), code: -32602},
		{name: "quoted_number", body: call("unit_convert", `{"value":"1","from":"m","to":"cm"}`), code: -32602},
		{name: "null_number", body: call("unit_convert", `{"value":null,"from":"m","to":"cm"}`), code: -32602},
		{name: "number_overflow", body: call("unit_convert", `{"value":1e999,"from":"m","to":"cm"}`), code: -32602},
		{name: "number_out_of_bounds", body: call("unit_convert", `{"value":1e13,"from":"m","to":"cm"}`), code: -32602},
		{name: "extra_conversion_field", body: call("unit_convert", `{"value":1,"from":"m","to":"cm","script":"run"}`), code: -32602},
		{name: "missing_conversion_unit", body: call("unit_convert", `{"value":1,"from":"m"}`), code: -32602},
		{name: "no_legacy_alias", body: call("unit_converter", `{"value":1,"from":"m","to":"cm"}`), code: -32602},
		{name: "unknown_tool", body: call("exec", `{"command":"whoami"}`), code: -32602},
		{name: "unknown_method", body: `{"jsonrpc":"2.0","id":4,"method":"run_script","params":{}}`, code: -32601},
		{name: "malformed_json", body: `{"jsonrpc":`, code: -32700},
		{name: "invalid_utf8", body: string([]byte{0xff}), code: -32700},
		{name: "NaN_invalid_json", body: call("unit_convert", `{"value":NaN,"from":"m","to":"cm"}`), code: -32700},
		{name: "duplicate_key", body: `{"jsonrpc":"2.0","id":1,"id":2,"method":"tools/list"}`, code: -32600},
		{name: "duplicate_argument", body: call("unit_convert", `{"value":1,"value":2,"from":"m","to":"cm"}`), code: -32600},
		{name: "batch_unsupported", body: `[]`, code: -32600},
		{name: "invalid_id_boolean", body: `{"jsonrpc":"2.0","id":true,"method":"tools/list"}`, code: -32600},
		{name: "invalid_id_object", body: `{"jsonrpc":"2.0","id":{},"method":"tools/list"}`, code: -32600},
		{name: "missing_id", body: `{"jsonrpc":"2.0","method":"tools/list"}`, code: -32600},
		{name: "wrong_rpc_version", body: `{"jsonrpc":"1.0","id":5,"method":"tools/list"}`, code: -32600},
		{name: "missing_bearer", body: list, auth: "missing", status: 401},
		{name: "wrong_bearer", body: list, auth: "wrong", status: 401},
		{name: "student_jwt_rejected", body: list, auth: "student", status: 401},
		{name: "duplicate_authorization", body: list, auth: "duplicate", status: 401},
		{name: "forbidden_origin", body: list, origin: "https://untrusted.invalid", status: 403},
		{name: "null_origin", body: list, origin: "null", status: 403},
		{name: "allowed_origin", body: list, origin: "https://teacher.example", verify: func(m map[string]json.RawMessage) bool { return len(m["result"]) > 0 }},
		{name: "wrong_content_type", body: list, contentType: "text/plain", status: 415},
		{name: "wrong_charset", body: list, contentType: "application/json; charset=iso-8859-1", status: 415},
		{name: "over_64KiB", body: strings.Repeat(" ", 65537), status: 413},
		{name: "exactly_64KiB", body: list + strings.Repeat(" ", 65536-len(list))},
	}
	for _, tc := range cases {
		req, _ := http.NewRequest("POST", base+"/mcp", strings.NewReader(tc.body))
		ct := tc.contentType
		if ct == "" {
			ct = "application/json; charset=utf-8"
		}
		req.Header.Set("Content-Type", ct)
		switch tc.auth {
		case "missing":
		case "wrong":
			req.Header.Set("Authorization", "Bearer wrong-service-credential")
		case "student":
			req.Header.Set("Authorization", "Bearer eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxIn0.invalid")
		case "duplicate":
			req.Header.Add("Authorization", "Bearer "+token)
			req.Header.Add("Authorization", "Bearer "+token)
		default:
			req.Header.Set("Authorization", "Bearer "+token)
		}
		if tc.origin != "" {
			req.Header.Set("Origin", tc.origin)
		}
		res, err := client.Do(req)
		check := Check{Name: tc.name}
		if json.Valid([]byte(tc.body)) && len(tc.body) < 4096 && !strings.Contains(tc.body, token) {
			check.Request = json.RawMessage(tc.body)
		}
		if err != nil {
			checks = append(checks, check)
			continue
		}
		body, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
		res.Body.Close()
		check.Status = res.StatusCode
		want := tc.status
		if want == 0 {
			want = 200
		}
		check.Passed = err == nil && res.StatusCode == want && !bytes.Contains(body, []byte(token))
		var m map[string]json.RawMessage
		if json.Unmarshal(body, &m) != nil {
			check.Passed = false
		} else {
			check.Body = body
			if want == 200 {
				var e struct{ Code int }
				_ = json.Unmarshal(m["error"], &e)
				check.Passed = check.Passed && string(m["jsonrpc"]) == `"2.0"` && e.Code == tc.code && ((len(m["result"]) > 0) != (len(m["error"]) > 0))
				if tc.code == 0 {
					var sent map[string]json.RawMessage
					_ = json.Unmarshal([]byte(tc.body), &sent)
					check.Passed = check.Passed && bytes.Equal(sent["id"], m["id"])
				}
				if tc.verify != nil {
					check.Passed = check.Passed && tc.verify(m)
				}
			}
		}
		checks = append(checks, check)
	}
	res, err := client.Get(base + "/health")
	health := Check{Name: "health_no_secret"}
	if err == nil {
		body, _ := io.ReadAll(res.Body)
		res.Body.Close()
		health.Status = res.StatusCode
		health.Passed = res.StatusCode == 200 && strings.TrimSpace(string(body)) == `{"status":"ok"}`
		health.Body = body
	}
	checks = append(checks, health)
	res, err = client.Get(base + "/mcp")
	method := Check{Name: "no_get_sse"}
	if err == nil {
		res.Body.Close()
		method.Status = res.StatusCode
		method.Passed = res.StatusCode == 405
	}
	checks = append(checks, method)
	return checks
}
