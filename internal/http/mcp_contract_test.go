package httpapi

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// Compare saved official operation contracts to generated annotations. Additional
// frozen fields (optional PUT token, tool schemas and audit detail) are allowed.
func TestMCPOpenAPIContract(t *testing.T) {
	data, e := documentation.ReadFile("web/swagger/openapi.json")
	if e != nil {
		t.Fatal(e)
	}
	var generated map[string]any
	if json.Unmarshal(data, &generated) != nil {
		t.Fatal("generated JSON")
	}
	raw, e := os.ReadFile("../../openspec/changes/add-week6-mcp-tool-integration/phase0-openapi-evidence.json")
	if e != nil {
		t.Fatal(e)
	}
	var official struct {
		Operations []struct {
			Method, Path string
			Contract     map[string]any
		}
		Schemas map[string]any
	}
	if json.Unmarshal(raw, &official) != nil {
		t.Fatal("official evidence")
	}
	object := func(v any) map[string]any { m, _ := v.(map[string]any); return m }
	schemas := object(object(generated["components"])["schemas"])
	resolve := func(v any, defs map[string]any) map[string]any {
		m := object(v)
		if ref, ok := m["$ref"].(string); ok {
			return object(defs[strings.TrimPrefix(ref, "#/components/schemas/")])
		}
		return m
	}
	var subset func(any, any)
	subset = func(a, b any) {
		want, got := resolve(a, official.Schemas), resolve(b, schemas)
		if want["type"] != nil && want["type"] != got["type"] {
			t.Fatalf("type mismatch %v / %v", want, got)
		}
		required := map[string]bool{}
		if list, ok := got["required"].([]any); ok {
			for _, k := range list {
				required[k.(string)] = true
			}
		}
		if list, ok := want["required"].([]any); ok {
			for _, k := range list {
				if !required[k.(string)] {
					t.Fatalf("missing required %s in %v", k, got)
				}
			}
		}
		for k, v := range object(want["properties"]) {
			field := object(got["properties"])[k]
			if field == nil {
				t.Fatal("missing official field", k)
			}
			subset(v, field)
		}
		if want["items"] != nil {
			subset(want["items"], got["items"])
		}
	}
	mediaSchema := func(v any) any { return object(object(object(v)["content"])["application/json"])["schema"] }
	count := 0
	for path, v := range object(generated["paths"]) {
		if strings.HasPrefix(path, "/api/mcp/") {
			count += len(object(v))
		}
	}
	if count != 8 || len(official.Operations) != 8 {
		t.Fatal("eight operations required")
	}
	for _, op := range official.Operations {
		t.Run(strings.ToLower(op.Method)+" "+op.Path, func(t *testing.T) {
			actual := object(object(object(generated["paths"])[op.Path])[strings.ToLower(op.Method)])
			if actual["x-error-examples"] == nil {
				t.Fatal("missing safe error examples")
			}
			if actual == nil {
				t.Fatal("missing official operation")
			}
			for code, response := range object(op.Contract["responses"]) {
				got := object(actual["responses"])[code]
				if got == nil {
					t.Fatal("missing status", code)
				}
				if schema := mediaSchema(response); schema != nil {
					subset(schema, mediaSchema(got))
				}
			}
			if body := op.Contract["requestBody"]; body != nil {
				subset(mediaSchema(body), mediaSchema(actual["requestBody"]))
			}
			if object(actual["responses"])["409"] != nil {
				t.Fatal("invented 409")
			}
		})
	}
	update := object(schemas["httpapi.MCPUpdateInput"])
	for _, k := range update["required"].([]any) {
		if k == "auth_token" {
			t.Fatal("PUT token must be optional")
		}
	}
	for _, name := range []string{"httpapi.MCPCreateInput", "httpapi.MCPUpdateInput"} {
		if object(schemas[name])["additionalProperties"] != false {
			t.Fatal("strict request missing")
		}
	}
	server := object(schemas["mcp.Server"])
	for _, key := range []string{"auth_token", "token_cipher", "token_nonce", "key_id", "revision"} {
		if object(server["properties"])[key] != nil {
			t.Fatal("secret field documented")
		}
	}
}
