// Example 5: Real-time Auto Sync Demo
// 
// This example demonstrates:
// - Setting up automatic synchronization with timers
// - Background sync operations that don't block the main thread
// - Handling sync results and errors in real-time
// - Graceful shutdown of auto-sync mechanisms
// - Monitoring sync activity and statistics

package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/c0deZ3R0/go-sync-kit/cursor"
	"github.com/c0deZ3R0/go-sync-kit/storage/sqlite"
	synckit "github.com/c0deZ3R0/go-sync-kit/synckit"
)

// TaskEvent represents a task management event
type TaskEvent struct {
	EventID      string    `json:"id"`
	EventType    string    `json:"event_type"`
	TaskID       string    `json:"task_id"`
	Title        string    `json:"title"`
	Description  string    `json:"description"`
	Status       string    `json:"status"`
	AssigneeID   string    `json:"assignee_id"`
	AssigneeName string    `json:"assignee_name"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	Priority     string    `json:"priority"`
}

// Implement the Event interface
func (e *TaskEvent) ID() string {
	return e.EventID
}

func (e *TaskEvent) Type() string {
	return e.EventType
}

func (e *TaskEvent) AggregateID() string {
	return e.TaskID
}

func (e *TaskEvent) Data() interface{} {
	return e
}

func (e *TaskEvent) Metadata() map[string]interface{} {
	return map[string]interface{}{
		"assignee_id":   e.AssigneeID,
		"assignee_name": e.AssigneeName,
		"priority":      e.Priority,
		"status":        e.Status,
		"created_at":    e.CreatedAt,
		"updated_at":    e.UpdatedAt,
	}
}

// AutoSyncManager wraps the sync manager with automatic sync capabilities
type AutoSyncManager struct {
	manager    synckit.SyncManager
	ctx        context.Context
	cancel     context.CancelFunc
	interval   time.Duration
	syncCount  int64
	errorCount int64
	lastSync   time.Time
}

func NewAutoSyncManager(manager synckit.SyncManager, interval time.Duration) *AutoSyncManager {
	ctx, cancel := context.WithCancel(context.Background())
	return &AutoSyncManager{
		manager:  manager,
		ctx:      ctx,
		cancel:   cancel,
		interval: interval,
	}
}

func (a *AutoSyncManager) Start() {
	fmt.Printf("🚀 Starting auto-sync with %v interval\n", a.interval)
	
	go func() {
		ticker := time.NewTicker(a.interval)
		defer ticker.Stop()
		
		// Initial sync
		a.performSync()
		
		for {
			select {
			case <-a.ctx.Done():
				fmt.Println("⏹️ Auto-sync stopped")
				return
			case <-ticker.C:
				a.performSync()
			}
		}
	}()
}

func (a *AutoSyncManager) performSync() {
	fmt.Printf("\n🔄 Auto-sync triggered at %s\n", time.Now().Format("15:04:05"))
	
	result, err := a.manager.Sync(a.ctx)
	a.syncCount++
	a.lastSync = time.Now()
	
	if err != nil {
		a.errorCount++
		fmt.Printf("❌ Auto-sync failed: %v\n", err)
		return
	}
	
	fmt.Printf("✅ Auto-sync completed: %d pushed, %d pulled, %d conflicts resolved\n",
		result.EventsPushed, result.EventsPulled, result.ConflictsResolved)
}

func (a *AutoSyncManager) Stop() {
	fmt.Println("🛑 Stopping auto-sync...")
	a.cancel()
}

func (a *AutoSyncManager) Stats() (int64, int64, time.Time) {
	return a.syncCount, a.errorCount, a.lastSync
}

func main() {
	fmt.Println("=== Go Sync Kit Example 5: Real-time Auto Sync Demo ===\n")

	// Create stores for multiple clients
	fmt.Println("🏗️ Setting up multi-client environment...")

	storeA, err := sqlite.NewWithDataSource("autosync-client-a.db")
	if err != nil {
		log.Fatalf("Failed to create store A: %v", err)
	}
	defer storeA.Close()

	storeB, err := sqlite.NewWithDataSource("autosync-client-b.db") 
	if err != nil {
		log.Fatalf("Failed to create store B: %v", err)
	}
	defer storeB.Close()

	ctx := context.Background()

	// Create managers with different auto-sync intervals
	fmt.Println("🔧 Creating sync managers...")
	
	managerA, err := synckit.NewManager(
		synckit.WithStore(storeA),
		synckit.WithNullTransport(),
		synckit.WithLWW(),
	)
	if err != nil {
		log.Fatalf("Failed to create manager A: %v", err)
	}

	managerB, err := synckit.NewManager(
		synckit.WithStore(storeB),
		synckit.WithNullTransport(),
		synckit.WithLWW(),
	)
	if err != nil {
		log.Fatalf("Failed to create manager B: %v", err)
	}

	// Create auto-sync managers with different intervals
	autoSyncA := NewAutoSyncManager(managerA, 3*time.Second)
	autoSyncB := NewAutoSyncManager(managerB, 5*time.Second)

	// Start auto-sync on both clients
	autoSyncA.Start()
	autoSyncB.Start()

	// Simulate creating tasks on different clients over time
	go func() {
		time.Sleep(1 * time.Second)
		
		for i := 0; i < 10; i++ {
			// Create tasks alternating between clients
			if i%2 == 0 {
				// Client A creates a task
				task := &TaskEvent{
					EventID:      fmt.Sprintf("task-a-%d", i/2+1),
					EventType:    "task.created",
					TaskID:       fmt.Sprintf("task-%d", i+1),
					Title:        fmt.Sprintf("Task %d from Client A", i+1),
					Description:  fmt.Sprintf("This task was created by Client A at iteration %d", i),
					Status:       "todo",
					AssigneeID:   "user-alice",
					AssigneeName: "Alice",
					CreatedAt:    time.Now(),
					UpdatedAt:    time.Now(),
					Priority:     "medium",
				}
				
				version := cursor.IntegerCursor{Seq: uint64(100 + i)}
				err := storeA.Store(ctx, task, version)
				if err != nil {
					fmt.Printf("❌ Failed to store task in Client A: %v\n", err)
				} else {
					fmt.Printf("📝 Client A created: '%s'\n", task.Title)
				}
			} else {
				// Client B creates a task
				task := &TaskEvent{
					EventID:      fmt.Sprintf("task-b-%d", i/2+1),
					EventType:    "task.created",
					TaskID:       fmt.Sprintf("task-%d", i+1),
					Title:        fmt.Sprintf("Task %d from Client B", i+1),
					Description:  fmt.Sprintf("This task was created by Client B at iteration %d", i),
					Status:       "todo",
					AssigneeID:   "user-bob",
					AssigneeName: "Bob",
					CreatedAt:    time.Now(),
					UpdatedAt:    time.Now(),
					Priority:     "high",
				}
				
				version := cursor.IntegerCursor{Seq: uint64(200 + i)}
				err := storeB.Store(ctx, task, version)
				if err != nil {
					fmt.Printf("❌ Failed to store task in Client B: %v\n", err)
				} else {
					fmt.Printf("📝 Client B created: '%s'\n", task.Title)
				}
			}
			
			time.Sleep(2 * time.Second)
		}
	}()

	// Monitor sync stats
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		
		for {
			select {
			case <-ticker.C:
				fmt.Println("\n📊 Auto-Sync Statistics:")
				
				syncCountA, errorCountA, lastSyncA := autoSyncA.Stats()
				fmt.Printf("  🅰️  Client A: %d syncs, %d errors, last sync: %s\n",
					syncCountA, errorCountA, lastSyncA.Format("15:04:05"))
					
				syncCountB, errorCountB, lastSyncB := autoSyncB.Stats()
				fmt.Printf("  🅱️  Client B: %d syncs, %d errors, last sync: %s\n",
					syncCountB, errorCountB, lastSyncB.Format("15:04:05"))
			}
		}
	}()

	// Set up graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	fmt.Println("\n👀 Monitoring auto-sync activity... (Press Ctrl+C to stop)")
	fmt.Println("   📌 Client A syncs every 3 seconds")
	fmt.Println("   📌 Client B syncs every 5 seconds")
	fmt.Println("   📌 Tasks are being created continuously on both clients")

	// Wait for shutdown signal
	<-sigChan
	fmt.Println("\n🛑 Shutdown signal received...")

	// Stop auto-sync managers
	autoSyncA.Stop()
	autoSyncB.Stop()

	// Final statistics
	time.Sleep(100 * time.Millisecond)
	fmt.Println("\n📊 Final Auto-Sync Statistics:")
	
	syncCountA, errorCountA, lastSyncA := autoSyncA.Stats()
	fmt.Printf("  🅰️  Client A: %d syncs, %d errors, last sync: %s\n",
		syncCountA, errorCountA, lastSyncA.Format("15:04:05"))
		
	syncCountB, errorCountB, lastSyncB := autoSyncB.Stats()
	fmt.Printf("  🅱️  Client B: %d syncs, %d errors, last sync: %s\n",
		syncCountB, errorCountB, lastSyncB.Format("15:04:05"))

	// Show final event counts
	zeroVersion := cursor.IntegerCursor{Seq: 0}
	
	eventsA, err := storeA.Load(ctx, zeroVersion)
	if err == nil {
		fmt.Printf("  📄 Client A total events: %d\n", len(eventsA))
	}
	
	eventsB, err := storeB.Load(ctx, zeroVersion)
	if err == nil {
		fmt.Printf("  📄 Client B total events: %d\n", len(eventsB))
	}

	fmt.Println("\n🎉 Auto-Sync Demo Complete!")
	fmt.Println("\n💡 Key Takeaways:")
	fmt.Println("   • Auto-sync runs in background without blocking operations")
	fmt.Println("   • Different clients can have different sync intervals")
	fmt.Println("   • Sync statistics help monitor system health")
	fmt.Println("   • Graceful shutdown ensures clean termination")
	fmt.Println("   • Real-time sync keeps all clients eventually consistent")
}
