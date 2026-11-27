param([string]$ExpectedVersion = '')
$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path -Parent $PSScriptRoot
Push-Location -LiteralPath $projectRoot
try {
    $guiPath = Join-Path $projectRoot 'bin\Ferry.exe'
    $corePath = Join-Path $projectRoot 'bin\ferry-core.exe'

    Write-Host '== [1/4] Binary & Version Check =='
    foreach ($p in @($guiPath, $corePath)) {
        if (-not (Test-Path -LiteralPath $p -PathType Leaf)) { throw "Missing binary: $p" }
    }
    $guiSizeMB = [math]::Round((Get-Item -LiteralPath $guiPath).Length / 1MB, 2)
    $coreSizeMB = [math]::Round((Get-Item -LiteralPath $corePath).Length / 1MB, 2)
    $coreVerOut = (& $corePath --version).Trim()
    if ($LASTEXITCODE -ne 0 -or -not $coreVerOut) { throw 'ferry-core.exe --version failed.' }
    if ($ExpectedVersion -and ($coreVerOut -notmatch [regex]::Escape($ExpectedVersion))) {
        throw "Version mismatch: expected $ExpectedVersion, got '$coreVerOut'"
    }
    Write-Host ("[PASS] Ferry.exe ({0} MB), ferry-core.exe ({1} MB) -> {2}" -f $guiSizeMB, $coreSizeMB, $coreVerOut)

    Write-Host '== [2/4] System Proxy & Journal Check =='
    $inetKey = 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Internet Settings'
    $inet = Get-ItemProperty -LiteralPath $inetKey -ErrorAction SilentlyContinue
    $proxyEnabled = if ($inet -and $null -ne $inet.ProxyEnable) { [int]$inet.ProxyEnable } else { 0 }
    $proxyServer = if ($inet -and $inet.ProxyServer) { [string]$inet.ProxyServer } else { '' }
    $journalPath = Join-Path $env:LOCALAPPDATA 'Ferry\proxy-restore.dat'
    $journalExists = Test-Path -LiteralPath $journalPath -PathType Leaf
    $runningGui = Get-Process -Name 'Ferry' -ErrorAction SilentlyContinue
    if ($proxyEnabled -eq 1 -and -not $runningGui) {
        Write-Host ("[WARN] System proxy is enabled ({0}) while Ferry.exe is not running (journalExists={1})." -f $proxyServer, $journalExists)
    } else {
        Write-Host ("[PASS] WinINet ProxyEnable={0}, ProxyServer='{1}', journalExists={2}" -f $proxyEnabled, $proxyServer, $journalExists)
    }

    Write-Host '== [3/4] FerryCore Windows Service Health Check =='
    $svc = Get-CimInstance Win32_Service -Filter "Name='FerryCore'" -ErrorAction SilentlyContinue
    if (-not $svc) {
        Write-Host '[INFO] FerryCore Windows service is not installed (user-mode core active).'
    } else {
        $expectedSvcCore = Join-Path ([Environment]::GetFolderPath('CommonApplicationData')) 'Ferry\service\ferry-core.exe'
        $failureInfo = (& sc.exe qfailure FerryCore 2>$null | Out-String)
        $hasRecovery = $failureInfo -match 'RESTART'
        $authFile = Join-Path ([Environment]::GetFolderPath('CommonApplicationData')) 'Ferry\service-client\auth.json'
        $hashMatch = $false
        if (Test-Path -LiteralPath $authFile -PathType Leaf) {
            $authJson = Get-Content -LiteralPath $authFile -Raw | ConvertFrom-Json
            $guiHash = (Get-FileHash -LiteralPath $guiPath -Algorithm SHA256).Hash.ToLowerInvariant()
            $hashMatch = ($authJson.guiExeSha256 -eq $guiHash)
        }
        Write-Host ("[INFO] FerryCore State={0}, StartMode={1}, Path={2}, Recovery={3}, GUIHashMatch={4}" -f $svc.State, $svc.StartMode, $svc.PathName, $hasRecovery, $hashMatch)
        if (-not $hashMatch -or -not $hasRecovery -or ($svc.PathName -notmatch [regex]::Escape($expectedSvcCore))) {
            Write-Host '[WARN] Installed FerryCore service is outdated or mismatched with bin\Ferry.exe; launch Ferry and click "修复服务" to upgrade.'
        } else {
            Write-Host '[PASS] Installed FerryCore service matches current binaries and recovery policy.'
        }
    }

    Write-Host '== [4/4] Live Core Subprocess IPC, Crash Recovery & Port Release Check =='
    $env:CGO_ENABLED = '0'
    $env:FERRY_TEST_CORE = $corePath
    & go test -v -count=1 ./internal/ipc -run TestRealCoreProcessStressRestartAndCleanShutdown
    if ($LASTEXITCODE -ne 0) { throw 'Live core subprocess stability test failed.' }
    Write-Host '[PASS] Real machine core lifecycle, crash recovery, monotonic logs, and port cleanup verified.'
} finally {
    Remove-Item Env:\FERRY_TEST_CORE -ErrorAction SilentlyContinue
    Pop-Location
}
