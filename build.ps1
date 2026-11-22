# Builds livelink-hub.exe (Windows GUI) and optionally Linux binaries.
$ErrorActionPreference = 'Stop'
$go = 'go'
$env:GOTOOLCHAIN = 'local'
$env:CGO_ENABLED = '0'
Push-Location $PSScriptRoot
try {
    $windres = Get-ChildItem "$env:LOCALAPPDATA\Microsoft\WinGet\Packages" -Recurse -Filter windres.exe -ErrorAction SilentlyContinue | Select-Object -First 1
    if ($windres) {
        & $windres.FullName -O coff -o rsrc_windows_amd64.syso app.rc
        if ($LASTEXITCODE -ne 0) { throw 'windres failed' }
    } elseif (-not (Test-Path rsrc_windows_amd64.syso)) {
        Write-Warning 'windres not found: building without embedded .syso icon'
    }
    & $go vet ./...
    if ($LASTEXITCODE -ne 0) { throw 'go vet failed' }
    & $go build -trimpath -ldflags '-H windowsgui -s -w' -o livelink-hub.exe .
    if ($LASTEXITCODE -ne 0) { throw 'go build failed' }
    Get-Item livelink-hub.exe | Select-Object Name, Length, LastWriteTime
} finally {
    Pop-Location
}
