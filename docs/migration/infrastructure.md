# Phase 1 development infrastructure

Historical Phase 1 description below. Phase 2 now adds offline schema/migration/provisioning commands and a default initialization gate; see [Phase 2 operations](phase2-operations.md). The development Compose explicitly sets `STARTUP_MODE=infrastructure` to retain this health-only scaffold. It is not the eventual business deployment configuration.

Go currently implements only `GET /health`. Authentication, sessions, material APIs, SQL business schemas and migration commands are not implemented in this phase. Other paths return 404. This is not a cutover or a production deployment.

- `cmd/server`: wiring, configuration validation and startup dependency check.
- `internal/config`: environment parsing; no SECRET_KEY requirement.
- `internal/http`: HTTP mapping and generic health responses.
- `internal/health`: service combining repository connectivity and file probes.
- `internal/repository/mysql`: database/sql pool and MySQL connector; no DDL.
- `internal/storage`: temporary read/write probe only; no recovery/deletion of materials.
- `internal/security`: reserved package boundary; no authorization implemented.

Go 1.26.0, MySQL driver 1.9.3 and transitive dependency are pinned in go.mod/go.sum. Tool image: golang:1.26.0-alpine3.23. Runtime app is a scratch image with a non-root Go binary. MySQL 8.4.4, Nginx 1.28.0-alpine and the one-shot volume initializer alpine:3.23.3 are version-pinned. The initializer only prepares the new development volume directories; it is not a microservice and never mounts the legacy volume.

The new Compose project `campusclaw-go-dev` uses isolated `go-mysql` and `go-uploads` volumes. Only Nginx publishes a loopback port (8081 by default) and joins both frontend and internal backend networks. app/mysql stay on the internal backend with no host bindings. MySQL creates an empty development database, not the eight business tables. MySQL and storage must be available before Go listens; a subsequent outage gives generic 503 JSON. `/health` ignores cookies and does not create sessions.

Inject DB_PASSWORD and MYSQL_ROOT_PASSWORD through process environment, then use `docker compose up --build` for local development when desired. No real credentials belong in files. This phase does not require SECRET_KEY. Avoid `docker compose config` without `--quiet` when using secrets, since expanded configuration contains environment values.

The original Flask Dockerfile and code are unchanged. Use `docker compose -f docker-compose.legacy.yml` for the old runtime, retaining its original project/volume name. New Go Compose uses 8081 to avoid replacing the old 8080 entry point; final cutover belongs to Phase 5.

## Verification commands

With Go installed: `go test ./...` and `go build ./cmd/server`. Alternatively mount this repository at /src in the pinned Go tool image and run the commands there, writing the binary to /tmp/server.

`python scripts/verify_phase1.py` performs isolated Compose config/build/start, `nginx -t`, real MySQL health and outage checks, startup failure checks, published-port checks and generated-credential log checks. It creates random test-only credentials, a random project name and a temporary loopback port. It removes only that project's containers and volumes in a finally block. It never mounts old SQLite or upload data. Python here is a development test harness, not an application runtime dependency.

Phase 1 startup checks connectivity/storage only. Phase 2 will add business schema preparation and capability/constraint verification. Authentication without SECRET_KEY is only testable after Phase 3; the current configuration test verifies that omission is accepted without creating authentication behavior.
