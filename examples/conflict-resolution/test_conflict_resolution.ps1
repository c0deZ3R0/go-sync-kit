# Test script for conflict resolution scenarios
param(
    [int]$ServerPort = 8080,
    [int]$NumClients = 3,
    [int]$TestDuration = 30  # Duration in seconds
)

function Start-Server {
    param($Port)
    $serverJob = Start-Job -ScriptBlock {
        param($Port)
        Set-Location $using:PWD
        go run . -mode server -port $Port
    } -ArgumentList $Port
    
    # Wait for server to start and verify it's running
    $maxRetries = 10
    $retryCount = 0
    $serverReady = $false
    
    while (-not $serverReady -and $retryCount -lt $maxRetries) {
        try {
            Invoke-RestMethod -Uri "http://localhost:$Port/debug" -Method Get -ErrorAction Stop
            $serverReady = $true
        }
        catch {
            Start-Sleep -Seconds 1
            $retryCount++
        }
    }
    
    if (-not $serverReady) {
        throw "Server failed to start after $maxRetries attempts"
    } else {
        Write-Host "Server is ready"
    }
    return $serverJob
}

function Start-Client {
    param($Id, $Port)
    $clientPort = $Port + $Id
    $clientJob = Start-Job -ScriptBlock {
        param($Id, $ClientPort)
        Set-Location $using:PWD
        go run . -mode client -id "client$Id" -port $ClientPort
    } -ArgumentList $Id, $clientPort
    
    # Wait for client to start and verify it's running
    $maxRetries = 20  # Increased from 10
    $retryCount = 0
    $clientReady = $false
    
    Write-Host "Waiting for client on port $ClientPort to start..."
    
    while (-not $clientReady -and $retryCount -lt $maxRetries) {
        try {
            $response = Invoke-RestMethod -Uri "http://localhost:$ClientPort/counters" -Method Get -ErrorAction Stop
            Write-Host "Client on port $ClientPort is ready"
            $clientReady = $true
        }
        catch {
Write-Host ("Attempt {0}/{1}: Waiting for client on port {2}..." -f ($retryCount + 1), $maxRetries, $ClientPort)
            Start-Sleep -Seconds 2  # Increased from 1
            $retryCount++
        }
    }
    
    if (-not $clientReady) {
        throw "Client on port $ClientPort failed to start after $maxRetries attempts"
    }
    return $clientJob
}

function Test-CounterOperations {
    param(
        $ClientPorts,
        $CounterId,
        $NumOperations,
        $BatchSize = 5  # Number of operations before forcing sync
    )
    
    $total = 0
    
    for ($batch = 0; $batch -lt [Math]::Ceiling($NumOperations / $BatchSize); $batch++) {
        $batchStart = $batch * $BatchSize
        $batchEnd = [Math]::Min(($batch + 1) * $BatchSize, $NumOperations)
        $batchOps = $batchEnd - $batchStart
        
        Write-Host "`nStarting batch $($batch + 1) ($batchOps operations)"
        
        for ($i = 1; $i -le $batchOps; $i++) {
            # Randomly select a client
            $clientPort = Get-Random -InputObject $ClientPorts
            $value = Get-Random -Minimum 1 -Maximum 5  # Smaller increments for better tracking
            
            # Send increment request
            $body = @{
                id = $CounterId
                value = $value
            } | ConvertTo-Json
            
            $success = $false
            $retries = 0
            $maxRetries = 3
            
            while (-not $success -and $retries -lt $maxRetries) {
                try {
                    Invoke-RestMethod -Uri "http://localhost:$clientPort/counter/increment" -Method Post -Body $body -ContentType "application/json"
                    Write-Host "Client on port $clientPort incremented counter $CounterId by $value"
                    $total += $value
                    $success = $true
                }
                catch {
                    $retries++
                    Write-Warning ("Attempt $retries failed for port {0}: {1}" -f $clientPort, $_.Exception.Message)
                    Start-Sleep -Seconds 1
                }
            }
            
            if (-not $success) {
                Write-Warning "Failed to increment counter after $maxRetries attempts"
            }
            
            # Consistent delay between operations
            Start-Sleep -Milliseconds 500
        }
        
        # Force sync after each batch
        Write-Host "`nForcing sync after batch $($batch + 1)..."
        foreach ($port in $ClientPorts) {
            try {
                Invoke-RestMethod -Uri "http://localhost:$port/sync" -Method Post
                Write-Host "Forced sync on client port $port"
            }
            catch {
                Write-Warning ("Failed to force sync on port {0}: {1}" -f $port, $_.Exception.Message)
            }
        }
        
        # Wait for sync to complete
        Start-Sleep -Seconds 5
        
        # Verify consistency after each batch
        $values = Get-CounterValues -ClientPorts $ClientPorts -CounterId $CounterId
        $uniqueValues = $values.Values | Select-Object -Unique
        
        if ($uniqueValues.Count -eq 1) {
            Write-Host "Batch $($batch + 1) completed successfully. All clients at value: $($uniqueValues[0])" -ForegroundColor Green
        }
        else {
            Write-Host "Batch $($batch + 1) resulted in inconsistent values:" -ForegroundColor Yellow
            $values | Format-Table
        }
    }
    
    Write-Host "`nTotal expected increment: $total"
    return $total
}

function Get-CounterValues {
    param($ClientPorts, $CounterId)
    
    $values = @{}
    foreach ($port in $ClientPorts) {
        try {
            $result = Invoke-RestMethod -Uri "http://localhost:$port/counter/$CounterId" -Method Get
            $values[$port] = $result.value
            Write-Host "Client on port $port has value: $($result.value)"
        }
        catch {
            Write-Warning ("Failed to get counter value from port {0}: {1}" -f $port, $_.Exception.Message)
        }
    }
    return $values
}

# Main test execution
Write-Host "Starting conflict resolution test..."

# Start server
$serverJob = Start-Server -Port $ServerPort
Write-Host "Server started on port $ServerPort"

# Start clients
$clientJobs = @()
$clientPorts = @()
for ($i = 1; $i -le $NumClients; $i++) {
    $clientPort = $ServerPort + $i
    $clientJobs += Start-Client -Id $i -Port $ServerPort
    $clientPorts += $clientPort
    Write-Host "Client $i started on port $clientPort"
}

# Wait for everything to initialize
Start-Sleep -Seconds 5

# Create test counter
$testCounterId = "test_counter_$(Get-Random)"
$body = @{
    id = $testCounterId
} | ConvertTo-Json

Invoke-RestMethod -Uri "http://localhost:$($clientPorts[0])/counter/create" -Method Post -Body $body -ContentType "application/json"
Write-Host "Created test counter: $testCounterId"

# Run concurrent operations
Write-Host "Running concurrent operations..."
$expectedTotal = Test-CounterOperations -ClientPorts $clientPorts -CounterId $testCounterId -NumOperations 20 -BatchSize 5

# Final sync
Write-Host "`nPerforming final sync..."
foreach ($port in $clientPorts) {
    try {
        Invoke-RestMethod -Uri "http://localhost:$port/sync" -Method Post
        Write-Host "Final sync on client port $port"
    }
    catch {
        Write-Warning ("Failed final sync on port {0}: {1}" -f $port, $_.Exception.Message)
    }
}

# Wait for final sync to complete
Write-Host "Waiting for final sync to complete..."
Start-Sleep -Seconds 10

# Check final values
Write-Host "`nChecking final counter values..."
$finalValues = Get-CounterValues -ClientPorts $clientPorts -CounterId $testCounterId

# Verify consistency
$uniqueValues = $finalValues.Values | Select-Object -Unique
if ($uniqueValues.Count -eq 1) {
    Write-Host "`nSUCCESS: All clients have consistent value: $($uniqueValues[0])" -ForegroundColor Green
}
else {
    Write-Host "`nFAILURE: Clients have inconsistent values!" -ForegroundColor Red
    $finalValues | Format-Table -AutoSize
}

# Cleanup
Write-Host "`nCleaning up..."
foreach ($job in $clientJobs) {
    if ($job.State -eq 'Running') {
        Stop-Job -Job $job
    }
    Remove-Job -Job $job -Force
}

if ($serverJob.State -eq 'Running') {
    Stop-Job -Job $serverJob
}
Remove-Job -Job $serverJob -Force

Write-Host "Test completed!"
