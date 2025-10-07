## 🚀 State Machine Platform Release

This minor release aggregates all changes since v0.17.0 and introduces a complete, extensible state machine platform for Sync Kit, plus examples and integrations across components.

### 🌟 Highlights Since v0.17.0

• Foundational State Machine Framework (sync and transport)
• AutoSync enhancements with state awareness
• Realtime sync manager built on state machines
• DOT visualization for state machines
• Persistence: snapshot, auto-save, and restore
• Timeout handling to prevent stuck states
• Examples demonstrating all capabilities

--------

## ✨ What's New

### 1) State Machine Foundations

Commits/PRs:
- PR #48: feature/state-machine-implementation
- c85448a: Add state machine implementation foundation
- 4144bf2: Implement transport state machine framework
- 7fbacb3, 6c06a0b, 7d8268e: Enhance AutoSync and realtime sync manager with state awareness

Capabilities:
- Generic StateMachine[T] with transition rules and validators
- Observers for transition notifications and history tracking
- Metrics hooks for observability
- Transport and Sync components wired to state transitions

### 2) Enterprise Enhancements (This PR)

Commits/PRs:
- PR #49: feature/state-machine-enhancements
- 070cdbd: Persistence, DOT export, timeout handling, examples

Features:
- DOT Export: sm.ExportDOT() produces Graphviz diagrams
- Persistence: EnablePersistence/DisablePersistence/CreateSnapshot on the interface
  - Auto-save snapshots after transitions (configurable)
  - Load last snapshot automatically when enabling persistence
- Timeout Handling: Default and per-state timeouts, actions (transition/fail/reset), callbacks, and simple metrics

### 3) Examples

- examples/intermediate/10-state-machine-enhancements
  - Visualizes a machine, persists state across a simulated restart, and demonstrates timeouts
- examples/intermediate/08-stateful-resolvers and 09-advanced-observability updated to align with state-aware flows

--------

## 🧩 Code Changes (since v0.17.0)

- synckit/statemachine/*: new framework, DOT export, persistence (persistence.go), timeouts (timeout.go)
- synckit/stateful_realtime.go: realtime sync manager leveraging state machines
- storage and examples: refinements to support state-aware operations

Full commit list:
- aeb258a: Merge PR #49 (state-machine-enhancements)
- 070cdbd: Add persistence, DOT export, timeouts, expose interface methods, example
- 9fa3f67: Merge PR #48 (state-machine-implementation)
- 1b8fff5: Implement comprehensive state machine framework
- b954461: Example: Add Subscribe to MockTransport
- 7d8268e: Enhance auto-sync with transport state awareness
- 6c06a0b: Stateful realtime sync manager with transport state machine
- 4144bf2: Transport state machine framework
- 7fbacb3: Enhance AutoSync with state machine awareness
- b478d1c: Fix: simplify slog usage in observability hooks
- c85448a: Add state machine implementation foundation

--------

## 📚 Usage Snippets

DOT Export
```go
_ = os.WriteFile("machine.dot", []byte(sm.ExportDOT()), 0644)
```

Persistence
```go
store := statemachine.NewMemoryStatePersistence[SyncState]()
_ = sm.EnablePersistence(store, "sync-001", statemachine.DefaultPersistenceConfig())
```

Timeouts
```go	h := statemachine.NewTimeoutHandler(sm, statemachine.DefaultTimeoutConfig(SyncTimeout))
	h.SetTimeouts(map[State]time.Duration{Pulling: time.Second})
	sm.Subscribe(statemachine.NewTimeoutObserver(h))
```

--------

## ⚙️ Compatibility

- No breaking changes; features are additive and opt-in
- Existing APIs continue to work as before

--------

## 📦 Installation & Upgrade

```bash
go get github.com/c0deZ3R0/go-sync-kit@v0.18.0
``` 

--------

⚠️ Pre‑release Notice: Marked as pre‑release to allow broader validation of persistence and timeout strategies in diverse environments. The APIs are stable, and examples are provided to accelerate adoption.

