# Current Week5 isolated acceptance

Run `powershell -NoProfile -File tests/go_rag/run_align_acceptance.ps1`. The old `run_phase4.ps1 -Stage phase6` delegates here before accessing its obsolete fixed database. Each test creates a fresh random database/collection on verified independent tmpfs MySQL/Qdrant. Only controlled HTTP models are used. Real Edge and existing Nginx configuration exercise production Go handlers. No automatic cleanup, paid models, peer impersonation, classroom Compose, commits or deployment.

`run_all.py` compares URL/method case scripts dynamically against generated OpenAPI operations, without a fixed count. Cases cover current source CRUD/toggle, explicit binding, no-evidence generation, early candidate meta and final done. Week4 Ask keeps its exact zero-hit text and zero model calls. Fresh alignstorage tests supplement transaction/failure/recovery checks.

The browser runner injects APITEST_BASE_URL, APITEST_PASSWORD, APITEST_READY, APITEST_DB_CONTAINER, APITEST_GO_EVIDENCE and APITEST_SWAGGER_EVIDENCE. Only a random loopback proxy port and the new align_w5_<16hex> database from ready.json are accepted. SQL assertions are SELECT-only through the independent Docker MySQL client; passwords expand inside that container, never in CLI arguments. Never point this at classroom resources.

JSON/HTML reports are under ignored reports/apitest/<UTC>-<run>/. Browser screenshots, timestamps, eight DoD inputs/outputs and environment/version evidence are under this run's runtime/align-browser-*/. Missing environment or failed checks exit nonzero. Controlled accounts/results do not prove real-model teaching or genuine peer confirmation.

`python apitest/run_all.py --inject-failure` verifies a failure report/nonzero exit without contacting services. Per-method scripts share ordered setup; use the complete runner for dependent CRUD scenarios.
