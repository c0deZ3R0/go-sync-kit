# RabbitMQ Integration Test Runner for Windows PowerShell
# This script sets up RabbitMQ via Docker Compose and runs integration tests

param(
    [Parameter(Position=0)]
    [ValidateSet("test", "test-local", "start", "stop", "cleanup", "logs", "shell", "help")]
    [string]$Command = "test"
)

$ErrorActionPreference = "Stop"

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$ProjectRoot = Resolve-Path "$ScriptDir\..\.."
$ComposeFile = Join-Path $ScriptDir "docker-compose.test.yml"

# Colors for output (using Write-Host with colors)
function Write-Log {
    param([string]$Message)
    Write-Host "[$(Get-Date -Format 'yyyy-MM-dd HH:mm:ss')] $Message" -ForegroundColor Blue
}

function Write-Success {
    param([string]$Message)
    Write-Host "✓ $Message" -ForegroundColor Green
}

function Write-Error {
    param([string]$Message)
    Write-Host "✗ $Message" -ForegroundColor Red
}

function Write-Warning {
    param([string]$Message)
    Write-Host "⚠ $Message" -ForegroundColor Yellow
}

# Function to check if Docker is running
function Test-Docker {
    try {
        docker info | Out-Null
        Write-Success "Docker is running"
        return $true
    }
    catch {
        Write-Error "Docker is not running. Please start Docker and try again."
        exit 1
    }
}

# Function to check if docker-compose is available
function Get-ComposeCommand {
    if (Get-Command "docker-compose" -ErrorAction SilentlyContinue) {
        return "docker-compose"
    }
    elseif ((docker compose version 2>$null) -and ($LASTEXITCODE -eq 0)) {
        return "docker compose"
    }
    else {
        Write-Error "Neither 'docker-compose' nor 'docker compose' is available"
        exit 1
    }
}

# Function to start RabbitMQ services
function Start-Services {
    param([string]$ComposeCmd)
    
    Write-Log "Starting RabbitMQ services..."
    & $ComposeCmd.Split() -f $ComposeFile up -d rabbitmq
    
    Write-Log "Waiting for RabbitMQ to be healthy..."
    $maxAttempts = 60
    $attempt = 1
    
    while ($attempt -le $maxAttempts) {
        $status = & $ComposeCmd.Split() -f $ComposeFile ps rabbitmq
        if ($status -match "healthy") {
            Write-Success "RabbitMQ is healthy"
            return
        }
        
        if (($attempt % 10) -eq 0) {
            Write-Log "Still waiting for RabbitMQ... (attempt $attempt/$maxAttempts)"
        }
        
        Start-Sleep -Seconds 2
        $attempt++
    }
    
    Write-Error "RabbitMQ failed to become healthy within $($maxAttempts * 2) seconds"
    Show-RabbitMQLogs -ComposeCmd $ComposeCmd
    throw "RabbitMQ startup failed"
}

# Function to show RabbitMQ logs
function Show-RabbitMQLogs {
    param([string]$ComposeCmd)
    Write-Log "RabbitMQ logs:"
    & $ComposeCmd.Split() -f $ComposeFile logs rabbitmq
}

# Function to run tests
function Invoke-Tests {
    param([string]$ComposeCmd)
    
    Write-Log "Running integration tests..."
    
    # Run tests in the container with proper environment
    & $ComposeCmd.Split() -f $ComposeFile run --rm test-runner sh -c @"
cd /app && \
go mod download && \
go test -v -tags=integration ./transport/rabbitmq -run TestIntegration
"@
}

# Function to run tests locally
function Invoke-LocalTests {
    Write-Log "Running integration tests locally..."
    
    $env:RABBITMQ_URL = "amqp://synckit_user:synckit_pass@localhost:5672/"
    
    Push-Location $ProjectRoot
    try {
        go test -v -tags=integration ./transport/rabbitmq -run TestIntegration
    }
    finally {
        Pop-Location
    }
}

# Function to stop services
function Stop-Services {
    param([string]$ComposeCmd)
    
    Write-Log "Stopping services..."
    & $ComposeCmd.Split() -f $ComposeFile down
    Write-Success "Services stopped"
}

# Function to clean up everything
function Remove-Everything {
    param([string]$ComposeCmd)
    
    Write-Log "Cleaning up..."
    & $ComposeCmd.Split() -f $ComposeFile down -v --remove-orphans
    Write-Success "Cleanup complete"
}

# Function to show management UI info
function Show-ManagementInfo {
    Write-Host ""
    Write-Log "RabbitMQ Management UI is available at:"
    Write-Host "  URL: http://localhost:15672"
    Write-Host "  Username: synckit_user"
    Write-Host "  Password: synckit_pass"
    Write-Host ""
}

# Function to show usage
function Show-Usage {
    Write-Host "Usage: .\test-integration.ps1 [COMMAND]"
    Write-Host ""
    Write-Host "Commands:"
    Write-Host "  test        Start services and run integration tests (default)"
    Write-Host "  test-local  Run tests against locally running RabbitMQ"
    Write-Host "  start       Start RabbitMQ services only"
    Write-Host "  stop        Stop services"
    Write-Host "  cleanup     Stop services and remove volumes"
    Write-Host "  logs        Show RabbitMQ logs"
    Write-Host "  shell       Open shell in test container"
    Write-Host "  help        Show this help"
    Write-Host ""
}

# Main execution
switch ($Command) {
    "test" {
        Test-Docker | Out-Null
        $composeCmd = Get-ComposeCommand
        Write-Success "Using $composeCmd"
        
        try {
            Start-Services -ComposeCmd $composeCmd
            Show-ManagementInfo
            Invoke-Tests -ComposeCmd $composeCmd
            Write-Success "Integration tests completed successfully"
        }
        finally {
            Stop-Services -ComposeCmd $composeCmd
        }
    }
    
    "test-local" {
        Invoke-LocalTests
    }
    
    "start" {
        Test-Docker | Out-Null
        $composeCmd = Get-ComposeCommand
        Write-Success "Using $composeCmd"
        Start-Services -ComposeCmd $composeCmd
        Show-ManagementInfo
        Write-Log "RabbitMQ is running. Use '.\test-integration.ps1 stop' to stop services."
    }
    
    "stop" {
        $composeCmd = Get-ComposeCommand
        Stop-Services -ComposeCmd $composeCmd
    }
    
    "cleanup" {
        $composeCmd = Get-ComposeCommand
        Remove-Everything -ComposeCmd $composeCmd
    }
    
    "logs" {
        $composeCmd = Get-ComposeCommand
        Show-RabbitMQLogs -ComposeCmd $composeCmd
    }
    
    "shell" {
        $composeCmd = Get-ComposeCommand
        Write-Log "Opening shell in test container..."
        & $composeCmd.Split() -f $ComposeFile run --rm test-runner sh
    }
    
    "help" {
        Show-Usage
    }
    
    default {
        Write-Error "Unknown command: $Command"
        Show-Usage
        exit 1
    }
}
