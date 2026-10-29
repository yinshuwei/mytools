#Requires -Version 5.1
<#
.SYNOPSIS
  Compiles MyTools for the platforms it ships on.

.DESCRIPTION
  MyGo draws its window through purego and never imports cgo, so one machine
  cross-compiles the lot: the only thing needed is a Go toolchain.

  Every target gets its own directory, OUT/<os>-<arch>/, holding the binary
  and the resources it reads, and, unless -NoArchive, is packed as
  OUT/<name>-<version>-<os>-<arch>.tar.gz (tar ships with Windows 10 1803
  and later).

.EXAMPLE
  scripts\build.cmd
  scripts\build.cmd -Platform linux/amd64 -Version 1.2.3
  scripts\build.cmd -Local
#>
[CmdletBinding()]
param(
    # Version stamped into the binary (default: the git tag, else git
    # describe, else 0.1.0).
    [string] $Version,
    # Where the builds go.
    [string] $Out = "dist",
    # GOOS/GOARCH targets, comma separated, e.g. linux/amd64,windows/arm64,
    # or "all" (the default).
    [string] $Platform,
    # Build for this machine only.
    [switch] $Local,
    # Keep development features and symbols, instead of a release build.
    # (Not "Debug": CmdletBinding already defines a common -Debug parameter.)
    [switch] $Dev,
    # Do not pack the tar.gz archives.
    [switch] $NoArchive
)

$ErrorActionPreference = "Stop"

# A target's GOOS must not outlive the build, and removing a variable that is
# not set is an error: this clears the ones that are there.
function Clear-GoEnv {
    foreach ($v in "GOOS", "GOARCH", "CGO_ENABLED") {
        if (Test-Path "Env:$v") { Remove-Item "Env:$v" }
    }
}

$root = Split-Path -Parent $PSScriptRoot
$name = "mytools"
$pkg = "./cmd/mytools"
$allTargets = @(
    "linux/amd64", "linux/arm64",
    "darwin/amd64", "darwin/arm64",
    "windows/amd64", "windows/arm64"
)

if (-not $Version) {
    $described = $null
    if (Get-Command git -ErrorAction SilentlyContinue) {
        $described = & git -C $root describe --tags --always --dirty 2>$null
        if ($LASTEXITCODE -ne 0) { $described = $null }
    }
    if ($described) {
        $Version = $described.Trim() -replace '^v', ''
    }
    else {
        $Version = "0.1.0"
    }
}

if ($Local) {
    $targets = @("$(& go env GOOS)/$(& go env GOARCH)")
}
elseif (-not $Platform -or $Platform -eq "all") {
    $targets = $allTargets
}
else {
    $targets = $Platform -split ","
}

if (-not [System.IO.Path]::IsPathRooted($Out)) {
    $Out = Join-Path $root $Out
}
New-Item -ItemType Directory -Force -Path $Out | Out-Null

Push-Location $root
$syso = @()
try {
    # Explorer, the taskbar and the window take their icon from the icon
    # resource of the executable, which go build links only from a .syso in
    # the main package: mygo build writes one, scripts/winres does it here.
    # The files it writes go away in the finally, and only the Windows
    # builds read them.
    if (@($targets | Where-Object { $_.Trim() -like "windows/*" }).Count -gt 0) {
        # go run builds for this machine, so no target's GOOS may be in the
        # environment.
        Clear-GoEnv
        $syso = @(& go run ./scripts/winres -icon resources/icon.png -o cmd/mytools)
        if ($LASTEXITCODE -ne 0) { throw "scripts/winres failed" }
    }

    foreach ($target in $targets) {
        $target = $target.Trim()
        $parts = $target.Split("/")
        if ($parts.Count -ne 2 -or -not $parts[0] -or -not $parts[1]) {
            throw "target must be GOOS/GOARCH, not $target"
        }
        $os = $parts[0]
        $arch = $parts[1]
        $bin = if ($os -eq "windows") { "$name.exe" } else { $name }

        Write-Host "==> $target"
        $dir = Join-Path $Out "$os-$arch"
        if (Test-Path $dir) { Remove-Item -Recurse -Force $dir }
        New-Item -ItemType Directory -Force -Path $dir | Out-Null

        $ldflags = "-X main.version=$Version"
        $tagArgs = @()
        if (-not $Dev) {
            $ldflags = "-s -w $ldflags"
            $tagArgs = @("-tags", "mygo_noinspector")
        }
        if ($os -eq "windows") {
            # Without this the app opens a console window beside itself.
            $ldflags = "-H=windowsgui $ldflags"
        }

        $env:GOOS = $os
        $env:GOARCH = $arch
        $env:CGO_ENABLED = "0"

        $goArgs = @("build", "-trimpath") + $tagArgs +
            @("-ldflags", $ldflags, "-o", (Join-Path $dir $bin), $pkg)
        & go @goArgs
        if ($LASTEXITCODE -ne 0) {
            throw "go build failed for $target"
        }

        # At run time MyGo reads resources next to the executable.
        $resources = Join-Path $root "resources"
        if (Test-Path $resources) {
            Copy-Item -Recurse -Force $resources $dir
        }

        if (-not $NoArchive) {
            $archive = Join-Path $Out "$name-$Version-$os-$arch.tar.gz"
            if (Get-Command tar -ErrorAction SilentlyContinue) {
                & tar -czf $archive -C $Out "$os-$arch"
                if ($LASTEXITCODE -ne 0) {
                    Write-Warning "tar failed; skipped $archive"
                }
            }
            else {
                Write-Warning "tar is not available; skipped $archive"
            }
        }
    }
}
finally {
    Clear-GoEnv
    if ($syso) { Remove-Item $syso -Force -ErrorAction SilentlyContinue }
    Pop-Location
}

Write-Host "mytools $Version built for: $($targets -join ' ')"
Write-Host "in $Out"
