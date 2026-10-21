$ErrorActionPreference = 'Stop'

$scriptPath = Join-Path $PSScriptRoot 'rebuild-local-mysql-qa.ps1'
$source = Get-Content -Raw -LiteralPath $scriptPath
$tokens = $null
$parseErrors = $null
$ast = [System.Management.Automation.Language.Parser]::ParseFile(
    $scriptPath,
    [ref]$tokens,
    [ref]$parseErrors
)

if ($parseErrors.Count -gt 0) {
    throw "rebuild script has parse errors: $($parseErrors[0].Message)"
}

$goProxyParameter = $ast.ParamBlock.Parameters | Where-Object {
    $_.Name.VariablePath.UserPath -eq 'GoProxy'
}
if (-not $goProxyParameter) {
    throw 'rebuild script does not expose a GoProxy parameter'
}
if ($source -notmatch "'--build-arg'") {
    throw 'rebuild script does not pass a Docker build argument'
}
if ($source -notmatch '"GOPROXY=\$GoProxy"') {
    throw 'rebuild script does not forward GoProxy to Docker'
}
if ($source -match 'SetEnvironmentVariable') {
    throw 'rebuild script mutates the host process environment'
}
if (-not $source.Contains('$runArguments.Add($entry)')) {
    throw 'rebuild script does not pass complete environment entries to Docker'
}
if (-not $source.Contains('Invoke-WebRequest -UseBasicParsing -NoProxy -TimeoutSec 5')) {
    throw 'rebuild script does not bypass proxies for the local health check'
}

$skipBuildParameter = $ast.ParamBlock.Parameters | Where-Object {
    $_.Name.VariablePath.UserPath -eq 'SkipBuild'
}
if (-not $skipBuildParameter) {
    throw 'rebuild script does not expose a SkipBuild parameter'
}
if (-not $source.Contains('if (-not $SkipBuild)')) {
    throw 'rebuild script does not guard Docker build with SkipBuild'
}

foreach ($parameterName in @('TokenHmacKey', 'TokenEncryptionKeys')) {
    $parameter = $ast.ParamBlock.Parameters | Where-Object {
        $_.Name.VariablePath.UserPath -eq $parameterName
    }
    if (-not $parameter) {
        throw "rebuild script does not expose a $parameterName parameter"
    }
}

$mediaInitImageParameter = $ast.ParamBlock.Parameters | Where-Object {
    $_.Name.VariablePath.UserPath -eq 'MediaInitImage'
}
if (-not $mediaInitImageParameter) {
    throw 'rebuild script does not expose a MediaInitImage parameter'
}
if (-not $source.Contains('mkdir -p /data/announcement-media')) {
    throw 'rebuild script does not initialize the announcement media directory'
}
if (-not $source.Contains('chown -R 10001:10001 /data/announcement-media')) {
    throw 'rebuild script does not recursively assign announcement media ownership to the runtime user'
}
if (-not $source.Contains('find /data/announcement-media -type d -exec chmod 0750 {} +')) {
    throw 'rebuild script does not normalize announcement media directory permissions'
}
if (-not $source.Contains('find /data/announcement-media -type f -exec chmod 0640 {} +')) {
    throw 'rebuild script does not normalize announcement media file permissions'
}

foreach ($composeName in @('docker-compose.yml', 'docker-compose.dev.yml')) {
    $composeSource = Get-Content -Raw -LiteralPath (Join-Path (Split-Path -Parent $PSScriptRoot) $composeName)
    if (-not $composeSource.Contains('chown -R 10001:10001 /data/announcement-media')) {
        throw "$composeName does not recursively assign announcement media ownership"
    }
    if (-not $composeSource.Contains('find /data/announcement-media -type d -exec chmod 0750 {} +')) {
        throw "$composeName does not normalize announcement media directory permissions"
    }
    if (-not $composeSource.Contains('find /data/announcement-media -type f -exec chmod 0640 {} +')) {
        throw "$composeName does not normalize announcement media file permissions"
    }
}

foreach ($environmentName in @('TOKEN_HMAC_KEY', 'TOKEN_ENCRYPTION_KEYS')) {
    if (-not $source.Contains($environmentName)) {
        throw "rebuild script does not forward $environmentName"
    }
}

$createPosition = $source.IndexOf("@('create'")
$mediaInitPosition = $source.IndexOf("@('run', '--rm', '--user', '0:0'")
$connectPosition = $source.IndexOf("@('network', 'connect'")
$startPosition = $source.IndexOf("@('start', `$ContainerName)")
if ($mediaInitPosition -lt 0 -or $createPosition -lt 0 -or $connectPosition -lt 0 -or $startPosition -lt 0) {
    throw 'rebuild script does not use the create/connect/start lifecycle'
}
if (-not ($mediaInitPosition -lt $createPosition -and $createPosition -lt $connectPosition -and $connectPosition -lt $startPosition)) {
    throw 'rebuild script starts the replacement before attaching its database network'
}

Write-Output 'PASS rebuild-local-mysql-qa'
