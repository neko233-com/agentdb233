param([string]$Version = "latest")
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
& (Join-Path $ScriptDir "install-server.ps1") -Version $Version
