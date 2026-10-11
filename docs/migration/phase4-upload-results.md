# Phase 4 upload acceptance: 5.1–5.3

Date: 2026-10-09. Scope: upload only. Progress: 19/30; prior 16 completed tasks preserved. Tasks 5.4–5.7 and Phase 5 remain pending.

## Implementation and contract

- `POST /api/materials` uses existing Bearer authentication and teacher middleware. User/class/role come from the database-backed principal; query/form identity fields are rejected. Anonymous 401, student 403, identity/multiple-file errors 400 JSON. Text/type errors remain 400 HTML; oversize is 413 HTML. Storage/transaction failure is generic 500 JSON. Success is 201 with only `material_id`, as specified by this migration's frozen contract.
- Raw `.txt`/`.md` bytes stream to private UUID staging files with a 64 KiB copy buffer and a limit-plus-one reader. Default file limit is 20 MiB; multipart framing additionally has a bounded 1 MiB allowance. Strict UTF-8 rejects malformed sequences and NUL. Empty bytes are invalid; BOM-only is accepted like Flask, with exactly the first BOM removed from stored text. Original file bytes are retained. LONGTEXT construction is bounded by the configured file limit, not constant-memory; raw requests are not read without bounds.
- Display filenames follow Werkzeug-style NFKD/ASCII sanitization. Physical paths use server-generated UUIDs and an `os.Root` boundary. Exclusive destination reservation prevents overwrite even on UUID collision. No upload-directory static handler is registered. Unicode normalization uses pinned `golang.org/x/text v0.42.0`.
- Order: stage/validate → begin transaction → insert `materials` and `knowledge_entries` → move private file → commit → verify file → 201. Both rows use the same trusted class and uploader scope; no schema change.
- Pre-commit failure rolls back and removes only this upload's owned files. Any commit error is reconciled through an independent connection and bounded context; locking reads wait for an in-flight insertion. Confirmed absence permits compensation; complete matching rows plus matching file permit success. Unknown/inconsistent outcomes return 500 and retain the UUID file as recovery evidence. Committed files are never removed as error compensation.

## Executed verification

1. `go test ./internal/storage ./internal/material ./internal/http ./internal/config ./internal/repository/mysql ./cmd/server` — PASS (server compile; relevant package tests). Repeated after the final uppercase hidden-extension compatibility fix. Covers UTF-8 boundaries/BOM/NUL/empty/type, exact 20 MiB and bounded overrun, sanitization, concurrent names, destination collision preservation, staging creation and rename failure, transaction outcomes, HTTP responses and private-path 404.
2. `python scripts/verify_phase4_upload.py` — PASS. Actual MySQL 8.4.4 and Go 1.26.0 runner; fresh isolated database/schema/seeds and temporary upload roots, no host ports or Demo mounts. `TestPhase4UploadRealMySQL` passed all subtests in 3.572 seconds. Independent post-201 reads verify both rows and raw file. Conditional test-only MySQL triggers exercise first/second insert failure and rollback. Driver wrappers exercise confirmed rollback, committed-but-lost reply, delayed in-flight commit, and unavailable reconciliation against actual InnoDB transactions. These are controlled transaction-boundary faults, not claims of physical network or disk outage testing. Staging creation failure and source disappearance exercise filesystem errors. Existing committed files and six concurrent same-name uploads remain intact.
3. `python -m pytest -q tests/test_uploads.py -k 'upload_validation or valid_text_parsing or size_collision_and_traversal or successful_upload_is_committed or upload_identity'` — 5 passed, 3 deselected (22.50 seconds). Existing Flask code tested unchanged, using fixture-owned temporary SQLite and file directories. Go HTTP tests assert the corresponding migration response fields/statuses. No full-suite run.
4. Demo read-only checks: `http://127.0.0.1:8080/health` and `/login` both HTTP 200. No Demo account, database or upload mutation performed.
5. `openspec validate migrate-backend-to-go-mysql --strict` — PASS: `Change 'migrate-backend-to-go-mysql' is valid`. `git diff --check` passed.

Protection hash check: 105 of 106 monitored existing files stayed identical, including Flask code/tests, other changes and schema/migration files. `docs/course-auth-knowledge-implementation.md` changed externally during this turn; this implementation did not write or revert that file, and its current content is preserved.

Isolated test containers/network/volume retained, not deleted: `campusclaw-phase4-upload-c5b5b3be` (volume suffix `-data`). Harness stops its test database after verification. Generated test credentials and full tokens are not recorded.

## Limits and follow-up boundaries

- Automatic startup recovery/cleanup belongs to 5.5 and is not implemented. Unknown commit outcomes may leave a retained file requiring later database reconciliation. A failed HTTP response can correspond to a committed upload when reconciliation is unavailable; retry is not idempotent.
- List/detail/download/UI and full A/B API matrix remain 5.4–5.7. Formal ingress is unchanged; this is not a production cutover or entire Phase 4 acceptance.
- The retrieval-enabled Flask Demo can add `index_status` and indexing side effects. Those belong to its separate change; this migration implements its existing frozen plain-text upload contract, without introducing retrieval behavior.
- Linux directory fsync is exercised by isolated integration. Windows development skips directory fsync; crash/power-loss durability on Windows is not claimed.
