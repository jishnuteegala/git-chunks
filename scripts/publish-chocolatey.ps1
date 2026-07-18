param(
    [Parameter(Mandatory = $true)]
    [ValidatePattern('^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z.-]+)?$')]
    [string]$Version
)

$ErrorActionPreference = 'Stop'
if (-not $env:CHOCOLATEY_API_KEY) { throw 'CHOCOLATEY_API_KEY is required' }
$tempRoot = if ($env:RUNNER_TEMP) { $env:RUNNER_TEMP } else { $env:TEMP }

$packages = @(Get-ChildItem -Path dist -Filter "git-chunks.$Version.nupkg" -File)
if ($packages.Count -eq 0) {
    & "$PSScriptRoot/package-chocolatey.ps1" -Version $Version
    if ($LASTEXITCODE -ne 0) { throw 'Chocolatey package generation failed' }
    $packages = @(Get-ChildItem -Path dist -Filter "git-chunks.$Version.nupkg" -File)
}
if ($packages.Count -ne 1) {
    throw "expected one Chocolatey package for $Version, found $($packages.Count)"
}
$package = $packages[0].FullName
$feed = 'https://community.chocolatey.org/api/v2'
$query = "$feed/Packages()?`$filter=Id%20eq%20%27git-chunks%27%20and%20Version%20eq%20%27$Version%27"

function Test-PublicPackage {
    $response = Invoke-WebRequest -Uri $query -UseBasicParsing
    return $response.Content -match '<entry>'
}

if (Test-PublicPackage) {
    $remote = Join-Path $tempRoot "git-chunks.$Version.remote.nupkg"
    $localFiles = Join-Path $tempRoot "git-chunks.$Version.local"
    $remoteFiles = Join-Path $tempRoot "git-chunks.$Version.remote"
    Invoke-WebRequest -Uri "$feed/package/git-chunks/$Version" -OutFile $remote -UseBasicParsing
    Remove-Item $localFiles, $remoteFiles -Recurse -Force -ErrorAction SilentlyContinue
    [System.IO.Compression.ZipFile]::ExtractToDirectory($package, $localFiles)
    [System.IO.Compression.ZipFile]::ExtractToDirectory($remote, $remoteFiles)
    $localInstall = Get-FileHash (Join-Path $localFiles 'tools/chocolateyinstall.ps1') -Algorithm SHA256
    $remoteInstall = Get-FileHash (Join-Path $remoteFiles 'tools/chocolateyinstall.ps1') -Algorithm SHA256
    $localSpec = Get-FileHash (Join-Path $localFiles 'git-chunks.nuspec') -Algorithm SHA256
    $remoteSpec = Get-FileHash (Join-Path $remoteFiles 'git-chunks.nuspec') -Algorithm SHA256
    if ($localInstall.Hash -ne $remoteInstall.Hash -or $localSpec.Hash -ne $remoteSpec.Hash) {
        throw "public Chocolatey git-chunks $Version conflicts with the canonical package"
    }
    Write-Output "verified public Chocolatey git-chunks $Version"
    exit 0
}

$output = & choco push $package --source https://push.chocolatey.org/ --api-key $env:CHOCOLATEY_API_KEY 2>&1
$status = $LASTEXITCODE
$text = $output -join "`n"
if ($status -ne 0) {
    throw "Chocolatey push failed: $text"
}
Write-Output "Chocolatey accepted git-chunks $Version; public availability awaits moderation"
