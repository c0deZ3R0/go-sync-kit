package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/c0deZ3R0/go-sync-kit/example/client"
	"github.com/c0deZ3R0/go-sync-kit/example/server"
)

func main() {
	// Create context that's cancelled on interrupt
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Create a WaitGroup to manage our goroutines
	var wg sync.WaitGroup

	// Start the server
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := server.RunServer(ctx); err != nil {
			log.Printf("Server error: %v", err)
		}
	}()

	// Give the server a moment to start up
	time.Sleep(time.Second)

	// Start the client
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := client.RunClient(ctx); err != nil {
			log.Printf("Client error: %v", err)
		}
	}()

	// Wait for interrupt signal
	<-sigChan
	log.Println("Received interrupt signal, shutting down...")

	// Cancel context to initiate shutdown
	cancel()

	// Wait for both server and client to shut down
	wg.Wait()
	log.Println("Demo shutdown complete")
}
