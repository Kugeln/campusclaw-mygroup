param([string]$ConnectionFile = 'runtime/week5-phase1-preflight/test-connection.json')
$ErrorActionPreference = 'Stop'
$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '../..')).Path
Set-Location -LiteralPath $repoRoot
$connection = Get-Content -LiteralPath $ConnectionFile -Encoding UTF8 | ConvertFrom-Json
if ($connection.project -notmatch '^campusclaw-week5-rag-[a-f0-9]{8}$') { throw 'An isolated week5 project is required' }
$project = $connection.project
$network = "${project}_isolated"
$networkInfo = docker network inspect $network | ConvertFrom-Json
if ($LASTEXITCODE -ne 0 -or $networkInfo[0].Labels.'com.docker.compose.project' -ne $project) { throw 'Isolated network ownership mismatch' }
foreach ($service in @('mysql', 'qdrant')) {
    $state = docker inspect "${project}-${service}-1" --format '{{.State.Status}}'
    if ($LASTEXITCODE -ne 0 -or $state.Trim() -ne 'running') { throw "Test service unavailable: $service" }
}
$evidenceDir = Join-Path $repoRoot "runtime/$project"
New-Item -ItemType Directory -Path $evidenceDir -Force | Out-Null
$env:WEEK5_TEST_DSN = 'week5_test:' + $connection.password + '@tcp(mysql:3306)/week5_rag_test?charset=utf8mb4&timeout=5s'
$env:WEEK5_ACCOUNT_PASSWORD = $connection.password
$env:WEEK5_UPLOAD_ROOT = '/uploads'
$runID = [guid]::NewGuid().ToString('N').Substring(0,8)
$container = "${project}-verify-${runID}"
$arguments = @('run','--name',$container,'--network',$network,
    '--mount',"type=bind,source=$repoRoot,target=/src,readonly",
    '-v',"${project}_uploads-test:/uploads",'-v',"${project}_go-cache:/tmp/go-cache",
    '-e','WEEK5_TEST_DSN','-e','WEEK5_ACCOUNT_PASSWORD','-e','WEEK5_UPLOAD_ROOT',
    'campusclaw-week5-rag-test:local','-p','1','-tags','ragintegration',
    './internal/rag','./internal/repository/mysql','./internal/http',
    '-run','TestGoRAGRealServices|TestRAGSchemaIntegrity|TestChunkingAndRRF|TestEvidenceAnswers|TestEmbeddingProtocolsAndValidation|TestThresholdAndPayloadDoubleCheck',
    '-v','-count=1')
try {
    $previousPreference = $ErrorActionPreference
    $ErrorActionPreference = 'Continue'
    $output = (& docker @arguments 2>&1 | Out-String)
    $exitCode = $LASTEXITCODE
    $ErrorActionPreference = $previousPreference
    $redacted = $output.Replace($connection.password,'[redacted]')
    $redacted = [regex]::Replace($redacted,'eyJ[\w-]+\.[\w-]+\.[\w-]+','[redacted JWT]')
    $redacted | Set-Content -LiteralPath (Join-Path $evidenceDir "go-tests-$runID.log") -Encoding UTF8
    [ordered]@{project=$project;network=$network;container=$container;database='week5_rag_test';
        accepted=($exitCode -eq 0);exit_code=$exitCode;utc=[datetime]::UtcNow.ToString('o');
        log="go-tests-$runID.log";classroom_resources_touched=$false} |
        ConvertTo-Json | Set-Content -LiteralPath (Join-Path $evidenceDir "results-$runID.json") -Encoding UTF8
    Write-Output $redacted
    exit $exitCode
} finally {
    Remove-Item Env:WEEK5_TEST_DSN -ErrorAction SilentlyContinue
    Remove-Item Env:WEEK5_ACCOUNT_PASSWORD -ErrorAction SilentlyContinue
}
