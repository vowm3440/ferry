param([string]$Version = '0.3.0', [string]$BinDir = 'bin', [switch]$SkipFrontend, [switch]$SkipSmoke)
$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path -Parent $PSScriptRoot
$resolvedBin = if ([System.IO.Path]::IsPathRooted($BinDir)) { $BinDir } else { Join-Path $projectRoot $BinDir }
Push-Location -LiteralPath $projectRoot
try {
    & (Join-Path $PSScriptRoot 'build.ps1') -Version $Version -BinDir $resolvedBin -SkipFrontend:$SkipFrontend
    if (-not $SkipFrontend) {
        Push-Location -LiteralPath (Join-Path $projectRoot 'frontend')
        try {
            & pnpm test
            if ($LASTEXITCODE -ne 0) { throw 'Frontend unit tests failed.' }
            if (-not $SkipSmoke) {
                $env:FERRY_SMOKE_DIST = Join-Path $projectRoot 'frontend\dist'
                try {
                    & pnpm test:smoke
                    if ($LASTEXITCODE -ne 0) { throw 'Frontend smoke tests failed.' }
                } finally {
                    Remove-Item Env:\FERRY_SMOKE_DIST -ErrorAction SilentlyContinue
                }
            }
        } finally { Pop-Location }
    }
    $env:CGO_ENABLED = '0'
    $env:FERRY_TEST_CORE = Join-Path $resolvedBin 'ferry-core.exe'
    & go run ./scripts/check-source.go
    if ($LASTEXITCODE -ne 0) { throw 'Go source size limits failed.' }
    & go vet -tags 'with_gvisor,with_quic,with_utls,with_clash_api,with_wireguard' ./...
    if ($LASTEXITCODE -ne 0) { throw 'Go vet failed.' }
    $coverFile = Join-Path ([System.IO.Path]::GetTempPath()) ("ferry-cover-" + [guid]::NewGuid().ToString('N') + ".out")
    try {
        # Quoted: Windows PowerShell 5.1 splits an unquoted -name=a.b argument at the dot.
        & go test -tags 'with_gvisor,with_quic,with_utls,with_clash_api,with_wireguard' -count=1 "-coverprofile=$coverFile" ./...
        if ($LASTEXITCODE -ne 0) { throw 'Go tests failed.' }
        & go tool cover "-func=$coverFile" | Select-Object -Last 1
    } finally {
        Remove-Item -LiteralPath $coverFile -Force -ErrorAction SilentlyContinue
    }
    & go test -count=1 ./...
    if ($LASTEXITCODE -ne 0) { throw 'Default-tag tests failed.' }
    $guiDependencies = & go list -deps ./cmd/ferry
    if ($LASTEXITCODE -ne 0) { throw 'GUI dependency inspection failed.' }
    $forbidden = $guiDependencies | Where-Object { $_ -match '^github.com/sagernet/sing-box' -or $_ -match '^ferry/internal/core/' }
    if ($forbidden) { throw ('GUI imports the core: ' + ($forbidden -join ', ')) }
    Write-Host 'Tests passed; GUI does not link the sing-box core.'
} finally {
    Remove-Item Env:\FERRY_TEST_CORE -ErrorAction SilentlyContinue
    Pop-Location
}
