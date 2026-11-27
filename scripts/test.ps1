param([switch]$SkipFrontend)
$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path -Parent $PSScriptRoot
Push-Location -LiteralPath $projectRoot
try {
    & (Join-Path $PSScriptRoot 'build.ps1') -SkipFrontend:$SkipFrontend
    $env:CGO_ENABLED = '0'
    $env:FERRY_TEST_CORE = Join-Path $projectRoot 'bin\ferry-core.exe'
    & go run ./scripts/check-source.go
    if ($LASTEXITCODE -ne 0) { throw 'Go source size limits failed.' }
    & go vet -tags 'with_gvisor,with_quic,with_utls,with_clash_api,with_wireguard' ./...
    if ($LASTEXITCODE -ne 0) { throw 'Go vet failed.' }
    $coverFile = Join-Path ([System.IO.Path]::GetTempPath()) ("ferry-cover-" + [guid]::NewGuid().ToString('N') + ".out")
    try {
        # Quoted: Windows PowerShell 5.1 splits an unquoted -name=a.b argument at the dot.
        & go test -tags 'with_gvisor,with_quic,with_utls,with_clash_api,with_wireguard' -count=1 "-coverprofile=$coverFile" ./...
        if ($LASTEXITCODE -ne 0) { throw 'Go tests failed.' }
    } finally {
        Remove-Item -LiteralPath $coverFile -Force -ErrorAction SilentlyContinue
    }
    & go test ./...
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
