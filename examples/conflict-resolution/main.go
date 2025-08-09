package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	clientpkg "github.com/c0deZ3R0/go-sync-kit/examples/conflict_resolution/client"
	serverpkg "github.com/c0deZ3R0/go-sync-kit/examples/conflict_resolution/server"
)

func main() {
	mode := flag.String("mode", "demo", "Mode: server | client | demo")
	id := flag.String("id", "client1", "Client ID (for mode=client)")
	port := flag.Int("port", 8080, "Port for server or client control API")
	serverURL := flag.String("server-url", "http://localhost:8080", "Server sync URL (for mode=client)")
	flag.Parse()

	switch strings.ToLower(*mode) {
	case "server":
		startServer(*port)
	case "client":
		startClient(*id, *serverURL, *port)
	default:
		runDemo()
	}
}

func startServer(port int) {
	cfg := serverpkg.Config{
		Port:   port,
		DBPath: "./server_data.db",
		Logger: log.New(os.Stdout, "[Server] ", log.LstdFlags),
		UseWAL: true,
	}
	srv, err := serverpkg.New(cfg)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		if err := srv.Start(ctx); err != nil {
			log.Printf("Server stopped: %v", err)
		}
	}()

	// Wait for interrupt
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	_ = srv.Stop(context.Background())
}

func startClient(id, serverURL string, port int) {
	// Create client
	c, err := clientpkg.New(clientpkg.Config{
		ID:         id,
		ServerURL:  serverURL,
		DBPath:     fmt.Sprintf("./data/%s.db", id),
		Logger:     log.New(os.Stdout, fmt.Sprintf("[Client %s] ", id), log.LstdFlags),
		SyncPeriod: time.Second,
	})
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer c.Close()
	if err := c.Start(); err != nil {
		log.Fatalf("Failed to start client sync: %v", err)
	}
	// Control API
	mux := http.NewServeMux()
	mux.HandleFunc("/counters", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		_ = json.NewEncoder(w).Encode(c.ListCounters())
	})
	mux.HandleFunc("/counter/", func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/counter/"), "/")
		if len(parts) == 0 || parts[0] == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		id := parts[0]
		switch r.Method {
		case http.MethodGet:
			val, err := c.GetCounter(id)
			if err != nil {
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]int{"value": val})
		case http.MethodPost:
			// No direct POST on /counter/{id}; use specific endpoints
			w.WriteHeader(http.StatusNotFound)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/counter/create", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			ID string `json:"id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := c.CreateCounterSync(req.ID); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/counter/increment", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
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
		if req.Value == 0 {
			req.Value = 1
		}
		if err := c.IncrementCounter(context.Background(), req.ID, req.Value); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/sync", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if err := c.Sync(context.Background()); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/debug", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	addr := fmt.Sprintf(":%d", port)
	log.Printf("Client control API listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("Client API error: %v", err)
	}
}

// Existing demo harness (optional)
func runDemo() {
	// Create and start orchestrator
	orch := NewTestOrchestrator()
	fmt.Println("Starting server...")
	if err := orch.StartServer(); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
	fmt.Println("Adding clients...")
	if err := orch.AddClient("client1"); err != nil {
		log.Fatalf("Failed to add client1: %v", err)
	}
	if err := orch.AddClient("client2"); err != nil {
		log.Fatalf("Failed to add client2: %v", err)
	}
	fmt.Println("\nCreating counter1 from client1...")
	if err := orch.ExecuteCommand("client1", "c", "counter1"); err != nil {
		log.Printf("Error: %v", err)
	}
	time.Sleep(time.Second)
	orch.SyncAll()
	fmt.Println("Current counter values:")
	printCounterValues(orch, "counter1")
	fmt.Println("\nIncrementing counter1 from client1...")
	if err := orch.ExecuteCommand("client1", "i", "counter1"); err != nil {
		log.Printf("Error: %v", err)
	}
	time.Sleep(time.Second)
	orch.SyncAll()
	fmt.Println("Current counter values:")
	printCounterValues(orch, "counter1")
	fmt.Println("\nIncrementing counter1 from client2...")
	if err := orch.ExecuteCommand("client2", "i", "counter1"); err != nil {
		log.Printf("Error: %v", err)
	}
	time.Sleep(time.Second)
	orch.SyncAll()
	fmt.Println("Current counter values:")
	printCounterValues(orch, "counter1")
	fmt.Println("\nStopping all components...")
	orch.Stop()
}

func printCounterValues(orch *TestOrchestrator, counterID string) {
	val1, err := orch.GetCounter("client1", counterID)
	if err != nil {
		fmt.Printf("Client1 counter error: %v\n", err)
	} else {
		fmt.Printf("Client1 counter value: %d\n", val1)
	}
	val2, err := orch.GetCounter("client2", counterID)
	if err != nil {
		fmt.Printf("Client2 counter error: %v\n", err)
	} else {
		fmt.Printf("Client2 counter value: %d\n", val2)
	}
}
