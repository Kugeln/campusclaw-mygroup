# Phase 4 UI and complete material permission acceptance: 5.6–5.7

Date: 2026-10-09. Only tasks 5.6–5.7 are implemented in this round. Phase 5 has not started; no Nginx/Compose entry is changed. Previous authentication, upload and recovery result files remain unchanged.

## Reused resources and implementation

Go now serves public `/login` and `/materials` page shells plus the existing `/static/styles.css`, `/static/logo.png`, `/static/api.js`, `/static/login.js`, `/static/materials.js` URLs. `internal/http/web` contains copies owned by Go, embedded in the executable, so serving pages does not require Flask/Python or a filesystem deployment directory. Only explicitly named assets are reachable; directory indexes, uploaded files and knowledge/RAG resources are not exposed.

CSS, Logo, api.js and login.js are byte-identical to the current Flask files and tested as such. The two HTML files reuse the existing base/child markup, Chinese text, form IDs and controls, with Jinja inheritance flattened and static paths resolved. Only the knowledge link is omitted. The materials script retains the current flow and removes only the retrieval-status success messages and unused parsed result variable. The original `app/` files are unchanged.

There was no separate material detail HTML route in Flask: the original “阅读文本” action loads `/api/materials/{id}` into the `<pre>` on `/materials`. That interaction is retained, along with the existing download API. Teacher/student controls and class labels load through the trusted `/api/auth/me` endpoint; hiding upload controls is not authorization.

`sessionStorage`, explicit Bearer headers, same-origin-only requests, `credentials: omit`, `redirect: error`, no-token/401 clearing, 403 staying logged in, and failed-logout notices are reused. Names and original text are assigned via `textContent`, never `innerHTML`. The page/API handler adds the existing Flask CSP, nosniff and same-origin referrer policy; API errors including routing 404/405 receive no-store. No unsafe-inline/unsafe-eval policy was added. Download retains the previously confirmed `private, no-store` override.

## Verification actually run

1. `go test ./internal/http ./internal/auth ./internal/material ./internal/storage ./internal/repository/mysql ./cmd/server` — PASS. New page/static tests cover Chinese public shells, no cookies or reflected identity, reused asset bytes, strict headers, static allowlist/private-path 404 and unsupported method 405. Existing related authentication/upload/read/storage tests also pass. After completing no-store coverage for API routing errors, `go test ./internal/http ./cmd/server` passed again.
2. `python scripts/verify_phase4_ui.py` — PASS. Creates a new MySQL 8.4.4 container/network/volume with only a randomly assigned **127.0.0.1** database port, builds the actual Go server, and runs `go test -mod=readonly -count=1 -timeout=240s -v -tags phase4uiintegration ./internal/repository/mysql -run TestPhase4UIRealMySQL`. The Go server listens on a separate random loopback port, uses a test-owned temporary upload root and refuses a nonempty test database. Successful run: **15.435 seconds**, Go origin `http://127.0.0.1:55257` (stopped after the test).
3. The real-server matrix passed teacher uploads in A/B; both roles' scoped lists/details/downloads; foreign/missing identical 404; forged role/class/uploader/user fields and identity headers/cookies; student 403; teacher extra-field/query 400; list query 400; unsupported PUT/PATCH/DELETE and POST mutations 405; missing/multiple/empty/invalid-UTF8/NUL/unsupported files 400; **20 MiB + 1 byte** rejection 413; missing, malformed, tampered, wrong-signature, expired and revoked tokens 401. Before/after database and file fingerprints verify denied requests have no side effects. Response field counts, raw BOM bytes and safety headers are asserted.
4. `scripts/check_phase4_go_browser.py`, invoked by that integration test with the existing isolated Playwright/Edge environment — PASS. A fresh headless Edge context drives real login, page refresh, teacher upload, original-text reading, Blob download, byte comparison, object-URL revocation, logout and server revocation. It covers both classes and both roles, student-hidden upload plus direct backend 403, A/B labels, Chinese text, historical Chinese/hostile filename metadata, nonexecution of uploaded scripts, same-origin rejection, Bearer presence and cookie absence on all protected frontend requests, 403 notice without logout, failed-logout warning and revoked-token 401 redirect/clearing. No existing browser profile or Demo is used.
5. Flask targeted comparison: `.venv/Scripts/python.exe -m pytest -q -p no:cacheprovider --tb=short --basetemp <new runtime/flask-ui-UUID> tests/token_auth/test_http.py tests/token_auth/test_browser.py -k 'role_class_isolation_and_forgery or page_shell_health_and_upload_regression or invalid_authentication_http_matrix or cookie_only_cannot_authorize or real_browser'` — **7 passed, 4 deselected**, 28.11 seconds. Original Flask tests include three real Edge workflows and use temporary SQLite/files.
6. `.venv/Scripts/python.exe -m pytest -q tests/test_uploads.py -k 'upload_validation or forged_identity_cannot_grant_upload'` — **3 passed, 5 deselected**, 20.74 seconds. No existing tests were modified. This is Phase 4 targeted comparison, not the Phase 5 generalized shared differential suite.

Test setup corrections: combining the two existing Flask test folders caused a same-name `conftest` import collision; they were run separately. A reused pytest temporary/cache location had an access error, resolved with a new explicit temporary directory and disabled cache plugin. The first two Go integration attempts passed the HTTP matrix but failed the browser's Blob-revocation wait: Playwright's string-based predicate attempted eval under CSP. The test now polls a function through the browser protocol; the application's CSP was not weakened and the assertion was retained. Only the final successful run is claimed as browser acceptance.

## Isolation, compatibility and remaining boundaries

- Download cache header remains **Go `private, no-store` / current Flask `no-store`**, as explicitly confirmed by the user; the migration spec is the acceptance basis. Attachment filename encoding is semantically compatible, not required to be byte-identical to Werkzeug.
- Upload filename sanitization retains the current Flask contract: non-ASCII characters may be removed by secure-filename normalization (browser fixture `browser-中文.md` displays as `browser-.md`). Chinese original text and existing Chinese filename metadata display correctly; no new filename contract is introduced.
- Knowledge/RAG routes, `index_status`/`cleanup_required`, and their UI are not migrated. Details/downloads remain the original material business flow.
- The Go UI is an embedded snapshot; later Flask UI edits require deliberate synchronization. Sole-instance startup recovery constraints from 5.5 remain. Formal ingress/HTTPS, cutover and broader compatibility work remain Phase 5/6; this test did not deploy or switch the Demo.
- Read-only Demo checks returned `/health` 200 and `/login` 200. No Demo database, account, upload file or browser session was changed.
- Retained isolated resources: `campusclaw-phase4-ui-7d4c8d96`, `campusclaw-phase4-ui-33e64bae` (failed test attempts), and `campusclaw-phase4-ui-479be491` (successful). Each has its own `-data` volume. Only their test MySQL containers are stopped; containers/networks/volumes and binaries under `runtime/` remain. No database/volume deletion or rebuild was performed.

## Changed files and final status

- `internal/http/handler.go`, new `pages.go` and `pages_test.go`.
- `internal/http/web/templates/{login,materials}.html`, `web/static/{styles.css,logo.png,api.js,login.js,materials.js}`, and provenance `web/README.md`.
- `internal/repository/mysql/material_ui_integration_test.go`.
- `scripts/verify_phase4_ui.py`, `scripts/check_phase4_go_browser.py`.
- This acceptance record and this change's `tasks.md`.

`openspec validate migrate-backend-to-go-mysql --strict` — **PASS**, `Change 'migrate-backend-to-go-mysql' is valid`. `git diff --check` also passed (existing LF/CRLF advisories only). Tasks **5.6–5.7 completed**, Phase 4 **7/7**, total **23/30**. Phase 5 and later tasks remain unchecked.

The initial hash inventory covered 202 existing tracked/untracked files. Final comparison found only the intended existing-file edits to `internal/http/handler.go` and this change's `tasks.md`; original Flask assets/tests, the course report, prior result records, schema, other changes and deployment configuration are unchanged. No commit, push or archive.
