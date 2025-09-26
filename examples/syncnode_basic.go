// Package main demonstrates the new SyncNode API for go-sync-kit.
// This example shows how to use the preferred SyncNode interface instead of the deprecated SyncManager.
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/c0deZ3R0/go-sync-kit/storage/memstore"
	"github.com/c0deZ3R0/go-sync-kit/synckit"
	"github.com/c0deZ3R0/go-sync-kit/transport/memchan"
)

func main() {
	fmt.Println("SyncNode Basic Example")
	fmt.Println("=====================")

	// Example 1: Using NewNode with individual options (recommended)
	fmt.Println("\n1. Creating SyncNode with NewNode():")
	
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

	fmt.Printf("✓ SyncNode created successfully\n")

	// Example 2: Using preset function for in-memory setup
	fmt.Println("\n2. Creating SyncNode with preset function:")
	
	store2 := memstore.New()
	transport2 := memchan.New(32)
	
	inMemoryNode, err := synckit.NewInMemoryNode(store2, transport2)
	if err != nil {
		log.Fatalf("Failed to create in-memory SyncNode: %v", err)
	}
	defer inMemoryNode.Close()

	fmt.Printf("✓ In-memory SyncNode created successfully\n")

	// Example 3: Performing sync operations
	fmt.Println("\n3. Performing sync operations:")
	
	ctx := context.Background()

	// Sync operation
	result, err := node.Sync(ctx)
	if err != nil {
		log.Printf("Sync failed: %v", err)
	} else {
		fmt.Printf("✓ Sync completed: %d events pushed, %d events pulled\n", 
			result.EventsPushed, result.EventsPulled)
	}

	// Push operation
	pushResult, err := node.Push(ctx)
	if err != nil {
		log.Printf("Push failed: %v", err)
	} else {
		fmt.Printf("✓ Push completed: %d events pushed\n", pushResult.EventsPushed)
	}

	// Pull operation
	pullResult, err := node.Pull(ctx)
	if err != nil {
		log.Printf("Pull failed: %v", err)
	} else {
		fmt.Printf("✓ Pull completed: %d events pulled\n", pullResult.EventsPulled)
	}

	fmt.Println("\n4. SyncNode API Benefits:")
	fmt.Println("   • Cleaner, more intuitive API")
	fmt.Println("   • Preset functions for common configurations")
	fmt.Println("   • Better documentation and examples")
	fmt.Println("   • Full backward compatibility with SyncManager")
	fmt.Println("   • Improved error handling and validation")

	fmt.Println("\nExample completed successfully!")
}