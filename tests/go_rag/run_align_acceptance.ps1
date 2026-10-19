# Current Week5 acceptance only. No paid models, classroom Compose or cleanup.
$ErrorActionPreference='Stop'
$repoRoot=(Resolve-Path (Join-Path $PSScriptRoot '../..')).Path
Set-Location -LiteralPath $repoRoot
$run=[guid]::NewGuid().ToString('N').Substring(0,8)
$out=Join-Path $repoRoot "runtime/align-acceptance-$run"
New-Item -ItemType Directory -Path $out | Out-Null
$results=@()
foreach($stage in @('All','Phase2','Phase3')) {
 & powershell -NoProfile -File tests/go_rag/run_align_storage.ps1 -Stage $stage *> (Join-Path $out "$stage.log")
 $code=$LASTEXITCODE
 $results+=@{stage=$stage;exit_code=$code}
 if($code -ne 0){throw "Fresh $stage regression failed; retained evidence: $out"}
}
$env:GOCACHE=Join-Path $repoRoot 'runtime/align-week5-phase1/go-cache'
& go test ./cmd/... ./internal/... -count=1 *> (Join-Path $out 'unit.log')
if($LASTEXITCODE -ne 0){throw 'Current Go unit regression failed'}
& node --test tests/go_rag/sse.test.cjs *> (Join-Path $out 'sse.log')
if($LASTEXITCODE -ne 0){throw 'Shared SSE parser regression failed'}
& powershell -NoProfile -File scripts/generate_openapi.ps1 -Check *> (Join-Path $out 'openapi.log')
if($LASTEXITCODE -ne 0){throw 'OpenAPI generation drift or environment failure'}
$env:APITEST_GO_EVIDENCE=Join-Path $out 'go-evidence.json'
@{utc=[datetime]::UtcNow.ToString('o');controlled_tests_passed=$true;exit_code=0;stages=$results;real_models=$false} | ConvertTo-Json -Depth 5 | Set-Content $env:APITEST_GO_EVIDENCE -Encoding UTF8
$env:APITEST_RESULT_POINTER=Join-Path $out 'api-pointer.json'
& powershell -NoProfile -File tests/go_rag/run_align_browser.ps1 -Phase5
if($LASTEXITCODE -ne 0){exit $LASTEXITCODE}
# The first browser run supplies automated UI/Swagger evidence for 5.1.
# Only after the complete API matrix passes, accept DoDs in another fresh fixture.
& powershell -NoProfile -File tests/go_rag/run_align_browser.ps1 -Phase5 -BrowserOnly
exit $LASTEXITCODE
