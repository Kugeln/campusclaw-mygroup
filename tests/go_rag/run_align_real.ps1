param([Parameter(Mandatory=$true)][string]$Evidence,[switch]$SingleCitationRetest,[switch]$RemainingTeaching)
$ErrorActionPreference='Stop'
$repoRoot=(Resolve-Path (Join-Path $PSScriptRoot '../..')).Path
$evidenceRoot=(Resolve-Path -LiteralPath $Evidence).Path
if(!$evidenceRoot.StartsWith((Join-Path $repoRoot 'runtime/align-real-'),[StringComparison]::OrdinalIgnoreCase)){throw 'Isolated evidence path required'}
$ledger=Get-Content (Join-Path $evidenceRoot 'budget.json') -Encoding UTF8 | ConvertFrom-Json
if($SingleCitationRetest -and $RemainingTeaching){throw 'Select one explicitly authorized mode'}
if($RemainingTeaching){
 if($ledger.dialogues -ne 3 -or $ledger.embeddings -ne 4 -or !$ledger.blocked){throw 'Original stopped 3/4 budget required; no repeated remaining run'}
} elseif($SingleCitationRetest){
 if($ledger.dialogues -ne 2 -or $ledger.embeddings -ne 3 -or !$ledger.blocked){throw 'Original stopped 2/3 budget required; no second attempt'}
} elseif($ledger.dialogues -ne 1 -or $ledger.embeddings -ne 1 -or $ledger.blocked){throw 'Fresh 1/1 successful connectivity required; teaching cannot rerun'}
$c=Get-Content (Join-Path $repoRoot 'runtime/align-storage-connection.json') -Encoding UTF8 | ConvertFrom-Json
$q=Get-Content (Join-Path $repoRoot 'runtime/align-stream-connection.json') -Encoding UTF8 | ConvertFrom-Json
if($c.container -notmatch '^campusclaw-align-storage-[a-f0-9]{8}$' -or $c.network -ne "$($c.container)-net" -or $q.container -notmatch '^campusclaw-align-stream-[a-f0-9]{8}-qdrant$' -or $q.network -ne $c.network){throw 'Independent targets required'}
$mi=(docker inspect $c.container | ConvertFrom-Json)[0];$qi=(docker inspect $q.container | ConvertFrom-Json)[0]
if(!$mi.State.Running -or !$qi.State.Running -or $mi.Config.Labels.'campusclaw.phase' -ne 'align-storage' -or $qi.Config.Labels.'campusclaw.phase' -ne 'align-stream' -or !$mi.HostConfig.Tmpfs.'/var/lib/mysql' -or !$qi.HostConfig.Tmpfs.'/qdrant/storage' -or $mi.Mounts.Count -ne 0 -or $qi.Mounts.Count -ne 0){throw 'Independent running tmpfs resources required'}
$ni=(docker network inspect $c.network | ConvertFrom-Json)[0]
if(!$ni.Internal -or $ni.Labels.'campusclaw.phase' -ne 'align-storage'){throw 'Independent internal network required'}
$config=@{}
foreach($line in [IO.File]::ReadAllLines((Join-Path $repoRoot '.env.course.local'))){if($line -match '^((?:EMBEDDING|DIALOGUE)_[A-Z_]+)=(.*)$'){$config[$Matches[1]]=$Matches[2]}}
$vars=@('DIALOGUE_BASE_URL','DIALOGUE_MODEL','DIALOGUE_API_KEY','EMBEDDING_BASE_URL','EMBEDDING_MODEL','EMBEDDING_API_KEY','EMBEDDING_DIMENSION','EMBEDDING_API_FORMAT')
foreach($key in $vars){if(!$config[$key]){throw 'Missing official configuration'};[Environment]::SetEnvironmentVariable($key,$config[$key],'Process')}
$env:ALIGN_STORAGE_DSN="root:$($c.password)@tcp($($c.container):3306)/?charset=utf8mb4&timeout=5s"
$env:ALIGN_STORAGE_CONTAINER=$c.container;$env:ALIGN_STREAM_QDRANT="http://$($q.container):6333"
$env:ALIGN_REAL_EVIDENCE='/evidence';$env:ALIGN_REAL_AUTHORIZED='course-12-8'
if($SingleCitationRetest){$env:ALIGN_REAL_AUTHORIZED='citation-retest-1-1';$env:ALIGN_REAL_ATTEMPT='citation-retest-'+[guid]::NewGuid().ToString('N').Substring(0,8)}
if($RemainingTeaching){$env:ALIGN_REAL_AUTHORIZED='remaining-real-9-4';$env:ALIGN_REAL_ATTEMPT='remaining-real-'+[guid]::NewGuid().ToString('N').Substring(0,8)}
$scopedReal=$SingleCitationRetest -or $RemainingTeaching
$run=[guid]::NewGuid().ToString('N').Substring(0,8);$app="campusclaw-align-real-$run"
$cache=Join-Path $c.evidence 'linux-go-cache';$code=1
try {
 $arguments=@('run','-d','--pull=never','--name',$app,'--label','campusclaw.phase=align-real','--network','bridge','--mount',"type=bind,source=$repoRoot,target=/src,readonly",'--mount',"type=bind,source=$evidenceRoot,target=/evidence",'--mount',"type=bind,source=$cache,target=/tmp/go-cache")
 foreach($key in ($vars+@('ALIGN_STORAGE_DSN','ALIGN_STORAGE_CONTAINER','ALIGN_STREAM_QDRANT','ALIGN_REAL_EVIDENCE','ALIGN_REAL_AUTHORIZED','ALIGN_REAL_ATTEMPT'))){$arguments+=@('-e',$key)}
 $testName=if($RemainingTeaching){'TestAlignPhase5RemainingReal'}elseif($SingleCitationRetest){'TestAlignPhase5CitationRetest'}else{'TestAlignPhase5RealTeaching'}
 $gate=if($scopedReal){"network-ready-$($env:ALIGN_REAL_ATTEMPT)"}else{'network-ready'}
 $command='while [ ! -f /evidence/'+$gate+' ]; do sleep 0.2; done; exec go test -p 1 -tags alignstorage ./internal/repository/mysql -run "^'+$testName+'$" -v -count=1 -timeout 15m'
 $arguments+=@('--entrypoint','sh','campusclaw-week5-rag-test:local','-c',$command)
 docker @arguments | Out-Null
 if($LASTEXITCODE -ne 0){throw 'New real test process failed to start'}
 docker network connect $c.network $app
 if($LASTEXITCODE -ne 0){throw 'New process internal network attachment failed'}
 [IO.File]::WriteAllText((Join-Path $evidenceRoot $gate),'ready')
 $code=[int]((docker wait $app | Out-String).Trim())
 $previous=$ErrorActionPreference;$ErrorActionPreference='Continue'
 $log=(docker logs $app 2>&1 | Out-String).Replace($c.password,'[redacted]')
 $ErrorActionPreference=$previous
 foreach($key in @('DIALOGUE_API_KEY','EMBEDDING_API_KEY')){$log=$log.Replace($config[$key],'[redacted]')}
 $logName=if($scopedReal){"$($env:ALIGN_REAL_ATTEMPT)-fixture.log"}else{'teaching.log'}
 [IO.File]::WriteAllText((Join-Path $evidenceRoot $logName),$log,[Text.UTF8Encoding]::new($false))
 Write-Output $log
} finally {
 $resultName=if($scopedReal){"$($env:ALIGN_REAL_ATTEMPT)-run.json"}else{'run-result.json'}
 @{utc=[datetime]::UtcNow.ToString('o');app=$app;exit_code=$code;mysql=$c.container;qdrant=$q.container;network=$c.network;cleanup=$false;classroom_modified=$false;single_citation_retest=[bool]$SingleCitationRetest;attempt=$env:ALIGN_REAL_ATTEMPT} | ConvertTo-Json | Set-Content (Join-Path $evidenceRoot $resultName) -Encoding UTF8
 foreach($key in ($vars+@('ALIGN_STORAGE_DSN','ALIGN_STORAGE_CONTAINER','ALIGN_STREAM_QDRANT','ALIGN_REAL_EVIDENCE','ALIGN_REAL_AUTHORIZED','ALIGN_REAL_ATTEMPT'))){[Environment]::SetEnvironmentVariable($key,$null,'Process')}
 Write-Output "Retained real teaching evidence: $evidenceRoot"
}
exit $code
