param([string]$ConnectionFile='runtime/week5-phase1-preflight/test-connection.json',[ValidateSet('phase4','phase5','phase6','realmodel')][string]$Stage='phase4',[string]$ModelEnvFile='')
$ErrorActionPreference='Stop'
$repoRoot=(Resolve-Path (Join-Path $PSScriptRoot '../..')).Path
Set-Location -LiteralPath $repoRoot
if ($Stage -eq 'phase6') {
 # Superseding change: never use the old fixed database or reverse assertions.
 & powershell -NoProfile -File tests/go_rag/run_align_acceptance.ps1
 exit $LASTEXITCODE
}
$connection=Get-Content -LiteralPath $ConnectionFile -Encoding UTF8 | ConvertFrom-Json
$project=$connection.project
if ($project -notmatch '^campusclaw-week5-rag-[a-f0-9]{8}$') { throw 'Existing isolated project required' }
$network=docker network inspect "${project}_isolated" | ConvertFrom-Json
if ($LASTEXITCODE -ne 0 -or $network[0].Labels.'com.docker.compose.project' -ne $project) { throw 'Network ownership mismatch' }
if ($Stage -eq 'phase6') {
 & powershell -NoProfile -File tests/go_rag/run_phase3.ps1 -ConnectionFile $ConnectionFile
 if ($LASTEXITCODE -ne 0) {throw 'Fresh Go fault regression failed'}
}
$runID=[guid]::NewGuid().ToString('N').Substring(0,8)
$evidenceDir=Join-Path $repoRoot "runtime/$project/${Stage}-$runID"
$previous=Get-ChildItem (Join-Path $repoRoot "runtime/$project") -Directory -Filter 'phase4-*' | Sort-Object LastWriteTime -Descending | Where-Object { Test-Path -LiteralPath (Join-Path $_.FullName 'browser.json') } | Select-Object -First 1
if ($previous) {$env:WEEK5_BROWSER_PREVIOUS_READY=Join-Path $previous.FullName 'ready.json'}
[IO.Directory]::CreateDirectory($evidenceDir) | Out-Null
$appName="${project}-ui-${runID}"
$proxyName="${project}-proxy-${runID}"
$env:WEEK5_TEST_DSN='week5_test:'+$connection.password+'@tcp(mysql:3306)/week5_rag_test?charset=utf8mb4&timeout=5s'
$env:WEEK5_ACCOUNT_PASSWORD=$connection.password
$env:WEEK5_BROWSER_EVIDENCE='/evidence'
if ($Stage -eq 'phase6') {$env:WEEK5_APITEST='1'}
if ($Stage -eq 'realmodel') {
 $priorVars=@{}
 if ($ModelEnvFile) {
  $modelPath=(Resolve-Path -LiteralPath $ModelEnvFile).Path
  git check-ignore --quiet -- $modelPath
  if ($LASTEXITCODE -ne 0) {throw 'Model configuration must be Git ignored'}
  foreach($entry in [IO.File]::ReadAllLines($modelPath)){if($entry -match '^([A-Z_]+)=(.*)$'){$priorVars[$Matches[1]]=$Matches[2]}}
 } else {
  $prior=(docker inspect campusclaw-retrieval-demo-app-1 | ConvertFrom-Json)[0]
  if ($LASTEXITCODE -ne 0) {throw 'Prior Doubao configuration unavailable; supply -ModelEnvFile'}
  foreach($entry in $prior.Config.Env){$parts=$entry -split '=',2;if($parts.Count -eq 2){$priorVars[$parts[0]]=$parts[1]}}
 }
 $realKeys=@('EMBEDDING_BASE_URL','EMBEDDING_API_KEY','EMBEDDING_MODEL','EMBEDDING_DIMENSION','DIALOGUE_BASE_URL','DIALOGUE_API_KEY','DIALOGUE_MODEL')
 foreach($key in $realKeys){if(!$priorVars[$key]){throw "Missing real configuration: $key"};[Environment]::SetEnvironmentVariable($key,$priorVars[$key],'Process')}
 $env:WEEK5_REAL_MODEL='1';$env:EMBEDDING_API_FORMAT=if($priorVars['EMBEDDING_API_FORMAT']){$priorVars['EMBEDDING_API_FORMAT']}else{'ark_multimodal'};$env:RAG_ENABLED='true';$env:QDRANT_URL='http://qdrant:6333'
}
$exitCode=1
try {
 $arguments=@('run','-d','--name',$appName,'--network',"${project}_isolated",'--mount',"type=bind,source=$repoRoot,target=/src,readonly",'--mount',"type=bind,source=$evidenceDir,target=/evidence",'-v',"${project}_go-cache:/tmp/go-cache",'-e','WEEK5_TEST_DSN','-e','WEEK5_ACCOUNT_PASSWORD','-e','WEEK5_BROWSER_EVIDENCE','-e','WEEK5_APITEST','campusclaw-week5-rag-test:local','-p','1','-tags','tutorbrowser','./internal/http','-run','^TestTutorBrowserFixture$','-v','-count=1')
 if ($Stage -eq 'realmodel') {
  $extras=@('--pull=never','--rm','-e','WEEK5_REAL_MODEL','-e','EMBEDDING_API_FORMAT','-e','RAG_ENABLED','-e','QDRANT_URL')
  foreach($key in $realKeys){$extras+=@('-e',$key)}
  $arguments=$arguments[0..1]+$extras+$arguments[2..($arguments.Count-1)]
 }
 docker @arguments | Out-Null
 if ($LASTEXITCODE -ne 0) {throw 'Fixture runner failed to start'}
 if ($Stage -eq 'realmodel') {
  docker network connect bridge $appName
  if ($LASTEXITCODE -ne 0) {throw 'Real model outbound attachment failed'}
  [IO.File]::WriteAllText((Join-Path $evidenceDir 'network-ready'),'ready')
 }
 $ready=Join-Path $evidenceDir 'ready.json'
 $deadline=[datetime]::UtcNow.AddSeconds(90)
 while (!(Test-Path -LiteralPath $ready)) {
  if ([datetime]::UtcNow -gt $deadline) {throw 'Fixture startup timed out'}
  $state=docker inspect $appName --format '{{.State.Status}}'
  if ($LASTEXITCODE -ne 0 -or $state.Trim() -eq 'exited') {throw 'Fixture exited before readiness; inspect redacted real-stats.json'}
  Start-Sleep -Milliseconds 500
 }
 $config="daemon off;`n"+[IO.File]::ReadAllText((Join-Path $repoRoot 'deploy/nginx.conf')).Replace('http://app:8080',"http://${appName}:8080")
 $configPath=Join-Path $evidenceDir 'nginx.conf'
 [IO.File]::WriteAllText($configPath,$config,[Text.UTF8Encoding]::new($false))
 # Internal networks do not publish host ports. Only this new test proxy joins
 # the existing bridge for loopback publishing, then the private test network.
 docker run -d --name $proxyName --network bridge --mount "type=bind,source=$evidenceDir,target=/phase4,readonly" -p '127.0.0.1::8080' --entrypoint sh nginx:1.28.0-alpine -c 'while [ ! -f /phase4/proxy-ready ]; do sleep 0.2; done; exec nginx -c /phase4/nginx.conf' | Out-Null
 if ($LASTEXITCODE -ne 0) {throw 'Isolated proxy failed to start'}
 docker network connect "${project}_isolated" $proxyName
 if ($LASTEXITCODE -ne 0) {throw 'Isolated proxy attachment failed'}
 [IO.File]::WriteAllText((Join-Path $evidenceDir 'proxy-ready'),'ready')
 $port=docker port $proxyName 8080/tcp
 $env:WEEK5_BROWSER_ORIGIN='http://'+$port.Trim()
 $proxyReady=$false
 for ($attempt=0; $attempt -lt 30; $attempt++) {
  try { $health=Invoke-WebRequest -UseBasicParsing -Uri ($env:WEEK5_BROWSER_ORIGIN+'/health') -TimeoutSec 2; if ($health.StatusCode -eq 200) {$proxyReady=$true;break} } catch {Start-Sleep -Milliseconds 300}
 }
 if (!$proxyReady) {throw 'Test proxy did not become healthy'}
 $env:WEEK5_BROWSER_READY=$ready
 $env:WEEK5_BROWSER_RESULT=Join-Path $evidenceDir 'browser.json'
 if ($Stage -eq 'phase6') {
  $fixture=Get-Content -Encoding UTF8 -LiteralPath $ready | ConvertFrom-Json
  $env:APITEST_BASE_URL="http://${proxyName}:8080"
  $env:APITEST_GATEWAY_URL="http://${appName}:8090"
  $env:APITEST_DB_HOST='mysql';$env:APITEST_DB_NAME='week5_rag_test';$env:APITEST_DB_USER='week5_test';$env:APITEST_DB_PASSWORD=$connection.password
  $env:APITEST_PASSWORD=$connection.password;$env:APITEST_A2=$fixture.a2_username;$env:APITEST_NAMESPACE="week5_$runID"
  $goEvidence=Get-ChildItem (Join-Path $repoRoot "runtime/$project") -Filter 'phase3-*.json' | Sort-Object LastWriteTime -Descending | Select-Object -First 1
  $swaggerEvidence=Get-ChildItem (Join-Path $repoRoot "runtime/$project") -Directory -Filter 'phase5-*' | Sort-Object LastWriteTime -Descending | Where-Object {Test-Path -LiteralPath (Join-Path $_.FullName 'browser.json')} | Select-Object -First 1
  if (!$swaggerEvidence) {throw 'Phase5 browser evidence required'}
  $env:APITEST_GO_EVIDENCE="/src/runtime/$project/$($goEvidence.Name)"
  $env:APITEST_SWAGGER_EVIDENCE="/src/runtime/$project/$($swaggerEvidence.Name)/browser.json"
  $env:PYTHONPATH='/dependencies'
  $pymysql=(Resolve-Path (Join-Path $repoRoot '.venv/Lib/site-packages/pymysql')).Path
  $reports=Join-Path $repoRoot 'reports';[IO.Directory]::CreateDirectory($reports) | Out-Null
  $pyArgs=@('run','--name',"${project}-apitest-${runID}",'--network',"${project}_isolated",'--mount',"type=bind,source=$repoRoot,target=/src,readonly",'--mount',"type=bind,source=$reports,target=/src/reports",'--mount',"type=bind,source=$pymysql,target=/dependencies/pymysql,readonly")
  foreach ($key in @('APITEST_BASE_URL','APITEST_GATEWAY_URL','APITEST_DB_HOST','APITEST_DB_NAME','APITEST_DB_USER','APITEST_DB_PASSWORD','APITEST_PASSWORD','APITEST_A2','APITEST_NAMESPACE','APITEST_GO_EVIDENCE','APITEST_SWAGGER_EVIDENCE','PYTHONPATH')) {$pyArgs+=@('-e',$key)}
  $pyArgs+=@('python:3.12-slim','python','/src/apitest/run_all.py')
  & docker @pyArgs
  $exitCode=$LASTEXITCODE
  if ($exitCode -ne 0) {throw 'API acceptance failed; see ignored HTML report'}
 } else {
  $browserScript=if ($Stage -eq 'phase5') {'tests/go_rag/check_swagger_browser.py'} elseif ($Stage -eq 'realmodel') {'tests/go_rag/check_real_tutor_browser.py'} else {'tests/go_rag/check_tutor_browser.py'}
  & (Join-Path $repoRoot '.venv/Scripts/python.exe') $browserScript
  $exitCode=$LASTEXITCODE
  if ($exitCode -ne 0) {throw 'Browser acceptance failed'}
 }
} finally {
 [IO.File]::WriteAllText((Join-Path $evidenceDir 'stop'),'stop')
 $ErrorActionPreference='Continue'
 if ($Stage -eq 'realmodel') {$fixtureExit='ephemeral real-model fixture auto-removed';$log='Real-model evidence recorded in real-stats.json and browser.json; no credentials logged.'}
 else {$fixtureExit=(& docker wait $appName | Out-String).Trim();$log=(& docker logs $appName 2>&1 | Out-String).Replace($connection.password,'[redacted]')}
 [IO.File]::WriteAllText((Join-Path $evidenceDir 'fixture.log'),$log,[Text.UTF8Encoding]::new($false))
 $realVerified=$false
 if ($Stage -eq 'realmodel' -and $exitCode -eq 0 -and (Test-Path -LiteralPath (Join-Path $evidenceDir 'browser.json'))) {$realVerified=(Get-Content -Encoding UTF8 (Join-Path $evidenceDir 'browser.json') | ConvertFrom-Json).real_model_verified -eq $true}
 @{project=$project;app=$appName;proxy=$proxyName;exit_code=$exitCode;fixture_exit_code=$fixtureExit;schema_upgrade_executed=$false;real_model_verified=$realVerified} | ConvertTo-Json | Set-Content -Encoding UTF8 -LiteralPath (Join-Path $evidenceDir 'result.json')
 'WEEK5_TEST_DSN','WEEK5_ACCOUNT_PASSWORD','WEEK5_BROWSER_EVIDENCE','WEEK5_BROWSER_ORIGIN','WEEK5_BROWSER_READY','WEEK5_BROWSER_RESULT','WEEK5_BROWSER_PREVIOUS_READY','WEEK5_APITEST','APITEST_BASE_URL','APITEST_GATEWAY_URL','APITEST_DB_HOST','APITEST_DB_NAME','APITEST_DB_USER','APITEST_DB_PASSWORD','APITEST_PASSWORD','APITEST_A2','APITEST_NAMESPACE','APITEST_GO_EVIDENCE','APITEST_SWAGGER_EVIDENCE','PYTHONPATH' | ForEach-Object {Remove-Item "Env:$_" -ErrorAction SilentlyContinue}
 if ($Stage -eq 'realmodel') {foreach($key in ($realKeys+@('WEEK5_REAL_MODEL','EMBEDDING_API_FORMAT','RAG_ENABLED','QDRANT_URL'))){[Environment]::SetEnvironmentVariable($key,$null,'Process')}}
}
exit $exitCode
