$ErrorActionPreference='Stop'
$repoRoot=(Resolve-Path (Join-Path $PSScriptRoot '../..')).Path
$connection=Join-Path $repoRoot 'runtime/align-stream-connection.json'
if (Test-Path -LiteralPath $connection) { throw 'Stream target already recorded; inspect it rather than replace it' }
$c=Get-Content (Join-Path $repoRoot 'runtime/align-storage-connection.json') -Encoding UTF8 | ConvertFrom-Json
if ($c.container -notmatch '^campusclaw-align-storage-[a-f0-9]{8}$' -or $c.network -ne "$($c.container)-net") { throw 'Invalid independent network' }
$net=docker network inspect $c.network | ConvertFrom-Json
if ($LASTEXITCODE -ne 0 -or -not $net[0].Internal -or $net[0].Labels.'campusclaw.phase' -ne 'align-storage') { throw 'Internal test network required' }
$name='campusclaw-align-stream-'+[guid]::NewGuid().ToString('N').Substring(0,8)+'-qdrant'
# New container only, local image only, no host ports, no persistent volumes,
# no Compose and no mutation or cleanup of any existing collection/container.
docker run -d --pull=never --name $name --restart=no --network $c.network --label campusclaw.phase=align-stream --tmpfs /qdrant/storage:rw qdrant/qdrant:v1.13.4
if ($LASTEXITCODE -ne 0) { throw 'New test Qdrant startup failed' }
@{container=$name;network=$c.network;image='qdrant/qdrant:v1.13.4';utc=[datetime]::UtcNow.ToString('o')} | ConvertTo-Json | Set-Content $connection -Encoding UTF8
Write-Output "New isolated Qdrant: $name (tmpfs, internal network, no host ports)"
