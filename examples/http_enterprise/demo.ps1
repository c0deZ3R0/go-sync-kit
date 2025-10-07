# Enterprise HTTP Example Demo Script
# Starts the server and runs the client demonstration

Write-Host "================================================================================" -ForegroundColor Cyan
Write-Host "       Go-Sync-Kit Enterprise HTTP Transport Demo                            " -ForegroundColor Cyan
Write-Host "================================================================================" -ForegroundColor Cyan
Write-Host ""

# Get paths
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$ServerDir = Join-Path $ScriptDir "server"
$ClientDir = Join-Path $ScriptDir "client"
$ServerExe = Join-Path $ServerDir "enterprise-server.exe"

Write-Host "[BUILD] Building binaries..." -ForegroundColor Yellow
Write-Host ""

# Build server
Set-Location $ServerDir
Write-Host "   Building server..." -NoNewline
go build -o enterprise-server.exe 2>&1 | Out-Null
if ($LASTEXITCODE -eq 0) {
    Write-Host " SUCCESS" -ForegroundColor Green
} else {
    Write-Host " FAILED" -ForegroundColor Red
    exit 1
}

# Build client
Set-Location $ClientDir
Write-Host "   Building client..." -NoNewline
go build -o enterprise-client.exe 2>&1 | Out-Null
if ($LASTEXITCODE -eq 0) {
    Write-Host " SUCCESS" -ForegroundColor Green
} else {
    Write-Host " FAILED" -ForegroundColor Red
    exit 1
}

Write-Host ""
Write-Host "[START] Starting enterprise server..." -ForegroundColor Yellow
Write-Host ""

# Start server in background
$ServerProcess = Start-Process -FilePath $ServerExe -WorkingDirectory $ServerDir -PassThru -WindowStyle Hidden

# Wait for server to start
Write-Host "   Waiting for server to be ready..." -NoNewline
Start-Sleep -Seconds 3
Write-Host " READY" -ForegroundColor Green

Write-Host ""
Write-Host "[CLIENT] Running client examples..." -ForegroundColor Yellow
Write-Host ""
Write-Host "================================================================================" -ForegroundColor DarkGray
Write-Host ""

# Run client
Set-Location $ClientDir
& .\enterprise-client.exe
$ClientExitCode = $LASTEXITCODE

Write-Host ""
Write-Host "================================================================================" -ForegroundColor DarkGray
Write-Host ""

# Cleanup
Write-Host "[STOP] Stopping server..." -ForegroundColor Yellow
Stop-Process -Id $ServerProcess.Id -Force -ErrorAction SilentlyContinue
Write-Host "   Server stopped" -ForegroundColor Green

Write-Host ""
if ($ClientExitCode -eq 0) {
    Write-Host "[SUCCESS] Demo completed successfully!" -ForegroundColor Green
} else {
    Write-Host "[WARNING] Demo completed with errors (exit code: $ClientExitCode)" -ForegroundColor Yellow
}

Write-Host ""
Write-Host "TIP: Review the code in server/main.go and client/main.go to see how it works!" -ForegroundColor Cyan
Write-Host ""
