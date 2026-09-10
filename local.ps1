param(
    [ValidateSet('up', 'down', 'restart', 'status', 'logs')]
    [string]$Action = 'up',
    [switch]$UseBuiltArtifacts
)

$ErrorActionPreference = 'Stop'

$workspace = $PSScriptRoot
$coreDir = Join-Path $workspace 'core\deploy'
$portalDir = Join-Path $workspace 'portal'
$coreCompose = Join-Path $coreDir 'docker-compose.yml'
$coreEnv = Join-Path $coreDir '.env'
$portalCompose = Join-Path $portalDir 'docker-compose.yml'
$portalArtifactsCompose = Join-Path $portalDir 'docker-compose.artifacts.yml'
$portalEnv = Join-Path $portalDir '.env'

function Invoke-Compose {
    param(
        [string]$WorkingDirectory,
        [string[]]$ComposeFiles,
        [string]$EnvFile,
        [string[]]$Arguments
    )

    Push-Location $WorkingDirectory
    try {
        $composeArguments = @('compose', '--env-file', $EnvFile)
        foreach ($composeFile in $ComposeFiles) {
            $composeArguments += @('-f', $composeFile)
        }
        $composeArguments += $Arguments
        & docker @composeArguments
        if ($LASTEXITCODE -ne 0) {
            throw "docker compose failed in $WorkingDirectory"
        }
    }
    finally {
        Pop-Location
    }
}

function Wait-HttpHealthy {
    param(
        [string]$Name,
        [string]$Url,
        [int]$Attempts = 60
    )

    for ($attempt = 1; $attempt -le $Attempts; $attempt++) {
        try {
            $response = Invoke-WebRequest -UseBasicParsing -Uri $Url -TimeoutSec 3
            if ($response.StatusCode -ge 200 -and $response.StatusCode -lt 300) {
                Write-Host "$Name is ready: $Url"
                return
            }
        }
        catch {
            if ($attempt -eq $Attempts) {
                throw "$Name did not become healthy at $Url"
            }
        }
        Start-Sleep -Seconds 2
    }
}

function Assert-Configuration {
    foreach ($path in @($coreEnv, $portalEnv)) {
        if (-not (Test-Path -LiteralPath $path)) {
            throw "Missing required environment file: $path"
        }
    }

    $portalCoreUrl = Get-Content -LiteralPath $portalEnv |
        Where-Object { $_ -match '^CORE_BASE_URL=' } |
        Select-Object -First 1
    if ($portalCoreUrl -ne 'CORE_BASE_URL=http://host.docker.internal:8080') {
        throw "portal/.env must use CORE_BASE_URL=http://host.docker.internal:8080"
    }
}

function Show-Status {
    Write-Host "`nCore"
    Invoke-Compose $coreDir @($coreCompose) $coreEnv @('ps')
    Write-Host "`nPortal"
    Invoke-Compose $portalDir @($portalCompose) $portalEnv @('ps')
}

function Start-Portal {
    if ($UseBuiltArtifacts) {
        Push-Location (Join-Path $portalDir 'frontend')
        try {
            & npm run build
            if ($LASTEXITCODE -ne 0) {
                throw 'Portal frontend build failed'
            }
        }
        finally {
            Pop-Location
        }

        & docker image inspect portal-portal-api:latest *> $null
        if ($LASTEXITCODE -ne 0) {
            throw 'portal-portal-api:latest is missing; run a full Portal build first'
        }
        Invoke-Compose $portalDir @($portalCompose, $portalArtifactsCompose) $portalEnv @('up', '-d', '--no-build')
        return
    }

    Invoke-Compose $portalDir @($portalCompose) $portalEnv @('up', '-d', '--build')
}

Assert-Configuration

switch ($Action) {
    'up' {
        Invoke-Compose $coreDir @($coreCompose) $coreEnv @('up', '-d')
        Wait-HttpHealthy 'Core' 'http://127.0.0.1:8080/health'
        Start-Portal
        Wait-HttpHealthy 'Portal' 'http://127.0.0.1:3000/health'
        Show-Status
    }
    'down' {
        Invoke-Compose $portalDir @($portalCompose, $portalArtifactsCompose) $portalEnv @('down')
        Invoke-Compose $coreDir @($coreCompose) $coreEnv @('down')
    }
    'restart' {
        Invoke-Compose $portalDir @($portalCompose, $portalArtifactsCompose) $portalEnv @('down')
        Invoke-Compose $coreDir @($coreCompose) $coreEnv @('up', '-d')
        Wait-HttpHealthy 'Core' 'http://127.0.0.1:8080/health'
        Start-Portal
        Wait-HttpHealthy 'Portal' 'http://127.0.0.1:3000/health'
        Show-Status
    }
    'status' {
        Show-Status
    }
    'logs' {
        Invoke-Compose $portalDir @($portalCompose) $portalEnv @('logs', '--tail', '100', 'portal-api', 'portal-web')
        Invoke-Compose $coreDir @($coreCompose) $coreEnv @('logs', '--tail', '100', 'sub2api')
    }
}
