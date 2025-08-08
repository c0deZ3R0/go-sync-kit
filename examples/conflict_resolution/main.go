package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/c0deZ3R0/go-sync-kit/examples/conflict_resolution/server"
)

func main() {
	// Parse command line flags
	mode := flag.String("mode", "", "Mode to run: server, client, or dashboard")
	clientID := flag.String("id", "", "Client ID (required for client mode)")
	port := flag.Int("port", 8080, "Port to listen on")
	flag.Parse()

	if *mode == "" {
		fmt.Println("Mode is required. Use -mode [server|client|dashboard]")
		os.Exit(1)
	}

	ctx := context.Background()
	logger := log.New(os.Stdout, fmt.Sprintf("[%s] ", *mode), log.LstdFlags)

	switch *mode {
	case "server":
		runServer(ctx, *port, logger)
	case "client":
		if *clientID == "" {
			fmt.Println("Client ID is required for client mode. Use -id [client_id]")
			os.Exit(1)
		}
		runClient(ctx, *clientID, *port, logger)
	case "dashboard":
		runDashboard(ctx, *port, logger)
	default:
		fmt.Printf("Unknown mode: %s\n", *mode)
		os.Exit(1)
	}
}

func runServer(ctx context.Context, port int, logger *log.Logger) {
	// Create server config
	config := server.Config{
		Port:   port,
		DBPath: filepath.Join("data", "server.db"),
		Logger: logger,
		UseWAL: true,
	}

	// Create and start server
	srv, err := server.New(config)
	if err != nil {
		logger.Fatalf("Failed to create server: %v", err)
	}

	// Start server and wait for shutdown
	if err := srv.Start(ctx); err != nil {
		logger.Fatalf("Server error: %v", err)
	}
}

func runClient(ctx context.Context, clientID string, port int, logger *log.Logger) {
	logger.Printf("Starting client %s on port %d...", clientID, port)
	// TODO: Implement client
}

func runDashboard(ctx context.Context, port int, logger *log.Logger) {
	logger.Printf("Starting dashboard on port %d...", port)
	// TODO: Implement dashboard
}
