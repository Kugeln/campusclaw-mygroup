# Isolated task 6.2 rehearsal

This is an executable rehearsal, not the default deployment or a production
cutover/rollback procedure. It does not implement tasks 6.3 or 6.4.

From the repository root (PowerShell):

```powershell
.\.venv\Scripts\python.exe -m unittest discover -s tests/cutover -p test_gate.py -v
go test -mod=readonly ./internal/migration ./internal/transfer
.\.venv\Scripts\python.exe tests/cutover/run.py
openspec validate migrate-backend-to-go-mysql --strict
```

Requires the existing locked Python dependencies, Docker Compose v2, and Go
dependencies from the unchanged `go.mod`/`go.sum`. The rehearsal image builds
server/admin/migrate with Go 1.26.0; it uses Alpine 3.23.3, MySQL 8.4.4 and
Nginx 1.28.0-alpine. If the default Go proxy is unavailable, set the process-only
`CUTOVER_GOPROXY` for the Docker build (and `GOPROXY` for a host Go test):

```powershell
$env:CUTOVER_GOPROXY = 'https://goproxy.cn'
.\.venv\Scripts\python.exe tests/cutover/run.py
```

Versions/checksums remain locked; neither sum verification nor TLS is disabled.
No permanent Go config or dependency file is changed.

Each invocation generates a new `campusclaw-phase5-cutover-<hex>` Compose
project and saves the complete rehearsal Compose file under `runtime/<name>`.
It never loads the default Compose file. MySQL and app have no host ports;
only Nginx has a randomly assigned loopback port, explicitly excluding 8080.
The frontend/backend networks, MySQL data volume and target upload volume are
new project-scoped resources. The backend network is internal. Actual container
mount/port metadata is audited and saved without reading secret environment
values into the report. Nginx mounts only its configuration, never uploads.

The source is a new host Flask server with `MATERIAL_BACKEND=sqlite`, synthetic
data from `tests/http_contract/fixtures.json`, nonempty placeholder tables,
nonzero account versions and an inert old sessions row. No RAG source, provider,
Qdrant, Demo volume or real credentials are used. Source login/logout is exercised
before source HTTP is stopped; SQLite backup and matching file copies are then
made with no writer. Snapshot mounts in migration containers are read-only.
Go receives only the distinct target volume. Offline tool containers finish
before Go starts, and Go stops before the final offline verification.

The same isolated ingress starts in maintenance (503). Faults are injected
before handover: a corrupt source copy, an insert-rejecting trigger on the new
target, and corrupted target body content. Each must fail without a success
report or opening ingress. The trigger is removed only from the owned database;
the corrupted target body is restored only from the owned fixture and reverified.
No database or volume is dropped. A failed import also proves normal Go startup
refuses its unverified target. Pending migration evidence is not a success receipt.

Successful preflight/import/verify use the actual `cmd/admin` and `cmd/migrate`
executables, not a second migration implementation. Eight tables are checked;
all seven data tables, original IDs, every field fingerprint, file SHA-256,
body SHA-256 and source association are compared. The target sessions table must
be empty; all account versions must equal the stopped source versions.

Only after verification and normal Go health does the runner write the separate
Nginx upstream config, run `nginx -t`, reload, and verify the same new port now
reaches Go. Source and target use different signing secrets. The real proxy
matrix tests old Token/old SID rejection, re-login, trusted identity, A/B and
teacher/student reads, original attachment bytes/Chinese filename, rejected
forged uploads, public page shells and private URL denial. It leaves data
unchanged, stops Go, then runs the offline verifier again against the source.

Failure exits 1 and closes the isolated ingress/stops its services. Success exits
0 only after all assertions. Either way, the report is saved as
`runtime/<name>/result.json`; migration reports, SQLite/file backups, images,
containers, networks and volumes remain. There is no automatic removal command.
Retained containers have generated test DB/JWT configuration; reports redact
secrets and full Tokens. The only Demo interaction is separate read-only GETs
to `/health` and `/login` before/after; no login, RAG request or data operation.

Rollback limits are recorded in the result document: keep writes stopped until
acceptance; switching a proxy back is not sufficient after new target writes or
version increments. Reconcile those writes/versions first, preserve backups, and
rotate signing secrets for re-login on either direction. This rehearsal does
not restore a stale snapshot, operate on real data, or automate production rollback.
