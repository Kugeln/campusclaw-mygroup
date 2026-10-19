param([ValidateSet('Capture','Schema','Legacy','Store','Delete','RecoveryPrepare','RecoveryFinish','Phase2','Phase2API','Phase2Routing','Phase2Evidence','Phase2Roles','Phase2History','Phase3Core','Phase3','All')][string]$Stage='All',[string]$RecoveryRun='')
$ErrorActionPreference='Stop'
$repoRoot=(Resolve-Path (Join-Path $PSScriptRoot '../..')).Path
$c=Get-Content (Join-Path $repoRoot 'runtime/align-storage-connection.json') -Encoding UTF8 | ConvertFrom-Json
if ($c.container -notmatch '^campusclaw-align-storage-[a-f0-9]{8}$' -or $c.network -ne "$($c.container)-net") { throw 'Invalid isolated target' }
$info=docker inspect $c.container | ConvertFrom-Json
if ($LASTEXITCODE -ne 0 -or $info[0].Config.Labels.'campusclaw.phase' -ne 'align-storage' -or -not $info[0].State.Running) { throw 'New isolated MySQL is unavailable' }
if ($info[0].Mounts | Where-Object { $_.Type -ne 'tmpfs' }) { throw 'Foreign/persistent mount refused' }
$net=docker network inspect $c.network | ConvertFrom-Json
if (-not $net[0].Internal -or $net[0].Labels.'campusclaw.phase' -ne 'align-storage') { throw 'Isolated internal network required' }
$runID=[guid]::NewGuid().ToString('N').Substring(0,8)
$env:ALIGN_STORAGE_DSN="root:$($c.password)@tcp($($c.container):3306)/?charset=utf8mb4&timeout=5s"
$env:ALIGN_STORAGE_CONTAINER=$c.container
if ($Stage -like 'Recovery*' -and $RecoveryRun -notmatch '^[a-f0-9]{8}$') { throw 'A unique recovery run ID is required' }
$env:ALIGN_STORAGE_RECOVERY=$RecoveryRun
$cache=Join-Path $c.evidence 'linux-go-cache'
New-Item -ItemType Directory -Force -Path $cache | Out-Null
$pattern = if ($Stage -eq 'Capture') { '^TestAlignCapture$' } elseif ($Stage -eq 'All') { '^TestAlign(Schema|Legacy|Store|Delete)' } else { "^TestAlign$Stage" }
if ($Stage -eq 'Phase3Core') { $pattern='^TestAlignPhase3(Preparation|Four|Stream|Turn|Lifecycle|Flush)' }
$env:ALIGN_STREAM_QDRANT=''
if ($Stage -eq 'Phase3') {
 $q=Get-Content (Join-Path $repoRoot 'runtime/align-stream-connection.json') -Encoding UTF8 | ConvertFrom-Json
 if ($q.container -notmatch '^campusclaw-align-stream-[a-f0-9]{8}-qdrant$' -or $q.network -ne $c.network) { throw 'Invalid independent Qdrant target' }
 $qi=docker inspect $q.container | ConvertFrom-Json
 if ($LASTEXITCODE -ne 0 -or -not $qi[0].State.Running -or $qi[0].Config.Labels.'campusclaw.phase' -ne 'align-stream' -or -not $qi[0].NetworkSettings.Networks.($c.network)) { throw 'Isolated Qdrant required' }
 if ($qi[0].Mounts | Where-Object { $_.Type -ne 'tmpfs' }) { throw 'Persistent Qdrant mount refused' }
 if (-not $qi[0].HostConfig.Tmpfs.'/qdrant/storage') { throw 'Qdrant tmpfs required' }
 $env:ALIGN_STREAM_QDRANT="http://$($q.container):6333"
}
$argsList=@('run','--pull=never','--name',"$($c.container)-$($Stage.ToLower())-$runID",'--network',$c.network,
 '--mount',"type=bind,source=$repoRoot,target=/src,readonly",'--mount',"type=bind,source=$($c.evidence),target=/evidence",
 '--mount',"type=bind,source=$cache,target=/tmp/go-cache",'-e','ALIGN_STORAGE_DSN','-e','ALIGN_STORAGE_CONTAINER','-e','ALIGN_STORAGE_RECOVERY',
 '-e','ALIGN_STREAM_QDRANT','-e','ALIGN_STORAGE_EVIDENCE=/evidence','campusclaw-week5-rag-test:local','-p','1','-tags','alignstorage','./internal/repository/mysql','-run',$pattern,'-v','-count=1','-timeout','15m')
try {
 $previous=$ErrorActionPreference; $ErrorActionPreference='Continue'
 $output=(& docker @argsList 2>&1 | Out-String); $code=$LASTEXITCODE; $ErrorActionPreference=$previous
 $output=$output.Replace($c.password,'[redacted]')
 $log="$($Stage.ToLower())-$runID.log"
 $output | Set-Content (Join-Path $c.evidence $log) -Encoding UTF8
 @{stage=$Stage;container=$c.container;exit_code=$code;log=$log;utc=[datetime]::UtcNow.ToString('o');existing_databases_touched=$false;qdrant_accessed=($Stage -eq 'Phase3')} | ConvertTo-Json | Set-Content (Join-Path $c.evidence "$($Stage.ToLower())-$runID.json") -Encoding UTF8
 Write-Output $output
 exit $code
} finally { Remove-Item Env:ALIGN_STORAGE_DSN,Env:ALIGN_STORAGE_CONTAINER,Env:ALIGN_STORAGE_RECOVERY,Env:ALIGN_STREAM_QDRANT -ErrorAction SilentlyContinue }
