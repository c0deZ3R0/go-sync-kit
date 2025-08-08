package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/c0deZ3R0/go-sync-kit/examples/conflict_resolution/client"
	"github.com/c0deZ3R0/go-sync-kit/storage/sqlite"
	"github.com/c0deZ3R0/go-sync-kit/transport/httptransport"
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
	// Create database directory
	dbPath := filepath.Join("data", "server.db")
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		logger.Fatalf("Failed to create database directory: %v", err)
	}

	// Create SQLite store
	storeConfig := &sqlite.Config{
		DataSourceName: fmt.Sprintf("file:%s", dbPath),
		EnableWAL:     true,
		Logger:        logger,
	}

	store, err := sqlite.New(storeConfig)
	if err != nil {
		logger.Fatalf("Failed to create SQLite store: %v", err)
	}
	defer store.Close()

	// Create HTTP handler with /sync prefix
	mux := http.NewServeMux()

	// Add debug handler
	mux.HandleFunc("/debug", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Debug endpoint working")
	})

	// Create logging middleware
	loggingHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger.Printf("%s %s", r.Method, r.URL.Path)
		mux.ServeHTTP(w, r)
	})

	// Add sync handler
	syncHandler := httptransport.NewSyncHandler(store, logger)
	mux.Handle("/sync/", http.StripPrefix("/sync", syncHandler))

	// Log available endpoints
	logger.Printf("Available endpoints:\n - /debug\n - /sync/latest-version\n - /sync/push\n - /sync/pull")

	// Create HTTP server
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: loggingHandler,
	}

	// Create error channel
	errChan := make(chan error, 1)

	// Start server
	go func() {
		logger.Printf("Server listening on port %d", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- fmt.Errorf("server error: %w", err)
		}
	}()

	// Wait for shutdown
	select {
	case <-ctx.Done():
		logger.Println("Shutting down server...")
		if err := srv.Shutdown(context.Background()); err != nil {
			logger.Printf("Error during shutdown: %v", err)
		}
	case err := <-errChan:
		logger.Fatalf("Server error: %v", err)
	}
}

func runClient(ctx context.Context, clientID string, port int, logger *log.Logger) {
	// Create client config
	config := client.Config{
		ID:         clientID,
		ServerURL:  "http://localhost:8080",
		DBPath:     filepath.Join("data", fmt.Sprintf("client_%s.db", clientID)),
		Logger:     logger,
		SyncPeriod: 5 * time.Second,
	}

	// Create and start client
	c, err := client.New(config)
	if err != nil {
		logger.Fatalf("Failed to create client: %v", err)
	}
	defer c.Close()

	// Start automatic sync
	if err := c.StartSync(ctx); err != nil {
		logger.Fatalf("Failed to start sync: %v", err)
	}

	// Create a counter
	if err := c.CreateCounter(ctx, "counter1"); err != nil {
		logger.Printf("Failed to create counter: %v", err)
	}

	// Start HTTP server for client API
	mux := http.NewServeMux()

	// Add counter creation endpoint
	mux.HandleFunc("/counter/create", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			ID string `json:"id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if err := c.CreateCounter(r.Context(), req.ID); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	// Add increment endpoint
	mux.HandleFunc("/counter/increment", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			ID    string `json:"id"`
			Value int    `json:"value"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if err := c.IncrementCounter(r.Context(), req.ID, req.Value); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	// Add get counter endpoint
	mux.HandleFunc("/counter/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		counterID := strings.TrimPrefix(r.URL.Path, "/counter/")
		value, err := c.GetCounter(counterID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":    counterID,
			"value": value,
		})
	})

	// Add list counters endpoint
	mux.HandleFunc("/counters", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		counters := c.ListCounters()
		json.NewEncoder(w).Encode(counters)
	})

	// Add manual sync endpoint
	mux.HandleFunc("/sync", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if err := c.Sync(r.Context()); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	// Create HTTP server
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	// Start server
	logger.Printf("Client API listening on port %d", port)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Fatalf("Server error: %v", err)
	}
}

func runDashboard(ctx context.Context, port int, logger *log.Logger) {
	logger.Printf("Starting dashboard on port %d...", port)
	// TODO: Implement dashboard
}
