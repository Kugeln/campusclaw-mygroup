# Phase 5.1 shared HTTP contract suite (OpenSpec task 6.1)

Run from the repository root in PowerShell:

```powershell
.\.venv\Scripts\python.exe -m unittest discover -s tests/http_contract -p test_harness.py -v
go test -mod=readonly ./tests/http_contract/fixture
.\.venv\Scripts\python.exe tests/http_contract/run.py
openspec validate migrate-backend-to-go-mysql --strict
```

For a focused regression run, repeat `--case` with exact existing case IDs:

```powershell
.\.venv\Scripts\python.exe tests/http_contract/run.py --case multipart.truncated --case upload.success.txt
```

Selection invokes the original shared case functions without changing their
assertions. Unknown IDs fail before creating resources. A selected run records
`full_suite: false`, its selected IDs and `not_executed_count`; omitted cases
are not presented as passed or skipped. `selection_accepted` can be true while
the full-suite `accepted` remains false. Exit 0 for a selected run means only
that selection passed; it is never task 6.1 acceptance.

Prerequisites: the repository's locked `requirements.txt` installed in `.venv`,
Go matching `go.mod`, and Docker with `mysql:8.4.4` available. No browser or RAG
provider is required. The Go fixture command above compiles the helper; the live
runner actually seeds and queries MySQL. Do not run `fixture` manually against an
existing database.

## Isolation and fixture

Every invocation creates a random `campusclaw-phase5-contract-<hex>` network,
MySQL container and **new** `-data` volume. Only MySQL's data path is mounted;
there are no Demo volume or host source mounts. MySQL binds a random loopback
port. A fresh `phase5_contract_<hex>` database is provisioned by that container.
The fixture helper refuses nonempty databases or existing upload directories.

Flask uses an independent SQLite file with **`MATERIAL_BACKEND=sqlite`** in both
environment and app config. `create_app` is served through a real threaded
Werkzeug HTTP server, not Flask's test client. Go uses the actual built
`cmd/server` executable and real MySQL. Both listen on independent random
127.0.0.1 ports; adapters explicitly refuse port 8080, 80, 443 and remote hosts.
No existing endpoint can be supplied as a test target.

`fixtures.json` is the common source for explicit user/class/material IDs,
roles, metadata, timestamps, original text and attachment bytes. It includes
both roles in A/B, an empty class, Chinese filename metadata and HTML-like
plain text. Generated account passwords are shared only between the two test
fixtures; JWT signing secrets are independently random. New password salts
differ by design. `SECRET_KEY` is absent during valid startup; negative startup
checks supply a synthetic legacy key and prove it cannot replace JWT config.

Files live under the new `runtime/<name>/flask-uploads` and `go-uploads`;
SQLite, binaries and machine-readable reports also stay under this ignored
runtime directory. Source Demo `app/` and `scripts/` are imported/read, never
edited. RAG initialization is disabled by the SQLite backend setting; no
Qdrant client or external retrieval/answering request is made.

At completion the runner stops only its own HTTP servers and MySQL container.
It **does not delete** any containers, networks, volumes, databases or files,
including failed attempts. Resources accumulate intentionally; the report
names them. Generated credentials are never printed or written to reports;
as with normal MySQL Docker initialization, the test container retains its
generated database environment. No real password, JWT secret or full Token
is needed on the command line.

## Comparison and reporting rules

Every case in `cases.py` runs against both live services through the same
function. Adapter differences are limited to startup/fixture infrastructure,
read-only DB inspection and negative startup invocation. Denied/read requests
check for material/body table and private file side effects. Assertions cover
HS256/claims/8 hours, login errors, Bearer rejection, me/revocation/relogin,
permissions, bidirectional isolation, uploads, bytes, safe download semantics,
methods, validators and public page/static shells. Startup config checks are
explicit non-HTTP companions because rejected startup has no listening server.
Independent-key token rejection is tested without performing a cutover.

- JSON error fields, status codes, identity, list ordering and attachment bytes
  are not normalized away. Two successful assertions can still produce a
  **pending** differential result when JSON/media types or Allow headers differ.
- Generated UUIDs and timestamps are validated for shape and correspondence to
  subsequent reads; only those inherently dynamic values are omitted from the
  successful-upload cross-backend summary. JWT signatures/claims are verified
  in memory. Full Tokens are redacted from the saved HTTP trace.
- Framework-generated HTML error bodies are not compared byte-for-byte;
  status and media type are retained. JSON versus HTML remains a difference.
- Content-Disposition is compared as attachment plus decoded filename, with
  RFC 6266 `filename*` taking precedence over its ASCII fallback.
- Download `Flask: no-store` / `Go: private, no-store` remains an explicitly
  accepted pair. The user's later five boundary decisions are enumerated in
  `accepted_differences.py`: exactly 29 case IDs, covering bare query separators,
  oversize-plus-identity, large form fields, method capabilities and validators.
  Each backend must satisfy its exact status/media/JSON or method/validator
  checks before a pair is accepted. Previously fixed errors and RAG paths are
  absent from the registry. Failed assertions can never be waived by a rule.
  Every accepted pair records its rule ID alongside the unchanged HTTP trace.
- Go conditional attachment responses must contain the complete original bytes,
  decoded attachment filename, nosniff and `private, no-store`; conditional logo
  bytes must equal its initial response. Flask matching conditional responses
  must remain empty 304. Missing validators are an explicit user decision,
  not a normalization of 200 and 304. Allow must match registered capabilities,
  not merely contain an arbitrary subset of methods.
- Static validators are tested using each service's own returned validator;
  framework/path-derived raw ETags need not equal one another. Material
  attachment mtimes are identical in the fixtures.
- The known RAG additions to the Flask material JavaScript are outside scope.
  Shared base assets are byte-compared; material JavaScript is checked for safe
  text/Blob behavior rather than equating the out-of-scope retrieval UI.

The full report is printed as a path to `runtime/<name>/report.json`. It records
per-case HTTP observations, backend pass/fail/skip counts and classifications:
`match`, `accepted`, `pending` (unconfirmed edge semantics), `incompatible`
(a required contract assertion failed). Pending is **not** acceptance or skip.
Unexpected test exceptions are retained as failures for investigation. The
report includes setup failure explicitly and never converts missing MySQL to
a skip or a passing integration result.

Exit codes: **0** executed selection complete and compatible; **1** complete but unresolved
differences/failures; **2** infrastructure/setup failure. Do not mark task 6.1
complete or proceed to cutover on 1/2. See the checked-in migration result
record for the reviewed outcome; do not silently accept a new difference.

## Scope and historical security mapping

This is base authentication/material HTTP verification, not Nginx cutover,
real-data migration, browser automation or a full test run. `/knowledge`,
`/api/search`, `/api/ask`, indexing and RAG responses are excluded. Existing
historical tests are preserved. Old SID/CSRF rotation is replaced here by
explicit Bearer-only authorization, cookie-only rejection, session-version
revocation, independent signing keys, no auth cookie and trusted identities;
there is no requirement to restore Cookie Sessions or issue CSRF tokens.

The runner never probes the formal Demo as a test target. Any requested Demo
`/health` and `/login` checks are separate read-only GET observations and do
not imply its RAG workflow was re-tested.
