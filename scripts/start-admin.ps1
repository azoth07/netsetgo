$ErrorActionPreference = 'Stop'
$executable = Join-Path $PSScriptRoot 'NetSetGo.exe'
if (-not (Test-Path -LiteralPath $executable)) {
    $executable = Join-Path (Split-Path -Parent $PSScriptRoot) 'dist/NetSetGo.exe'
}
if (-not (Test-Path -LiteralPath $executable)) {
    throw 'Build NetSetGo.exe first with scripts/build.ps1.'
}
Start-Process -FilePath $executable -WorkingDirectory (Split-Path -Parent $executable) -Verb RunAs -WindowStyle Hidden
