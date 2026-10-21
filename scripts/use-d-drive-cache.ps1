# Copyright (C) 2025 QuantumNous
#
# Keep disposable build data off the system drive and fail early when either
# local drive is too full for a production build.

param(
  [double]$MinimumSystemDriveFreeGB = 20,
  [double]$MinimumBuildDriveFreeGB = 25
)

$ErrorActionPreference = 'Stop'
$repositoryRoot = Split-Path -Parent $PSScriptRoot
$gitCommonDir = & git -C $repositoryRoot rev-parse --path-format=absolute --git-common-dir 2>$null
if ($LASTEXITCODE -eq 0 -and $gitCommonDir) {
  $mainRepositoryRoot = Split-Path -Parent $gitCommonDir.Trim()
  $workspaceRoot = Split-Path -Parent $mainRepositoryRoot
} else {
  $workspaceRoot = Split-Path -Parent $repositoryRoot
}
$cacheRoot = Join-Path $workspaceRoot '.build-cache'

function Get-DriveFreeGB([string]$driveLetter) {
  $drive = [System.IO.DriveInfo]::new($driveLetter)
  return [math]::Round($drive.AvailableFreeSpace / 1GB, 2)
}

$systemFreeGB = Get-DriveFreeGB 'C'
$buildFreeGB = Get-DriveFreeGB 'D'
if ($systemFreeGB -lt $MinimumSystemDriveFreeGB) {
  throw "C: has only $systemFreeGB GB free; clean system caches before building."
}
if ($buildFreeGB -lt $MinimumBuildDriveFreeGB) {
  throw "D: has only $buildFreeGB GB free; clean build caches before building."
}

$paths = @{
  TEMP = Join-Path $cacheRoot 'tmp'
  TMP = Join-Path $cacheRoot 'tmp'
  GOCACHE = Join-Path $cacheRoot 'go-build'
  GOMODCACHE = Join-Path $cacheRoot 'go-mod'
  GOTMPDIR = Join-Path $cacheRoot 'go-tmp'
  BUN_INSTALL_CACHE_DIR = Join-Path $cacheRoot 'bun'
  npm_config_cache = Join-Path $cacheRoot 'npm'
  XDG_CACHE_HOME = Join-Path $cacheRoot 'xdg'
  PLAYWRIGHT_BROWSERS_PATH = Join-Path $cacheRoot 'playwright'
  PIP_CACHE_DIR = Join-Path $cacheRoot 'pip'
  UV_CACHE_DIR = Join-Path $cacheRoot 'uv'
  DOCKER_CONFIG = Join-Path $cacheRoot 'docker-config'
  BUILDX_CONFIG = Join-Path $cacheRoot 'buildx'
}

foreach ($entry in $paths.GetEnumerator()) {
  New-Item -ItemType Directory -Force -Path $entry.Value | Out-Null
  Set-Item -Path "Env:$($entry.Key)" -Value $entry.Value
}

$env:COMPOSE_PROGRESS = 'plain'
Write-Host "Build cache root: $cacheRoot"
Write-Host "Free space: C: $systemFreeGB GB; D: $buildFreeGB GB"
