param(
    [Parameter(Mandatory = $true)]
    [ValidatePattern('^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z.-]+)?$')]
    [string]$Version
)

$ErrorActionPreference = 'Stop'
$root = if ($env:PUBLISH_CHOCOLATEY_ROOT) { $env:PUBLISH_CHOCOLATEY_ROOT } else { Split-Path $PSScriptRoot -Parent }
$dist = Join-Path $root 'dist'
$checksums = Join-Path $dist 'checksums.txt'
if (-not (Test-Path $checksums)) { throw "missing $checksums" }

function Get-ReleaseChecksum([string]$Architecture) {
    $name = "git-chunks_${Version}_windows_${Architecture}.zip"
    $line = Select-String -Path $checksums -Pattern "^[a-fA-F0-9]{64}  $([regex]::Escape($name))$"
    if (@($line).Count -ne 1) { throw "expected one checksum for $name" }
    $archive = Join-Path $dist $name
    if (-not (Test-Path $archive)) { throw "missing canonical archive $archive" }
    if ((Get-FileHash $archive -Algorithm SHA256).Hash.ToLowerInvariant() -ne $line.Line.Substring(0, 64).ToLowerInvariant()) {
        throw "canonical archive checksum mismatch: $name"
    }
    return $line.Line.Substring(0, 64).ToLowerInvariant()
}

$amd64 = Get-ReleaseChecksum 'amd64'
$arm64 = Get-ReleaseChecksum 'arm64'
$stage = Join-Path $env:TEMP "git-chunks-chocolatey-$Version"
Remove-Item $stage -Recurse -Force -ErrorAction SilentlyContinue
New-Item (Join-Path $stage 'tools') -ItemType Directory -Force | Out-Null

$nuspec = @"
<?xml version="1.0" encoding="utf-8"?>
<package xmlns="http://schemas.microsoft.com/packaging/2015/06/nuspec.xsd">
  <metadata>
    <id>git-chunks</id>
    <version>$Version</version>
    <title>git-chunks</title>
    <authors>Jishnu Teegala</authors>
    <owners>Jishnu Teegala</owners>
    <licenseUrl>https://github.com/jishnuteegala/git-chunks/blob/main/LICENSE</licenseUrl>
    <projectUrl>https://github.com/jishnuteegala/git-chunks</projectUrl>
    <packageSourceUrl>https://github.com/jishnuteegala/git-chunks</packageSourceUrl>
    <requireLicenseAcceptance>false</requireLicenseAcceptance>
    <description>git-chunks splits pending changes into multiple commits based on file count or working-tree size, optionally pushing each commit to reduce per-push size and server workload.</description>
    <summary>Commit and push changes in chunks to avoid SCM push size limits</summary>
    <releaseNotes>https://github.com/jishnuteegala/git-chunks/releases/tag/v$Version</releaseNotes>
    <tags>git cli push chunk</tags>
  </metadata>
</package>
"@
Set-Content -Path (Join-Path $stage 'git-chunks.nuspec') -Value $nuspec -Encoding UTF8

$install = @"
`$ErrorActionPreference = 'Stop'
`$tools = Split-Path -Parent `$MyInvocation.MyCommand.Definition
`$packageArgs = @{
  packageName    = 'git-chunks'
  unzipLocation = `$tools
  url64bit       = 'https://github.com/jishnuteegala/git-chunks/releases/download/v$Version/git-chunks_${Version}_windows_amd64.zip'
  checksum64     = '$amd64'
  checksumType64 = 'sha256'
}

if ([System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture -eq [System.Runtime.InteropServices.Architecture]::Arm64) {
  `$packageArgs.url64bit = 'https://github.com/jishnuteegala/git-chunks/releases/download/v$Version/git-chunks_${Version}_windows_arm64.zip'
  `$packageArgs.checksum64 = '$arm64'
}
Install-ChocolateyZipPackage @packageArgs
"@
Set-Content -Path (Join-Path $stage 'tools/chocolateyinstall.ps1') -Value $install -Encoding UTF8

$choco = if ($env:PUBLISH_CHOCOLATEY_CLI) { $env:PUBLISH_CHOCOLATEY_CLI } else { 'choco' }
& $choco pack (Join-Path $stage 'git-chunks.nuspec') --outputdirectory $dist --limit-output
if ($LASTEXITCODE -ne 0) { throw 'choco pack failed' }
$package = Join-Path $dist "git-chunks.$Version.nupkg"
if (-not (Test-Path $package)) { throw "missing generated package $package" }
Write-Output "generated $package"
