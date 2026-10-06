#requires -Version 5.1
<#
.SYNOPSIS
Download missing portable build tools and build the complete Synology SPK.
.DESCRIPTION
Requires 64-bit Windows and PowerShell 5.1+. Reuses compatible Python and Go
or downloads pinned versions to work/toolchains/. Produces the versioned SPK
and SHA256 file in dist/. No system installation or administrator rights needed.
.EXAMPLE
.\BUILD.ps1
.EXAMPLE
.\BUILD.ps1 -Python 'C:\Python313\python.exe' -Go 'C:\Go\bin\go.exe'
#>
[CmdletBinding()]
param(
    [string]$Python = $env:SYNOWAKE_PYTHON,
    [string]$Go = $env:SYNOWAKE_GO
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'
$originalEnvironment = @{}
foreach ($name in @('PSModulePath', 'GOROOT', 'GOTOOLCHAIN', 'GOCACHE', 'GOPATH')) {
    $originalEnvironment[$name] = [Environment]::GetEnvironmentVariable($name, 'Process')
}
# Windows PowerShell may be started from PowerShell 7 with its module path.
$env:PSModulePath = (Join-Path $PSHOME 'Modules') + [IO.Path]::PathSeparator + $env:PSModulePath

function Get-PythonArguments {
    param([string]$Executable)
    if ([System.IO.Path]::GetFileNameWithoutExtension($Executable) -eq 'py') { '-3' }
}

function Test-BuildTool {
    param([string]$Executable, [string]$Kind)
    try {
        if ($Kind -eq 'python') {
            $arguments = @(Get-PythonArguments $Executable)
            & $Executable @arguments -c 'import sys; sys.exit(0 if sys.version_info >= (3, 10) else 1)' 2>$null
            return $LASTEXITCODE -eq 0
        }
        $versionOutput = & $Executable version 2>$null
        return ($LASTEXITCODE -eq 0 -and $versionOutput -match '\bgo(\d+\.\d+(?:\.\d+)?)\b' -and
            [version]$Matches[1] -ge $script:minimumGo)
    }
    catch { return $false }
}

function Expand-BuildArchive {
    param([string]$ArchivePath, [string]$Destination)
    Add-Type -AssemblyName System.IO.Compression.FileSystem
    $destinationRoot = [IO.Path]::GetFullPath($Destination).TrimEnd('\') + '\'
    $archive = [IO.Compression.ZipFile]::OpenRead($ArchivePath)
    try {
        foreach ($entry in $archive.Entries) {
            $target = [IO.Path]::GetFullPath((Join-Path $Destination $entry.FullName))
            if (-not $target.StartsWith($destinationRoot, [StringComparison]::OrdinalIgnoreCase)) {
                throw "Unsafe ZIP entry: $($entry.FullName)"
            }
            if ($entry.FullName.EndsWith('/')) {
                [void][IO.Directory]::CreateDirectory($target)
            }
            else {
                [void][IO.Directory]::CreateDirectory([IO.Path]::GetDirectoryName($target))
                [IO.Compression.ZipFileExtensions]::ExtractToFile($entry, $target, $true)
            }
        }
    }
    finally { $archive.Dispose() }
}

function Get-PortableTool {
    param([string]$Kind, [string]$Architecture)
    $tool = $script:toolchains.$Kind
    $download = $tool.windows.$Architecture
    $directory = Join-Path $script:toolchainRoot "$Kind-$($tool.version)-$Architecture"
    $executable = Join-Path $directory $tool.executable
    $marker = Join-Path $directory '.ready'
    if ((Test-Path -LiteralPath $marker) -and (Test-Path -LiteralPath $executable) -and
        (Get-Content -LiteralPath $marker -Raw).Trim() -eq $download.sha256 -and
        (Test-BuildTool $executable $Kind)) {
        Write-Host "Using cached $Kind $($tool.version)."
        return $executable
    }

    New-Item -ItemType Directory -Path $script:toolchainRoot -Force | Out-Null
    $archive = Join-Path $script:toolchainRoot ([uri]$download.url).Segments[-1]
    if (-not (Test-Path -LiteralPath $archive)) {
        $partial = "$archive.part"
        if (-not (Test-Path -LiteralPath $partial) -or
            (Get-FileHash -LiteralPath $partial -Algorithm SHA256).Hash -ne $download.sha256) {
            Write-Host "Downloading $Kind $($tool.version)..."
            Invoke-WebRequest -Uri $download.url -OutFile $partial -UseBasicParsing
        }
        if ((Get-FileHash -LiteralPath $partial -Algorithm SHA256).Hash -ne $download.sha256) {
            throw "SHA256 mismatch for $Kind download. The archive will not be used."
        }
        Move-Item -LiteralPath $partial -Destination $archive -Force
    }
    if ((Get-FileHash -LiteralPath $archive -Algorithm SHA256).Hash -ne $download.sha256) {
        throw "SHA256 mismatch for cached archive '$archive'. Remove that ZIP and retry."
    }
    Write-Host "Extracting $Kind $($tool.version)..."
    Expand-BuildArchive $archive $directory
    if (-not (Test-BuildTool $executable $Kind)) {
        throw "Downloaded $Kind does not meet the build requirements."
    }
    Set-Content -LiteralPath $marker -Value $download.sha256 -Encoding ASCII
    return $executable
}

function Resolve-BuildTool {
    param([string]$Requested, [string]$Kind, [string]$Architecture)
    $names = if ($Requested) { @($Requested) } elseif ($Kind -eq 'python') {
        @('python', 'python3', 'py')
    } else { @('go') }
    foreach ($name in $names) {
        $command = Get-Command -Name $name -CommandType Application -ErrorAction SilentlyContinue |
            Select-Object -First 1
        if ($null -eq $command) { continue }
        if (-not $Requested -and $command.Source -like '*\Microsoft\WindowsApps\*') { continue }
        if (Test-BuildTool $command.Source $Kind) { return $command.Source }
    }
    if ($Requested) {
        throw "Invalid $Kind override '$Requested'; Python 3.10+ and Go $script:minimumGo+ are required."
    }
    return Get-PortableTool $Kind $Architecture
}

try {
    if ($env:OS -ne 'Windows_NT' -or -not [Environment]::Is64BitOperatingSystem) {
        throw 'The build bootstrap requires 64-bit Windows. On other systems use scripts/build.py with installed Python and Go.'
    }
    $hostArchitecture = $env:PROCESSOR_ARCHITEW6432
    if (-not $hostArchitecture) { $hostArchitecture = $env:PROCESSOR_ARCHITECTURE }
    $architecture = switch ($hostArchitecture) {
        'AMD64' { 'amd64' }
        'ARM64' { 'arm64' }
        default { throw "Unsupported Windows architecture: $hostArchitecture" }
    }
    [Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor
        [Net.SecurityProtocolType]::Tls12
    $script:toolchains = Get-Content -LiteralPath (Join-Path $PSScriptRoot 'scripts/toolchains.json') -Raw |
        ConvertFrom-Json
    $script:toolchainRoot = Join-Path $PSScriptRoot 'work/toolchains'
    $module = Get-Content -LiteralPath (Join-Path $PSScriptRoot 'go.mod') -Raw
    if ($module -notmatch '(?m)^go\s+(\d+\.\d+(?:\.\d+)?)\s*$') { throw 'Missing Go version in go.mod.' }
    $script:minimumGo = [version]$Matches[1]
    $env:GOROOT = $null
    $env:GOTOOLCHAIN = 'local'
    $pythonExecutable = Resolve-BuildTool $Python 'python' $architecture
    $goExecutable = Resolve-BuildTool $Go 'go' $architecture
    $pythonArguments = @(Get-PythonArguments $pythonExecutable)

    # Keep Go caches in the ignored work directory and use the selected toolchain.
    $env:GOCACHE = Join-Path $PSScriptRoot 'work/go-cache'
    $env:GOPATH = Join-Path $PSScriptRoot 'work/go-path'
    Write-Host 'Building SynoWake: Linux/amd64 backend, UI assets, SPK and SHA256...'
    & $pythonExecutable @pythonArguments (Join-Path $PSScriptRoot 'scripts/build.py') --go $goExecutable
    exit $LASTEXITCODE
}
catch {
    [Console]::Error.WriteLine("BUILD failed: {0}", $_.Exception.Message)
    exit 1
}
finally {
    foreach ($name in $originalEnvironment.Keys) {
        [Environment]::SetEnvironmentVariable($name, $originalEnvironment[$name], 'Process')
    }
}
