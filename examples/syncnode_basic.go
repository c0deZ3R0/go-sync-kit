// Package main demonstrates the new SyncNode API for go-sync-kit.
// This example shows how to use the preferred SyncNode interface instead of the deprecated SyncManager.
package main

import (
	"context"
	"log"

	"github.com/c0deZ3R0/go-sync-kit/storage/memstore"
	"github.com/c0deZ3R0/go-sync-kit/synckit"
	"github.com/c0deZ3R0/go-sync-kit/transport/memchan"
)

func main() {
	log.Printf("SyncNode Basic Example")
	log.Printf("=====================")

	// Example 1: Using NewNode with individual options (recommended)
	log.Printf("\n1. Creating SyncNode with NewNode():")
	
	store := memstore.New()
	transport := memchan.New(16)
	
	node, err := synckit.NewNode(
		synckit.WithStore(store),
		synckit.WithTransport(transport),
		synckit.WithBatchSize(50),
		synckit.WithLWW(), // Last-Write-Wins conflict resolution
	)
	if err != nil {
		log.Fatalf("Failed to create SyncNode: %v", err)
	}
	defer node.Close()

	log.Printf("✓ SyncNode created successfully")

	// Example 2: Using preset function for in-memory setup
	log.Printf("\n2. Creating SyncNode with preset function:")
	
	store2 := memstore.New()
	transport2 := memchan.New(32)
	
	inMemoryNode, err := synckit.NewInMemoryNode(store2, transport2)
	if err != nil {
		log.Fatalf("Failed to create in-memory SyncNode: %v", err)
	}
	defer inMemoryNode.Close()

	log.Printf("✓ In-memory SyncNode created successfully")

	// Example 3: Performing sync operations
	log.Printf("\n3. Performing sync operations:")
	
	ctx := context.Background()

	// Sync operation
	result, err := node.Sync(ctx)
	if err != nil {
		log.Printf("Sync failed: %v", err)
	} else {
		log.Printf("✓ Sync completed: %d events pushed, %d events pulled", 
			result.EventsPushed, result.EventsPulled)
	}

	// Push operation
	pushResult, err := node.Push(ctx)
	if err != nil {
		log.Printf("Push failed: %v", err)
	} else {
		log.Printf("✓ Push completed: %d events pushed", pushResult.EventsPushed)
	}

	// Pull operation
	pullResult, err := node.Pull(ctx)
	if err != nil {
		log.Printf("Pull failed: %v", err)
	} else {
		log.Printf("✓ Pull completed: %d events pulled", pullResult.EventsPulled)
	}

	log.Printf("\n4. SyncNode API Benefits:")
	log.Printf("   • Cleaner, more intuitive API")
	log.Printf("   • Preset functions for common configurations")
	log.Printf("   • Better documentation and examples")
	log.Printf("   • Full backward compatibility with SyncManager")
	log.Printf("   • Improved error handling and validation")

	log.Printf("\nExample completed successfully!")
}