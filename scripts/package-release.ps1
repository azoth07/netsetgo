param(
    [string]$Tag
)

$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path -Parent $PSScriptRoot
Push-Location $projectRoot
try {
    $config = Get-Content -LiteralPath 'mygo.json' -Raw -Encoding UTF8 | ConvertFrom-Json
    $version = $config.version
    if ($version -notmatch '^\d+\.\d+\.\d+$') {
        throw 'mygo.json version must use major.minor.patch format.'
    }
    if ($Tag -and $Tag -cne "v$version") {
        throw "Tag $Tag does not match mygo.json version v$version."
    }

    & "$PSScriptRoot/build.ps1"

    $releaseDirectory = Join-Path $projectRoot "dist/release/v$version"
    New-Item -ItemType Directory -Path $releaseDirectory -Force | Out-Null
    $installerName = "netsetgo-setup-$version-amd64.exe"
    $archiveName = "netsetgo-$version-windows-amd64.zip"
    Copy-Item -LiteralPath "dist/windows-amd64/$installerName" -Destination $releaseDirectory -Force
    $portableFiles = @(
        'dist/windows-amd64/NetSetGo.exe'
        'dist/windows-amd64/icon.png'
        'dist/config.example.ini'
        'dist/start-admin.ps1'
        'dist/README.md'
    )
    Compress-Archive -LiteralPath $portableFiles -DestinationPath "$releaseDirectory/$archiveName" -Force
    $checksums = foreach ($name in @($installerName, $archiveName)) {
        $hash = Get-FileHash -LiteralPath "$releaseDirectory/$name" -Algorithm SHA256
        '{0}  {1}' -f $hash.Hash.ToLowerInvariant(), $name
    }
    $utf8 = New-Object System.Text.UTF8Encoding($false)
    [System.IO.File]::WriteAllText("$releaseDirectory/SHA256SUMS.txt", ($checksums -join "`n") + "`n", $utf8)
    $notes = Get-Content -LiteralPath "$PSScriptRoot/release-notes.md" -Raw -Encoding UTF8
    $notes = $notes.Replace('{{version}}', $version).Replace('{{installer}}', $installerName).Replace('{{archive}}', $archiveName)
    [System.IO.File]::WriteAllText("$releaseDirectory/RELEASE_NOTES.md", $notes + "`n", $utf8)
    Write-Output $releaseDirectory
} finally {
    Pop-Location
}
