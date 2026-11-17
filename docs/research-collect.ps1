param(
    [string]$Target = 'D:\Program Files (x86)\NetSetMan\netsetman.exe',
    [string]$ReaCli = 'C:\Users\Administrator\AppData\Local\npm-cache\_npx\af752b90495482d7\node_modules\rea-agents\scripts\rea.mjs'
)

$ErrorActionPreference = 'Stop'
$utf8 = New-Object System.Text.UTF8Encoding($false)
$evidenceRoot = Join-Path $PSScriptRoot 'research-evidence'
[System.IO.Directory]::CreateDirectory($evidenceRoot) | Out-Null
$resourceOutput = & node $ReaCli inspect-pe-resources $Target --format json
if ($LASTEXITCODE -ne 0) { throw 'REA PE resource inspection failed.' }
[System.IO.File]::WriteAllText((Join-Path $evidenceRoot 'pe-resources.json'), ($resourceOutput -join "`n"), $utf8)

$targetItem = Get-Item -LiteralPath $Target
$helpPath = Join-Path $targetItem.DirectoryName 'NSM-Help\nsm5help_en.htm'
$manifest = [ordered]@{
    capturedAt = (Get-Date).ToString('o')
    source = 'PowerShell file metadata and SHA-256, not REA Evidence'
    target = $Target
    size = $targetItem.Length
    sha256 = (Get-FileHash -LiteralPath $Target -Algorithm SHA256).Hash
    fileVersion = $targetItem.VersionInfo.FileVersion
    productVersion = $targetItem.VersionInfo.ProductVersion
    helpPath = $helpPath
    helpSha256 = (Get-FileHash -LiteralPath $helpPath -Algorithm SHA256).Hash
    reaCli = $ReaCli
    reaVersion = '6.9.0'
    nodeVersion = (& node --version)
}
[System.IO.File]::WriteAllText((Join-Path $evidenceRoot 'source-manifest.json'), ($manifest | ConvertTo-Json -Depth 5), $utf8)

$helpLines = [System.IO.File]::ReadAllLines($helpPath, [System.Text.Encoding]::UTF8)
$sections = @(
    @(184, 235),
    @(243, 258),
    @(267, 290),
    @(299, 320),
    @(410, 440),
    @(880, 905)
)
$excerpts = foreach ($section in $sections) {
    for ($lineNumber = $section[0]; $lineNumber -le $section[1]; $lineNumber++) {
        $plainText = [System.Net.WebUtility]::HtmlDecode(($helpLines[$lineNumber - 1] -replace '<[^>]+>', '')).Trim()
        if ($plainText) { '{0}: {1}' -f $lineNumber, $plainText }
    }
}
[System.IO.File]::WriteAllLines((Join-Path $evidenceRoot 'help-excerpts.txt'), $excerpts, $utf8)

$targetBytes = [System.IO.File]::ReadAllBytes($Target)
$candidates = foreach ($encodingName in @('ASCII', 'Unicode')) {
    $encoding = [System.Text.Encoding]::GetEncoding($encodingName)
    $decoded = $encoding.GetString($targetBytes)
    foreach ($match in [regex]::Matches($decoded, '(?i)GetAdaptersAddresses|GetAdaptersInfo|EnableStatic|EnableDHCP|SetDNSServerSearchOrder|Win32_NetworkAdapterConfiguration|Win32_NetworkAdapter|iphlpapi\.dll|netsh\.exe|TACTIVATIONFORM|DHCPEnabled')) {
            [ordered]@{
                encoding = $encodingName
                fileOffset = $match.Index * $(if ($encodingName -eq 'Unicode') { 2 } else { 1 })
                text = $match.Value
            }
    }
}
[System.IO.File]::WriteAllText((Join-Path $evidenceRoot 'binary-string-candidates.json'), (@($candidates) | ConvertTo-Json -Depth 4), $utf8)
$manifest | ConvertTo-Json -Depth 5
