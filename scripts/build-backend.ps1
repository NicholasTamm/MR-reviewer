# Build the MR Reviewer backend server as a standalone binary using PyInstaller.
# Must be run on Windows. Output: frontend\resources\backend\mr-reviewer-server.exe
#
# Usage: powershell -ExecutionPolicy Bypass -File scripts\build-backend.ps1

$ErrorActionPreference = "Stop"

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$RepoRoot = Split-Path -Parent $ScriptDir
$OutputDir = Join-Path $RepoRoot "frontend\resources\backend"

Set-Location $RepoRoot

Write-Host "==> Installing PyInstaller..."
pip install pyinstaller

Write-Host "==> Building backend binary..."
pyinstaller mr-reviewer-server.spec `
    --distpath $OutputDir `
    --workpath "$env:TEMP\mr-reviewer-pyinstaller" `
    --noconfirm

$Binary = Join-Path $OutputDir "mr-reviewer-server.exe"
Write-Host "==> Binary built: $Binary"

Write-Host "==> Verifying binary..."
& $Binary --help
Write-Host "==> Build successful!"
