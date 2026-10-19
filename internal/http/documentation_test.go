package httpapi

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestDocumentationRoutesAndContract(t *testing.T) {
	h := Handler(healthy{})
	var spec struct {
		OpenAPI string                                `json:"openapi"`
		Paths   map[string]map[string]json.RawMessage `json:"paths"`
	}
	data, e := documentation.ReadFile("web/swagger/openapi.json")
	if e != nil || json.Unmarshal(data, &spec) != nil || spec.OpenAPI != "3.0.3" {
		t.Fatal("invalid document")
	}
	expected := map[string]bool{}
	// Enumerate actual mux registration expressions, including DELETE. No fixed
	// operation count or separately maintained endpoint list can mask drift.
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		source, err := parser.ParseFile(token.NewFileSet(), file, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(source, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || len(call.Args) < 1 {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || (selector.Sel.Name != "Handle" && selector.Sel.Name != "HandleFunc") {
				return true
			}
			literal, ok := call.Args[0].(*ast.BasicLit)
			if !ok || literal.Kind != token.STRING {
				return true
			}
			pattern, err := strconv.Unquote(literal.Value)
			if err != nil {
				t.Fatal(err)
			}
			parts := strings.SplitN(pattern, " ", 2)
			if len(parts) != 2 {
				return true
			}
			path := strings.TrimSuffix(parts[1], "{$}")
			if strings.HasPrefix(path, "/api/") || path == "/openapi.json" || path == "/swagger/index.html" || path == "/swagger/" {
				expected[strings.ToLower(parts[0])+" "+path] = true
			}
			return true
		})
	}
	for path, methods := range spec.Paths {
		for method, raw := range methods {
			if !expected[method+" "+path] {
				t.Fatal("invented operation", method, path)
			}
			delete(expected, method+" "+path)
			var op struct {
				Security  []map[string][]string      `json:"security"`
				Responses map[string]json.RawMessage `json:"responses"`
			}
			if json.Unmarshal(raw, &op) != nil || len(op.Responses) == 0 {
				t.Fatal("missing response")
			}
			if strings.HasPrefix(path, "/api/") && path != "/api/auth/login" && len(op.Security) != 1 {
				t.Fatal("missing Bearer", path)
			}
		}
	}
	if len(expected) != 0 {
		t.Fatal("undocumented registered routes", expected)
	}
	if _, exists := spec.Paths["/api/conversations/{id}"]["delete"]; exists {
		t.Fatal("public conversation DELETE invented")
	}
	for _, path := range []string{"/openapi.json", "/swagger/index.html", "/swagger/", "/swagger/init.js", "/swagger/vendor/swagger-ui-bundle.js", "/swagger/vendor/swagger-ui.css"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 200 || w.Body.Len() == 0 || w.Header().Get("Set-Cookie") != "" {
			t.Fatal("anonymous document", path)
		}
	}
	for _, path := range []string{"/swagger/.env", "/swagger/vendor/LICENSE", "/swagger/missing"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 404 {
			t.Fatal("document path escape", path)
		}
	}
}

func TestAlignedOpenAPISchemasAndSSE(t *testing.T) {
	data, e := documentation.ReadFile("web/swagger/openapi.json")
	if e != nil {
		t.Fatal(e)
	}
	var doc map[string]any
	if json.Unmarshal(data, &doc) != nil {
		t.Fatal("JSON")
	}
	obj := func(v any) map[string]any { m, _ := v.(map[string]any); return m }
	components := obj(doc["components"])
	schemas := obj(components["schemas"])
	resolve := func(v any) map[string]any {
		m := obj(v)
		if ref, ok := m["$ref"].(string); ok {
			return obj(schemas[strings.TrimPrefix(ref, "#/components/schemas/")])
		}
		return m
	}
	security := obj(obj(components["securitySchemes"])["bearerAuth"])
	if security["type"] != "http" || security["scheme"] != "bearer" {
		t.Fatal("Bearer conversion lost")
	}
	paths := obj(doc["paths"])
	operation := func(path, method string) map[string]any { return obj(obj(paths[path])[method]) }
	request := func(op map[string]any) map[string]any {
		return resolve(obj(obj(obj(op["requestBody"])["content"])["application/json"])["schema"])
	}
	agent := request(operation("/api/agents", "post"))
	prompt := obj(obj(agent["properties"])["prompt"])
	if prompt["maxLength"] != float64(2000) || prompt["minLength"] != nil || agent["additionalProperties"] != false || len(agent["required"].([]any)) != 5 {
		t.Fatal("empty role/complete arrays/strict input contract")
	}
	skill := request(operation("/api/skills/{id}", "put"))
	choices, ok := skill["oneOf"].([]any)
	if !ok || len(choices) != 2 {
		t.Fatal("toggle/source oneOf missing")
	}
	for _, choice := range choices {
		if obj(choice)["additionalProperties"] != false || len(obj(choice)["required"].([]any)) != 1 {
			t.Fatal("mixed toggle/source accepted")
		}
	}
	for _, route := range []struct{ path, method, success string }{{"/api/skills", "post", "201"}, {"/api/skills/{id}", "get", "200"}, {"/api/skills/{id}", "put", "200"}, {"/api/skills/{id}", "delete", "204"}} {
		op := operation(route.path, route.method)
		responses := obj(op["responses"])
		if responses[route.success] == nil || (route.method != "get" && responses["403"] == nil) {
			t.Fatal("skill lifecycle response/role omitted", route)
		}
	}
	stream := operation("/api/conversations/{id}/messages", "post")
	responses := obj(stream["responses"])
	if obj(obj(responses["200"])["content"])["text/event-stream"] == nil || obj(obj(responses["503"])["content"])["application/json"] == nil || obj(obj(responses["502"])["content"])["application/json"] == nil {
		t.Fatal("HTTP/SSE boundary drift")
	}
	events := obj(stream["x-sse-events"])
	meta := resolve(events["meta"])
	status := obj(obj(meta["properties"])["status"])
	values, ok := status["enum"].([]any)
	if !ok || len(values) != 1 || values[0] != "ok" || len(events) != 4 {
		t.Fatal("new SSE event schemas missing")
	}
	if obj(stream["x-sse"])["meta_before_model"] != true {
		t.Fatal("meta timing documentation missing")
	}
	materialErrors := obj(obj(operation("/api/materials/{id}/file", "get")["responses"])["500"])
	if obj(materialErrors["content"])["text/html"] == nil || obj(materialErrors["content"])["application/json"] == nil {
		t.Fatal("legacy storage HTML and identity JSON error media omitted")
	}
	if _, e := os.Stat("../../scripts/generate_week5_openapi.py"); !os.IsNotExist(e) {
		t.Fatal("second handwritten contract source remains")
	}
}

func TestDocumentedStrictFieldsMatchHandlers(t *testing.T) {
	data, err := documentation.ReadFile("web/swagger/openapi.json")
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Components struct {
			Schemas map[string]struct {
				Properties map[string]any `json:"properties"`
			} `json:"schemas"`
		} `json:"components"`
	}
	if json.Unmarshal(data, &doc) != nil {
		t.Fatal("JSON")
	}
	for _, tc := range []struct{ file, function, schema string }{{"tutor.go", "Save", "httpapi.AgentInput"}, {"conversation.go", "Create", "httpapi.CreateConversationInput"}, {"conversation.go", "Messages", "httpapi.SendMessageInput"}} {
		source, err := parser.ParseFile(token.NewFileSet(), tc.file, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		fields := map[string]bool{}
		for _, decl := range source.Decls {
			f, ok := decl.(*ast.FuncDecl)
			if !ok || f.Name.Name != tc.function {
				continue
			}
			ast.Inspect(f.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				name, ok := call.Fun.(*ast.Ident)
				if !ok || name.Name != "tutorObject" {
					return true
				}
				for _, arg := range call.Args[2:] {
					literal, ok := arg.(*ast.BasicLit)
					if !ok {
						t.Fatal("non-literal strict field")
					}
					key, err := strconv.Unquote(literal.Value)
					if err != nil {
						t.Fatal(err)
					}
					fields[key] = true
				}
				return true
			})
		}
		properties := doc.Components.Schemas[tc.schema].Properties
		if len(fields) == 0 || len(fields) != len(properties) {
			t.Fatal("handler/schema field drift", tc.function, fields)
		}
		for field := range fields {
			if properties[field] == nil {
				t.Fatal("undocumented strict field", tc.function, field)
			}
		}
	}
}
