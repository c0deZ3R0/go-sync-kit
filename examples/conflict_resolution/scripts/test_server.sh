#!/bin/bash

# Build and run server
echo "Building and starting server..."
go run ../main.go -mode server -port 8080 &
SERVER_PID=$!

# Wait for server to start
sleep 2

# Test server health
echo "Testing server health..."
curl -v http://localhost:8080/health

# Send test events
echo "Sending test events..."
curl -X POST http://localhost:8080/push \
  -H "Content-Type: application/json" \
  -d '{
    "id": "test-counter-1",
    "type": "CounterCreated",
    "counterId": "counter1",
    "value": 0,
    "timestamp": "2023-08-08T00:00:00Z",
    "clientId": "test",
    "metadata": {
      "test": true
    }
  }'

# Get latest version
echo "Getting latest version..."
curl http://localhost:8080/latest-version

# Cleanup
kill $SERVER_PID
