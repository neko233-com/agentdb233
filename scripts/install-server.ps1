# agentdb233-server installer (Windows PowerShell)
# iwr -useb https://raw.githubusercontent.com/neko233-com/agentdb233/main/scripts/install-server.ps1 | iex
# iwr ... | iex; Install-AgentDB233Server -Ver v0.1.0

param(
    [string]$Version = "latest"
)

$ErrorActionPreference = "Stop"
$BinaryName = "agentdb233-server"
$Repo = "neko233-com/agentdb233"

function Get-LatestVersion {
    try {
        $r = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest"
        return ($r.tag_name -replace '^[vV]', '')
    } catch {
        return "0.1.0"
    }
}

function Install-AgentDB233Server {
    param([string]$Ver)

    $arch = "amd64"
    if ($env:PROCESSOR_ARCHITECTURE -eq "ARM64") { $arch = "arm64" }

    $asset = "$BinaryName-windows-$arch.exe"
    $url = "https://github.com/$Repo/releases/download/v$Ver/$asset"
    $installDir = Join-Path $env:LOCALAPPDATA "agentdb233"
    New-Item -ItemType Directory -Force -Path $installDir | Out-Null
    $dest = Join-Path $installDir "$BinaryName.exe"

    Write-Host "Downloading $url ..."
    Invoke-WebRequest -Uri $url -OutFile $dest -UseBasicParsing
    Write-Host "Installed to $dest"
    Write-Host "Add to PATH: $installDir"
    Write-Host "Run: `$env:AGENTDB233_DATA=`"$env:USERPROFILE\\.agentdb233-server`"; agentdb233-server start"
    Write-Host "Status: agentdb233-server status"
    Write-Host "Change port: agentdb233-server set-port 32390"
    Write-Host "Enable boot autostart: agentdb233-server enable-autostart"
    Write-Host "Run MCP stdio: agentdb233-server mcp"
}

if ($Version -eq "latest") {
    $Version = Get-LatestVersion
}
$Version = $Version -replace '^[vV]', ''

Write-Host "Installing agentdb233-server v$Version ..."
Install-AgentDB233Server -Ver $Version
