param([switch]$Check)
$ErrorActionPreference='Stop'
$repoRoot=(Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$previousLocation=Get-Location
$previousCache=$env:GOMODCACHE; $previousBuildCache=$env:GOCACHE; $previousProxy=$env:GOPROXY
try {
 Set-Location -LiteralPath $repoRoot
 $env:GOMODCACHE=Join-Path $repoRoot 'runtime/openapi-tools/modcache'
 $env:GOCACHE=Join-Path $repoRoot 'runtime/align-week5-phase1/go-cache'
 if (!$env:GOPROXY) { $env:GOPROXY='https://goproxy.cn,direct' }
 # Exclude downloaded tool fixtures and the independent conversion module.
 go run github.com/swaggo/swag/cmd/swag@v1.16.6 init -g cmd/server/main.go -d . --parseInternal --exclude runtime,.venv,scripts/openapi-convert --outputTypes json -o runtime/week5-openapi-v2
 if ($LASTEXITCODE -ne 0) { throw 'swag annotation generation failed' }
 $arguments=@('-C','scripts/openapi-convert','run','.','-in','../../runtime/week5-openapi-v2/swagger.json','-out','../../internal/http/web/swagger/openapi.json')
 if ($Check) { $arguments+='-check' }
 & go @arguments
 if ($LASTEXITCODE -ne 0) { throw 'OpenAPI conversion or artifact drift check failed' }
} finally {
 $env:GOMODCACHE=$previousCache; $env:GOCACHE=$previousBuildCache; $env:GOPROXY=$previousProxy
 Set-Location -LiteralPath $previousLocation
}
