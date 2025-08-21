// Example 10: State Machine Enhancements
//
// This example demonstrates the advanced state machine enhancements:
// 1. State Machine Visualization (DOT export for Graphviz)
// 2. State Persistence across restarts
// 3. Timeout Handling for stuck states
//
// These features address enterprise requirements for:
// - Better debugging and documentation through visual state diagrams
// - Resilient recovery from crashes and restarts
// - Prevention of stuck states in long-running processes

package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/c0deZ3R0/go-sync-kit/synckit/statemachine"
)

// SyncState represents the states of our sync process
type SyncState int

const (
	SyncIdle SyncState = iota
	SyncInitializing
	SyncPulling
	SyncPushing
	SyncResolvingConflicts
	SyncCompleted
	SyncFailed
	SyncTimeout
)

func (s SyncState) String() string {
	switch s {
	case SyncIdle:
		return "idle"
	case SyncInitializing:
		return "initializing"
	case SyncPulling:
		return "pulling"
	case SyncPushing:
		return "pushing"
	case SyncResolvingConflicts:
		return "resolving_conflicts"
	case SyncCompleted:
		return "completed"
	case SyncFailed:
		return "failed"
	case SyncTimeout:
		return "timeout"
	default:
		return "unknown"
	}
}

func main() {
	fmt.Println("=== Go Sync Kit Example 10: State Machine Enhancements ===\n")

	// Scenario 1: State Machine Visualization
	fmt.Printf("%s\n", strings.Repeat("=", 80))
	fmt.Println("📊 Scenario 1: State Machine Visualization")
	fmt.Printf("%s\n", strings.Repeat("=", 80))

	visualizationDemo()

	// Scenario 2: State Persistence
	fmt.Printf("\n%s\n", strings.Repeat("=", 80))
	fmt.Println("💾 Scenario 2: State Persistence")
	fmt.Printf("%s\n", strings.Repeat("=", 80))

	persistenceDemo()

	// Scenario 3: Timeout Handling
	fmt.Printf("\n%s\n", strings.Repeat("=", 80))
	fmt.Println("⏰ Scenario 3: Timeout Handling")
	fmt.Printf("%s\n", strings.Repeat("=", 80))

	timeoutDemo()

	// Integration Demo: All Features Together
	fmt.Printf("\n%s\n", strings.Repeat("=", 80))
	fmt.Println("🔗 Integration Demo: All Features Together")
	fmt.Printf("%s\n", strings.Repeat("=", 80))

	integrationDemo()

	fmt.Printf("\n%s\n", strings.Repeat("=", 80))
	fmt.Println("🎉 State Machine Enhancements Demo Complete!")
	fmt.Println("\n💡 Key Features Demonstrated:")
	fmt.Println("   ✅ DOT graph generation for state machine visualization")
	fmt.Println("   ✅ State persistence across restarts for enterprise resilience")
	fmt.Println("   ✅ Timeout handling to prevent stuck states")
	fmt.Println("   ✅ Complete integration with metrics and observability")
	fmt.Println("   ✅ Enterprise-grade debugging and monitoring capabilities")
	fmt.Printf("%s\n", strings.Repeat("=", 80))
}

// visualizationDemo demonstrates the DOT export functionality
func visualizationDemo() {
	fmt.Println("Creating a state machine with comprehensive state transitions...")

	// Create state machine with all transitions
	sm, err := statemachine.NewBuilder(SyncIdle).
		WithName("SyncStateMachine").
		Allow(SyncIdle, SyncInitializing).
		Allow(SyncInitializing, SyncPulling, SyncFailed).
		Allow(SyncPulling, SyncPushing, SyncResolvingConflicts, SyncFailed, SyncTimeout).
		Allow(SyncPushing, SyncCompleted, SyncFailed, SyncTimeout).
		Allow(SyncResolvingConflicts, SyncPushing, SyncFailed, SyncTimeout).
		Allow(SyncCompleted, SyncIdle).
		Allow(SyncFailed, SyncIdle).
		Allow(SyncTimeout, SyncIdle, SyncFailed).
		Build()

	if err != nil {
		log.Fatalf("Failed to create state machine: %v", err)
	}

	fmt.Println("✅ State machine created with comprehensive transitions")

	// Perform some transitions to show current state highlighting
	fmt.Println("\n🔄 Performing state transitions:")
	transitions := []SyncState{SyncInitializing, SyncPulling, SyncResolvingConflicts}
	
	for _, state := range transitions {
		if err := sm.Transition(state); err != nil {
			fmt.Printf("❌ Failed to transition to %s: %v\n", state, err)
		} else {
			fmt.Printf("  ➡️  Transitioned to: %s\n", state)
		}
	}

	// Export DOT representation
	fmt.Println("\n📊 Generating DOT representation for visualization:")
	dotContent := sm.ExportDOT()
	
	// Save to file for visualization
	dotFile := "state_machine_diagram.dot"
	if err := os.WriteFile(dotFile, []byte(dotContent), 0644); err != nil {
		fmt.Printf("⚠️  Failed to write DOT file: %v\n", err)
	} else {
		fmt.Printf("✅ DOT file saved as: %s\n", dotFile)
		fmt.Println("💡 To generate a visual diagram, run:")
		fmt.Printf("   dot -Tpng %s -o state_machine_diagram.png\n", dotFile)
		fmt.Printf("   dot -Tsvg %s -o state_machine_diagram.svg\n", dotFile)
	}

	// Show a portion of the DOT content
	fmt.Println("\n📋 DOT Content Preview:")
	lines := strings.Split(dotContent, "\n")
	previewLines := 15
	if len(lines) < previewLines {
		previewLines = len(lines)
	}
	
	for i := 0; i < previewLines; i++ {
		fmt.Printf("  %s\n", lines[i])
	}
	
	if len(lines) > previewLines {
		fmt.Printf("  ... (%d more lines)\n", len(lines)-previewLines)
	}
}

// persistenceDemo demonstrates state persistence across restarts
func persistenceDemo() {
	fmt.Println("Demonstrating state persistence for enterprise resilience...")

	// Create in-memory persistence (in production, you'd use a database)
	persistence := statemachine.NewMemoryStatePersistence[SyncState]()
	machineID := "sync-manager-001"

	// Create state machine
	sm, err := statemachine.NewBuilder(SyncIdle).
		WithName("PersistentSyncMachine").
		Allow(SyncIdle, SyncInitializing).
		Allow(SyncInitializing, SyncPulling, SyncFailed).
		Allow(SyncPulling, SyncPushing, SyncResolvingConflicts, SyncFailed).
		Allow(SyncPushing, SyncCompleted, SyncFailed).
		Allow(SyncResolvingConflicts, SyncPushing, SyncFailed).
		Allow(SyncCompleted, SyncIdle).
		Allow(SyncFailed, SyncIdle).
		Build()

	if err != nil {
		log.Fatalf("Failed to create state machine: %v", err)
	}

	// Enable persistence with auto-save
	persistenceConfig := statemachine.DefaultPersistenceConfig()
	persistenceConfig.AutoSave = true
	
	fmt.Printf("📦 Enabling persistence for machine ID: %s\n", machineID)
	if err := sm.EnablePersistence(persistence, machineID, persistenceConfig); err != nil {
		fmt.Printf("❌ Failed to enable persistence: %v\n", err)
		return
	}

	fmt.Println("✅ Persistence enabled with auto-save")

	// Simulate a sync process with state transitions
	fmt.Println("\n🔄 Simulating sync process (with automatic state persistence):")
	syncSteps := []struct {
		state       SyncState
		description string
	}{
		{SyncInitializing, "Starting sync operation"},
		{SyncPulling, "Pulling remote changes"},
		{SyncResolvingConflicts, "Resolving conflicts"},
		{SyncPushing, "Pushing resolved changes"},
	}

	for _, step := range syncSteps {
		fmt.Printf("  ➡️  %s: %s\n", step.state, step.description)
		if err := sm.Transition(step.state); err != nil {
			fmt.Printf("    ❌ Transition failed: %v\n", err)
			continue
		}
		
		// Small delay to simulate work
		time.Sleep(100 * time.Millisecond)
	}

	currentState := sm.Current()
	fmt.Printf("\n💾 Current state before 'restart': %s\n", currentState)

	// Simulate application restart by creating a new state machine
	fmt.Println("🔄 Simulating application restart...")

	newSM, err := statemachine.NewBuilder(SyncIdle).
		WithName("RestoredSyncMachine").
		Allow(SyncIdle, SyncInitializing).
		Allow(SyncInitializing, SyncPulling, SyncFailed).
		Allow(SyncPulling, SyncPushing, SyncResolvingConflicts, SyncFailed).
		Allow(SyncPushing, SyncCompleted, SyncFailed).
		Allow(SyncResolvingConflicts, SyncPushing, SyncFailed).
		Allow(SyncCompleted, SyncIdle).
		Allow(SyncFailed, SyncIdle).
		Build()

	if err != nil {
		log.Fatalf("Failed to create restored state machine: %v", err)
	}

	// Enable persistence - this should load the saved state
	fmt.Printf("🔄 Restoring state for machine ID: %s\n", machineID)
	
	if err := newSM.EnablePersistence(persistence, machineID, persistenceConfig); err != nil {
		fmt.Printf("❌ Failed to restore persistence: %v\n", err)
		return
	}

	restoredState := newSM.Current()
	fmt.Printf("✅ State restored after restart: %s\n", restoredState)

	if restoredState == currentState {
		fmt.Println("🎉 SUCCESS: State was successfully persisted and restored!")
	} else {
		fmt.Printf("⚠️  State restoration issue: expected %s, got %s\n", currentState, restoredState)
	}

	// Continue from where we left off
	fmt.Println("\n▶️  Continuing sync process from restored state...")
	if err := newSM.Transition(SyncCompleted); err == nil {
		fmt.Printf("  ➡️  Sync completed successfully!\n")
	}

	// Show persistence statistics
	fmt.Println("\n📊 Persistence Statistics:")
	if machines, err := persistence.ListMachines(context.Background()); err == nil {
		fmt.Printf("  • Persisted machines: %v\n", machines)
	}
}

// timeoutDemo demonstrates timeout handling for stuck states
func timeoutDemo() {
	fmt.Println("Demonstrating timeout handling to prevent stuck states...")

	// Create state machine
	sm, err := statemachine.NewBuilder(SyncIdle).
		WithName("TimeoutAwareSyncMachine").
		Allow(SyncIdle, SyncInitializing).
		Allow(SyncInitializing, SyncPulling, SyncFailed).
		Allow(SyncPulling, SyncPushing, SyncFailed, SyncTimeout).
		Allow(SyncPushing, SyncCompleted, SyncFailed, SyncTimeout).
		Allow(SyncTimeout, SyncFailed, SyncIdle).
		Allow(SyncFailed, SyncIdle).
		Allow(SyncCompleted, SyncIdle).
		Build()

	if err != nil {
		log.Fatalf("Failed to create state machine: %v", err)
	}

	// Create timeout configuration
	timeoutConfig := statemachine.DefaultTimeoutConfig(SyncTimeout)
	timeoutConfig.DefaultTimeout = 2 * time.Second // Short timeout for demo
	timeoutConfig.TimeoutAction = statemachine.TimeoutActionTransition

	// Create timeout handler
	timeoutHandler := statemachine.NewTimeoutHandler(sm, timeoutConfig)

	// Set specific timeouts for different states
	timeouts := map[SyncState]time.Duration{
		SyncPulling: 1 * time.Second,  // Very short for demo
		SyncPushing: 3 * time.Second,  // Slightly longer
	}
	timeoutHandler.SetTimeouts(timeouts)

	// Set up timeout callback for monitoring
	timeoutMetrics := statemachine.NewTimeoutMetrics()
	timeoutHandler.OnTimeout(func(state SyncState, duration time.Duration) {
		fmt.Printf("⏰ TIMEOUT: State %s timed out after %v\n", state, duration)
		timeoutMetrics.RecordTimeout(state.String(), duration)
	})

	// Create and subscribe timeout observer for automatic timeout management
	timeoutObserver := statemachine.NewTimeoutObserver(timeoutHandler)
	sm.Subscribe(timeoutObserver)

	fmt.Printf("✅ Timeout handler configured with default timeout: %v\n", timeoutConfig.DefaultTimeout)
	fmt.Println("📋 State-specific timeouts:")
	for state, timeout := range timeouts {
		fmt.Printf("  • %s: %v\n", state, timeout)
	}

	// Start with normal transitions
	fmt.Println("\n🔄 Starting sync process with timeout monitoring:")
	
	fmt.Printf("  ➡️  Transitioning to: %s\n", SyncInitializing)
	sm.Transition(SyncInitializing)
	
	fmt.Printf("  ➡️  Transitioning to: %s (will timeout in %v)\n", SyncPulling, timeouts[SyncPulling])
	sm.Transition(SyncPulling)

	// Wait for timeout to occur
	fmt.Println("  ⏳ Waiting for timeout to occur...")
	time.Sleep(2 * time.Second)

	currentState := sm.Current()
	fmt.Printf("  🔍 Current state after timeout: %s\n", currentState)

	if currentState == SyncTimeout {
		fmt.Println("  ✅ SUCCESS: Timeout handling worked correctly!")
		
		// Transition from timeout to recovery
		fmt.Printf("  🔄 Recovering from timeout...\n")
		sm.Transition(SyncFailed)
		fmt.Printf("  ➡️  Transitioned to recovery state: %s\n", sm.Current())
		
		// Reset to idle
		sm.Transition(SyncIdle)
		fmt.Printf("  ➡️  Reset to: %s\n", sm.Current())
	}

	// Show timeout metrics
	fmt.Println("\n📊 Timeout Metrics:")
	metrics := timeoutMetrics.GetMetrics()
	fmt.Printf("  • Total timeouts: %d\n", metrics.TotalTimeouts)
	fmt.Printf("  • Last timeout: %s\n", metrics.LastTimeout.Format("15:04:05.000"))
	fmt.Printf("  • Max timeout time: %v\n", metrics.MaxTimeoutTime)
	fmt.Println("  • Timeouts by state:")
	for state, count := range metrics.TimeoutsByState {
		fmt.Printf("    - %s: %d\n", state, count)
	}

	// Demonstrate successful operation within timeout
	fmt.Println("\n✅ Demonstrating successful operation within timeout limits:")
	
	sm.Transition(SyncInitializing)
	sm.Transition(SyncPushing) // This has a 3-second timeout
	fmt.Printf("  ➡️  In %s state (timeout in %v)\n", SyncPushing, timeouts[SyncPushing])
	
	// Complete within timeout
	time.Sleep(1 * time.Second)
	sm.Transition(SyncCompleted)
	fmt.Printf("  ✅ Completed successfully within timeout: %s\n", sm.Current())
}

// integrationDemo shows all features working together
func integrationDemo() {
	fmt.Println("Demonstrating all enhancements working together...")

	// Create comprehensive state machine
	sm, err := statemachine.NewBuilder(SyncIdle).
		WithName("EnhancedSyncMachine").
		Allow(SyncIdle, SyncInitializing).
		Allow(SyncInitializing, SyncPulling, SyncFailed).
		Allow(SyncPulling, SyncPushing, SyncResolvingConflicts, SyncFailed, SyncTimeout).
		Allow(SyncPushing, SyncCompleted, SyncFailed, SyncTimeout).
		Allow(SyncResolvingConflicts, SyncPushing, SyncFailed, SyncTimeout).
		Allow(SyncCompleted, SyncIdle).
		Allow(SyncFailed, SyncIdle).
		Allow(SyncTimeout, SyncFailed, SyncIdle).
		Build()

	if err != nil {
		log.Fatalf("Failed to create enhanced state machine: %v", err)
	}

	fmt.Println("✅ Enhanced state machine created")

	// 1. Enable persistence
	persistence := statemachine.NewMemoryStatePersistence[SyncState]()
	persistenceConfig := statemachine.DefaultPersistenceConfig()
	persistenceConfig.AutoSave = true

	machineID := "enhanced-sync-001"
	sm.EnablePersistence(persistence, machineID, persistenceConfig)
	fmt.Printf("💾 Persistence enabled for: %s\n", machineID)

	// 2. Setup timeout handling
	timeoutConfig := statemachine.DefaultTimeoutConfig(SyncTimeout)
	timeoutConfig.DefaultTimeout = 5 * time.Second
	timeoutHandler := statemachine.NewTimeoutHandler(sm, timeoutConfig)
	
	timeoutHandler.SetTimeouts(map[SyncState]time.Duration{
		SyncPulling:            3 * time.Second,
		SyncPushing:            4 * time.Second,
		SyncResolvingConflicts: 2 * time.Second, // Shorter for conflict resolution
	})

	timeoutMetrics := statemachine.NewTimeoutMetrics()
	timeoutHandler.OnTimeout(func(state SyncState, duration time.Duration) {
		fmt.Printf("  ⏰ State %s timed out after %v\n", state, duration)
		timeoutMetrics.RecordTimeout(state.String(), duration)
	})

	timeoutObserver := statemachine.NewTimeoutObserver(timeoutHandler)
	sm.Subscribe(timeoutObserver)
	fmt.Println("⏰ Timeout handling configured")

	// 3. Run a complete sync workflow
	fmt.Println("\n🚀 Running complete enhanced sync workflow:")
	
	workflow := []struct {
		state SyncState
		delay time.Duration
		description string
	}{
		{SyncInitializing, 200 * time.Millisecond, "Initialize sync"},
		{SyncPulling, 500 * time.Millisecond, "Pull remote changes"},
		{SyncResolvingConflicts, 1500 * time.Millisecond, "Resolve conflicts"},
		{SyncPushing, 300 * time.Millisecond, "Push changes"},
		{SyncCompleted, 100 * time.Millisecond, "Complete sync"},
	}

	for _, step := range workflow {
		fmt.Printf("  ➡️  %s: %s\n", step.state, step.description)
		if err := sm.Transition(step.state); err != nil {
			fmt.Printf("    ❌ Transition failed: %v\n", err)
			continue
		}
		
		// Simulate work with delay
		time.Sleep(step.delay)
	}

	fmt.Printf("✅ Workflow completed. Final state: %s\n", sm.Current())

	// 4. Generate visualization
	fmt.Println("\n📊 Generating final state machine visualization...")
	dotContent := sm.ExportDOT()
	
	visualFile := "enhanced_sync_machine.dot"
	if err := os.WriteFile(visualFile, []byte(dotContent), 0644); err == nil {
		fmt.Printf("✅ Enhanced state machine diagram saved as: %s\n", visualFile)
	}

	// 5. Show comprehensive metrics
	fmt.Println("\n📈 Final Enhancement Summary:")
	fmt.Printf("  🎨 Visualization: State diagram exported (%d lines)\n", len(strings.Split(dotContent, "\n")))
	
	if machines, err := persistence.ListMachines(context.Background()); err == nil {
		fmt.Printf("  💾 Persistence: %d machines persisted\n", len(machines))
	}
	
	finalMetrics := timeoutMetrics.GetMetrics()
	fmt.Printf("  ⏰ Timeouts: %d total timeouts handled\n", finalMetrics.TotalTimeouts)
	
	fmt.Printf("  📊 State History: %d transitions recorded\n", len(sm.History()))

	fmt.Println("\n🎉 All enhancements successfully demonstrated!")
	fmt.Println("💡 This demonstrates enterprise-ready state machine capabilities:")
	fmt.Println("   • Visual debugging through Graphviz integration")
	fmt.Println("   • Resilient recovery through state persistence")
	fmt.Println("   • Robust timeout handling for production reliability")
	fmt.Println("   • Comprehensive observability and metrics")
}
