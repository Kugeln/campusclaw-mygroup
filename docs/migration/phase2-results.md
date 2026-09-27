# Phase 2 interruption recovery and verification — 2026-09-27

Scope: MySQL Schema and Migration only. Phase 2 tasks 3.1–3.5 are complete; total 9/30. Task 2.1 remains unchecked because its authentication acceptance belongs to Phase 3. No Authentication, Go Session, CSRF, material HTTP API, deployment or archive was implemented/performed.

## Interruption evidence

The previous session launched `python scripts/verify_phase2.py` through PowerShell at approximately 14:57 Asia/Shanghai. Its last recorded output at 14:58:04 was `PASS isolated MySQL ready`, with execution session 39554 still running. The awaited Docker runner command was:

```sh
gofmt -w cmd internal && go mod tidy && go test -count=1 -v -tags integration ./... && go build -o /tmp/admin ./cmd/admin && go build -o /tmp/migrate ./cmd/migrate && go build -o /tmp/server ./cmd/server
```

The script captured all subprocess output without a command timeout, so no intermediate Go output reached the session. The log does not establish exactly which inner command was active when execution was stopped; no original integration success/exit result exists. A separate preceding ordinary Go test/build command had returned exit 0 and was not mistaken for integration acceptance.

Read-only recovery found `campusclaw-phase2-test-f9b5e455` running MySQL 8.4.4, with only `/var/lib/mysql` tmpfs, no host data mounts and no published port. Its private network remained. All eight tables existed; classes/users/materials/sessions were empty. No Python/Go migration runner or Go test container remained. No Phase 2 persistent volume existed. No workspace SQLite database, pending receipt, report temporary file or incomplete binary was found. The old exited Flask container and `campusclaw-mygroup_material-data` volume were left untouched; their data were not inspected.

The repository remains untracked as recorded in Phase 0, so git diff cannot identify a reliable implementation baseline. The existing baseline SHA-256 manifest was used instead: all 37 protected legacy implementation/test, approved proposal/design/spec, and retrieval-change files matched. No Flask baseline rerun was necessary.

## Existing work retained and completed

Retained the prior eight-table `schema_v1.sql`, MySQL connector strict/zero-ID mode, migration-only SQLite dependency and schema copies, `cmd/admin`, `cmd/migrate`, `internal/migration/source.go`, transfer snapshot/publication implementation, fresh/account provisioning and integration harness/tests. These files were coherent but Phase 2 tasks were all unchecked and no integration pass was recorded.

Added version-1 metadata validation; normal startup initialization checks before file probes/listening; physical SQLite snapshot association and typed field hashes; pending fresh receipt recovery; independent bounded commit reconciliation; receipt integrity checks that allow new rows and account/session changes; and additional schema, ordering, ID, corruption, active-session and commit-outcome tests. Explicit report outputs cannot overwrite existing files or source snapshots. Development Compose retains the Phase 1 health scaffold through explicit `STARTUP_MODE=infrastructure`; no business deployment is enabled by this setting.

The first recovery test run reproduced a concrete failure: `TestFreshAndAccountCommands` could not create its fault-injection trigger (MySQL code 1419). Earlier schema, migration, preflight and rollback tests passed. Enabled trigger creation only on the disposable harness database with `--log-bin-trust-function-creators=1`; application privileges/configuration were not expanded. This explains a reproducible test failure, but the absent original final log does not prove that this was the exact original stopping point.

The harness now mounts source read-only, omits formatting/dependency mutation during verification, bounds Docker commands and Go tests, emits 15-second wait notices, serializes test packages sharing its disposable database, and checks resource cleanup. Formatting was applied only to the Phase 2 Go files edited during recovery.

## Final evidence

Final command: `python scripts/verify_phase2.py`, exit **0**, isolated project `campusclaw-phase2-test-71850fb0`. Pinned Go image `golang:1.26.0-alpine3.23`; MySQL `mysql:8.4.4`; generated test credentials only. All SQLite fixtures and uploads were container-local temporary fake data; no real database or upload directory was mounted.

| Acceptance | Final result |
|---|---|
| 3.1 schema/constraints | PASS: eight InnoDB tables, strict conversion, PK/UNIQUE/NOT NULL/FK/CHECK, same-class composite FK, case/trailing-space/Unicode keys, UTC time plus ID ordering, 20 MiB body hash, auto-increment after large explicit IDs |
| 3.1 schema preparation/version checks | PASS: repeat preparation, read-only catalog match, missing CHECK rejected by validation and repeat preparation |
| 3.2 source preflight | PASS: normal eight-table source; missing file, bad FK, long username, unknown password format, body corruption and altered schema rejected; SQLite bytes unchanged |
| 3.3 migration | PASS: seven complete tables, row/field fingerprints, source SHA-256, body/file hashes, signed 64-bit values including >2^53, zero/negative IDs; valid and expired old sessions excluded |
| 3.3 rerun/rollback | PASS: identical target verification, differing session_version rejected without overwrite, injected rollback and retry, active target sessions refused and preserved |
| 3.3 uncertain commit | PASS: injected lost reply after real MySQL commit and after rollback; database handle unavailable then reconnect; complete files/evidence retained for unknown outcome; confirmed rollback files removed |
| 3.4 reports/gate | PASS: missing initialization evidence, schema drift, body/file damage and coherent database-plus-file corruption cannot pass readiness; verified target and later active sessions accepted without clearing sessions |
| 3.5 provisioning | PASS: four hashed sample accounts and two complete A/B materials, empty placeholder structure, seed failure rollback/retry, repeat fresh preserves passwords/files, class idempotency, user creation and invalid role/class rejection |
| 3.5 interrupted report publication | PASS: post-commit receipt publication failure recovers exact pending fingerprint without new seed IDs/passwords |
| Go tests | PASS: 10 Phase 2 top-level integration tests plus 10 nested preflight/commit scenarios; configuration and health top-level tests also pass (health includes 3 nested cases) |
| Build/dependency boundary | PASS: admin, migrate, server builds; server dependency listing excludes modernc.org and internal/migration |
| OpenSpec validation | `openspec validate migrate-backend-to-go-mysql --strict`: PASS; final-phase task 7.3 remains unchecked because the overall change is not finished |
| Preserved baseline | 37 protected file hashes unchanged; embedded SQLite schema copies byte-identical to legacy schemas |
| Historical tests retained | Flask 36 passed and Phase 0/1 acceptance reused; no expensive baseline/Compose rebuild rerun |

The commit-result tests inject outcomes around real database transactions; they do not claim packet-level network fault simulation. Startup integrity is exercised through the same read-only gate called before the server listener, not a new authentication/HTTP compatibility test.

## Cleanup and next boundary

After evidence capture, the interrupted tmpfs container/network were removed only after rechecking the exact name, image, lack of mounts/ports and exclusive network membership. Each recovery test run cleaned its own resources; the final harness verified no named container residue. No Phase 2 persistent volume was created. Unknown source/target data were never reset or reconstructed.

No observed Phase 2 blocker remains for Phase 3. Phase 3 still needs authentication/Session/CSRF and the remaining authentication portion of 2.1; subsequent HTTP compatibility/cutover acceptance is not implied by these results. Stop here. Offline usage and failure handling: [phase2-operations.md](phase2-operations.md).

## Final closeout after usage-limit interruption — 2026-09-27

The preceding recovery completed implementation and the final integration/build checks above. Its final additional Docker inventory request was not executed because automatic approval review hit the Codex usage limit; this was not a test failure. This results file had been saved, but the attempted task-checkbox edit had failed before writing, leaving 3.1–3.5 unchecked.

This closeout performed only resource inspection and documentation/task updates:

- `docker ps -a --filter name=campusclaw-phase2-test`, `docker network ls --filter name=campusclaw-phase2-test`, and `docker volume ls --filter name=campusclaw-phase2-test` all completed successfully with no matching resources. No cleanup was needed; no other Docker resources or real project data were touched.
- Confirmed the full existing acceptance record and reused its passing evidence. No Phase 2 integration tests or Flask baseline were rerun.
- Marked only tasks 3.1–3.5 complete and corrected the obsolete “Phase 2 not started” note. Phase 2 is **5/5**; overall tasks are **9/30**. Task 2.1 and all Phase 3/later tasks remain unchecked.
- Re-ran `openspec validate migrate-backend-to-go-mysql --strict`: exit **0**, `Change 'migrate-backend-to-go-mysql' is valid`.
- No observed blocker to Phase 3. No implementation changes, retrieval-change edits, deployment or archive were performed during closeout. Work stops at Phase 2.
