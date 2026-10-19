param([Parameter(Mandatory=$true)][string]$Subnet)
$ErrorActionPreference = 'Stop'
if ($Subnet -notmatch '^10\.254\.\d{1,3}\.0/24$') { throw 'A manually preflighted private /24 subnet is required' }
$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '../..')).Path
$runID = [guid]::NewGuid().ToString('N').Substring(0,8)
$name = "campusclaw-align-storage-$runID"
$evidence = Join-Path $repoRoot "runtime/$name"
New-Item -ItemType Directory -Path $evidence | Out-Null
$password = [guid]::NewGuid().ToString('N') + [guid]::NewGuid().ToString('N')
$envFile = Join-Path $evidence 'mysql.env'
[IO.File]::WriteAllText($envFile, "MYSQL_ROOT_PASSWORD=$password`nMYSQL_ROOT_HOST=%`n")
docker network create --internal --subnet $Subnet --label campusclaw.phase=align-storage "$name-net" | Out-Null
if ($LASTEXITCODE -ne 0) { throw 'New isolated network creation failed' }
docker run -d --pull=never --name $name --label campusclaw.phase=align-storage --network "$name-net" --tmpfs /var/lib/mysql:rw --env-file $envFile mysql:8.4.4 --ngram-token-size=2 --max-allowed-packet=67108864 | Out-Null
if ($LASTEXITCODE -ne 0) { throw 'New isolated MySQL creation failed' }
$info = docker inspect $name | ConvertFrom-Json
if ($info[0].Mounts | Where-Object { $_.Type -ne 'tmpfs' }) { throw 'Unexpected persistent or foreign mount' }
@{ container=$name; network="$name-net"; password=$password; evidence=$evidence; created_utc=[datetime]::UtcNow.ToString('o') } | ConvertTo-Json | Set-Content (Join-Path $repoRoot 'runtime/align-storage-connection.json') -Encoding UTF8
@{ container=$name; network="$name-net"; image='mysql:8.4.4'; mounts=$info[0].Mounts; classroom_resources_touched=$false; qdrant_accessed=$false } | ConvertTo-Json -Depth 5 | Set-Content (Join-Path $evidence 'isolation.json') -Encoding UTF8
Write-Output "Created isolated MySQL: $name; private internal network, no published ports or existing volumes."
