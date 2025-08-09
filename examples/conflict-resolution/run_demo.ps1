# Create background jobs for server and clients
$server = Start-Job -ScriptBlock {
    go run . -mode server -port 8080
}

# Give the server a moment to start
Start-Sleep -Seconds 2

# Start three clients
$client1 = Start-Job -ScriptBlock {
    go run . -mode client -id client1 -port 8081
}

$client2 = Start-Job -ScriptBlock {
    go run . -mode client -id client2 -port 8082
}

$client3 = Start-Job -ScriptBlock {
    go run . -mode client -id client3 -port 8083
}

Write-Host "Server and clients started. Press Ctrl+C to stop all processes..."

try {
    # Keep script running and show output from all jobs
    while ($true) {
        Receive-Job -Job $server, $client1, $client2, $client3 | ForEach-Object {
            Write-Host $_
        }
        Start-Sleep -Seconds 1
    }
} finally {
    # Cleanup on script exit
    $server, $client1, $client2, $client3 | Stop-Job
    $server, $client1, $client2, $client3 | Remove-Job
    Write-Host "`nAll processes stopped."
}
