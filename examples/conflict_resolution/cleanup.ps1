# Kill any existing go processes
Write-Host "Cleaning up processes..." -ForegroundColor Yellow
Get-Process | Where-Object { $_.ProcessName -eq 'go' } | Stop-Process -Force

# Kill any processes using our ports
$ports = @(8080, 8081, 8082)
foreach ($port in $ports) {
    $process = Get-NetTCPConnection -LocalPort $port -ErrorAction SilentlyContinue | Select-Object -ExpandProperty OwningProcess
    if ($process) {
        Stop-Process -Id $process -Force
        Write-Host "Killed process using port $port" -ForegroundColor Gray
    }
}

# Clear any existing data
Write-Host "Cleaning up database files..." -ForegroundColor Yellow
Remove-Item -Path "data\\*.db" -Force -ErrorAction SilentlyContinue
