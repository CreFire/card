param(
    [string]$Root = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
)

$ErrorActionPreference = "Stop"

function Get-AgentDirs {
    param([string]$CodexDir)

    $agentsDir = Join-Path $CodexDir "agents"
    if (-not (Test-Path $agentsDir)) {
        return @()
    }

    return Get-ChildItem -Path $agentsDir -Directory | Sort-Object Name
}

function Read-FrontmatterName {
    param([string]$SkillPath)

    if (-not (Test-Path $SkillPath)) {
        return $null
    }

    $lines = Get-Content $SkillPath
    foreach ($line in $lines) {
        if ($line -match '^name:\s*(.+?)\s*$') {
            return $matches[1].Trim()
        }
    }
    return $null
}

$codexDir = Join-Path $Root "codex"
if (-not (Test-Path $codexDir)) {
    throw "codex directory not found: $codexDir"
}

$skillsDir = Join-Path $codexDir "skills"
New-Item -ItemType Directory -Force $skillsDir | Out-Null

$agents = Get-AgentDirs -CodexDir $codexDir
$generatedAt = Get-Date -Format "yyyy-MM-dd HH:mm:ss zzz"
$report = New-Object System.Collections.Generic.List[string]

$report.Add("# Codex Sync Report")
$report.Add("")
$report.Add("- generated_at: $generatedAt")
$report.Add("- root: $Root")
$report.Add("")
$report.Add("## Agents")
$report.Add("")

foreach ($agent in $agents) {
    $agentName = $agent.Name
    $summaryPath = Join-Path $agent.FullName "memory\framework-summary.md"
    $skillPath = Join-Path $skillsDir "$agentName\SKILL.md"
    $skillName = Read-FrontmatterName -SkillPath $skillPath

    $summaryState = if (Test-Path $summaryPath) { "ok" } else { "missing" }
    $skillState = if (Test-Path $skillPath) { "ok" } else { "missing" }
    $skillNameText = if ($skillName) { $skillName } else { "-" }

    $report.Add("### $agentName")
    $report.Add("")
    $report.Add("- framework-summary: $summaryState")
    $report.Add("- skill: $skillState")
    $report.Add("- skill_name: $skillNameText")
    $report.Add("- agent_dir: $($agent.FullName)")
    $report.Add("")
}

$reportPath = Join-Path $codexDir "SYNC-REPORT.md"
Set-Content -Path $reportPath -Value $report -Encoding UTF8

Write-Host "Wrote sync report: $reportPath"
