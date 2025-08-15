package server

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	"github.com/c0deZ3R0/go-sync-kit/storage/sqlite"
	"github.com/c0deZ3R0/go-sync-kit/synckit"
	"github.com/c0deZ3R0/go-sync-kit/transport/httptransport"
)

// Server represents the counter synchronization server
type Server struct {
	store      synckit.EventStore
	handler    *httptransport.SyncHandler
	httpServer *http.Server
	logger     *log.Logger
}

// Config holds server configuration
type Config struct {
	Port   int
	DBPath string
	Logger *log.Logger
	UseWAL bool
}

// New creates a new server instance
func New(config Config) (*Server, error) {
	if config.Logger == nil {
		config.Logger = log.New(os.Stdout, "[Server] ", log.LstdFlags)
	}

	// Ensure the database directory exists
	if err := os.MkdirAll(filepath.Dir(config.DBPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	// Create SQLite store
	storeConfig := &sqlite.Config{
		DataSourceName: fmt.Sprintf("file:%s", config.DBPath),
		EnableWAL:      config.UseWAL,
		Logger:         config.Logger,
	}

	store, err := sqlite.New(storeConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create SQLite store: %w", err)
	}

	// Create HTTP handler for sync
	// Use default version parser (store.ParseVersion) and convert log.Logger to slog.Logger
	slogger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	handler := httptransport.NewSyncHandler(store, slogger, nil, nil)

	// Wrap with a mux to add /debug endpoint expected by tests
	mux := http.NewServeMux()
	mux.Handle("/sync", handler)
	mux.HandleFunc("/debug", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	// Create HTTP server
	httpServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", config.Port),
		Handler: mux,
	}

	return &Server{
		store:      store,
		handler:    handler,
		httpServer: httpServer,
		logger:     config.Logger,
	}, nil
}

// Start starts the server
func (s *Server) Start(ctx context.Context) error {
	// Log server information
	s.logger.Printf("Starting server on %s", s.httpServer.Addr)
	s.logger.Printf("Using SQLite store")

	// Create channels for server status
	errChan := make(chan error, 1)

	// Start server in a goroutine
	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- fmt.Errorf("server error: %w", err)
		}
	}()

	// Wait for context cancellation or server error
	select {
	case <-ctx.Done():
		return s.Stop(context.Background())
	case err := <-errChan:
		return err
	}
}

// Stop gracefully stops the server
func (s *Server) Stop(ctx context.Context) error {
	s.logger.Println("Stopping server...")

	// Shutdown HTTP server
	if err := s.httpServer.Shutdown(ctx); err != nil {
		s.logger.Printf("Error shutting down HTTP server: %v", err)
		return err
	}

	// Close store
	if err := s.store.Close(); err != nil {
		s.logger.Printf("Error closing store: %v", err)
		return err
	}

	s.logger.Println("Server stopped")
	return nil
}

// GetStore returns the event store
func (s *Server) GetStore() synckit.EventStore {
	return s.store
}
