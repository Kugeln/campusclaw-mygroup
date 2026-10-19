package main

import (
	"campusclaw/mcp-server/internal/checks"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"time"
)

func main() {
	base := os.Getenv("MCP_TEST_URL")
	if base == "" {
		base = "http://mcp-demo:8090"
	}
	if len(os.Args) == 2 && os.Args[1] == "offline" {
		client := http.Client{Timeout: 2 * time.Second}
		req, _ := http.NewRequest("POST", base+"/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+os.Getenv("MCP_AUTH_TOKEN"))
		res, err := client.Do(req)
		if res != nil {
			res.Body.Close()
		}
		passed := err != nil
		_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"name": "stopped_service_unavailable", "passed": passed})
		if !passed {
			os.Exit(1)
		}
		return
	}
	token := os.Getenv("MCP_AUTH_TOKEN")
	if token == "" {
		os.Exit(2)
	}
	results := checks.Run(base, token)
	_ = json.NewEncoder(os.Stdout).Encode(results)
	for _, r := range results {
		if !r.Passed {
			os.Exit(1)
		}
	}
}
