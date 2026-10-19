# CampusClaw course MCP service (Phase 1)

This independent Go module implements the frozen course HTTP JSON-RPC subset.
It does not implement full MCP Streamable HTTP, sessions, GET SSE, or an SDK.
No MySQL, Qdrant, model provider, or main CampusClaw process is required.

From `mcp-server/`, set `MCP_AUTH_TOKEN` in the process environment, then run
`go run ./cmd/mcp-server`. Do not put real credentials on the command line or
in source files. The default listener is `:8090`; `MCP_LISTEN_ADDR` can override it.
Missing/invalid credentials cause a nonzero exit. `GET /health` returns only
`{"status":"ok"}`. `POST /mcp` requires the dedicated service Bearer credential.
Student JWTs do not grant service access.

`MCP_ALLOWED_ORIGINS` optionally specifies comma-separated exact HTTP(S) origins
(no paths or wildcard). Absent Origin is allowed for server-to-server clients;
supplied Origin is denied unless explicitly allowed. No CORS browser access is
enabled implicitly. Content-Type must be `application/json`, optionally UTF-8.
Requests larger than 64 KiB return 413. Tools have a three-second cancellable
execution deadline and are never automatically retried. The main application's
ten-second tool-stage budget, discovery cache and Base URL adapter come later.

Example request bodies (send through an HTTP client with the runtime credential):

```json
{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"course-client","version":"1"}}}
{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}
{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"formula_hint","arguments":{"query":"去括号这一步我不会"}}}
{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"unit_convert","arguments":{"value":1,"from":"m","to":"cm"}}}
```

The last response has `content:[{"type":"text","text":"1 m = 100 cm"}]` and
`isError:false`. Formula hints cover brackets/distributivity and circle area,
with a general next-step fallback; they never evaluate arbitrary expressions.
Units: m/cm/mm/km, g/kg/mg, s/min/h; values are finite with absolute value at most
1e12. Wrong types/fields/units use JSON-RPC -32602. Known but incompatible unit
dimensions return a tool result with `isError:true`; this domain failure is
distinct from malformed JSON (-32700), invalid envelopes (-32600), and unknown
methods (-32601). Unknown tool names use -32602. No old unit_converter alias.
The service neither evaluates input as code nor reads files or calls a model.

IDs must be strings or JSON numbers and are echoed without numeric conversion;
missing/null/array/object/boolean IDs, duplicate JSON keys and batch requests are
rejected. Initialize requires the declared version, capabilities object and
nonempty clientInfo name/version. This stateless subset does not require a
session handshake to be remembered across subsequent HTTP requests.

Run `go test ./... -count=1` and `go vet ./...`. From the repository root,
`python mcp-server/run_phase1.py --docker` also builds a unique Compose project,
checks the resolved configuration, starts only its new MCP service, runs an
independent Go client on its private network, stops the service and verifies
unavailability, then removes only that owned project's containers/network.
Reports go to ignored `runtime/week6-phase1-<run>/`, never contain the ephemeral
credential, and record failures with nonzero exit. Docker access is required.
The build creates uniquely tagged test images, retained for review; no existing
images, containers, volumes or networks are removed. No `.env` is read or changed.

`compose.test.yml` has no published host ports, volumes or main-app services.
It requires an explicit unique project via `-p` and `MCP_TEST_PROJECT`, and a
runtime-only `MCP_AUTH_TOKEN`; the runner generates these automatically. It also
inspects occupied Docker subnets and selects a nonoverlapping private /24 via
`MCP_TEST_SUBNET`, without changing Docker's default address pools. The
Docker build copies only Go sources/module metadata, runs as UID 10001, and uses
a read-only scratch runtime. The container healthcheck targets default port 8090.
Timeout/cancellation fault tests inject a controlled executor in Go tests; the
production binary has no delay/fault tool or test control endpoint. They exercise
real TCP but are distinct from the separate production-process/network checks.

This fixture does not connect MCP to CampusClaw's application or default Compose.
Database, management API, skills, SSE, browser and model integration remain in
the later authorized phases.
