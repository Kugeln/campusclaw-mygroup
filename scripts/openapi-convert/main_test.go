package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

// Validate the delivered artifact, including enabled=false, non-null empty
// arrays and SSE enums. These are independent semantic checks of the converter.
func TestGeneratedInputAndEventContracts(t *testing.T) {
	doc, err := openapi3.NewLoader().LoadFromFile("../../internal/http/web/swagger/openapi.json")
	if err != nil {
		t.Fatal(err)
	}
	skill := doc.Paths.Find("/api/skills/{id}").Put.RequestBody.Value.Content.Get("application/json").Schema.Value
	for _, tc := range []struct {
		body  string
		valid bool
	}{
		{`{"enabled":false}`, true}, {`{"enabled":true}`, true}, {`{"source":"合法source"}`, true},
		{`{}`, false}, {`{"source":"x","enabled":false}`, false}, {`{"enabled":null}`, false}, {`{"source":null}`, false}, {`{"enabled":true,"class_id":2}`, false},
	} {
		var v any
		if json.Unmarshal([]byte(tc.body), &v) != nil {
			t.Fatal("test input")
		}
		err := skill.VisitJSON(v)
		if (err == nil) != tc.valid {
			t.Fatalf("%s valid=%t: %v", tc.body, tc.valid, err)
		}
	}
	agent := doc.Paths.Find("/api/agents").Post.RequestBody.Value.Content.Get("application/json").Schema.Value
	for _, tc := range []struct {
		prompt    string
		materials any
		valid     bool
	}{{"", []any{}, true}, {strings.Repeat("𠮷", 2000), []any{}, true}, {strings.Repeat("𠮷", 2001), []any{}, false}, {"", nil, false}} {
		v := map[string]any{"name": "测试", "prompt": tc.prompt, "enabled": false, "skill_ids": []any{}, "material_ids": tc.materials}
		if err := agent.VisitJSON(v); (err == nil) != tc.valid {
			t.Fatalf("empty role/array/rune bounds valid=%t: %v", tc.valid, err)
		}
	}
	meta := doc.Components.Schemas["httpapi.SSEMeta"].Value
	for _, status := range []string{"ok", "no_evidence"} {
		v := map[string]any{"status": status, "citations": []any{}, "skill_name": "", "message_id": float64(1)}
		if err := meta.VisitJSON(v); (err == nil) != (status == "ok") {
			t.Fatal("SSE status enum", err)
		}
	}
}
