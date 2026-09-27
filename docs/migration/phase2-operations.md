# Phase 2 offline tools

These commands operate only on an explicitly selected disposable database and copied offline uploads/SQLite snapshot during this phase. No deployment or cutover has been performed. Authentication, Go Session, CSRF and material HTTP routes remain unimplemented.

Build `cmd/admin`, `cmd/migrate`, and `cmd/server` with Go 1.26.0. MySQL driver v1.9.3 and migration-only modernc.org/sqlite v1.36.3 are locked. The server dependency graph excludes the SQLite driver and `internal/migration`. The Python verification harness is a development tool only.

Inject `DB_HOST`, `DB_PORT`, `DB_NAME`, `DB_USER`, `DB_PASSWORD`, and `UPLOAD_ROOT` into the command environment. `UPLOAD_ROOT` is the separate target copy. Do not put passwords on command lines. MySQL must provide the version-1 InnoDB/utf8mb4/NO PAD binary schema capabilities and at least 32 MiB max_allowed_packet; the isolated harness pins MySQL 8.4.4 and uses 64 MiB. Connection SQL mode is strict and preserves explicit zero IDs.

## Migrating an isolated snapshot

1. Prepare a stopped, checkpointed SQLite **copy**, plus matching uploads **copy** containing `files/{UUID}`. The preflight refuses WAL/SHM/journal sidecars, altered schema, invalid relationships, unsupported fields, missing files or mismatched bodies. It reads with `mode=ro` and `query_only`, and checks the SQLite hash before and after reading.
2. `migrate -mode preflight -source /snapshot/source.db -source-uploads /snapshot/uploads -report /reports/preflight.json`
3. `admin -mode schema` prepares the eight tables separately from data transactions. Existing incompatible schema is rejected, not repaired. Version-1 metadata includes columns, nullability/defaults/collations, indexes, foreign keys and enforced CHECK definitions; auto-increment counters are excluded from structural comparison.
4. `migrate -mode import -source /snapshot/source.db -source-uploads /snapshot/uploads -report /reports/import.json`
5. `migrate -mode verify -source /snapshot/source.db -source-uploads /snapshot/uploads -report /reports/verify.json`

Explicit report paths must be new files outside the source snapshot. Seven tables are compared by IDs, counts and complete row fingerprints, with typed field SHA-256 values and separate original-file/body hashes. Reports contain no plaintext password hashes or bodies. The physical SQLite SHA-256 associates successful migration evidence with its source snapshot, including the excluded old sessions. Target sessions must be empty for import and offline verification; active sessions are never cleared to enable reruns.

Import permits an empty target with empty file directories, or an identical target for verification. It never UPSERTs differing rows. A transaction failure removes only files created by that attempt. A lost commit reply triggers a separate bounded database reconciliation: verified complete target succeeds, confirmed empty target can retry, conflicting or unavailable target fails and preserves evidence. An interruption that leaves files with an empty database is deliberately rejected pending operator inspection; no automatic deletion/rebuild of unknown state occurs.

Successful import publishes `.campusclaw-initialization.json` only after verification. `.campusclaw-pending.json` records the pre-commit fingerprint; it is not success evidence. A matching nonempty target can republish a missing receipt without reimport. No source deletion, real snapshot migration, ingress switching or rollback is automatic.

## Fresh initialization and accounts

After `admin -mode schema`, inject `SEED_TEACHER_A_PASSWORD`, `SEED_STUDENT_A1_PASSWORD`, `SEED_TEACHER_B_PASSWORD`, and `SEED_STUDENT_B1_PASSWORD`, then run `admin -mode fresh` on an empty disposable target. It creates A/B classes, teacher_a/student_a1/teacher_b/student_b1, two complete sample materials and their files/bodies. Placeholder tables remain structure-only. Password creation is solely for offline provisioning; no authentication is implemented.

Fresh reruns validate existing initialization evidence without resetting passwords, sessions or files. If commit completed but receipt publication failed, a retry accepts only the exact pending seed fingerprint and republishes the receipt without regenerating passwords or material IDs. Unknown nonempty targets are refused.

`admin -mode class -name C` creates or returns an existing class. `admin -mode user -name student_c -role student -class-id 3` reads `ACCOUNT_PASSWORD` from the environment, rejects invalid roles/classes and never overwrites an existing username. Example IDs are illustrative; use the ID returned by the class command.

## Startup and verification boundaries

Normal server startup verifies schema, successful initialization evidence, source association for migrate mode, current relationships/files/bodies, and original immutable records/file fingerprints before the storage probe or listener. New rows and account/session changes are allowed; sessions are not cleared. `STARTUP_MODE=infrastructure` is the explicit health-only Phase 1 scaffold in the existing development Compose, not a cutover mode. Compose still does not auto-initialize business data. This exception must not be used for a later business deployment.

`python scripts/verify_phase2.py` uses a random private Docker network, a tmpfs MySQL instance with no host port or real-data mount, container-local SQLite fixtures/files, and read-only repository mounting. Each command has a deadline; long commands emit 15-second progress notices. Test processes have a 120-second Go timeout, and the complete Go runner has a 600-second deadline. The harness removes only its named containers/network and verifies cleanup. Trigger creation is enabled only in this disposable MySQL instance for transaction failure injection.

The metadata catalog is a readable version-1 structural contract recorded on the pinned server, not a check of `SELECT VERSION()`. Other server versions require the same capability/integration acceptance; metadata incompatibility fails closed for review.
