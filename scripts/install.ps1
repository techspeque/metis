# Metis installer for Windows — downloads the latest release, verifies its
# checksum, installs it for the current user, and puts it on PATH.
#
# Usage (PowerShell 5.1 or later, no administrator rights needed):
#   irm https://raw.githubusercontent.com/techspeque/metis/main/scripts/install.ps1 | iex
#
# Or, from a checkout:
#   powershell -ExecutionPolicy Bypass -File scripts\install.ps1
#
# Overrides, as environment variables (the same names install.sh uses):
#   METIS_VERSION         install a specific version (e.g. v0.0.16); default: latest release
#   METIS_INSTALL_DIR     install directory; default: %LOCALAPPDATA%\Programs\metis
#   METIS_NO_MODIFY_PATH  set to 1 to leave the user PATH alone
#
# Needs nothing but PowerShell and network access to github.com: no package
# manager, no Go toolchain, no elevation. Everything lands under the user's
# profile, and PATH is changed for the user only.

$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'

$Repo = 'techspeque/metis'
$InstallDir = if ($env:METIS_INSTALL_DIR) { $env:METIS_INSTALL_DIR } else { Join-Path $env:LOCALAPPDATA 'Programs\metis' }

function Info([string]$Message) { Write-Host "==> $Message" -ForegroundColor Cyan }
function Fail([string]$Message) { Write-Host "error: $Message" -ForegroundColor Red; exit 1 }

# TLS 1.2 is the floor github.com accepts; Windows PowerShell 5.1 does not
# offer it by default.
try {
    [Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12
} catch {
    # PowerShell 7 negotiates TLS itself.
}

# Requests go through the system proxy, as every other tool on the machine does.
$WebDefaults = @{ UseBasicParsing = $true }
try {
    $proxy = [System.Net.WebRequest]::GetSystemWebProxy()
    $proxy.Credentials = [System.Net.CredentialCache]::DefaultCredentials
    [System.Net.WebRequest]::DefaultWebProxy = $proxy
} catch {
    # No system proxy to honour.
}

function Get-Arch {
    try {
        switch ([System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString()) {
            'X64'   { return 'amd64' }
            'Arm64' { return 'arm64' }
        }
    } catch {
        # Older .NET: fall through to the environment.
    }
    $arch = if ($env:PROCESSOR_ARCHITEW6432) { $env:PROCESSOR_ARCHITEW6432 } else { $env:PROCESSOR_ARCHITECTURE }
    switch ($arch) {
        'AMD64' { return 'amd64' }
        'ARM64' { return 'arm64' }
    }
    Fail "unsupported architecture: $arch (releases are built for amd64 and arm64)"
}

function Get-LatestTag {
    try {
        $release = Invoke-RestMethod @WebDefaults -Uri "https://api.github.com/repos/$Repo/releases/latest" -Headers @{ 'User-Agent' = 'metis-install' }
        if ($release.tag_name) { return $release.tag_name }
    } catch {
        # The API may be rate-limited; the release page redirect carries the same answer.
    }
    try {
        $response = Invoke-WebRequest @WebDefaults -Uri "https://github.com/$Repo/releases/latest" -MaximumRedirection 0 -ErrorAction SilentlyContinue
        $location = $response.Headers['Location']
        if ($location) { return ($location -split '/')[-1] }
    } catch {
        if ($_.Exception.Response -and $_.Exception.Response.Headers['Location']) {
            return ($_.Exception.Response.Headers['Location'] -split '/')[-1]
        }
    }
    Fail 'could not determine the latest release (set METIS_VERSION to pin one)'
}

function Get-FileSha256([string]$Path) {
    return (Get-FileHash -Path $Path -Algorithm SHA256).Hash.ToLowerInvariant()
}

function Add-ToUserPath([string]$Dir) {
    if ($env:METIS_NO_MODIFY_PATH -eq '1') { return }

    $current = [Environment]::GetEnvironmentVariable('Path', 'User')
    $parts = @()
    if ($current) { $parts = $current -split ';' | Where-Object { $_ -ne '' } }
    if ($parts -contains $Dir) {
        Info "$Dir is already on the user PATH"
    } else {
        try {
            [Environment]::SetEnvironmentVariable('Path', (($parts + $Dir) -join ';'), 'User')
            Info "Added $Dir to the user PATH (open a new terminal to pick it up)"
        } catch {
            Info "Could not change the user PATH ($($_.Exception.Message)); add it yourself:"
            Info "  `$env:Path = `"$Dir;`$env:Path`""
        }
    }
    # This session sees it right away either way.
    if (($env:Path -split ';') -notcontains $Dir) {
        $env:Path = "$Dir;$env:Path"
    }
}

$arch = Get-Arch
$tag = if ($env:METIS_VERSION) { $env:METIS_VERSION } else { Get-LatestTag }
$version = $tag -replace '^v', ''
$archive = "metis_${version}_windows_${arch}.zip"
$baseUrl = "https://github.com/$Repo/releases/download/$tag"

Info "Installing metis $tag (windows/$arch) to $InstallDir"

$tmp = Join-Path ([IO.Path]::GetTempPath()) ("metis-install-" + [Guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $tmp | Out-Null

try {
    $zipPath = Join-Path $tmp $archive
    try {
        Invoke-WebRequest @WebDefaults -Uri "$baseUrl/$archive" -OutFile $zipPath
    } catch {
        Fail "download failed: $baseUrl/$archive ($($_.Exception.Message))"
    }

    $sumsPath = Join-Path $tmp 'checksums.txt'
    try {
        Invoke-WebRequest @WebDefaults -Uri "$baseUrl/checksums.txt" -OutFile $sumsPath
    } catch {
        Fail "download failed: $baseUrl/checksums.txt ($($_.Exception.Message))"
    }
    $expectedLine = Get-Content $sumsPath | Where-Object { $_ -match "\s$([regex]::Escape($archive))$" } | Select-Object -First 1
    if (-not $expectedLine) { Fail "no checksum for $archive in checksums.txt" }
    $expected = ($expectedLine -split '\s+')[0].ToLowerInvariant()
    $actual = Get-FileSha256 $zipPath
    if ($expected -ne $actual) { Fail "checksum mismatch for $archive (expected $expected, got $actual)" }
    Info 'Checksum verified'

    $extract = Join-Path $tmp 'extract'
    Expand-Archive -Path $zipPath -DestinationPath $extract -Force
    $exe = Get-ChildItem -Path $extract -Filter 'metis.exe' -Recurse | Select-Object -First 1
    if (-not $exe) { Fail "metis.exe not found in $archive" }

    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
    $target = Join-Path $InstallDir 'metis.exe'
    try {
        Copy-Item -Path $exe.FullName -Destination $target -Force
    } catch {
        Fail "could not write $target ($($_.Exception.Message)); close any running metis and retry, or set METIS_INSTALL_DIR"
    }

    $installed = & $target --version 2>&1
    Info "Installed: $installed"
} finally {
    Remove-Item -Path $tmp -Recurse -Force -ErrorAction SilentlyContinue
}

Add-ToUserPath $InstallDir
