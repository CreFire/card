$ErrorActionPreference = "Stop"

$composeFile = Join-Path $PSScriptRoot "docker-compose.yml"

docker compose -f $composeFile up -d
docker compose -f $composeFile ps
