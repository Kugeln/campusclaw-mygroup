# Week 5 / Week 6 test image preparation

Run commands from the repository root. These steps build images only; they do
not authorize starting the classroom Compose project or reusing its databases.
Use Docker Desktop with Linux containers, Docker Compose v2, Go 1.26, Python 3
with the repository's test dependencies, Node.js and Edge/Playwright for browser
checks. Check `docker version`, `docker compose version`, and available disk
space before downloading. Never pass real credentials as build arguments.

## Image inputs and an online first build

| Input | Consumer | Preparation |
|---|---|---|
| `golang:1.26.0-alpine3.23` | backend, independent MCP, Go test base | pull this exact version |
| `alpine:3.23.3` | initializer/storage setup | pull this exact version |
| `mysql:8.4.4` | isolated SQL tests | pull; ngram=2 is configured by fixtures |
| `qdrant/qdrant:v1.13.4` | isolated vector tests | pull |
| `nginx:1.28.0-alpine` | existing proxy configuration | pull |
| `campusclaw-week5-rag-test:local` | MCP test Dockerfile and legacy Week 5 runners | build from the current repository as shown below |

```powershell
docker pull golang:1.26.0-alpine3.23
docker pull alpine:3.23.3
docker pull mysql:8.4.4
docker pull qdrant/qdrant:v1.13.4
docker pull nginx:1.28.0-alpine
docker build --pull=false --build-arg GOPROXY=https://proxy.golang.org,direct -f tests/go_rag/Dockerfile -t campusclaw-week5-rag-test:local .
docker build --pull=false -f tests/go_mcp/Dockerfile -t campusclaw-week6-mcp-test:prepared .
```

Check every exit code; stop on failure. The first build fetches the versions in
`go.mod`/`go.sum`. The MCP test build inherits that module cache and runs
`GOPROXY=off go mod download`; it also installs `mariadb-client` and
`mariadb-connector-c` from the Alpine package repository. Consequently
`--pull=false` does **not** mean an offline build. A reachable approved Go proxy
and Alpine package mirror are required for an uncached build. Use the
`GOPROXY` build argument for an approved mirror; never embed proxy credentials.

The fixed `:local` tag is an interface used by existing runners, not proof of
provenance. On a new machine build it from source. On a shared machine first
inspect its image ID and coordinate before replacing the tag; use an isolated
Docker context for preparation if the existing tag is in use. For validation
without changing that tag, make a temporary copy of the MCP test Dockerfile
whose first `FROM` names a uniquely tagged, freshly source-built test base.
All other instructions must stay identical. Production Dockerfiles stay intact.

Production image build checks, without starting services:

```powershell
docker build --pull=false -f Dockerfile.backend --target server -t campusclaw-verify-server:local .
docker build --pull=false -f Dockerfile.backend --target initializer -t campusclaw-verify-initializer:local .
docker build --pull=false -f mcp-server/Dockerfile --target server -t campusclaw-verify-mcp:local mcp-server
docker build --pull=false -f mcp-server/Dockerfile --target probe -t campusclaw-verify-probe:local mcp-server
```

Use unique verification tags in a shared environment. Root `.dockerignore`
retains Go sources, embedded frontend/Swagger assets, migration SQL/catalogs,
test Dockerfiles and the public `.env.example`; it excludes local evidence,
secrets, snapshots and caches. The independent MCP context has its own allowlist.

## Restricted networks and offline transfer

Prepare the exact images on a trusted connected machine of the same platform.
Record repository revision, Dockerfile/go.mod/go.sum hashes, image IDs and
registry digests. Export with `docker image save -o <private-image-bundle.tar>
<explicit-image-tags>`, record `Get-FileHash -Algorithm SHA256`, securely transfer,
verify the hash and provenance, then `docker image load -i <private-image-bundle.tar>`.
Keep the bundle outside the build context and Git. Never export containers or
their environment, database volumes, production data or secrets.

An image archive is not a BuildKit cache export. Runners that rebuild may still
execute the Alpine package installation. A fully offline rebuild therefore also
needs a verified BuildKit cache exported/imported with a compatible buildx
builder, or an approved internal Alpine mirror and the populated Go module
cache. Importing only the stock Go/Alpine images is insufficient. If these
prerequisites are missing, stop and report the test as not executed; do not
silently substitute an unrelated workstation image or retry downloads endlessly.
Do not run image prune or remove images used by other projects.

## Before executing tests

Inspect required image IDs and verify ownership of newly chosen projects,
subnets, ports and collections. The Week 6 runner creates fresh random resources
and credentials; `python tests/go_mcp/run_phase6.py` without `--reuse-*` needs no
historical runtime directory. It still requires the prepared test base above.

Week 5's `prepare_align_storage.ps1 -Subnet <checked-free-10.254.x.0/24>` and
`prepare_align_stream.ps1` create the independent tmpfs fixtures and ignored
`runtime/align-*-connection.json` descriptors. Run preparation only after
checking the actual subnet/routes and receiving permission for test resources.
Do not copy another machine's descriptors, credentials or old database names.
The old `run.ps1` preflight descriptor is likewise generated locally, not a
missing source file. See each runner's README and explicit isolation guards.

No real model or peer participation is implied by these builds. Week 6 T71/T68
retain the user-approved controlled-model/automated-cross-check standard and
the separately disclosed original real-model/human/peer gaps.
