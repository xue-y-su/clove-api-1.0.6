$ErrorActionPreference = 'Stop'

$scriptPath = Join-Path $PSScriptRoot 'verify-runtime-source.ps1'
$tempDirectory = Join-Path ([System.IO.Path]::GetTempPath()) ("cloveapi-runtime-test-" + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $tempDirectory | Out-Null

try {
    $matchingFixture = @{
        container = @{
            Id = 'container-current'
            Image = 'sha256:image-current'
            State = @{ StartedAt = '2026-07-27T01:00:00Z' }
        }
        image = @{
            Id = 'sha256:image-current'
            Created = '2026-07-27T00:55:00Z'
            Config = @{ Labels = @{
                'org.opencontainers.image.revision' = 'commit-current'
                'io.cloveapi.frontend.default.sha256' = 'default-current'
                'io.cloveapi.frontend.classic.sha256' = 'classic-current'
            } }
        }
    }
    $matchingPath = Join-Path $tempDirectory 'matching.json'
    $matchingFixture | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath $matchingPath -Encoding utf8

    $matchingOutput = & pwsh -NoProfile -File $scriptPath `
        -ContainerName 'fixture' `
        -ExpectedCommit 'commit-current' `
        -ExpectedDefaultFrontendHash 'default-current' `
        -ExpectedClassicFrontendHash 'classic-current' `
        -InspectFixturePath $matchingPath
    if ($LASTEXITCODE -ne 0) {
        throw "matching runtime exited with $LASTEXITCODE`: $matchingOutput"
    }
    $matching = $matchingOutput | ConvertFrom-Json
    if (-not $matching.source_matches) {
        throw 'matching runtime was reported as stale'
    }
    if ($matching.container_id -ne 'container-current' -or $matching.image_id -ne 'sha256:image-current') {
        throw 'runtime evidence omitted container or image identity'
    }

    $staleFixture = $matchingFixture.Clone()
    $staleFixture.image = @{
        Id = 'sha256:image-stale'
        Created = '2026-07-26T00:55:00Z'
        Config = @{ Labels = @{
            'org.opencontainers.image.revision' = 'commit-stale'
            'io.cloveapi.frontend.default.sha256' = 'default-stale'
            'io.cloveapi.frontend.classic.sha256' = 'classic-current'
        } }
    }
    $stalePath = Join-Path $tempDirectory 'stale.json'
    $staleFixture | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath $stalePath -Encoding utf8

    $staleOutput = & pwsh -NoProfile -File $scriptPath `
        -ContainerName 'fixture' `
        -ExpectedCommit 'commit-current' `
        -ExpectedDefaultFrontendHash 'default-current' `
        -ExpectedClassicFrontendHash 'classic-current' `
        -InspectFixturePath $stalePath 2>&1
    if ($LASTEXITCODE -eq 0) {
        throw 'stale runtime unexpectedly passed verification'
    }
    $stale = $staleOutput | ConvertFrom-Json
    if ($stale.source_matches) {
        throw 'stale runtime response did not report the mismatch'
    }
    if ($stale.mismatches -notcontains 'revision' -or $stale.mismatches -notcontains 'default_frontend') {
        throw 'stale runtime response did not identify each mismatched artifact'
    }

    Write-Output 'PASS verify-runtime-source'
}
finally {
    Remove-Item -LiteralPath $tempDirectory -Recurse -Force -ErrorAction SilentlyContinue
}
