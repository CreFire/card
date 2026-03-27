$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$RepoRoot = Split-Path -Parent $ScriptDir

$ProtoSrcDir = Join-Path $RepoRoot "proto"
$OutDir = Join-Path $RepoRoot "server\src\proto\pb"
$ProtocExe = Join-Path $RepoRoot "tool\bin\protoc.exe"
$ProtocGenGo = Join-Path $RepoRoot "server\tools\.bin\protoc-gen-go.exe"

if (-not (Test-Path $ProtoSrcDir)) {
    throw "proto source directory not found: $ProtoSrcDir"
}

if (-not (Test-Path $ProtocExe)) {
    throw "protoc not found: $ProtocExe"
}

if (-not (Test-Path $ProtocGenGo)) {
    throw "protoc-gen-go not found: $ProtocGenGo"
}

New-Item -ItemType Directory -Force -Path $OutDir | Out-Null
Get-ChildItem $OutDir -File -Filter *.pb.go -ErrorAction SilentlyContinue | Remove-Item -Force

$protoFiles = @(Get-ChildItem $ProtoSrcDir -File -Filter *.proto | Sort-Object Name)
if ($protoFiles.Count -eq 0) {
    Write-Host "no proto files found under $ProtoSrcDir, nothing to generate"
    exit 0
}

$env:PATH = "$(Join-Path $RepoRoot 'server\tools\.bin');$env:PATH"

foreach ($protoFile in $protoFiles) {
    Write-Host "generate $($protoFile.Name)"
    & $ProtocExe `
        "--proto_path=$ProtoSrcDir" `
        "--go_out=paths=source_relative:$OutDir" `
        $protoFile.Name

    if ($LASTEXITCODE -ne 0) {
        throw "protoc generation failed: $($protoFile.Name)"
    }
}

Write-Host "server proto generated to $OutDir"
