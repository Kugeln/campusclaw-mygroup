param([string]$ConnectionFile='runtime/week5-phase1-preflight/test-connection.json')
$ErrorActionPreference='Stop'
$repoRoot=(Resolve-Path (Join-Path $PSScriptRoot '../..')).Path
Set-Location -LiteralPath $repoRoot
$connection=Get-Content -LiteralPath $ConnectionFile -Encoding UTF8 | ConvertFrom-Json
$project=$connection.project
if ($project -notmatch '^campusclaw-week5-rag-[a-f0-9]{8}$') { throw 'Existing isolated week5 project required' }
$info=docker network inspect "${project}_isolated" | ConvertFrom-Json
if ($LASTEXITCODE -ne 0 -or $info[0].Labels.'com.docker.compose.project' -ne $project) { throw 'Isolated network ownership mismatch' }
$state=docker inspect "${project}-mysql-1" --format '{{.State.Status}}'
if ($LASTEXITCODE -ne 0 -or $state.Trim() -ne 'running') { throw 'Existing isolated MySQL must be running' }
$env:WEEK5_TEST_DSN='week5_test:'+$connection.password+'@tcp(mysql:3306)/week5_rag_test?charset=utf8mb4&timeout=5s'
$env:WEEK5_ACCOUNT_PASSWORD=$connection.password
$runID=[guid]::NewGuid().ToString('N').Substring(0,8)
$container="${project}-phase2-${runID}"
$evidenceDir=Join-Path $repoRoot "runtime/$project"
$argsList=@('run','--name',$container,'--network',"${project}_isolated",'--mount',"type=bind,source=$repoRoot,target=/src,readonly",'-v',"${project}_go-cache:/tmp/go-cache",'-e','WEEK5_TEST_DSN','-e','WEEK5_ACCOUNT_PASSWORD','campusclaw-week5-rag-test:local','-p','1','-tags','tutorintegration','./internal/tutor','./internal/http','-run','TestTutorConfigurationRealMySQL|TestSelectedLoadedAndNoEvidence|TestConfigurationAndGuidanceConstraints','-v','-count=1')
try {
 $ErrorActionPreference='Continue'
 $output=(& docker @argsList 2>&1 | Out-String)
 $exitCode=$LASTEXITCODE
 $ErrorActionPreference='Stop'
 $output=$output.Replace($connection.password,'[redacted]')
 $output=[regex]::Replace($output,'eyJ[\w-]+\.[\w-]+\.[\w-]+','[redacted JWT]')
 $output | Set-Content -Encoding UTF8 -LiteralPath (Join-Path $evidenceDir "phase2-$runID.log")
 @{project=$project;container=$container;exit_code=$exitCode;controlled_tests_passed=($exitCode -eq 0);real_model_verified=$false;schema_upgrade_executed=$false;utc=[datetime]::UtcNow.ToString('o');log="phase2-$runID.log"} | ConvertTo-Json | Set-Content -Encoding UTF8 -LiteralPath (Join-Path $evidenceDir "phase2-$runID.json")
 Write-Output $output
 exit $exitCode
} finally {
 Remove-Item Env:WEEK5_TEST_DSN -ErrorAction SilentlyContinue
 Remove-Item Env:WEEK5_ACCOUNT_PASSWORD -ErrorAction SilentlyContinue
}
