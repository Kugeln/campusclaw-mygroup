// Package server implements the course HTTP JSON-RPC subset, not full MCP transport.
package server

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"
)

const ProtocolVersion = "2025-03-26"
const MaxBody = 64 * 1024
const ToolTimeout = 3 * time.Second

type Config struct {
	Token          string
	AllowedOrigins []string
}

type Server struct {
	tokenHash [32]byte
	origins   map[string]bool
	registry  *Registry
}

func New(config Config, registry *Registry) (*Server, error) {
	if config.Token == "" || strings.ContainsAny(config.Token, " \t\r\n,") || registry == nil {
		return nil, errors.New("valid service credential and registry required")
	}
	s := &Server{tokenHash: sha256.Sum256([]byte(config.Token)), origins: map[string]bool{}, registry: registry}
	for _, origin := range config.AllowedOrigins {
		u, err := url.Parse(origin)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || origin != u.Scheme+"://"+u.Host {
			return nil, errors.New("invalid allowed origin")
		}
		s.origins[origin] = true
	}
	return s, nil
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type response struct {
	Version string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

func send(w http.ResponseWriter, id json.RawMessage, value any, code int, message string) {
	if len(id) == 0 {
		id = json.RawMessage("null")
	}
	v := response{Version: "2.0", ID: id, Result: value}
	if code != 0 {
		v.Result = nil
		v.Error = &rpcError{code, message}
	}
	_ = json.NewEncoder(w).Encode(v)
}

func httpError(w http.ResponseWriter, code int, text string) {
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": text})
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if r.URL.Path == "/health" && r.Method == "GET" {
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		return
	}
	if r.URL.Path != "/mcp" {
		httpError(w, 404, "Not found")
		return
	}
	if r.Method != "POST" {
		w.Header().Set("Allow", "POST")
		httpError(w, 405, "POST required")
		return
	}
	values := r.Header.Values("Authorization")
	valid := false
	if len(values) == 1 {
		parts := strings.Split(values[0], " ")
		if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") && parts[1] != "" {
			hash := sha256.Sum256([]byte(parts[1]))
			valid = subtle.ConstantTimeCompare(hash[:], s.tokenHash[:]) == 1
		}
	}
	if !valid {
		w.Header().Set("WWW-Authenticate", "Bearer")
		httpError(w, 401, "Service authentication required")
		return
	}
	// Server-to-server clients normally omit Origin. A supplied origin must match exactly.
	if origins := r.Header.Values("Origin"); len(origins) > 0 && (len(origins) != 1 || !s.origins[origins[0]]) {
		httpError(w, 403, "Origin not allowed")
		return
	}
	media, mediaParams, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if len(r.Header.Values("Content-Type")) != 1 || err != nil || media != "application/json" || (mediaParams["charset"] != "" && !strings.EqualFold(mediaParams["charset"], "utf-8")) {
		httpError(w, 415, "UTF-8 application/json required")
		return
	}
	if r.ContentLength > MaxBody {
		httpError(w, 413, "Request body too large")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, MaxBody)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			httpError(w, 413, "Request body too large")
		} else {
			httpError(w, 400, "Unable to read request")
		}
		return
	}
	if !utf8.Valid(body) || !json.Valid(body) {
		send(w, nil, nil, -32700, "Invalid UTF-8 JSON")
		return
	}
	if !uniqueKeys(body) {
		send(w, nil, nil, -32600, "Duplicate keys or excessive JSON nesting")
		return
	}
	var obj map[string]json.RawMessage
	if json.Unmarshal(body, &obj) != nil || obj == nil {
		send(w, nil, nil, -32600, "JSON-RPC object required")
		return
	}
	id := obj["id"]
	if !validID(id) {
		send(w, nil, nil, -32600, "String or numeric request id required")
		return
	}
	var version, method string
	if json.Unmarshal(obj["jsonrpc"], &version) != nil || version != "2.0" || json.Unmarshal(obj["method"], &method) != nil || method == "" {
		send(w, id, nil, -32600, "Invalid JSON-RPC request")
		return
	}
	for key := range obj {
		if key != "id" && key != "jsonrpc" && key != "method" && key != "params" {
			send(w, id, nil, -32600, "Unknown request field")
			return
		}
	}
	params := obj["params"]
	if len(params) == 0 {
		params = json.RawMessage("{}")
	}
	switch method {
	case "initialize":
		var p struct {
			ProtocolVersion string                     `json:"protocolVersion"`
			Capabilities    map[string]json.RawMessage `json:"capabilities"`
			ClientInfo      struct {
				Name    string `json:"name"`
				Version string `json:"version"`
			} `json:"clientInfo"`
		}
		if json.Unmarshal(params, &p) != nil || p.ProtocolVersion != ProtocolVersion || p.Capabilities == nil || p.ClientInfo.Name == "" || p.ClientInfo.Version == "" {
			send(w, id, nil, -32602, "Compatible protocolVersion, capabilities and clientInfo required")
			return
		}
		send(w, id, map[string]any{"protocolVersion": ProtocolVersion, "capabilities": map[string]any{"tools": map[string]bool{"listChanged": false}}, "serverInfo": map[string]string{"name": "campusclaw-course-mcp", "version": "1.0.0"}}, 0, "")
	case "tools/list":
		if _, err := object(params); err != nil {
			send(w, id, nil, -32602, "tools/list expects empty object params")
			return
		}
		send(w, id, map[string]any{"tools": s.registry.tools}, 0, "")
	case "tools/call":
		p, err := object(params, "name", "arguments")
		if err != nil {
			send(w, id, nil, -32602, "Tool name and arguments required")
			return
		}
		var name string
		if json.Unmarshal(p["name"], &name) != nil {
			send(w, id, nil, -32602, "Invalid tool name")
			return
		}
		for _, tool := range s.registry.tools {
			if tool.Name != name {
				continue
			}
			args, err := tool.validate(p["arguments"])
			if err != nil {
				send(w, id, nil, -32602, "Arguments do not match tool inputSchema")
				return
			}
			ctx, cancel := context.WithTimeout(r.Context(), ToolTimeout)
			defer cancel()
			if ctx.Err() != nil {
				send(w, id, result("工具执行已取消。", true), 0, "")
				return
			}
			// One invocation only. Buffered completion avoids a blocked sender on cancellation.
			completed := make(chan CallResult, 1)
			go func() {
				defer func() {
					if recover() != nil {
						completed <- result("工具执行失败。", true)
					}
				}()
				completed <- tool.execute(ctx, args)
			}()
			select {
			case value := <-completed:
				if ctx.Err() != nil {
					value = result("工具执行超时或已取消。", true)
				}
				send(w, id, value, 0, "")
			case <-ctx.Done():
				send(w, id, result("工具执行超时或已取消。", true), 0, "")
			}
			return
		}
		send(w, id, nil, -32602, "Unknown tool")
	default:
		send(w, id, nil, -32601, "Unknown method")
	}
}

func validID(id json.RawMessage) bool {
	if len(id) == 0 || bytes.Equal(id, []byte("null")) {
		return false
	}
	var value any
	d := json.NewDecoder(bytes.NewReader(id))
	d.UseNumber()
	if d.Decode(&value) != nil {
		return false
	}
	switch value.(type) {
	case string, json.Number:
		return true
	}
	return false
}

// encoding/json otherwise silently accepts duplicate object fields.
func uniqueKeys(body []byte) bool {
	d := json.NewDecoder(bytes.NewReader(body))
	d.UseNumber()
	var walk func(int) bool
	walk = func(depth int) bool {
		if depth > 32 {
			return false
		}
		t, err := d.Token()
		if err != nil {
			return false
		}
		if delim, ok := t.(json.Delim); ok {
			keys := map[string]bool{}
			for d.More() {
				if delim == '{' {
					key, err := d.Token()
					if err != nil {
						return false
					}
					k, ok := key.(string)
					if !ok || keys[k] {
						return false
					}
					keys[k] = true
				}
				if !walk(depth + 1) {
					return false
				}
			}
			_, err = d.Token()
			return err == nil
		}
		return true
	}
	return walk(0)
}
