# Example 10: State Machine Enhancements

This example demonstrates three critical enterprise enhancements to the Go Sync Kit's state machine framework:

## 🎯 Features Demonstrated

### 1. State Machine Visualization (DOT Export)
- **Purpose**: Generate visual diagrams of state machines for debugging and documentation
- **Implementation**: `ExportDOT()` method generates Graphviz DOT format
- **Benefits**: 
  - Better understanding of complex state transitions
  - Visual debugging capabilities
  - Documentation and team communication
  - Architecture visualization

### 2. State Persistence Across Restarts
- **Purpose**: Resume state machines from their last known state after crashes/restarts
- **Implementation**: Pluggable persistence layer with auto-save capability
- **Benefits**:
  - Enterprise-grade resilience
  - Zero data loss on unexpected restarts
  - Seamless recovery from interruptions
  - Production reliability

### 3. Timeout Handling for Stuck States
- **Purpose**: Prevent states from getting stuck indefinitely
- **Implementation**: Configurable timeouts with automatic recovery actions
- **Benefits**:
  - Prevents hung processes
  - Automatic recovery from network issues
  - Configurable timeout policies
  - Production monitoring integration

## 🚀 Running the Example

```bash
cd examples/intermediate/10-state-machine-enhancements
go run main.go
```

## 📊 Generated Artifacts

The example generates several files:

### State Machine Diagrams
- `state_machine_diagram.dot` - Basic sync state machine
- `enhanced_sync_machine.dot` - Complete enhanced state machine

### Visualization Commands
```bash
# Generate PNG diagram
dot -Tpng state_machine_diagram.dot -o state_machine_diagram.png

# Generate SVG diagram  
dot -Tsvg state_machine_diagram.dot -o state_machine_diagram.svg

# Generate PDF diagram
dot -Tpdf state_machine_diagram.dot -o state_machine_diagram.pdf
```

## 🏗️ Architecture

### State Machine Structure
```
SyncIdle ──→ SyncInitializing ──→ SyncPulling ──→ SyncPushing ──→ SyncCompleted
    ↑              ↓                    ↓             ↓              ↓
    └──────────── SyncFailed ←────────────────────────┘              ↓
    ↑                                  ↓                             ↓
    └─────────── SyncTimeout ←─────────┴─────────────────────────────┘
```

### Key Components

#### 1. Visualization Engine
- **DOT Generator**: Creates Graphviz-compatible diagrams
- **State Highlighting**: Shows current state in red, initial state in green
- **Legend Support**: Automatic legend generation
- **Customizable Styling**: Colors and shapes for different state types

#### 2. Persistence Layer
- **Interface-Based**: Pluggable persistence implementations
- **Auto-Save**: Automatic state saving after transitions
- **Snapshot System**: Complete state machine snapshots
- **Recovery Logic**: Automatic state restoration on startup

#### 3. Timeout Management
- **State-Specific Timeouts**: Different timeouts per state
- **Configurable Actions**: Various timeout handling strategies
- **Metrics Collection**: Comprehensive timeout statistics
- **Observer Integration**: Automatic timeout management

## 💡 Enterprise Use Cases

### 1. Long-Running Sync Processes
- **Challenge**: Sync operations can take hours and may be interrupted
- **Solution**: State persistence ensures work isn't lost on restarts
- **Benefit**: Resume exactly where you left off

### 2. Production Debugging
- **Challenge**: Complex state machines are hard to debug in production
- **Solution**: Visual diagrams help understand system behavior
- **Benefit**: Faster troubleshooting and better documentation

### 3. Network Reliability
- **Challenge**: Network issues can cause processes to hang
- **Solution**: Timeout handling automatically recovers from stuck states
- **Benefit**: Self-healing systems with better reliability

## 🔧 Configuration Options

### Persistence Configuration
```go
config := DefaultPersistenceConfig()
config.AutoSave = true                    // Save after each transition
config.SaveInterval = 5 * time.Minute    // Periodic saves if AutoSave is false
config.RetentionPeriod = 24 * time.Hour  // How long to keep snapshots
config.MaxSnapshots = 10                 // Maximum snapshots per machine
```

### Timeout Configuration
```go
config := DefaultTimeoutConfig(FailureState)
config.DefaultTimeout = 5 * time.Minute       // Default timeout
config.TimeoutAction = TimeoutActionTransition // What to do on timeout
config.MaxRetries = 3                         // Recovery attempts
config.RetryDelay = 30 * time.Second          // Delay between retries
```

## 📈 Monitoring Integration

### Timeout Metrics
- Total timeouts occurred
- Timeouts by state breakdown
- Average timeout duration
- Maximum timeout duration
- Last timeout timestamp

### Persistence Metrics
- Number of persisted machines
- Snapshot creation frequency
- Recovery success rate
- Storage usage statistics

## 🎉 Key Benefits

1. **Enterprise Resilience**: Never lose progress due to crashes
2. **Visual Debugging**: Understand complex state flows instantly
3. **Production Reliability**: Automatic recovery from stuck states
4. **Comprehensive Monitoring**: Rich metrics for operational insights
5. **Zero Downtime**: Seamless recovery and continuation of operations

## 🔗 Integration with Existing Code

These enhancements are fully backward compatible. Existing state machines continue to work unchanged, with new features being opt-in through configuration.

```go
// Basic state machine (unchanged)
sm := statemachine.NewBuilder(InitialState).Allow(...).Build()

// Enhanced with persistence
sm.EnablePersistence(persistence, "machine-id", config)

// Enhanced with timeouts
timeoutHandler := statemachine.NewTimeoutHandler(sm, timeoutConfig)
sm.Subscribe(statemachine.NewTimeoutObserver(timeoutHandler))

// Enhanced with visualization
dotContent := sm.ExportDOT()
```

This demonstrates how enterprise-grade features can be added without breaking existing implementations.
