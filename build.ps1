# BingPaperDesktop Build Script for Windows (PowerShell)
# This script builds the application for multiple platforms.

$ErrorActionPreference = "Stop"

$AppName = "BingPaperDesktop"
$OutputDir = "build\bin"

# Check if Wails CLI is installed
if (!(Get-Command wails -ErrorAction SilentlyContinue)) {
    Write-Host "Error: Wails CLI not found. Please install it first:" -ForegroundColor Red
    Write-Host "go install github.com/wailsapp/wails/v2/cmd/wails@latest" -ForegroundColor Red
    exit 1
}

Write-Host "--- Starting build process for $AppName ---" -ForegroundColor Cyan

# Function to clean build directory
function Clean-OutputDir {
    Write-Host "Cleaning build directory..." -ForegroundColor Yellow
    if (Test-Path $OutputDir) {
        Remove-Item -Path $OutputDir -Recurse -Force
    }
    New-Item -ItemType Directory -Path $OutputDir -Force | Out-Null
}

# Function to build for a specific platform
function Build-Platform {
    param(
        [string]$Platform,
        [string]$ExtraArgs = ""
    )
    Write-Host "Building for $Platform..." -ForegroundColor Green
    if ($ExtraArgs) {
        wails build -platform $Platform $ExtraArgs
    } else {
        wails build -platform $Platform
    }
}

# 1. Clean before build
Clean-OutputDir

# 2. Build for Windows (amd64)
Write-Host "Building Windows standalone executable..." -ForegroundColor Green
Build-Platform -Platform "windows/amd64"

# Check if NSIS is installed to generate a Windows installer
if (Get-Command makensis -ErrorAction SilentlyContinue) {
    Write-Host "NSIS detected. Generating Windows installer..." -ForegroundColor Gray
    # Use -s to skip frontend build since it was just built
    Build-Platform -Platform "windows/amd64" -ExtraArgs "-nsis -s"
} else {
    Write-Host "NSIS not found. Skipping Windows installer generation." -ForegroundColor Yellow
}

# 3. Build for Linux (amd64)
Build-Platform -Platform "linux/amd64"

# 4. Build for macOS (Universal)
# Note: Full macOS .app bundle creation is typically only supported on macOS hosts.
Write-Host "Attempting macOS (Darwin) build..." -ForegroundColor Gray
try {
    Build-Platform -Platform "darwin/universal"
} catch {
    Write-Host "Warning: macOS build failed. This is expected if you are not on a macOS host." -ForegroundColor Yellow
}

Write-Host "`n--- Build process completed! ---" -ForegroundColor Cyan
Write-Host "Outputs are located in: $OutputDir"
Get-ChildItem $OutputDir | Select-Object Name, Length, LastWriteTime
