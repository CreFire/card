$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$RepoRoot = Split-Path -Parent $ScriptDir

$ServerDocconfDir = Join-Path $RepoRoot "server\configdoc\json"
$ClientRoot = Join-Path $RepoRoot "client\wuziqi"
$ClientConfigDir = Join-Path $ClientRoot "generated\config\json"

New-Item -ItemType Directory -Force -Path $ClientConfigDir | Out-Null

if (-not (Test-Path $ServerDocconfDir)) {
    throw "server docconf directory not found: $ServerDocconfDir"
}

Get-ChildItem $ClientConfigDir -Filter *.json -ErrorAction SilentlyContinue | Remove-Item -Force
Get-ChildItem $ServerDocconfDir -Filter *.json -ErrorAction SilentlyContinue | Copy-Item -Destination $ClientConfigDir -Force

Write-Host "client config synced to $ClientConfigDir"
