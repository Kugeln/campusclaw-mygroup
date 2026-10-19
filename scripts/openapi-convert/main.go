// Package main converts annotation output only. It has no route/schema list.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/getkin/kin-openapi/openapi2"
	"github.com/getkin/kin-openapi/openapi2conv"
	"github.com/getkin/kin-openapi/openapi3"
)

func object(v any) map[string]any { m, _ := v.(map[string]any); return m }
func resolve(s map[string]any, schemas map[string]any) map[string]any {
	if ref, ok := s["$ref"].(string); ok && strings.HasPrefix(ref, "#/components/schemas/") {
		return object(schemas[strings.TrimPrefix(ref, "#/components/schemas/")])
	}
	return s
}
func convert(data []byte) ([]byte, error) {
	var old openapi2.T
	if err := json.Unmarshal(data, &old); err != nil {
		return nil, err
	}
	doc, err := openapi2conv.ToV3(&old)
	if err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(doc)
	if err != nil {
		return nil, err
	}
	var root map[string]any
	if err = json.Unmarshal(encoded, &root); err != nil {
		return nil, err
	}
	root["openapi"] = "3.0.3"
	root["servers"] = []any{map[string]any{"url": "/"}}
	components := object(root["components"])
	schemas := object(components["schemas"])
	for _, value := range object(components["securitySchemes"]) {
		scheme := object(value)
		if scheme["type"] == "apiKey" && scheme["in"] == "header" && scheme["name"] == "Authorization" {
			scheme["type"] = "http"
			scheme["scheme"] = "bearer"
			scheme["bearerFormat"] = "JWT"
			delete(scheme, "name")
			delete(scheme, "in")
		}
	}
	for _, item := range object(root["paths"]) {
		for _, raw := range object(item) {
			op := object(raw)
			if op == nil {
				continue
			}
			body := object(op["requestBody"])
			for _, media := range object(body["content"]) {
				schema := resolve(object(object(media)["schema"]), schemas)
				if choices, ok := op["x-request-oneof"].([]any); ok {
					properties := object(schema["properties"])
					branches := []any{}
					for _, rawKeys := range choices {
						keys, ok := rawKeys.([]any)
						if !ok {
							return nil, fmt.Errorf("invalid annotated oneOf")
						}
						fields := map[string]any{}
						for _, key := range keys {
							name, ok := key.(string)
							if !ok || properties[name] == nil {
								return nil, fmt.Errorf("unknown annotated oneOf field")
							}
							fields[name] = properties[name]
						}
						branches = append(branches, map[string]any{"type": "object", "properties": fields, "required": keys, "additionalProperties": false})
					}
					delete(schema, "properties")
					delete(schema, "required")
					delete(schema, "type")
					schema["oneOf"] = branches
				}
				if strict, _ := op["x-strict-request"].(bool); strict && schema != nil && schema["oneOf"] == nil {
					schema["additionalProperties"] = false
				}
			}
			responses := object(op["responses"])
			if extension, ok := op["x-sse"].(map[string]any); ok {
				response := object(responses["200"])
				content := object(response["content"])
				var envelope map[string]any
				for _, media := range content {
					envelope = resolve(object(object(media)["schema"]), schemas)
				}
				if envelope == nil || len(object(envelope["properties"])) == 0 {
					return nil, fmt.Errorf("missing annotated SSE envelope")
				}
				op["x-sse-events"] = envelope["properties"]
				media := map[string]any{"schema": map[string]any{"type": "string"}}
				if examples := extension["examples"]; examples != nil {
					media["examples"] = examples
				}
				response["content"] = map[string]any{"text/event-stream": media}
				// Only the successful stream uses event-stream; pre-meta failures are JSON.
				for code, value := range responses {
					if code != "200" {
						r := object(value)
						for _, v := range object(r["content"]) {
							r["content"] = map[string]any{"application/json": v}
							break
						}
					}
				}
			}
			for code, name := range object(op["x-response-media"]) {
				response := object(responses[code])
				if response == nil {
					return nil, fmt.Errorf("unknown annotated response")
				}
				if mediaName, ok := name.(string); ok {
					for _, v := range object(response["content"]) {
						response["content"] = map[string]any{mediaName: v}
						break
					}
				} else if variants := object(name); variants != nil {
					content := map[string]any{}
					for kind, schema := range variants {
						content[kind] = map[string]any{"schema": schema}
					}
					response["content"] = content
				} else {
					return nil, fmt.Errorf("invalid annotated response media")
				}
			}
		}
	}
	out, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return nil, err
	}
	out = append(out, '\n')
	loader := openapi3.NewLoader()
	validated, err := loader.LoadFromData(out)
	if err != nil {
		return nil, err
	}
	if err = validated.Validate(context.Background()); err != nil {
		return nil, err
	}
	return out, nil
}
func main() {
	in := flag.String("in", "", "swag JSON input")
	out := flag.String("out", "", "OpenAPI3 artifact")
	check := flag.Bool("check", false, "fail instead of writing if artifact differs")
	flag.Parse()
	if *in == "" || *out == "" {
		fmt.Fprintln(os.Stderr, "-in and -out required")
		os.Exit(2)
	}
	data, err := os.ReadFile(*in)
	if err == nil {
		data, err = convert(data)
	}
	if err == nil {
		if *check {
			var current []byte
			current, err = os.ReadFile(*out)
			if err == nil && !bytes.Equal(current, data) {
				err = fmt.Errorf("OpenAPI artifact drift: regenerate from handler annotations")
			}
		} else {
			err = os.WriteFile(*out, data, 0644)
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
