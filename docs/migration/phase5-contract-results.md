# Phase 5 shared HTTP contract results — task 6.1

Date: 2026-10-09. **Suite implemented and run; 6.1 acceptance NOT passed.**
Tasks remain **23/30**, Phase 4 **7/7**. Task 6.1 stays unchecked; no work on
6.2–6.4 or Phase 6, no business implementation or specification changes.

## Actual isolated run

The same case functions drove two real, independent HTTP services. Flask used
SQLite and explicit `MATERIAL_BACKEND=sqlite`; Go used MySQL **8.4.4**. Equivalent
fixtures supplied five users, A/B and an empty class, three material/body pairs,
equal IDs, timestamps, file mtimes and raw bytes. Account passwords were
generated, with independently salted hashes. Signing secrets were independently
generated; no `SECRET_KEY` was required. Nothing was copied from real Demo data.

Final run resources: `campusclaw-phase5-contract-8bcdb72b`, new volume
`campusclaw-phase5-contract-8bcdb72b-data`, database `phase5_contract_8bcdb72b`.
Flask origin was `http://127.0.0.1:56819`; Go origin was
`http://127.0.0.1:56820`. Both HTTP servers and the test MySQL container were
stopped after verification. Isolated container/network/volume and runtime files
were retained. No existing DB or volume was deleted/rebuilt.

The first exploratory run, `campusclaw-phase5-contract-dedb1c85`, remains retained
too. It exposed an error in the test's attachment-name parser: Werkzeug sends
both an ASCII filename fallback and RFC 6266 `filename*`, and the parser chose
the fallback. Only the test parser was corrected, with a regression unit test.
The final full run below supersedes those false filename failures. It also
adds missing/invalid JWT configuration and independent-key/relogin checks.
One later launch attempt hit an automatic approval timeout; its permitted retry
ran successfully. It did not create another set of resources.

Commands actually run from the project root:

```powershell
.\.venv\Scripts\python.exe -m py_compile tests/http_contract/run.py tests/http_contract/adapters.py tests/http_contract/cases.py
.\.venv\Scripts\python.exe -m unittest discover -s tests/http_contract -p test_harness.py -v
go test -mod=readonly ./tests/http_contract/fixture
.\.venv\Scripts\python.exe tests/http_contract/run.py
openspec validate migrate-backend-to-go-mysql --strict
```

Harness unit tests: **3 passed**. Go fixture package compiled successfully
(`no test files`; not represented as a live MySQL test). The shared runner built
the actual Go server, seeded a new real MySQL database, then ran the HTTP suite.
Python dependencies matched the existing locks: Flask 3.1.2, Werkzeug 3.1.3,
PyJWT 2.10.1, PyMySQL 1.1.1. No dependencies or lockfiles changed.

| Backend | Passed | Failed | Skipped | Total |
|---|---:|---:|---:|---:|
| Flask / SQLite | 109 | 1 | 0 | 110 |
| Go / MySQL | 98 | 12 | 0 | 110 |

Totals include **106 shared HTTP cases and four invalid-startup configuration
cases** per backend. Cases can make multiple requests. A pass is an individual
backend assertion result, not proof that the pair's responses match. Pair
comparison: **77 match, 1 accepted difference, 31 pending differences,
1 explicit incompatibility**. Runner exit code: **1**, not success.

Machine-readable per-case results, reasons and divergent response observations:
[phase5-contract-comparison.json](phase5-contract-comparison.json).
The full redacted HTTP trace remains at
`runtime/campusclaw-phase5-contract-8bcdb72b/report.json`.
Repeatable setup/commands/exit codes and comparison rules are in
[tests/http_contract/README.md](../../tests/http_contract/README.md).

## Confirmed compatible behavior

- Correct login has exactly `access_token`, `token_type`, `expires_in`,
  `redirect`; HS256 validates, required `sub/ver/iat/exp` are checked and the
  lifetime is 28800 seconds. Wrong credentials and malformed login inputs
  yield the same 401 JSON. Authentication is Bearer-only, not SID/CSRF.
- Missing, malformed, tampered, expired, wrong-signature/algorithm/claim tokens,
  cookie-only credentials and comma-ambiguous Authorization are rejected.
  The matrix actually covers me, list, detail, file and upload requests.
- Missing/empty/whitespace/short `JWT_SIGNING_SECRET` cannot start auth, even
  with a synthetic `SECRET_KEY`. Another stack's signed Token is rejected;
  local re-login succeeds. This checks key separation without a cutover.
- Trusted me identities, logout version increment/204, old Token rejection for
  both clients, and re-login work on both databases. Both roles' A/B lists,
  details, original text and original file bytes match, with stable ordering
  and empty-list behavior. Foreign/missing detail/file have identical 404 JSON.
- Teachers upload TXT/MD/BOM and students receive 403. Forged role/class/uploader
  fields cannot grant authority. Single-error missing/multiple/empty/invalid
  UTF-8/NUL/unsupported/over-limit uploads reject identically and do not change
  either material table or private files. Quoted multipart boundaries work.
- Chinese stored filename metadata downloads correctly after RFC filename
  decoding; newly uploaded names follow the existing secure-name normalization
  on both sides. Raw UTF-8/BOM bytes are preserved; plain text is not rendered
  as HTML. Public pages contain the existing Chinese base controls, with safe
  text/Blob JavaScript, matching shared base assets and no embedded credentials.
  Private upload URL probes return 404. Anonymous health works on both sides.

## Differences requiring a decision

Only the first row is an accepted exception. **Pending means unresolved, not
waived, skipped or ready for cutover.** HTML error bodies are not byte-compared,
but statuses, JSON fields and media types remain visible. The classification
distinguishes a clear spec violation from edges whose precedence/coverage needs
an explicit contract decision; all unresolved rows block task acceptance.

| Classification / cases | Flask actual | Go actual | Implication |
|---|---|---|---|
| Accepted, 1: download cache | `no-store` | `private, no-store` | User-confirmed difference; Go satisfies migration spec. |
| **Incompatible, 1:** truncated multipart | 400, JSON `Exactly one file is required` | **500**, JSON `Material storage unavailable` | Client format error violates the required 400 input-error behavior; not a storage failure. Neither side left material/file writes. |
| Pending, 1: `GET /api/materials?&` | 200, scoped list | 400, `Client scope parameters are not accepted` | Raw query separator versus parsed query parameters; standard nonempty parameters return 400 on both. |
| Pending, 1: invalid extension followed by role field | 400 JSON `Client identity fields are not accepted` | 400 HTML | Different validation priority and response shape. |
| Pending, 1: invalid UTF-8 followed by second file | 400 JSON `Exactly one file is required` | 400 HTML | Different validation priority and response shape. |
| Pending, 1: over-limit file followed by class field | 400 JSON identity-field error | 413 HTML | Different priority changes HTTP status. |
| Pending, 1: >1 MiB role form field then file | 413 HTML | 400 JSON identity-field error | Framework form-field limit versus streamed identity rejection. |
| Pending, 15: PUT/PATCH/DELETE on five routes | 405, Allow includes OPTIONS | 405, Allow omits OPTIONS | Method denial matches; Allow differs. Flask error media is HTML, Go is plain text. |
| Pending, 5: OPTIONS on those five routes | 200 with Allow | 405 with Allow | Automatic Flask OPTIONS is absent in Go. |
| Pending, 2: file and logo validators | ETag and Last-Modified present | Both absent | Download cache exception does not cover validators. Static logo also has Flask `no-cache` versus no Go Cache-Control. |
| Pending, 4: file/logo conditional GET | Matching If-None-Match / If-Modified-Since → 304 | 200 | Conditional behavior differs despite equal attachment mtimes and same logo bytes. |

The five method-test routes are `/api/materials`, a fixture's detail and file,
`/api/auth/login`, and `/login`. ETags themselves are framework/path-derived;
each backend's own ETag is used in its conditional request, so their literal
values are not incorrectly required to match.

The **single Flask failure** is the `?&` case. The **12 Go failures** are
truncated multipart (1), oversize+identity (1), large form field (1), OPTIONS
(5), and conditional GET (4). Other pending cases pass their local assertions
but differ in the explicit cross-backend comparison. No failures were hidden
by adapting expected business results based on backend name.

## Protection and next gate

Read-only GET checks of the existing Demo at port 8080 returned **200** for
`/health` (`{"status":"ok"}`) and **200** for `/login` with CampusClaw content.
These checks did not log in, upload, query RAG, alter accounts, or use the Demo
as a suite target. This confirms the two requested endpoints, not a new full
RAG acceptance. The three existing Demo data volumes, MySQL/Qdrant services,
uploads, Flask implementation, scripts, default Compose and Nginx stayed intact.

OpenSpec strict validation: **PASS**,
`Change 'migrate-backend-to-go-mysql' is valid`. Existing task checkboxes and
historical acceptance files are unchanged. All new code is confined to
`tests/http_contract/`; this result and the comparison JSON are new docs.
No real-data migration, deployment, stash, commit, push or archive occurred.

**Not ready for 6.2.** Next work should first confirm the unresolved HTTP edge
contracts and authorize narrowly scoped remediation of the multipart 500 and
any differences that must be aligned, then rerun this same suite. No business
or spec fix has been smuggled into this testing-only task; no automatic
progression to isolated switching rehearsal is warranted.
