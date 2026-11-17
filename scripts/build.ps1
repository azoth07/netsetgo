$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path -Parent $PSScriptRoot
Push-Location $projectRoot
try {
    & go test ./...
    if ($LASTEXITCODE -ne 0) { throw 'Tests failed.' }
    New-Item -ItemType Directory -Path 'dist' -Force | Out-Null
    & go run github.com/egoist/mygo/cmd/mygo@v0.8.4-0.20261108082913-9233663736ca build -platform windows/amd64 -o dist .
    if ($LASTEXITCODE -ne 0) { throw 'MyGo build failed.' }
    $builtExecutable = Join-Path $projectRoot 'dist/windows-amd64/NetSetGo.exe'
    if (-not (Test-Path -LiteralPath $builtExecutable)) {
        throw "MyGo did not produce $builtExecutable."
    }
    Copy-Item -LiteralPath $builtExecutable -Destination 'dist/NetSetGo.exe' -Force
    Copy-Item -LiteralPath 'config.example.ini' -Destination 'dist/config.example.ini'
    Copy-Item -LiteralPath 'scripts/start-admin.ps1' -Destination 'dist/start-admin.ps1'
    Copy-Item -LiteralPath 'README.md' -Destination 'dist/README.md'
    Write-Output (Join-Path $projectRoot 'dist/NetSetGo.exe')
} finally {
    Pop-Location
}
