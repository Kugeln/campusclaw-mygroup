# MCP infrastructure (Phase 2)

This package contains an HTTP JSON-RPC client, outbound policy and AES-GCM vault.
It registers no management routes, skill bindings, SSE hooks or model tools.

`NewPolicy` takes explicit scheme/host/port/CIDR targets. Base URLs have no path,
userinfo, query or fragment; trailing slashes are removed and `/mcp` is appended.
Every RPC resolves DNS again, rejects the entire answer if any address is unsafe
or outside the target CIDRs, then dials a verified IP directly. Redirects, proxies,
loopback, link-local/metadata, unspecified and multicast destinations are refused.
Configure the exact internal Compose subnet rather than allowing all private IPs.

`NewClient` holds the dedicated service credential, never a user's JWT. Initialize,
List and Call use cancellable three-second budgets, exact IDs and bounded UTF-8
JSON responses. Ordinary text is returned as untrusted data. RPC/HTTP/protocol
failures use fixed errors; tool `isError` remains distinguishable in the result.
No automatic request retries or ten-second turn orchestration are implemented.
Output injection filtering and safe model/SSE placement remain Phase 4 work.

`NewVault(activeID, keyring)` requires 32-byte AES keys. Each Seal uses a fresh
random nonce and authenticates the class/name binding. Retained key IDs can be
provided in the keyring; missing/wrong keys and ciphertext tampering fail closed.
Server rename decrypts under the previous binding and reseals under the new one.
No plaintext fallback exists. Do not log or serialize credentials or keyrings.

`LoadConfig` is available for later dependency assembly. With `MCP_ENABLED=true`
it requires `MCP_MASTER_KEY` (base64 of 32 random bytes), `MCP_KEY_ID`, and
`MCP_ALLOWED_TARGETS` (JSON array of scheme/host/port/cidrs). Inject secrets at
runtime through environment/Secret handling, independently of JWT_SIGNING_SECRET.
No real `.env` is read or changed. Enabling this configuration alone does not add
business routes in Phase 2. The RAG database readiness gate requires v4 regardless
of the MCP feature flag; disabling MCP is not a database rollback.

MySQL infrastructure is in `internal/repository/mysql/mcp*.go`. It accepts trusted
`auth.Principal` objects, exposes only safe server views, publishes discovery in a
transaction with a revision/operation lease, and appends terminal audit rows.
No stable-ID skill binding table, audit cleanup or public API is introduced.
An execution followed by process death before audit insertion can leave no row;
audit errors must be propagated by future orchestration without replaying calls.
