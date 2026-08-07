# 下载并解压 nmap 便携版到 tools/nmap/windows/（Windows 平台）
# 用法: powershell -ExecutionPolicy Bypass -File scripts/setup-nmap.ps1
param(
    [string]$Version = "7.92",
    [string]$Arch = "win32"
)
$ErrorActionPreference = "Stop"
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
$Root = Split-Path -Parent $PSScriptRoot
$Dest = Join-Path $Root "tools\nmap\windows"
$Url = "https://nmap.org/dist/nmap-$Version-$Arch.zip"
$Zip = Join-Path $env:TEMP "nmap-$Version-$Arch.zip"

Write-Host "Downloading $Url ..."
Invoke-WebRequest -Uri $Url -OutFile $Zip
New-Item -ItemType Directory -Force -Path $Dest | Out-Null
Expand-Archive -Path $Zip -DestinationPath $Dest -Force

$Exe = Get-ChildItem -Path $Dest -Recurse -Filter nmap.exe | Select-Object -First 1
if (-not $Exe) {
    Write-Error "nmap.exe not found after extraction"
}
Write-Host "nmap installed at: $($Exe.FullName)"
& $Exe.FullName --version
