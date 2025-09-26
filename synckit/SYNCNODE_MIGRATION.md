# SyncNode Wrapper Migration Guide

## Current Implementation

SyncNode is currently implemented as a **type alias**:
```go
type SyncNode = SyncManager
```

This provides:
- ✅ **Perfect API compatibility** - All SyncManager methods automatically available
- ✅ **Zero overhead** - No additional wrapper calls
- ✅ **Compile-time safety** - Any interface changes automatically propagated

## Future Wrapper Struct Migration

If SyncNode needs to become a wrapper struct in the future, the following requirements **MUST** be met to maintain test compatibility and prevent regressions:

### 1. Interface Completeness

**All SyncManager methods must be forwarded:**

```go
type SyncNode struct {
    manager SyncManager
}

// Core sync operations
func (s *SyncNode) Sync(ctx context.Context) (*SyncResult, error) {
    return s.manager.Sync(ctx)
}

func (s *SyncNode) Push(ctx context.Context) (*SyncResult, error) {
    return s.manager.Push(ctx)
}

func (s *SyncNode) Pull(ctx context.Context) (*SyncResult, error) {
    return s.manager.Pull(ctx)
}

// Lifecycle management
func (s *SyncNode) StartAutoSync(ctx context.Context) error {
    return s.manager.StartAutoSync(ctx)
}

func (s *SyncNode) StopAutoSync() error {
    return s.manager.StopAutoSync()
}

func (s *SyncNode) Subscribe(handler func(*SyncResult)) error {
    return s.manager.Subscribe(handler)
}

func (s *SyncNode) Close() error {
    return s.manager.Close()
}
```

### 2. Test Compatibility Requirements

**The following tests will verify wrapper implementation:**

#### TestSyncNodeLifecycle
- ✅ **Interface check**: `var _ SyncManager = node` must compile
- ✅ **Method availability**: All lifecycle methods must be callable
- ✅ **Proper behavior**: StartAutoSync/StopAutoSync must work with sync intervals

#### TestSyncNodeManagerIdenticalBehavior  
- ✅ **Behavioral parity**: SyncNode and SyncManager must return identical results
- ✅ **Error handling**: Both must succeed/fail in the same scenarios
- ✅ **Result values**: EventsPushed, EventsPulled, ConflictsResolved must match

### 3. SyncResult Contract

**These fields are tested and must be maintained:**
```go
type SyncResult struct {
    EventsPushed    int    // Number of events sent to remote
    EventsPulled    int    // Number of events received from remote
    ConflictsResolved int  // Number of conflicts resolved
    // ... other fields
}
```

### 4. Option Support

**All ManagerOption functions must work with NewNode:**
- ✅ `WithStore()` - Required, validated by builder
- ✅ `WithTransport()` - Required, validated by builder  
- ✅ `WithSyncInterval()` - Used by lifecycle tests
- ✅ All other options (WithBatchSize, WithLWW, etc.) - Must pass through

### 5. Test Helper Compatibility

**Current test helpers must continue working:**
```go
// TestEventStore - Simple in-memory event store
// TestTransport - Simple transport implementation
// Both provide realistic behavior for meaningful tests
```

## Migration Validation Checklist

When implementing wrapper struct:

- [ ] All existing SyncManager tests still pass
- [ ] All SyncNode-specific tests pass
- [ ] `TestSyncNodeLifecycle` passes
- [ ] `TestSyncNodeManagerIdenticalBehavior` passes  
- [ ] All ManagerOption functions work with NewNode
- [ ] SyncResult fields maintain same semantics
- [ ] No behavioral changes in sync operations
- [ ] Performance impact is acceptable
- [ ] Documentation updated to reflect new architecture

## Testing Strategy

**Run these test suites to verify successful migration:**

```bash
# Verify all existing functionality
go test ./synckit -run TestNewManager

# Verify SyncNode functionality  
go test ./synckit -run TestNewNode

# Verify behavioral parity (critical)
go test ./synckit -run "TestSyncNodeLifecycle|TestSyncNodeManagerIdenticalBehavior"

# Full test suite
go test ./synckit
```

## Breaking Changes to Avoid

**Do NOT change these without updating tests:**
- SyncResult field names or types
- SyncManager interface method signatures  
- ManagerOption function signatures
- Required vs optional configuration parameters
- Error handling behavior in sync operations

---

**This document ensures that any future SyncNode wrapper implementation maintains 100% backward compatibility and passes all existing tests.**