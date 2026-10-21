param(
    [Parameter(Mandatory = $true)]
    [string]$ContainerName,

    [Parameter(Mandatory = $true)]
    [string]$ExpectedCommit,

    [string]$ExpectedDefaultFrontendHash = '',

    [string]$ExpectedClassicFrontendHash = '',

    [string]$InspectFixturePath = ''
)

$ErrorActionPreference = 'Stop'

function Read-InspectEvidence {
    if ($InspectFixturePath) {
        return Get-Content -Raw -LiteralPath $InspectFixturePath | ConvertFrom-Json
    }

    $container = docker inspect $ContainerName | ConvertFrom-Json | Select-Object -First 1
    if (-not $container) {
        throw "container '$ContainerName' was not found"
    }
    $image = docker image inspect $container.Image | ConvertFrom-Json | Select-Object -First 1
    if (-not $image) {
        throw "image '$($container.Image)' was not found"
    }
    return [pscustomobject]@{ container = $container; image = $image }
}

function Read-Label {
    param(
        [object]$Image,
        [string]$Name
    )

    if (-not $Image.Config -or -not $Image.Config.Labels) {
        return ''
    }
    $property = $Image.Config.Labels.PSObject.Properties[$Name]
    if (-not $property) {
        return ''
    }
    return [string]$property.Value
}

try {
    $evidence = Read-InspectEvidence
    $revision = Read-Label -Image $evidence.image -Name 'org.opencontainers.image.revision'
    $defaultFrontendHash = Read-Label -Image $evidence.image -Name 'io.cloveapi.frontend.default.sha256'
    $classicFrontendHash = Read-Label -Image $evidence.image -Name 'io.cloveapi.frontend.classic.sha256'

    $mismatches = [System.Collections.Generic.List[string]]::new()
    if ($revision -ne $ExpectedCommit) {
        $mismatches.Add('revision')
    }
    if ($ExpectedDefaultFrontendHash -and $defaultFrontendHash -ne $ExpectedDefaultFrontendHash) {
        $mismatches.Add('default_frontend')
    }
    if ($ExpectedClassicFrontendHash -and $classicFrontendHash -ne $ExpectedClassicFrontendHash) {
        $mismatches.Add('classic_frontend')
    }

    $result = [ordered]@{
        expected_commit = $ExpectedCommit
        image_revision = $revision
        image_id = [string]$evidence.image.Id
        container_id = [string]$evidence.container.Id
        image_created_at = [string]$evidence.image.Created
        container_started_at = [string]$evidence.container.State.StartedAt
        default_frontend_sha256 = $defaultFrontendHash
        classic_frontend_sha256 = $classicFrontendHash
        source_matches = $mismatches.Count -eq 0
        mismatches = @($mismatches)
    }
    $result | ConvertTo-Json -Compress
    if ($mismatches.Count -gt 0) {
        exit 1
    }
}
catch {
    [ordered]@{
        expected_commit = $ExpectedCommit
        source_matches = $false
        mismatches = @('inspection')
        error = $_.Exception.Message
    } | ConvertTo-Json -Compress
    exit 1
}
