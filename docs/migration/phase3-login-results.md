# Phase 3 tasks 4.1–4.3 — login and JWT

Scope: only 4.1–4.3. No logout, protected-route middleware, me endpoint, material API, deployment, database mutation or Demo change. Historical Phase 0–2 evidence is unchanged.

## Implementation

- Reuse `provision.PasswordHash` unchanged for account creation. `security.VerifyPassword` accepts the migration-approved Werkzeug PBKDF2-SHA256/1,000,000 format, uses the stored salt string as UTF-8 bytes, and compares derived bytes in constant time.
- Read-only parameterized MySQL login query joins the current class. Teacher/student login uses current account version; unknown accounts and wrong passwords return the same 401 JSON. Database details are not logged or returned.
- Pin `github.com/golang-jwt/jwt/v5` to **v5.3.1** in go.mod/go.sum. Only HS256 is accepted. Required sub/ver/iat/exp are strictly checked, including integer precision, bounds, future issuance, exact 28800-second lifetime and exclusive expiry boundary.
- `POST /api/auth/login` accepts JSON, URL-encoded and multipart credentials. Success returns access_token, token_type=Bearer, expires_in=28800 and redirect=/materials. Success and failure use Cache-Control: no-store. No Cookie or CSRF dependency, no account/session writes, no sliding renewal.
- Normal server startup requires `JWT_SIGNING_SECRET` (nonblank, at least 32 UTF-8 bytes), before opening a database. No SECRET_KEY fallback. Configure it externally with random material; no usable default is supplied. Existing explicit infrastructure mode remains health-only and does not register login; offline admin/migration commands do not require a signing key.

## Verification

Environment: local Go 1.27.1/windows-amd64; module Go directive remains 1.26.0. No Docker command, live database connection, migration or full test suite was run.

Commands and results:

```text
go test ./internal/security ./internal/auth ./internal/config ./internal/http ./internal/repository/mysql ./cmd/server  # PASS
go test ./internal/auth ./internal/repository/mysql  # repository PASS; auth fixture missing (corrected below)
go test ./internal/auth  # PASS after correcting fixture generation
openspec validate migrate-backend-to-go-mysql --strict  # PASS
```

The initial package run passed password, token, configuration and HTTP tests and compiled the server/repository. After adding query-boundary and Flask-fixture tests, the repository passed; the new auth test initially failed because fixture generation froze app.time but not PyJWT's datetime clock. Freezing both clocks generated and verified the synthetic fixture, and the final auth rerun passed. No assertion was removed or weakened.

Coverage includes independent Python hashlib password vector, Phase 2 random-salt hashes, malformed hashes, missing/invalid secrets, JSON/form/multipart login, wrong/unknown credentials, input type errors, fixed error bodies and no logging, algorithm/signature/claim rejection, large integer precision, fixed-clock expiry, no renewal, repeated issuance not revoking prior tokens, and absent out-of-scope routes. A synthetic JWT produced and verified by existing `app/tokens.py` is accepted by Go, including an ID above 2^53. The fixture uses only test credentials and a historical frozen clock.

MySQL query testing uses a database/sql driver double to check bound username, class join, row scanning, absent users and sanitized failures; this is not a real-MySQL integration claim. True current-user/version authorization and persistent logout validation remain tasks 4.4–4.6.

Dependency download initially encountered default proxy connectivity failure; a process-local GOPROXY=https://goproxy.cn retry succeeded with checksum verification enabled. Initial test invocation could not write the default Go cache; the same targeted command passed with the required cache permissions.

## Compatibility and stopping point

No intended difference for the supported login/JWT contract: fields, status codes, lifetime, claims and no-store match Flask. Token serialization/key order need not be byte-identical. Password formats outside the migration-approved PBKDF2 format remain unsupported, as specified. This is not a complete Go authentication deployment: no GET /login page, me, logout or protected business routes were added, and no Nginx/Compose entry was switched.

Tasks 4.1–4.3 complete; overall **12/30**. Task 2.1 stays unchecked within the requested scope; 4.4–4.6 and Phase 4 remain untouched. No commit, push or archive.
