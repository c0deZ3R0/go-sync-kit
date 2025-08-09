#!/bin/bash

# Clear any existing data
rm -f data/*.db

# Function to start a new terminal with a command
start_component() {
    local title=$1
    local command=$2
    
    if command -v gnome-terminal &> /dev/null; then
        gnome-terminal --title="$title" -- bash -c "echo 'Starting $title...'; $command; exec bash"
    elif command -v osascript &> /dev/null; then
        # macOS
        osascript -e "tell app \"Terminal\" to do script \"echo 'Starting $title...'; $command\""
    else
        # Fallback
        xterm -T "$title" -e "echo 'Starting $title...'; $command; exec bash" &
    fi
}

# Start server
start_component "Server" "go run main.go -mode server -port 8080"

# Wait a moment for server to initialize
sleep 2

# Start client1
start_component "Client 1" "go run main.go -mode client -id client1 -port 8081"

# Start client2
start_component "Client 2" "go run main.go -mode client -id client2 -port 8082"

# Show available test commands
echo -e "\nTest Commands:" 
echo -e "\033[33m1. Create Counter:\033[0m"
echo "curl -X POST http://localhost:8081/counter/create -H 'Content-Type: application/json' -d '{\"id\":\"counter1\"}'"
echo ""

echo -e "\033[33m2. Increment Counter:\033[0m"
echo "curl -X POST http://localhost:8081/counter/increment -H 'Content-Type: application/json' -d '{\"id\":\"counter1\",\"value\":5}'"
echo ""

echo -e "\033[33m3. Get Counter Value:\033[0m"
echo "curl http://localhost:8081/counter/counter1"
echo ""

echo -e "\033[33m4. Force Sync:\033[0m"
echo "curl -X POST http://localhost:8081/sync"
echo ""

echo -e "\033[33m5. List All Counters:\033[0m"
echo "curl http://localhost:8081/counters"
echo ""

echo -e "\033[90mNote: Replace 8081 with 8082 to test client2\033[0m"
