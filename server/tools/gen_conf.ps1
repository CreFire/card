$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$ServerRoot = Split-Path -Parent $ScriptDir
$RepoRoot = Split-Path -Parent $ServerRoot

$ExcelDir = Join-Path $RepoRoot "excel"
$ProtoOutDir = Join-Path $ServerRoot "src\\proto"
$JsonOutDir = Join-Path $ServerRoot "docconf"
$LocalBinDir = Join-Path $ServerRoot "tools\\.bin"
$StageRoot = Join-Path $ServerRoot "tools\\.luban_stage"
$StageProtoDir = Join-Path $StageRoot "proto"
$StageJsonDir = Join-Path $StageRoot "json"
$StageConfFile = Join-Path $StageRoot "luban.conf"

function Resolve-LubanDll {
    $candidates = @()

    if ($env:LUBAN_DLL) {
        $candidates += $env:LUBAN_DLL
    }

    $candidates += @(
        (Join-Path $RepoRoot "tool\\Luban\\Luban.dll"),
        (Join-Path $RepoRoot "tool\\luban\\Luban.dll"),
        (Join-Path $RepoRoot "tools\\Luban\\Luban.dll"),
        (Join-Path $RepoRoot "tools\\luban\\Luban.dll")
    )

    foreach ($candidate in $candidates) {
        if ($candidate -and (Test-Path $candidate)) {
            return (Resolve-Path $candidate).Path
        }
    }

    throw "未找到 Luban.dll。请设置环境变量 LUBAN_DLL，或将 Luban 放到 tool/Luban 或 tools/Luban。"
}

function Resolve-Protoc {
    $cmd = Get-Command protoc -ErrorAction SilentlyContinue
    if ($cmd) {
        return $cmd.Source
    }

    $candidates = @(
        (Join-Path $RepoRoot "tool\\bin\\protoc.exe"),
        (Join-Path $RepoRoot "tool\\protoc\\bin\\protoc.exe"),
        (Join-Path $RepoRoot "tools\\protoc\\bin\\protoc.exe")
    )

    foreach ($candidate in $candidates) {
        if (Test-Path $candidate) {
            return (Resolve-Path $candidate).Path
        }
    }

    throw "未找到 protoc。请先安装 protoc，或将其放到 tool/bin 或 tool/protoc/bin。"
}

function Ensure-ProtocGenGo {
    New-Item -ItemType Directory -Force -Path $LocalBinDir | Out-Null
    $plugin = Join-Path $LocalBinDir "protoc-gen-go.exe"
    if (-not (Test-Path $plugin)) {
        Write-Host "安装 protoc-gen-go 到 $LocalBinDir"
        $env:GOBIN = $LocalBinDir
        go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.5
        if ($LASTEXITCODE -ne 0) {
            throw "安装 protoc-gen-go 失败。"
        }
    }
    return $plugin
}

function New-LubanStage {
    Remove-Item -Recurse -Force $StageRoot -ErrorAction SilentlyContinue
    New-Item -ItemType Directory -Force -Path $StageRoot | Out-Null
    New-Item -ItemType Directory -Force -Path $StageProtoDir | Out-Null
    New-Item -ItemType Directory -Force -Path $StageJsonDir | Out-Null

    $enumFile = Join-Path $ExcelDir "enum.xlsx"
    if (-not (Test-Path $enumFile)) {
        throw "未找到枚举定义文件: $enumFile"
    }
    Copy-Item $enumFile (Join-Path $StageRoot "enum.xlsx") -Force

    Get-ChildItem $ExcelDir -Filter *.xlsx | ForEach-Object {
        if ($_.Name -in @("enum.xlsx", "__tables__.xlsx", "__beans__.xlsx", "__enums__.xlsx")) {
            return
        }

        $targetName = if ($_.Name.StartsWith("#")) { $_.Name } else { "#" + $_.Name }
        Copy-Item $_.FullName (Join-Path $StageRoot $targetName) -Force
    }

    @'
{
  "groups": [
    { "names": ["s"], "default": true }
  ],
  "schemaFiles": [
    { "fileName": "enum.xlsx", "type": "enum" }
  ],
  "dataDir": ".",
  "targets": [
    { "name": "server", "manager": "Tables", "groups": ["s"], "topModule": "cfg" }
  ],
  "xargs": []
}
'@ | Set-Content -Path $StageConfFile -Encoding UTF8
}

function Sync-GeneratedFiles {
    $generatedProto = Join-Path $StageProtoDir "schema.proto"
    $generatedPbGo = Join-Path $StageProtoDir "schema.pb.go"

    if (-not (Test-Path $generatedProto)) {
        throw "未找到生成的 schema.proto: $generatedProto"
    }
    if (-not (Test-Path $generatedPbGo)) {
        throw "未找到生成的 schema.pb.go: $generatedPbGo"
    }

    Get-ChildItem $ProtoOutDir -Filter "schema*.proto" -ErrorAction SilentlyContinue | Remove-Item -Force
    Get-ChildItem $ProtoOutDir -Filter "schema*.pb.go" -ErrorAction SilentlyContinue | Remove-Item -Force
    Copy-Item $generatedProto (Join-Path $ProtoOutDir "schema.proto") -Force
    Copy-Item $generatedPbGo (Join-Path $ProtoOutDir "schema.pb.go") -Force

    Get-ChildItem $JsonOutDir -Filter *.json -ErrorAction SilentlyContinue | Remove-Item -Force
    Get-ChildItem $StageJsonDir -Filter *.json -ErrorAction SilentlyContinue | ForEach-Object {
        Copy-Item $_.FullName (Join-Path $JsonOutDir $_.Name) -Force
    }
}

New-Item -ItemType Directory -Force -Path $ProtoOutDir | Out-Null
New-Item -ItemType Directory -Force -Path $JsonOutDir | Out-Null
New-LubanStage

$lubanDll = Resolve-LubanDll
$protoc = Resolve-Protoc
$protocGenGo = Ensure-ProtocGenGo
$stageSchemaFile = Join-Path $StageProtoDir "schema.proto"

Write-Host "使用 Luban: $lubanDll"
Write-Host "Proto 输出: $ProtoOutDir"
Write-Host "JSON 输出: $JsonOutDir"

$env:DOTNET_ROLL_FORWARD = "Major"

dotnet $lubanDll `
    -t server `
    -c protobuf3 `
    -d protobuf3-json `
    --conf $StageConfFile `
    -x outputCodeDir=$StageProtoDir `
    -x outputDataDir=$StageJsonDir

if ($LASTEXITCODE -ne 0) {
    throw "Luban 导出失败。"
}

if (-not (Test-Path $stageSchemaFile)) {
    throw "Luban 未生成 schema.proto: $stageSchemaFile"
}

$env:PATH = "$LocalBinDir;$env:PATH"

& $protoc `
    "--proto_path=$StageProtoDir" `
    "--go_out=paths=source_relative:$StageProtoDir" `
    "--go_opt=Mschema.proto=backend/src/proto" `
    $stageSchemaFile

if ($LASTEXITCODE -ne 0) {
    throw "protoc 生成 pb.go 失败。"
}

Sync-GeneratedFiles

Write-Host ""
Write-Host "生成完成:"
Write-Host "  - $(Join-Path $ProtoOutDir 'schema.proto')"
Write-Host "  - $(Join-Path $ProtoOutDir 'schema.pb.go')"
Write-Host "  - $JsonOutDir"
