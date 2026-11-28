param([string]$Version = '0.3.0', [string]$BinDir = 'bin', [switch]$SkipFrontend, [switch]$DevTools)
$ErrorActionPreference = 'Stop'
if ($Version -notmatch '^\d+\.\d+\.\d+(?:-[A-Za-z0-9.-]+)?$') { throw "Invalid build version: $Version" }
$projectRoot = Split-Path -Parent $PSScriptRoot
$resolvedBin = if ([System.IO.Path]::IsPathRooted($BinDir)) { $BinDir } else { Join-Path $projectRoot $BinDir }
Push-Location -LiteralPath $projectRoot
try {
    $env:CGO_ENABLED = '0'
    if (-not $SkipFrontend) {
        Push-Location -LiteralPath (Join-Path $projectRoot 'frontend')
        try {
            & pnpm install --frozen-lockfile
            if ($LASTEXITCODE -ne 0) { throw 'Frontend dependency installation failed.' }
            & pnpm check
            if ($LASTEXITCODE -ne 0) { throw 'Frontend type check failed.' }
            & pnpm build
            if ($LASTEXITCODE -ne 0) { throw 'Frontend build failed.' }
        } finally { Pop-Location }
    }
    $assetSource = Join-Path $projectRoot 'frontend\dist'
    $assetTarget = Join-Path $projectRoot 'internal\ui\dist'
    if (-not (Test-Path -LiteralPath (Join-Path $assetSource 'index.html'))) { throw 'Build frontend first.' }
    if (Test-Path -LiteralPath $assetTarget) {
        $resolvedTarget = (Resolve-Path -LiteralPath $assetTarget).Path
        if ($resolvedTarget -ne (Join-Path $projectRoot 'internal\ui\dist')) { throw 'Unexpected asset destination.' }
        Remove-Item -LiteralPath $resolvedTarget -Recurse -Force
    }
    New-Item -ItemType Directory -Path $assetTarget -Force | Out-Null
    Get-ChildItem -LiteralPath $assetSource | Copy-Item -Destination $assetTarget -Recurse
    # Desktop WebView always uses the Wails bridge; strip unreachable browser-demo chunks.
    Get-ChildItem -Path (Join-Path $assetTarget 'assets') -Filter 'demo-*.js' -File -ErrorAction SilentlyContinue | Remove-Item -Force
    New-Item -ItemType Directory -Path $resolvedBin -Force | Out-Null
    Remove-Item -Path (Join-Path $resolvedBin '*.old') -Force -ErrorAction SilentlyContinue
    function Step-AsideIfLocked([string]$exePath) {
        if (-not (Test-Path -LiteralPath $exePath -PathType Leaf)) { return }
        try {
            $stream = [System.IO.File]::Open($exePath, [System.IO.FileMode]::Open, [System.IO.FileAccess]::Write, [System.IO.FileShare]::None)
            $stream.Dispose()
        } catch [System.IO.IOException] {
            $backup = "$exePath.$([guid]::NewGuid().ToString('N').Substring(0,8)).old"
            Move-Item -LiteralPath $exePath -Destination $backup -Force
        }
    }
    $coreLdflags = "-s -w -X ferry/internal/release.Version=$Version"
    $guiLdflags = "-s -w -H windowsgui -X ferry/internal/release.Version=$Version"
    $coreOut = Join-Path $resolvedBin 'ferry-core.exe'
    $guiOut = Join-Path $resolvedBin 'Ferry.exe'
    Step-AsideIfLocked $coreOut
    & go build -trimpath -tags 'with_gvisor,with_quic,with_utls,with_clash_api,with_wireguard' -ldflags $coreLdflags -o $coreOut ./cmd/ferry-core
    if ($LASTEXITCODE -ne 0) { throw 'Core build failed.' }
    Step-AsideIfLocked $guiOut
    & go build -trimpath -ldflags $guiLdflags -o $guiOut ./cmd/ferry
    if ($LASTEXITCODE -ne 0) { throw 'GUI build failed.' }
    if ($DevTools) {
        # Local end-to-end testing only: honours FERRY_DEVTOOLS_PORT (unauthenticated WebView2
        # debugging port). Separate file name so that package.ps1 can never ship it.
        $devOut = Join-Path $resolvedBin 'Ferry-devtools.exe'
        Step-AsideIfLocked $devOut
        & go build -trimpath -tags 'devtools' -ldflags $guiLdflags -o $devOut ./cmd/ferry
        if ($LASTEXITCODE -ne 0) { throw 'GUI devtools build failed.' }
    }
    # GPL-3.0: binaries must always travel with the licence text and third-party notices.
    Copy-Item -LiteralPath (Join-Path $projectRoot 'LICENSE'),(Join-Path $projectRoot 'NOTICE') -Destination $resolvedBin -Force
    Get-Item -LiteralPath $guiOut,$coreOut | Select-Object Name,Length
} finally { Pop-Location }
