# Phase 3 completion — tasks 4.4–4.6 and 2.1

Scope: Bearer authentication, current MySQL identity, me, teacher guard and transactional version revocation. No Phase 4 routes, Flask edits, Nginx/Compose switch, migration-tool/schema changes, commit, push or archive. Previous Phase 3 login evidence remains in phase3-login-results.md.

## Behavior and Flask compatibility

- Each protected request accepts exactly one Authorization Bearer credential, verifies the existing JWT contract and queries the current user joined to a valid class. Missing/invalid/expired/tampered credentials or invalid/deleted users/version mismatches return 401 `{error: "Authentication required"}` with WWW-Authenticate: Bearer. Cookie, form/query tokens and client identity fields never authorize.
- Current role/class always come from MySQL. A principal has private fields and read-only accessors, stored under a private context key. Signed/client-supplied role/class cannot override it. Teacher guard returns 403 `{error: "Teacher role required"}` before an action executes.
- GET /api/auth/me returns only user_id, role, class_id and class_name. Auth responses use no-store. No Session repository, SID Cookie, anonymous session or CSRF is introduced.
- POST /api/auth/logout starts a MySQL transaction, atomically increments session_version, checks exactly one affected row and returns 204 with an empty body only after successful commit. Query/write/commit failures return 500 `{error: "Authentication storage unavailable"}` without database details. Any uncertain commit remains a failure response; it does not imply the Token is definitely still valid.
- Current schema and Flask implementation have no enabled/disabled column. Invalid state is represented by missing user, invalid role/class or invalid/mismatched version, as specified. No new disabled-state schema or account administration API was invented.
- Health bypasses authentication. The infrastructure mode stays health-only. No production teacher/material routes are added; a test-only handler verifies teacher rejection and trusted A/B scope. Full material resource isolation remains Phase 4.

## Tests actually run

1. `go test ./internal/auth ./internal/http ./internal/repository/mysql ./internal/config ./internal/security ./cmd/server` — PASS on local Go 1.27.1/windows-amd64. Includes existing JWT invalid-signature/claims/expiry tests and new middleware tests for missing/ambiguous Header, Cookie rejection, current identity, A/B class scope, teacher guard, live changes, invalid users, query/revoke errors, no sensitive logs, health independence and revocation. No real database used in this first run.
2. `python scripts/verify_phase3_auth.py` — PASS, exit 0. Uses mysql:8.4.4 and golang:1.26.0-alpine3.23; source bind mounted read-only. Builds the actual server, checks its dependency graph, then runs `go test -mod=readonly -count=1 -timeout=180s -v -tags phase3integration ./internal/repository/mysql -run TestPhase3RealMySQL` — PASS (5.385 seconds).
3. `openspec validate migrate-backend-to-go-mysql --strict` and scoped `git diff --check` — PASS.

Real-MySQL coverage:

- Requires an empty newly created database, prepares unchanged eight-table schema and fake fresh data. It refuses a nonempty database and has no DROP/TRUNCATE/database cleanup path.
- Starts the real server executable through its normal initialization gate with no SECRET_KEY. Login and me succeed. Missing DB_HOST or JWT_SIGNING_SECRET rejects startup before listening.
- Kills/restarts the Go process with the same database/key: unexpired Token survives; logout commits version 1; after another process restart both previously issued Tokens return 401 and a new login succeeds.
- Actual account queries and role/class changes use MySQL. An invalid negative version, changed version and deletion of an unreferenced test account reject authentication.
- The commit-failure test wraps the real driver transaction: executes UPDATE in InnoDB, performs real rollback, then reports an injected commit error. HTTP is 500 and stored version is unchanged. This is deterministic commit-failure injection, not a claim of packet-level network fault simulation.
- A real MySQL BIGINT overflow rejects UPDATE and leaves the prior version intact. Query unavailability is injected by closing a DB handle; the unit test also injects a query error. Neither is described as a MySQL server outage test.
- sessions remains empty throughout authentication. Application logs are checked for synthetic credentials; failures never print tokens or database error details.

## Phase 1 task 2.1

Accepted: existing cmd/internal boundary and locked dependencies retained; configuration unit tests pass, actual server builds and rejects missing required settings, normal initialized server starts and authenticates without SECRET_KEY using JWT_SIGNING_SECRET, and dependency listing excludes modernc.org, internal/migration and Python. This completes the previously deferred authentication acceptance, so 2.1 is now checked.

## Isolation and retained resources

Docker was initially stopped; started the user-installed Docker Desktop. Only new test resources were created:

- MySQL container/network prefix: `campusclaw-phase3-test-6ac52b75`
- Runner container: `campusclaw-phase3-test-6ac52b75-go`
- New test-only volume: `campusclaw-phase3-test-6ac52b75-data`

The MySQL container uses an internal network, has no published ports and mounts only its new named volume. The runner mounts this repository read-only and accesses no Demo database or upload directory. Test credentials are generated in memory. After acceptance the MySQL container was stopped; containers, network and volume were retained, with no database or Docker volume deletion. The runner is exited. The retained resources consume disk space and may be removed only in a separately authorized cleanup.

## Completion and limits

4.4–4.6 and 2.1 accepted; Phase 3 is 6/6; overall **16/30**. Existing HTTP fields/status/error bodies match the Flask Token contract reviewed in app/auth.py. This was source/contract comparison plus Go HTTP and real MySQL testing, not a live Flask/Go differential suite. No enabled field exists; no material API or browser-page migration was included. Already-authenticated in-flight requests are not rolled back on concurrent logout, consistent with the design. Stop before Phase 4.
