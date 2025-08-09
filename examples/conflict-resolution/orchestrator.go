package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/c0deZ3R0/go-sync-kit/examples/conflict_resolution/client"
	"github.com/c0deZ3R0/go-sync-kit/examples/conflict_resolution/server"
)

// TestOrchestrator manages server and clients for testing
type TestOrchestrator struct {
	server     *server.Server
	clients    map[string]*client.Client
	clientsMux sync.RWMutex
	wg         sync.WaitGroup
	ctx        context.Context
	cancel     context.CancelFunc
}

// NewTestOrchestrator creates a new test orchestrator
func NewTestOrchestrator() *TestOrchestrator {
	ctx, cancel := context.WithCancel(context.Background())
	return &TestOrchestrator{
		clients: make(map[string]*client.Client),
		ctx:     ctx,
		cancel:  cancel,
	}
}

// StartServer starts the counter server
func (t *TestOrchestrator) StartServer() error {
config := server.Config{
		Port: 8080,
		DBPath: "./server_data.db",
		Logger: log.New(os.Stdout, "[Server] ", log.LstdFlags),
		UseWAL: true,
	}

	srv, err := server.New(config)
	if err != nil {
		return err
	}
	t.server = srv

	t.wg.Add(1)
	go func() {
		defer t.wg.Done()
		if err := srv.Start(t.ctx); err != nil {
			log.Printf("Server error: %v\n", err)
		}
	}()

	// Wait a moment for server to start
	time.Sleep(time.Second)
	return nil
}

// AddClient adds a new client with the given ID
func (t *TestOrchestrator) AddClient(id string) error {
	t.clientsMux.Lock()
	defer t.clientsMux.Unlock()

	if _, exists := t.clients[id]; exists {
		return fmt.Errorf("client %s already exists", id)
	}

	config := client.Config{
		ID:         id,
		ServerURL:  "http://localhost:8080",
		DBPath:     fmt.Sprintf("./data/%s.db", id),
		SyncPeriod: time.Second,
	}

	c, err := client.New(config)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}
	t.clients[id] = c

	t.wg.Add(1)
	go func() {
		defer t.wg.Done()
		if err := c.Start(); err != nil {
			log.Printf("Client %s error: %v\n", id, err)
		}
	}()

	// Wait a moment for client to initialize
	time.Sleep(time.Second)
	return nil
}

// ExecuteCommand executes a command on the specified client
func (t *TestOrchestrator) ExecuteCommand(clientID, command, counterID string) error {
	t.clientsMux.RLock()
	c, exists := t.clients[clientID]
	t.clientsMux.RUnlock()

	if !exists {
		return fmt.Errorf("client %s not found", clientID)
	}

	switch command {
	case "c":
		return c.CreateCounterSync(counterID)
	case "i":
		return c.IncrementCounterSync(counterID)
	case "d":
		return c.DecrementCounterSync(counterID)
	case "r":
		return c.ResetCounterSync(counterID)
	default:
		return fmt.Errorf("unknown command: %s", command)
	}
}

// GetCounter gets the current value of a counter from a client
func (t *TestOrchestrator) GetCounter(clientID, counterID string) (int, error) {
	t.clientsMux.RLock()
	c, exists := t.clients[clientID]
	t.clientsMux.RUnlock()

	if !exists {
		return 0, fmt.Errorf("client %s not found", clientID)
	}

	return c.GetCounter(counterID)
}

// Stop stops all clients and the server
func (t *TestOrchestrator) Stop() {
	t.cancel()

	// Stop all clients
	t.clientsMux.Lock()
	for _, c := range t.clients {
		c.Stop()
	}
	t.clientsMux.Unlock()

	// Stop server
	if t.server != nil {
		_ = t.server.Stop(t.ctx)
	}

	t.wg.Wait()
}

// SyncAll triggers a manual sync on all clients and waits for completion
func (t *TestOrchestrator) SyncAll() {
	t.clientsMux.RLock()
	defer t.clientsMux.RUnlock()
	for id, c := range t.clients {
		if err := c.Sync(context.Background()); err != nil {
			log.Printf("Client %s manual sync error: %v", id, err)
		}
	}
	// Give a brief moment for any async handlers to reload local state
	time.Sleep(200 * time.Millisecond)
}
