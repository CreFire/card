$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$RepoRoot = Split-Path -Parent $ScriptDir

$ProtoSrcDir = Join-Path $RepoRoot "proto"
$ClientRoot = Join-Path $RepoRoot "client\wuziqi"
$ClientProtoDir = Join-Path $ClientRoot "proto\pb"

New-Item -ItemType Directory -Force -Path $ClientProtoDir | Out-Null

if (-not (Test-Path $ProtoSrcDir)) {
    throw "proto source directory not found: $ProtoSrcDir"
}

Get-ChildItem $ClientProtoDir -Filter *.proto -ErrorAction SilentlyContinue | Remove-Item -Force
Get-ChildItem $ProtoSrcDir -Filter *.proto -ErrorAction SilentlyContinue | Copy-Item -Destination $ClientProtoDir -Force

Write-Host "client proto synced to $ClientProtoDir"
