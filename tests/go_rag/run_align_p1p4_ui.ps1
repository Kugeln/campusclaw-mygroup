param([Parameter(Mandatory=$true)][string]$Evidence)
$ErrorActionPreference='Stop'
$repoRoot=(Resolve-Path (Join-Path $PSScriptRoot '../..')).Path
$evidencePath=(Resolve-Path -LiteralPath $Evidence).Path
if (!$evidencePath.StartsWith((Join-Path $repoRoot 'runtime/align-browser-'),[StringComparison]::OrdinalIgnoreCase)) { throw 'Independent browser evidence required' }
$id=Split-Path $evidencePath -Leaf
if ($id -notmatch '^align-browser-([a-f0-9]{8})$') { throw 'Unexpected fixture name' }
$run=$Matches[1];$app="campusclaw-align-ui-$run-app";$proxy="campusclaw-align-ui-$run-proxy"
$result=Get-Content (Join-Path $evidencePath 'browser.json') -Encoding UTF8 | ConvertFrom-Json
if ($result.status -ne 'passed' -or $result.peer_confirmation) { throw 'Completed technical-only fixture required' }
$info=(docker inspect $app | ConvertFrom-Json)[0]
if (!$info.State.Running -or $info.Config.Labels.'campusclaw.phase' -ne 'align-ui' -or @($info.NetworkSettings.Networks.PSObject.Properties.Name).Count -ne 1) { throw 'Independent internal test app required' }
$proxyInfo=(docker inspect $proxy | ConvertFrom-Json)[0]
if (!$proxyInfo.State.Running -or $proxyInfo.Config.Labels.'campusclaw.phase' -ne 'align-ui') { throw 'Independent test proxy required' }
# Read just the generated test-login password into a child-process environment.
# Never print or persist Config.Env, database passwords, JWTs or login secrets.
$testPassword=@($info.Config.Env | Where-Object { $_.StartsWith('ALIGN_BROWSER_PASSWORD=') })
if ($testPassword.Count -ne 1) { throw 'Generated fixture login unavailable' }
$env:ALIGN_BROWSER_PASSWORD=$testPassword[0].Substring('ALIGN_BROWSER_PASSWORD='.Length)
$env:ALIGN_BROWSER_ORIGIN=$result.environment.origin
$env:ALIGN_BROWSER_READY=Join-Path $evidencePath 'ready.json'
$isolation=Get-Content (Join-Path $evidencePath 'isolation.json') -Encoding UTF8 | ConvertFrom-Json
$databaseTargets=@($isolation.containers | Where-Object { $_.phase -eq 'align-storage' })
if ($databaseTargets.Count -ne 1 -or $databaseTargets[0].name -notmatch '^campusclaw-align-storage-[a-f0-9]{8}$') { throw 'Independent database target required' }
$env:ALIGN_STORAGE_CONTAINER=$databaseTargets[0].name
try {
 & (Join-Path $repoRoot '.venv/Scripts/python.exe') (Join-Path $repoRoot 'tests/go_rag/check_align_p1p4_ui.py')
 if ($LASTEXITCODE -ne 0) { throw 'Production UI supplement failed; stop and retain evidence' }
} finally {
 'ALIGN_BROWSER_PASSWORD','ALIGN_BROWSER_ORIGIN','ALIGN_BROWSER_READY','ALIGN_STORAGE_CONTAINER' | ForEach-Object { Remove-Item "Env:$_" -ErrorAction SilentlyContinue }
}
