$ErrorActionPreference = 'Stop'

Write-Host '[1/3] Building Linux backend binary...'
Push-Location "$PSScriptRoot\backend"
try {
    $env:CGO_ENABLED = '0'
    $env:GOOS = 'linux'
    $env:GOARCH = 'amd64'
    go build -trimpath -ldflags '-s -w' -o api-linux ./cmd/api
    if ($LASTEXITCODE -ne 0) { throw "Backend build failed with exit code $LASTEXITCODE" }
} finally {
    Pop-Location
}

Write-Host '[2/3] Building frontend assets...'
Push-Location "$PSScriptRoot\frontend"
try {
    npm run build
    if ($LASTEXITCODE -ne 0) { throw "Frontend build failed with exit code $LASTEXITCODE" }
} finally {
    Pop-Location
}

Write-Host '[3/3] Starting Docker services from local images...'
docker compose -f "$PSScriptRoot\docker-compose.yml" -f "$PSScriptRoot\docker-compose.local.yml" up -d --build
if ($LASTEXITCODE -ne 0) { throw "Docker Compose failed with exit code $LASTEXITCODE" }
docker compose -f "$PSScriptRoot\docker-compose.yml" -f "$PSScriptRoot\docker-compose.local.yml" ps
if ($LASTEXITCODE -ne 0) { throw "Docker Compose status failed with exit code $LASTEXITCODE" }
