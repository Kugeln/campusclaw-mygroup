# Phase 0/1 verification results — 2026-09-27

Scope ends at infrastructure. No authentication, Session, CSRF, material APIs, business DDL, data migration or deployment was performed. Legacy Flask/SQLite implementation and tests remain unchanged. Approved proposal/design/spec and add-traceable-vector-retrieval hashes match the recorded baseline.

## Root cause and fixes

MySQL connector incorrectly used `Config.Params["charset"] = "utf8mb4"`. In locked driver v1.9.3, connection.go handleParams emits these values as SQL system-variable assignments (`SET charset = utf8mb4`), not DSN options. Replaced this with `Config.Apply(driver.Charset("utf8mb4", ""))`, which emits `SET NAMES utf8mb4`. The original generic startup diagnostic reported *mysql.MySQLError; after the fix the same empty-database integration successfully started Go. Startup logging now provides error type/numeric server code only, never raw messages or DSNs.

The next host-entry check failed while Nginx was attached only to the internal network. Nginx now attaches to frontend plus backend; app/mysql stay exclusively on the internal backend without published ports. Test HTTP probes bypass host proxy settings and allow a bounded port-readiness wait.

## Evidence

| Check | Result |
|---|---|
| Previous Flask baseline | 36 passed, 186.42s; not rerun in continuation |
| Pinned Go container `go test ./...` | Passed after connector fix; config and health tests cover missing/invalid config, healthy/unhealthy dependencies, no Cookie and no business routes |
| `go build -o /tmp/server ./cmd/server` | Passed after connector fix |
| `python scripts/verify_phase1.py` | Exit 0; isolated project campusclaw-phase1-test-80060c6b |
| Compose config and Go image build | Passed |
| `nginx -t` | Passed |
| No SECRET_KEY | Stack successfully starts without it |
| Host → Nginx → Go, live MySQL | 200 with {"status":"ok"}, no Set-Cookie |
| Stop MySQL, same Nginx endpoint | 503 with {"status":"unhealthy"}, no Set-Cookie |
| Missing DB_PASSWORD | Go exits nonzero with configuration key name only |
| Database down on new app startup | Go exits nonzero, generic diagnostic |
| Docker published-port inspection | app/mysql PortBindings empty; only Nginx loopback entry |
| Secret handling | Random test-only credentials injected in process environment; command output/container logs checked for both values and clean; no actual user credentials used or copied to repository |
| Isolation/cleanup | Random project, fresh private volumes, no old data mounts; test containers/volumes removed |
| Baseline SHA-256 verification | Old code/tests, approved contracts and retrieval artifacts unchanged |

Go toolchain ran in golang:1.26.0-alpine3.23; no host Go installation needed. Docker Desktop works outside the sandbox. One diagnostic rebuild hit a transient BuildKit failure; subsequent image build and integration passed. No remaining environment installation is required for these checks.

Tasks completed: 1.1, 1.2, 2.2, 2.3 (4/30). 2.1 remains unchecked because its authentication acceptance depends on Phase 3; its infrastructure portion is complete. Phase 2 schema/migration work has not started. No remaining infrastructure blocker to Phase 2 was observed; proceeding requires the user's next instruction.
