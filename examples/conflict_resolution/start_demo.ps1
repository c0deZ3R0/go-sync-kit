# Kill any existing processes using our ports
Write-Host "Cleaning up ports..." -ForegroundColor Yellow
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
Remove-Item -Path "data\*.db" -Force -ErrorAction SilentlyContinue

# Function to start a new terminal with a command
function Start-DemoComponent {
    param (
        [string]$Title,
        [string]$Command
    )
    
    Start-Process pwsh -ArgumentList "-NoExit", "-Command", "Write-Host 'Starting $Title...' -ForegroundColor Green; $Command"
}

Write-Host "`nStarting components..." -ForegroundColor Cyan

# Start server
Write-Host "Starting server..." -ForegroundColor Yellow
Start-DemoComponent -Title "Server" -Command "go run main.go -mode server -port 8080 2>&1 | Tee-Object -FilePath .\logs\server.log"

# Wait for server to initialize
Write-Host "Waiting for server to initialize..." -ForegroundColor Gray
Start-Sleep -Seconds 3

# Start client1
Write-Host "Starting client 1..." -ForegroundColor Yellow
Start-DemoComponent -Title "Client 1" -Command "go run main.go -mode client -id client1 -port 8081 2>&1 | Tee-Object -FilePath .\logs\client1.log"
Start-Sleep -Seconds 2

# Start client2
Write-Host "Starting client 2..." -ForegroundColor Yellow
Start-DemoComponent -Title "Client 2" -Command "go run main.go -mode client -id client2 -port 8082 2>&1 | Tee-Object -FilePath .\logs\client2.log"
Start-Sleep -Seconds 2

# Show available test commands
Write-Host "`nTest Commands:" -ForegroundColor Cyan

Write-Host "1. Create Counter:" -ForegroundColor Yellow
Write-Host @'
Invoke-WebRequest -Method POST -Uri "http://localhost:8081/counter/create" `
    -Headers @{"Content-Type"="application/json"} `
    -Body '{"id":"counter1"}' | Select-Object -Expand Content
'@ 
Write-Host ""

Write-Host "2. Increment Counter:" -ForegroundColor Yellow
Write-Host @'
Invoke-WebRequest -Method POST -Uri "http://localhost:8081/counter/increment" `
    -Headers @{"Content-Type"="application/json"} `
    -Body '{"id":"counter1","value":5}' | Select-Object -Expand Content
'@ 
Write-Host ""

Write-Host "3. Get Counter Value:" -ForegroundColor Yellow
Write-Host @'
Invoke-WebRequest -Uri "http://localhost:8081/counter/counter1" | Select-Object -Expand Content
'@ 
Write-Host ""

Write-Host "4. Force Sync:" -ForegroundColor Yellow
Write-Host @'
Invoke-WebRequest -Method POST -Uri "http://localhost:8081/sync" | Select-Object -Expand Content
'@ 
Write-Host ""

Write-Host "5. List All Counters:" -ForegroundColor Yellow
Write-Host @'
Invoke-WebRequest -Uri "http://localhost:8081/counters" | Select-Object -Expand Content
'@ 
Write-Host ""

Write-Host "Note: Replace 8081 with 8082 to test client2" -ForegroundColor Gray
Write-Host "Tip: Commands are formatted for PowerShell. For cmd.exe or bash, use curl instead." -ForegroundColor Gray
