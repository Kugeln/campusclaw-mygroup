# Phase 4 material reads and startup recovery: 5.4–5.5

Date: 2026-10-09. Scope is only 5.4–5.5. Prior upload/authentication work is preserved. No UI, production ingress, migration cutover, or task 5.6/5.7 implementation.

## Material HTTP contract

- GET `/api/materials`: current authenticated principal's class is bound in SQL; items contain exactly `id`, `original_name`, `size`, `created_at`, `uploader`, ordered by created_at/id descending. Empty lists are `{"items":[]}`. Query scope parameters return 400 with `Client scope parameters are not accepted`.
- GET `/api/materials/{id}`: class-scoped material/body join, same five fields plus `plain_text`; no storage key/path. Original text is not rendered. Both roles can read their current class. Foreign and missing IDs return identical 404 `{"error":"Material not found"}` even with forged identity headers, cookies or query parameters.
- GET `/api/materials/{id}/file`: class query precedes filesystem access. Only canonical lowercase UUID storage keys and regular non-symlink files of the recorded size can open. The descriptor is checked before sending a success response. Original bytes, including BOM, are streamed with `text/plain; charset=utf-8`, attachment disposition, `nosniff`, and `private, no-store`. Unicode filenames use RFC 2231 encoding; control characters and path separators are removed from header metadata. No private directory is served statically.
- Read database failures, missing files and unsafe keys return a generic HTML 500 without internal details, matching Flask's read-route exception handling. Upload's existing JSON 500 contract is unchanged. Authentication errors retain Phase 3 behavior.
- **Existing cache-header discrepancy reported during implementation:** migration spec explicitly requires attachment `Cache-Control: private, no-store`. Flask's route sets that value, but `app/__init__.py` later overrides it with `no-store`. This implementation follows the unchanged migration spec; both forbid caching. No Flask change was made. Download disposition is semantically compatible, rather than byte-identical to Werkzeug's ASCII fallback plus extended filename encoding. Retrieval-specific `index_status` / `cleanup_required` extensions remain outside this migration's frozen contract.

## Recovery ordering and unknown commits

`cmd/server` calls `recovery.Startup` before listening. It first executes the existing read-only `transfer.Ready` schema/receipt/data/file gate. Unverified roots or databases cannot authorize file cleanup. The explicit infrastructure-only mode remains health-only and does not recover files.

After the gate, a serializable MySQL transaction locks all material rows/ranges and knowledge rows with current `FOR UPDATE` reads. These wait for outstanding insertion transactions, avoiding an old snapshot that could mistake an in-flight commit for an orphan. The recovery inventory checks missing/duplicate/mismatched-class body references, canonical keys, regular files, sizes and the SHA-256 of raw text after a leading BOM is removed. It validates every directory entry before beginning cleanup. Symlinks and unexpected directories fail the entire preflight.

Only then are staging files and confirmed unreferenced files removed. The same transaction is checked before each unlink, and candidate file identity/type is rechecked. No database rows are changed. Failure or timeout while resolving an outstanding transaction preserves its file and other pending evidence and prevents startup. Once a prior transaction commits, its referenced file is retained; once it rolls back, the confirmed orphan is removed. Recovery is repeatable and does not delete committed materials.

This is a sole-instance startup procedure, as required by design. The old writer must be stopped; it is not live maintenance or a distributed lock service. Operators must not run another uploader or modify this private directory concurrently. If an I/O failure occurs partway through cleanup, already-confirmed orphan removals can remain completed; startup still fails, and referenced material files are not cleanup candidates. A failing upload response can still represent an actually committed row; recovery does not make retries idempotent. No attempt is made to reconstruct missing original files or bodies.

## Tests and isolation

- `go test ./internal/storage ./internal/material ./internal/http ./internal/repository/mysql ./internal/recovery ./cmd/server` — PASS. Relevant existing tests plus new HTTP, repository fault/scan/scope tests, storage preflight and unsafe path tests. Windows symlink subcases run on the Linux verification runner instead. No unrelated full Go suite.
- `python scripts/verify_phase4_reads.py` — isolated MySQL 8.4.4 / Go 1.26.0 runner, schema created only after asserting the new database is empty, fresh fake accounts, temporary upload root, read-only source mount, no Demo mounts or host ports. Runs related Linux unit tests and `go test -mod=readonly -count=1 -timeout=180s -v -tags phase4readintegration ./internal/repository/mysql -run TestPhase4ReadRecoveryRealMySQL`.
- The real integration tests cover both roles and both classes, timestamp/ID ties, empty class and same-token current-class changes, Chinese disposition and original BOM bytes, query failures, missing original/body, wrong body, unsafe key/symlink/directory, unavailable/unverified startup, delayed commit/rollback, unresolved transaction timeout, idempotence and preservation of committed files. A compiled Go server is actually started; successful health/listener and authenticated detail are checked only after its recovery removed the fixture orphans.
- `python -m pytest -q tests/test_material_routes.py tests/test_recovery.py -k 'class_lists or scoped_details_and_files or private_attachment or bidirectional_class_isolation or recover_storage_and_refuse_incomplete'` — **6 passed, 4 deselected**, 33.18 seconds. Existing Flask tests unchanged; temporary SQLite/files only. Field/status expectations were checked against `app/routes.py`, `app/db.py`, `app/recovery.py`, and new Go assertions. This is targeted verification, not the Phase 5 shared differential suite.
- Demo read-only probes: `http://127.0.0.1:8080/health` and `/login` both 200. No login/upload/database or file writes against Demo.

The first real integration run passed in 5.362 seconds, using `campusclaw-phase4-read-f0124c0f`. A second run adds an explicit timeout-before-commit preservation assertion and uses `campusclaw-phase4-read-5300ac2a`. Both resource sets are retained (volume name is resource name plus `-data`); harness stops only its newly created MySQL container. No database, Docker volume or Demo file is deleted.

Final integration run: **PASS**, all subtests, 6.098 seconds. `openspec validate migrate-backend-to-go-mysql --strict`: **PASS**, `Change 'migrate-backend-to-go-mysql' is valid`. `git diff --check`: PASS (existing LF/CRLF advisory messages only). Tasks 5.4 and 5.5 are checked; total **21/30**. Tasks 5.6, 5.7 and Phase 5 remain unchecked.

Start-of-turn hashes covered 189 existing tracked/untracked files. Final comparison found only the intended existing-file edits (`cmd/server/main.go`, `internal/http/handler.go`, and this change's `tasks.md`); the course report, Flask/tests, other change artifacts, previous result records, Compose/Nginx and migration/schema files stayed unchanged. No reset/restore/clean/checkout, commit, push or archive was performed.

## Changed files

- `cmd/server/main.go`, `internal/http/handler.go`: startup and route wiring, preserving prior uncommitted auth/upload additions.
- `internal/http/material_reads.go`, `internal/http/material_reads_test.go`.
- `internal/material/read.go`.
- `internal/repository/mysql/material_reads.go`, `material_reads_test.go`, `material_reads_integration_test.go`, `material_recovery.go`.
- `internal/storage/read.go`, `recovery.go`, `recovery_test.go`.
- `internal/recovery/startup.go`.
- `scripts/verify_phase4_reads.py`.
- This record and the migration change's `tasks.md` only. Design/spec/proposal, other changes, schema, Flask, Compose/Nginx, dependencies and the course report are untouched.
