// HTTP Server Example for Go Sync Kit (Production Version)
// Demonstrates creating a sync server with graceful shutdown and signal handling
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/c0deZ3R0/go-sync-kit/storage/sqlite"
	"github.com/c0deZ3R0/go-sync-kit/synckit"
	"github.com/c0deZ3R0/go-sync-kit/transport/httptransport"
)

func main() {
	log.Printf("=== Go Sync Kit HTTP Server (Production) ===")

	// Event store (SQLite here, could be Postgres in prod)
	store, err := sqlite.New(&sqlite.Config{DataSourceName: "server.db"})
	if err != nil {
		log.Fatalf("failed to create store: %v", err)
	}
	defer func() {
		log.Printf("Closing store...")
		store.Close()
	}()

	// HTTP transport in server mode
	transport := httptransport.NewTransport("", nil, nil, nil)

	// SyncNode configured as server
	node, err := synckit.NewHTTPServerNode(store, transport)
	if err != nil {
		log.Fatalf("failed to create server node: %v", err)
	}
	defer func() {
		log.Printf("Closing sync node...")
		node.Close()
	}()

	// Expose sync handler
	handler := httptransport.NewSyncHandler(store, nil, nil, nil)
	http.Handle("/sync", handler)

	// Create HTTP server
	server := &http.Server{
		Addr:         ":8080",
		Handler:      nil, // use default mux
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		log.Printf("✅ Sync server listening on :8080")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server listen error: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Printf("🛑 Shutting down server...")

	// Create shutdown context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Gracefully shutdown the server
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("server forced to shutdown: %v", err)
	} else {
		log.Printf("✅ Server shutdown gracefully")
	}
}