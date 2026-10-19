param([switch]$Phase5,[switch]$BrowserOnly,[switch]$P1P4)
if ($P1P4) { $Phase5=$true; $BrowserOnly=$true }
$ErrorActionPreference='Stop'
$repoRoot=(Resolve-Path (Join-Path $PSScriptRoot '../..')).Path
$c=Get-Content (Join-Path $repoRoot 'runtime/align-storage-connection.json') -Encoding UTF8 | ConvertFrom-Json
$q=Get-Content (Join-Path $repoRoot 'runtime/align-stream-connection.json') -Encoding UTF8 | ConvertFrom-Json
if ($c.container -notmatch '^campusclaw-align-storage-[a-f0-9]{8}$' -or $c.network -ne "$($c.container)-net" -or $q.container -notmatch '^campusclaw-align-stream-[a-f0-9]{8}-qdrant$' -or $q.network -ne $c.network) { throw 'Independent test targets required' }
$mysql=(docker inspect $c.container | ConvertFrom-Json)[0];$qdrant=(docker inspect $q.container | ConvertFrom-Json)[0]
if (!$mysql.State.Running -or $mysql.Config.Labels.'campusclaw.phase' -ne 'align-storage' -or !$mysql.HostConfig.Tmpfs.'/var/lib/mysql') { throw 'Independent tmpfs MySQL unavailable' }
if (!$qdrant.State.Running -or $qdrant.Config.Labels.'campusclaw.phase' -ne 'align-stream' -or !$qdrant.HostConfig.Tmpfs.'/qdrant/storage') { throw 'Independent tmpfs Qdrant unavailable' }
if (@($mysql.Mounts)+@($qdrant.Mounts) | Where-Object { $_.Type -ne 'tmpfs' }) { throw 'Persistent mounts refused' }
$net=(docker network inspect $c.network | ConvertFrom-Json)[0]
if (!$net.Internal -or $net.Labels.'campusclaw.phase' -ne 'align-storage') { throw 'Independent internal network required' }
$run=[guid]::NewGuid().ToString('N').Substring(0,8)
$app="campusclaw-align-ui-$run-app";$proxy="campusclaw-align-ui-$run-proxy"
$evidence=Join-Path $repoRoot "runtime/align-browser-$run";New-Item -ItemType Directory -Force -Path $evidence | Out-Null
$cache=Join-Path $c.evidence 'linux-go-cache'
$password=[guid]::NewGuid().ToString('N')+[guid]::NewGuid().ToString('N')
$env:ALIGN_STORAGE_DSN="root:$($c.password)@tcp($($c.container):3306)/?charset=utf8mb4&timeout=5s"
$env:ALIGN_STORAGE_CONTAINER=$c.container;$env:ALIGN_STREAM_QDRANT="http://$($q.container):6333"
$env:ALIGN_BROWSER_EVIDENCE='/evidence';$env:ALIGN_BROWSER_PASSWORD=$password
$env:ALIGN_BROWSER_RETAIN=if($Phase5){'1'}else{'0'}
$exitCode=1;$fixtureCode='not started'
try {
 docker run -d --pull=never --name $app --label campusclaw.phase=align-ui --network $c.network --mount "type=bind,source=$repoRoot,target=/src,readonly" --mount "type=bind,source=$evidence,target=/evidence" --mount "type=bind,source=$cache,target=/tmp/go-cache" -e ALIGN_STORAGE_DSN -e ALIGN_STORAGE_CONTAINER -e ALIGN_STREAM_QDRANT -e ALIGN_BROWSER_EVIDENCE -e ALIGN_BROWSER_PASSWORD -e ALIGN_BROWSER_RETAIN campusclaw-week5-rag-test:local -p 1 -tags alignstorage ./internal/repository/mysql -run '^TestAlignPhase4BrowserFixture$' -v -count=1 -timeout 25h | Out-Null
 if ($LASTEXITCODE -ne 0) { throw 'Independent fixture failed to start' }
 $ready=Join-Path $evidence 'ready.json';$deadline=[datetime]::UtcNow.AddSeconds(120)
 while (!(Test-Path -LiteralPath $ready)) {
  $state=docker inspect $app --format '{{.State.Status}}'
  if ($LASTEXITCODE -ne 0 -or $state.Trim() -eq 'exited' -or [datetime]::UtcNow -gt $deadline) { throw 'Fixture readiness failed; see redacted fixture.log' }
  Start-Sleep -Milliseconds 400
 }
 $config="daemon off;`n"+[IO.File]::ReadAllText((Join-Path $repoRoot 'deploy/nginx.conf')).Replace('http://app:8080',"http://${app}:8080")
 [IO.File]::WriteAllText((Join-Path $evidence 'nginx.conf'),$config,[Text.UTF8Encoding]::new($false))
 # Only the new loopback proxy joins bridge; app/MySQL/Qdrant stay internal.
 docker run -d --pull=never --name $proxy --label campusclaw.phase=align-ui --network bridge --mount "type=bind,source=$evidence,target=/phase4,readonly" -p '127.0.0.1::8080' --entrypoint sh nginx:1.28.0-alpine -c 'while [ ! -f /phase4/proxy-ready ]; do sleep 0.2; done; exec nginx -c /phase4/nginx.conf' | Out-Null
 if ($LASTEXITCODE -ne 0) { throw 'New proxy failed to start' }
 docker network connect $c.network $proxy
 if ($LASTEXITCODE -ne 0) { throw 'New proxy network attachment failed' }
 [IO.File]::WriteAllText((Join-Path $evidence 'proxy-ready'),'ready')
 $address=(docker port $proxy 8080/tcp | Out-String).Trim()
 $env:ALIGN_BROWSER_ORIGIN="http://$address";$env:ALIGN_BROWSER_READY=$ready;$env:ALIGN_BROWSER_RESULT=Join-Path $evidence 'browser.json'
 $healthy=$false
 for ($attempt=0;$attempt -lt 40;$attempt++){try{$res=Invoke-WebRequest -UseBasicParsing -Uri "$($env:ALIGN_BROWSER_ORIGIN)/health" -TimeoutSec 2;if($res.StatusCode -eq 200){$healthy=$true;break}}catch{Start-Sleep -Milliseconds 250}}
 if (!$healthy) { throw 'New proxy unavailable' }
 # Validate while the new app is still running, so Docker DNS resolves upstream.
 $effective=(& docker exec $proxy sh -c 'nginx -T -c /phase4/nginx.conf 2>&1' | Out-String)
 if ($LASTEXITCODE -ne 0) { throw 'Nginx effective configuration check failed' }
 $effective | Set-Content (Join-Path $evidence 'nginx-effective.txt') -Encoding UTF8
 $checker=if($P1P4){'tests/go_rag/check_align_p1p4.py'}else{'tests/go_rag/check_align_browser.py'}
 & (Join-Path $repoRoot '.venv/Scripts/python.exe') (Join-Path $repoRoot $checker)
 $exitCode=$LASTEXITCODE
 if ($exitCode -ne 0) { throw 'Browser acceptance failed' }
 if ($Phase5 -and !$BrowserOnly) {
  $env:APITEST_BASE_URL=$env:ALIGN_BROWSER_ORIGIN;$env:APITEST_PASSWORD=$password
  $env:APITEST_READY=$ready;$env:APITEST_DB_CONTAINER=$c.container
  $env:APITEST_SWAGGER_EVIDENCE=$env:ALIGN_BROWSER_RESULT
  & (Join-Path $repoRoot '.venv/Scripts/python.exe') (Join-Path $repoRoot 'apitest/run_all.py')
  $exitCode=$LASTEXITCODE
  if ($exitCode -ne 0) { throw 'Current API acceptance failed' }
 }
} finally {
 if (!$Phase5) {
 [IO.File]::WriteAllText((Join-Path $evidence 'stop'),'stop')
 $ErrorActionPreference='Continue'
 $state=docker inspect $app --format '{{.State.Status}}'
 if ($LASTEXITCODE -eq 0) { $fixtureCode=(docker wait $app | Out-String).Trim();$log=(docker logs $app 2>&1 | Out-String).Replace($c.password,'[redacted]').Replace($password,'[redacted]');[IO.File]::WriteAllText((Join-Path $evidence 'fixture.log'),$log,[Text.UTF8Encoding]::new($false)) }
 $proxyInfo=(docker inspect $proxy | ConvertFrom-Json)[0]
 if ($LASTEXITCODE -eq 0 -and $proxyInfo.Config.Labels.'campusclaw.phase' -eq 'align-ui' -and $proxy -match '^campusclaw-align-ui-[a-f0-9]{8}-proxy$') { docker stop $proxy | Out-Null }
 } else {
  $fixtureCode='retained running; no stop/cleanup requested'
  if($env:ALIGN_BROWSER_ORIGIN){
   try { (Invoke-WebRequest -UseBasicParsing -Uri "$($env:ALIGN_BROWSER_ORIGIN)/__phase4/control" -TimeoutSec 5).Content | Set-Content (Join-Path $evidence 'stats.json') -Encoding UTF8 } catch { Write-Output 'Live stats unavailable; fixture and failed evidence retained' }
  }
  $log=(docker logs $app 2>&1 | Out-String).Replace($c.password,'[redacted]').Replace($password,'[redacted]')
  [IO.File]::WriteAllText((Join-Path $evidence 'fixture.log'),$log,[Text.UTF8Encoding]::new($false))
 }
 @{utc=[datetime]::UtcNow.ToString('o');app=$app;proxy=$proxy;mysql=$c.container;qdrant=$q.container;network=$c.network;exit_code=$exitCode;fixture_exit_code=$fixtureCode;classroom_resources_modified=$false;real_models_called=$false} | ConvertTo-Json | Set-Content (Join-Path $evidence 'result.json') -Encoding UTF8
 'ALIGN_STORAGE_DSN','ALIGN_STORAGE_CONTAINER','ALIGN_STREAM_QDRANT','ALIGN_BROWSER_EVIDENCE','ALIGN_BROWSER_PASSWORD','ALIGN_BROWSER_ORIGIN','ALIGN_BROWSER_READY','ALIGN_BROWSER_RESULT','ALIGN_BROWSER_RETAIN','APITEST_PASSWORD' | ForEach-Object { Remove-Item "Env:$_" -ErrorAction SilentlyContinue }
 Write-Output "Phase 4 evidence: $evidence"
}
exit $exitCode
