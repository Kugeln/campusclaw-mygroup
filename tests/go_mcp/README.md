# Phase 2 isolated verification

First follow [reproducible image preparation](../IMAGE_PREPARATION.md), including
the source-built Week 5 base, Alpine client packages and offline limitations.

Run `python tests/go_mcp/run_phase2.py` from the repository root with Docker access.
The runner creates a unique `campusclaw-w6-p2-<UTC>-<random>` project, private
nonoverlapping /24 network, MySQL 8.4.4 with ngram=2, and Phase 1's real MCP service.
No host ports are published. Existing container lifecycles are compared, and no
existing database, volume or Qdrant collection is used. Credentials are ephemeral
process environment values and are redacted before reports are persisted.

The test image uses `campusclaw-week5-rag-test:local`, explicitly built from
`tests/go_rag/Dockerfile` and the current go.mod/go.sum, as its dependency base.
It resets its inherited entrypoint, installs MariaDB command-line clients plus
their caching_sha2_password connector plugin, and never executes the old image's
source. Current source is mounted read-only at `/src`; only the new evidence
directory is writable at `/evidence`. No build argument carries credentials.
Prepare and verify this base on each machine rather than trusting an incidental
cached tag. An uncached client-package installation needs an Alpine mirror;
see the preparation guide before claiming an offline run. The production MCP
image remains its independent Phase 1 Dockerfile.

Reference catalog capture runs on a new empty test database by applying the
approved SQL, never by blessing the live migration target. The four per-DDL
hashes must match source SQL. A populated schema_mcp_steps.json is compared with
the reference and never silently overwritten. The old 41-step artifact is not
regenerated. The explicit capture command is separated from normal migration.

Tests create random `w6_*` databases on the dedicated server and exercise fresh,
v1/v2/v3 upgrades, repeat initialization, exact DDL/ledger crash boundaries,
drift rejection, named locks, class isolation, cache rollback, audits and real MCP
calls. Negative response/DNS/redirect/timeout tests use controlled real HTTP peers
on the test container's private IP. Core initialize/list/call also run against
the real Phase 1 binary; no main-app mock replaces it.

Backup verification dumps one populated isolated database and restores to a
different random database while the source still exists. Full catalog, ledger,
NULL-aware row digests and copied material file hashes must match. Only synthetic
fixture data enters the retained SQL backup. Chunk UUIDs/payload associations are
checked through database contents; Qdrant is not needed or reindexed here.

MySQL uses a dedicated tmpfs data directory with no named/external volume.
After evidence is saved, only the owned project is removed. This cleanup is not
used as migration rollback or as proof of restore. Each run retains its failures
and logs under ignored `runtime/week6-phase2-<run>/`, exits nonzero on failure,
and retains uniquely tagged build images for review. No real model is called.

Offline deployment still requires explicit target verification, stop-write and
backups before running the initializer. v4 adds three MCP tables plus ddl_hash
on the existing ledger, retains the completed v3 marker and adds version=4.
Initializer recovery accepts only catalog-proven checkpoints. It verifies the
original initialization receipt/data before resuming and writes a separate
`.campusclaw-schema-v4.json` bound to that receipt. Unknown or corrupt receipts
are preserved and rejected; HTTP startup checks readiness without running DDL.
