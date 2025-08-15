#!/usr/bin/env pwsh

# Synckit Dynamic Resolver Showcase Runner
# This script makes it easy to run the dynamic resolver showcase

Write-Host "🚀 Starting Synckit Dynamic Resolver Showcase..." -ForegroundColor Cyan
Write-Host "═══════════════════════════════════════════════════" -ForegroundColor Blue

# Check if Go is installed
if (-not (Get-Command "go" -ErrorAction SilentlyContinue)) {
    Write-Host "❌ Go is not installed or not in PATH" -ForegroundColor Red
    Write-Host "Please install Go 1.21 or later from https://golang.org/dl/" -ForegroundColor Yellow
    exit 1
}

# Check Go version
$goVersion = go version
Write-Host "✅ Found: $goVersion" -ForegroundColor Green

# Check if we're in the right directory
if (-not (Test-Path "simple_showcase.go")) {
    Write-Host "❌ simple_showcase.go not found in current directory" -ForegroundColor Red
    Write-Host "Please run this script from the dynamic-resolver-showcase directory" -ForegroundColor Yellow
    exit 1
}

# Ensure dependencies are available
Write-Host "📦 Checking dependencies..." -ForegroundColor Yellow
go mod tidy

if ($LASTEXITCODE -ne 0) {
    Write-Host "❌ Failed to resolve dependencies" -ForegroundColor Red
    exit 1
}

Write-Host "✅ Dependencies ready" -ForegroundColor Green

# Run the showcase
Write-Host "🎯 Launching Dynamic Resolver Showcase..." -ForegroundColor Cyan
Write-Host ""

go run simple_showcase.go

if ($LASTEXITCODE -ne 0) {
    Write-Host "❌ Showcase failed to run" -ForegroundColor Red
    exit 1
}

Write-Host ""
Write-Host "🎉 Showcase completed successfully!" -ForegroundColor Green
Write-Host "Thanks for exploring the go-sync-kit dynamic resolver!" -ForegroundColor Cyan
