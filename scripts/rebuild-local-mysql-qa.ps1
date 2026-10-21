param(
    [string]$ContainerName = 'cloveapi-current-source-mysql-qa',
    [string]$ImageTag = 'clove-api:qa-20260727-remediation',
    [int]$HostPort = 18104,
    [string]$GoProxy = 'https://proxy.golang.org,direct',
    [string]$TokenHmacKey = $env:TOKEN_HMAC_KEY,
    [string]$TokenEncryptionKeys = $env:TOKEN_ENCRYPTION_KEYS,
    [string]$MediaInitImage = 'busybox:1.36.1',
    [switch]$SkipBuild
)

$ErrorActionPreference = 'Stop'
$repositoryRoot = Split-Path -Parent $PSScriptRoot

function Get-SourceFingerprint {
    param([string]$PathPrefix)

    $lines = git -C $repositoryRoot ls-files -- $PathPrefix | Sort-Object
    $builder = [System.Text.StringBuilder]::new()
    foreach ($relativePath in $lines) {
        $absolutePath = Join-Path $repositoryRoot $relativePath
        if (-not (Test-Path -LiteralPath $absolutePath -PathType Leaf)) {
            continue
        }
        $hash = (Get-FileHash -LiteralPath $absolutePath -Algorithm SHA256).Hash.ToLowerInvariant()
        [void]$builder.Append($relativePath.Replace('\', '/')).Append(':').Append($hash).Append("`n")
    }
    $bytes = [System.Text.Encoding]::UTF8.GetBytes($builder.ToString())
    return [Convert]::ToHexString([System.Security.Cryptography.SHA256]::HashData($bytes)).ToLowerInvariant()
}

function Invoke-Docker {
    param([string[]]$Arguments)

    & docker @Arguments
    if ($LASTEXITCODE -ne 0) {
        throw "docker command failed: docker $($Arguments[0])"
    }
}

$container = docker inspect $ContainerName | ConvertFrom-Json | Select-Object -First 1
if (-not $container) {
    throw "container '$ContainerName' was not found"
}

$commit = (git -C $repositoryRoot rev-parse HEAD).Trim()
$defaultFrontendHash = Get-SourceFingerprint -PathPrefix 'web/default'
$classicFrontendHash = Get-SourceFingerprint -PathPrefix 'web/classic'

if (-not $SkipBuild) {
    Invoke-Docker -Arguments @(
        'build',
        '--label', "org.opencontainers.image.revision=$commit",
        '--label', "io.cloveapi.frontend.default.sha256=$defaultFrontendHash",
        '--label', "io.cloveapi.frontend.classic.sha256=$classicFrontendHash",
        '--build-arg', "GOPROXY=$GoProxy",
        '--tag', $ImageTag,
        $repositoryRoot
    )
}

$environmentEntries = [System.Collections.Generic.List[string]]::new()
$environmentOverrides = @{}
if (-not [string]::IsNullOrWhiteSpace($TokenHmacKey)) {
    $environmentOverrides['TOKEN_HMAC_KEY'] = $TokenHmacKey
}
if (-not [string]::IsNullOrWhiteSpace($TokenEncryptionKeys)) {
    $environmentOverrides['TOKEN_ENCRYPTION_KEYS'] = $TokenEncryptionKeys
}
foreach ($entry in $container.Config.Env) {
    $separator = $entry.IndexOf('=')
    if ($separator -le 0) {
        continue
    }
    $name = $entry.Substring(0, $separator)
    if ($environmentOverrides.ContainsKey($name)) {
        continue
    }
    $environmentEntries.Add($entry)
}
foreach ($override in $environmentOverrides.GetEnumerator()) {
    $environmentEntries.Add("$($override.Key)=$($override.Value)")
}

$rollbackName = "$ContainerName-rollback-$(Get-Date -Format 'yyyyMMddHHmmss')"
$newContainerCreated = $false
try {
    Invoke-Docker -Arguments @('stop', $ContainerName)
    Invoke-Docker -Arguments @('rename', $ContainerName, $rollbackName)

    $dataMount = $container.Mounts | Where-Object {
        $_.Type -eq 'volume' -and $_.Destination -eq '/data'
    } | Select-Object -First 1
    if (-not $dataMount) {
        throw "container '$ContainerName' does not have a /data Docker volume"
    }
    Invoke-Docker -Arguments @('run', '--rm', '--user', '0:0', '--network', 'none', '--volume', "$($dataMount.Name):/data", $MediaInitImage, 'sh', '-ec', 'mkdir -p /data/announcement-media && chown -R 10001:10001 /data/announcement-media && find /data/announcement-media -type d -exec chmod 0750 {} + && find /data/announcement-media -type f -exec chmod 0640 {} +')

    $runArguments = [System.Collections.Generic.List[string]]::new()
    foreach ($argument in @('create', '--name', $ContainerName, '--restart', 'unless-stopped', '--publish', "127.0.0.1:${HostPort}:3000")) {
        $runArguments.Add($argument)
    }
    foreach ($entry in $environmentEntries) {
        $runArguments.Add('--env')
        $runArguments.Add($entry)
    }
    foreach ($mount in $container.Mounts) {
        if ($mount.Type -eq 'volume') {
            $runArguments.Add('--volume')
            $runArguments.Add("$($mount.Name):$($mount.Destination)")
        }
    }
    $runArguments.Add($ImageTag)
    Invoke-Docker -Arguments $runArguments.ToArray()
    $newContainerCreated = $true

    foreach ($networkProperty in $container.NetworkSettings.Networks.PSObject.Properties) {
        $networkName = $networkProperty.Name
        if ($networkName -eq 'bridge') {
            continue
        }
        Invoke-Docker -Arguments @('network', 'connect', $networkName, $ContainerName)
    }
    Invoke-Docker -Arguments @('start', $ContainerName)

    $deadline = (Get-Date).AddMinutes(2)
    do {
        Start-Sleep -Seconds 2
        try {
            $response = Invoke-WebRequest -UseBasicParsing -NoProxy -TimeoutSec 5 -Uri "http://127.0.0.1:$HostPort/api/status"
            if ($response.StatusCode -eq 200) {
                break
            }
        }
        catch {
            if ((Get-Date) -ge $deadline) {
                throw
            }
        }
    } while ((Get-Date) -lt $deadline)

    if ((Get-Date) -ge $deadline) {
        throw 'replacement container did not become healthy within two minutes'
    }

    Invoke-Docker -Arguments @('rm', $rollbackName)
    [ordered]@{
        container = $ContainerName
        image = $ImageTag
        revision = $commit
        default_frontend_sha256 = $defaultFrontendHash
        classic_frontend_sha256 = $classicFrontendHash
        url = "http://127.0.0.1:$HostPort"
    } | ConvertTo-Json -Compress
}
catch {
    if ($newContainerCreated) {
        & docker rm --force $ContainerName | Out-Null
    }
    $rollbackExists = docker inspect $rollbackName 2>$null
    if ($LASTEXITCODE -eq 0 -and $rollbackExists) {
        Invoke-Docker -Arguments @('rename', $rollbackName, $ContainerName)
        Invoke-Docker -Arguments @('start', $ContainerName)
    }
    throw
}
