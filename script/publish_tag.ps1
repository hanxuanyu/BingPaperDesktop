# BingPaperDesktop Tag Publication Script for Windows (PowerShell)
# This script automates the process of creating and pushing a new tag to trigger the release workflow.

param (
    [string]$TagName
)

$ErrorActionPreference = "Stop"

# Change to the project root directory
Set-Location -Path $PSScriptRoot\..

# 1. Update local code
Write-Host "Updating local code..." -ForegroundColor Cyan
git pull origin master

# 2. Check current branch
$CurrentBranch = git rev-parse --abbrev-ref HEAD
if ($CurrentBranch -ne "master") {
    Write-Host "Error: You must be on the 'master' branch to publish a tag." -ForegroundColor Red
    exit 1
}

# 3. Check if local master is up-to-date with remote master
git fetch origin master
$Local = git rev-parse HEAD
$Remote = git rev-parse origin/master

if ($Local -ne $Remote) {
    Write-Host "Error: Local branch is not up-to-date with remote. Please push or pull changes first." -ForegroundColor Red
    exit 1
}

# 4. Get tag name from user
if (-not $TagName) {
    $TagName = Read-Host "Enter tag name (e.g., v1.0.0)"
}

if ($TagName -notlike "v*") {
    Write-Host "Error: Tag name must start with 'v' (e.g., v1.0.0)." -ForegroundColor Red
    exit 1
}

# 5. Create and force push the tag
Write-Host "Creating tag $TagName..." -ForegroundColor Cyan
git tag -f "$TagName"

Write-Host "Pushing tag $TagName to remote..." -ForegroundColor Cyan
git push origin "$TagName" -f

Write-Host "Successfully published tag $TagName. The release workflow should now be triggered." -ForegroundColor Green
