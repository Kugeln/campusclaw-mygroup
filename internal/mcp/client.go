package mcp

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"io"
	"mime"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"sync/atomic"
	"time"
	"unicode/utf8"
)

const ProtocolVersion = "2025-03-26"
const MaxPayload = 64 * 1024
const CallBudget = 3 * time.Second

var ToolName = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)
var ServerName = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,63}$`)

type Tool struct {
	Name        string          `json:"name" binding:"required"`
	Description string          `json:"description" binding:"required"`
	InputSchema json.RawMessage `json:"inputSchema" swaggertype:"object"`
}

func ValidateTools(tools []Tool) error {
	if len(tools) > 100 {
		return Invalid
	}
	names := map[string]bool{}
	for _, t := range tools {
		var schema map[string]json.RawMessage
		if !ToolName.MatchString(t.Name) || names[t.Name] || !utf8.ValidString(t.Description) || len(t.Description) > 16*1024 || !strictJSON(t.InputSchema) || json.Unmarshal(t.InputSchema, &schema) != nil || string(schema["type"]) != `"object"` {
			return Invalid
		}
		names[t.Name] = true
	}
	return nil
}

type CallResult struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	IsError bool `json:"isError"`
}
type Client struct {
	base, token string
	policy      *Policy
	sequence    atomic.Uint64
}

func NewClient(base, token string, policy *Policy) (*Client, error) {
	if _, _, e := policy.URL(base); e != nil {
		return nil, e
	}
	return &Client{base: base, token: token, policy: policy}, nil
}
func (c *Client) rpc(ctx context.Context, method string, params any, out any) error {
	ctx, cancel := context.WithTimeout(ctx, CallBudget)
	defer cancel()
	u, t, e := c.policy.URL(c.base)
	if e != nil {
		return e
	}
	ips, e := c.policy.Resolve(ctx, t)
	if e != nil {
		if ctx.Err() != nil {
			return Timeout
		}
		return e
	}
	id := strconv.FormatUint(c.sequence.Add(1), 10)
	request := struct {
		Version string `json:"jsonrpc"`
		ID      string `json:"id"`
		Method  string `json:"method"`
		Params  any    `json:"params"`
	}{"2.0", id, method, params}
	body, e := json.Marshal(request)
	if e != nil || len(body) > MaxPayload {
		return Invalid
	}
	// A fresh transport pins verified IPs; neither proxy settings nor DNS at dial
	// time can redirect the connection. No pooling bypasses subsequent DNS checks.
	transport := &http.Transport{Proxy: nil, DisableKeepAlives: true, TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12, ServerName: t.Host}, DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
		dialer := net.Dialer{}
		return dialer.DialContext(ctx, "tcp", net.JoinHostPort(ips[0].String(), strconv.Itoa(t.Port)))
	}}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	req, e := http.NewRequestWithContext(ctx, "POST", u.String(), bytes.NewReader(body))
	if e != nil {
		return Invalid
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	res, e := client.Do(req)
	if e != nil {
		if ctx.Err() != nil {
			return Timeout
		}
		return RemoteError
	}
	defer res.Body.Close()
	media, _, err := mime.ParseMediaType(res.Header.Get("Content-Type"))
	if res.StatusCode == 401 || res.StatusCode == 403 {
		return AuthFailed
	}
	if res.StatusCode != 200 || err != nil || media != "application/json" {
		return RemoteError
	}
	data, e := io.ReadAll(io.LimitReader(res.Body, MaxPayload+1))
	if ctx.Err() != nil {
		return Timeout
	}
	if e != nil || len(data) > MaxPayload || !strictJSON(data) {
		return RemoteError
	}
	var env map[string]json.RawMessage
	if json.Unmarshal(data, &env) != nil || string(env["jsonrpc"]) != `"2.0"` || string(env["id"]) != `"`+id+`"` {
		return RemoteError
	}
	_, ok := env["result"]
	_, failed := env["error"]
	if ok == failed || failed || string(env["result"]) == "null" {
		return RemoteError
	}
	if json.Unmarshal(env["result"], out) != nil {
		return RemoteError
	}
	return nil
}
func (c *Client) Initialize(ctx context.Context) error {
	var out struct {
		ProtocolVersion string                         `json:"protocolVersion"`
		Capabilities    map[string]json.RawMessage     `json:"capabilities"`
		ServerInfo      struct{ Name, Version string } `json:"serverInfo"`
	}
	if e := c.rpc(ctx, "initialize", map[string]any{"protocolVersion": ProtocolVersion, "capabilities": map[string]any{}, "clientInfo": map[string]string{"name": "campusclaw", "version": "week6"}}, &out); e != nil {
		return e
	}
	if out.ProtocolVersion != ProtocolVersion || out.ServerInfo.Name == "" || out.ServerInfo.Version == "" || out.Capabilities == nil {
		return RemoteError
	}
	var tools map[string]json.RawMessage
	if json.Unmarshal(out.Capabilities["tools"], &tools) != nil || tools == nil {
		return RemoteError
	}
	return nil
}
func (c *Client) List(ctx context.Context) ([]Tool, error) {
	var out struct {
		Tools *[]Tool `json:"tools"`
	}
	if e := c.rpc(ctx, "tools/list", map[string]any{}, &out); e != nil {
		return nil, e
	}
	if out.Tools == nil || ValidateTools(*out.Tools) != nil {
		return nil, RemoteError
	}
	return *out.Tools, nil
}
func (c *Client) Call(ctx context.Context, name string, args any) (CallResult, error) {
	var raw map[string]json.RawMessage
	var out CallResult
	if !ToolName.MatchString(name) {
		return out, Invalid
	}
	if e := c.rpc(ctx, "tools/call", map[string]any{"name": name, "arguments": args}, &raw); e != nil {
		return out, e
	}
	if raw == nil || raw["content"] == nil || (string(raw["isError"]) != "true" && string(raw["isError"]) != "false") {
		return out, RemoteError
	}
	b, _ := json.Marshal(raw)
	if json.Unmarshal(b, &out) != nil || len(out.Content) == 0 {
		return CallResult{}, RemoteError
	}
	var blocks []map[string]json.RawMessage
	if json.Unmarshal(raw["content"], &blocks) != nil {
		return CallResult{}, RemoteError
	}
	for _, block := range blocks {
		var text string
		if len(block["text"]) == 0 || string(block["text"]) == "null" || json.Unmarshal(block["text"], &text) != nil {
			return CallResult{}, RemoteError
		}
	}
	for _, t := range out.Content {
		if t.Type != "text" || !utf8.ValidString(t.Text) {
			return CallResult{}, RemoteError
		}
	}
	return out, nil
}
func strictJSON(data []byte) bool {
	if !utf8.Valid(data) || !json.Valid(data) {
		return false
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()
	var walk func(int) bool
	walk = func(depth int) bool {
		if depth > 32 {
			return false
		}
		v, e := d.Token()
		if e != nil {
			return false
		}
		if delim, ok := v.(json.Delim); ok {
			seen := map[string]bool{}
			for d.More() {
				if delim == '{' {
					k, e := d.Token()
					if e != nil {
						return false
					}
					s, ok := k.(string)
					if !ok || seen[s] {
						return false
					}
					seen[s] = true
				}
				if !walk(depth + 1) {
					return false
				}
			}
			_, e = d.Token()
			return e == nil
		}
		return true
	}
	return walk(0)
}
