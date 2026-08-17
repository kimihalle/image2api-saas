param(
  [Parameter(Mandatory = $true)][string]$BackupFile,
  [ValidatePattern('^[a-fA-F0-9]{64}$')][string]$ExpectedChecksum,
  [switch]$Force
)

$ErrorActionPreference = 'Stop'
$root = Resolve-Path (Join-Path $PSScriptRoot '..')
$file = Resolve-Path -LiteralPath $BackupFile

if (-not (Test-Path -LiteralPath $file -PathType Leaf)) {
  throw "Backup file does not exist: $file"
}
if (-not $Force) {
  throw 'Restore replaces database objects. Re-run with -Force after confirming the target environment.'
}
if ($ExpectedChecksum) {
  $actual = (Get-FileHash -LiteralPath $file -Algorithm SHA256).Hash.ToLowerInvariant()
  if ($actual -ne $ExpectedChecksum.ToLowerInvariant()) {
    throw "Backup checksum mismatch. Expected $ExpectedChecksum, got $actual."
  }
}

Push-Location $root
$servicesStopped = $false
try {
  docker compose stop web backend
  if ($LASTEXITCODE -ne 0) { throw 'Failed to stop application services before restore.' }
  $servicesStopped = $true
  docker compose cp $file 'postgres:/tmp/image2api-restore.dump'
  if ($LASTEXITCODE -ne 0) { throw 'Failed to copy backup into postgres container.' }
  docker compose exec -T postgres sh -lc 'pg_restore --clean --if-exists --no-owner --no-privileges -U "$POSTGRES_USER" -d "$POSTGRES_DB" /tmp/image2api-restore.dump'
  if ($LASTEXITCODE -ne 0) { throw 'pg_restore failed.' }
  Write-Host 'Database restore completed.' -ForegroundColor Green
} finally {
  docker compose exec -T postgres rm -f '/tmp/image2api-restore.dump' 2>$null
  if ($servicesStopped) {
    docker compose up -d backend web
  }
  Pop-Location
}
