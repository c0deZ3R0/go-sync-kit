@echo off
REM Start the server in a new window
start "Counter Server" cmd /k "go run . -mode server"

REM Wait a moment for the server to start
timeout /t 2

REM Start two clients in separate windows
start "Counter Client 1" cmd /k "go run . -mode client -id client1"
start "Counter Client 2" cmd /k "go run . -mode client -id client2"
