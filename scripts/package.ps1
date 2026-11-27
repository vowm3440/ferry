param([string]$Version = '0.2.0', [switch]$SkipBuild, [switch]$SkipSmoke, [switch]$Force)
$ErrorActionPreference = 'Stop'
if ($Version -notmatch '^\d+\.\d+\.\d+(?:-[A-Za-z0-9.-]+)?$') { throw 'Invalid package version.' }
$projectRoot = Split-Path -Parent $PSScriptRoot
Push-Location -LiteralPath $projectRoot
try {
    if (-not $SkipBuild) {
        & (Join-Path $PSScriptRoot 'test.ps1') -Version $Version -SkipSmoke:$SkipSmoke
    }
    foreach ($required in @('bin\Ferry.exe','bin\ferry-core.exe','LICENSE','NOTICE','README.md')) {
        if (-not (Test-Path -LiteralPath $required -PathType Leaf)) { throw "Missing package input: $required" }
    }
    Add-Type -AssemblyName System.IO.Compression
    Add-Type -AssemblyName System.IO.Compression.FileSystem
    $outputDirectory = Join-Path $projectRoot 'dist'
    New-Item -ItemType Directory -Path $outputDirectory -Force | Out-Null
    $binaryPath = Join-Path $outputDirectory "Ferry-$Version-windows-amd64.zip"
    $sourcePath = Join-Path $outputDirectory "Ferry-$Version-source.zip"
    $installerPath = Join-Path $outputDirectory "Ferry-$Version-windows-amd64-setup.exe"
    $sumsPath = Join-Path $outputDirectory "Ferry-$Version-SHA256SUMS.txt"
    foreach ($target in @($binaryPath,$sourcePath,$installerPath,$sumsPath)) {
        if (Test-Path -LiteralPath $target) {
            if ($Force) { Remove-Item -LiteralPath $target -Force }
            else { throw "Archive already exists; choose a new version or pass -Force: $target" }
        }
    }
    function Write-Archive([string]$destination, $entries) {
        $stream = [System.IO.File]::Open($destination, [System.IO.FileMode]::CreateNew)
        $archive = [System.IO.Compression.ZipArchive]::new($stream, [System.IO.Compression.ZipArchiveMode]::Create)
        try {
            foreach ($entry in $entries) {
                [System.IO.Compression.ZipFileExtensions]::CreateEntryFromFile($archive, $entry.Path, $entry.Name, [System.IO.Compression.CompressionLevel]::Optimal) | Out-Null
            }
        } finally { $archive.Dispose(); $stream.Dispose() }
    }
    $binaryEntries = @(
        @{ Path = (Join-Path $projectRoot 'bin\Ferry.exe'); Name = 'Ferry.exe' },
        @{ Path = (Join-Path $projectRoot 'bin\ferry-core.exe'); Name = 'ferry-core.exe' },
        @{ Path = (Join-Path $projectRoot 'LICENSE'); Name = 'LICENSE' },
        @{ Path = (Join-Path $projectRoot 'NOTICE'); Name = 'NOTICE' },
        @{ Path = (Join-Path $projectRoot 'README.md'); Name = 'README.md' }
    )
    Write-Archive $binaryPath $binaryEntries
    $sourceEntries = [System.Collections.Generic.List[object]]::new()
    $sourceFiles = @('go.mod','go.sum','LICENSE','NOTICE','README.md','.gitignore')
    $sourceRoots = @('build','cmd','internal','scripts','testdata','docs\plan','docs\research','frontend\src','frontend\tests','frontend\public')
    if (Test-Path -LiteralPath '.github') { $sourceRoots += '.github' }
    $sourceFiles += Get-ChildItem -LiteralPath frontend -File -Force | Select-Object -ExpandProperty FullName
    foreach ($sourceRoot in $sourceRoots) {
        $sourceFiles += Get-ChildItem -LiteralPath $sourceRoot -File -Recurse | Where-Object {
            $_.FullName -notmatch '[\\/](?:dist|\.keys)[\\/]' -and $_.Extension -notin @('.db','.exe','.tmp','.dat','.out','.key','.pem','.old')
        } | Select-Object -ExpandProperty FullName
    }
    foreach ($file in ($sourceFiles | Select-Object -Unique)) {
        $absolutePath = (Resolve-Path -LiteralPath $file).Path
        $relativePath = $absolutePath.Substring($projectRoot.Length + 1).Replace('\','/')
        $sourceEntries.Add(@{Path=$absolutePath;Name=$relativePath})
    }
    Write-Archive $sourcePath $sourceEntries
    $hashTargets = [System.Collections.Generic.List[string]]::new()
    $hashTargets.Add($binaryPath)
    $hashTargets.Add($sourcePath)

    $makensisExe = (Get-Command makensis -ErrorAction SilentlyContinue).Source
    if (-not $makensisExe) {
        $regDirs = @('HKLM:\SOFTWARE\NSIS','HKLM:\SOFTWARE\WOW6432Node\NSIS','HKCU:\Software\NSIS') |
            ForEach-Object { if (Test-Path $_) { (Get-ItemProperty $_).'(default)' } } |
            Where-Object { $_ }
        $candidates = @(
            "$env:ProgramFiles\NSIS\makensis.exe",
            "${env:ProgramFiles(x86)}\NSIS\makensis.exe",
            "D:\Program Files (x86)\NSIS\makensis.exe",
            "D:\Program Files\NSIS\makensis.exe",
            "$env:LOCALAPPDATA\Programs\NSIS\makensis.exe",
            "C:\ProgramData\chocolatey\bin\makensis.exe"
        ) + ($regDirs | ForEach-Object { Join-Path $_ 'makensis.exe' })
        $makensisExe = $candidates | Where-Object { Test-Path -LiteralPath $_ -PathType Leaf } | Select-Object -First 1
    }
    $installerScript = Join-Path $projectRoot 'build\installer.nsi'
    if ($makensisExe -and (Test-Path -LiteralPath $installerScript -PathType Leaf)) {
        & $makensisExe "/DVERSION=$Version" "/DOUTFILE=$installerPath" $installerScript
        if ($LASTEXITCODE -ne 0) { throw 'NSIS installer build failed.' }
        $hashTargets.Add($installerPath)
    } else {
        Write-Host 'Note: makensis not found; skipped NSIS installer (portable zip and source zip generated).'
    }
    $hashLines = foreach ($file in $hashTargets) {
        $hash = Get-FileHash -LiteralPath $file -Algorithm SHA256
        '{0}  {1}' -f $hash.Hash.ToLowerInvariant(), [System.IO.Path]::GetFileName($file)
    }
    $hashLines | Set-Content -LiteralPath $sumsPath -Encoding ASCII
    Get-Item -LiteralPath $hashTargets | Select-Object Name,Length
} finally { Pop-Location }
