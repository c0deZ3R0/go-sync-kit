package statemachine

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
	"crypto/rand"
	"encoding/hex"
)

// machine is the generic implementation of StateMachine interface.
type machine[T comparable] struct {
	config     StateMachineConfig[T]
	state      *AtomicState[T]
	stateStart time.Time // When we entered current state
	
	// Thread-safe fields
	mu          sync.RWMutex
	observers   []StateObserver[T]
	history     []StateTransition[T]
	
	// Metrics and validation
	validator StateValidator[T]
	metrics   StateMetrics[T]
	
	// Persistence support
	persistence      StatePersistence[T]
	machineID        string
	persistenceEnabled bool
	persistenceConfig PersistenceConfig
}

// New creates a new state machine with the given configuration.
func New[T comparable](config StateMachineConfig[T]) (StateMachine[T], error) {
	if config.TransitionRules == nil {
		return nil, errors.New("transition rules cannot be nil")
	}
	
	if config.MaxHistorySize <= 0 {
		config.MaxHistorySize = 50
	}
	
	m := &machine[T]{
		config:     config,
		state:      NewAtomicState(config.InitialState),
		stateStart: time.Now(),
		history:    make([]StateTransition[T], 0, config.MaxHistorySize),
		validator:  config.Validator,
		metrics:    config.Metrics,
	}
	
	// Record initial state
	if m.metrics != nil && config.EnableMetrics {
		m.metrics.RecordCurrentState(config.InitialState)
	}
	
	return m, nil
}

// Current returns the current state.
func (m *machine[T]) Current() T {
	return m.state.Load()
}

// Transition attempts to transition to the new state.
func (m *machine[T]) Transition(to T) error {
	return m.TransitionWithContext(to, nil)
}

// TransitionWithContext transitions with additional metadata.
func (m *machine[T]) TransitionWithContext(to T, metadata map[string]interface{}) error {
	from := m.state.Load()
	
	// Check if transition is valid
	if !m.CanTransition(to) {
		err := fmt.Errorf("invalid transition from %v to %v", from, to)
		
		// Notify observers of failed transition
		m.notifyTransitionFailed(from, to, err, metadata)
		
		// Record error metric
		if m.metrics != nil && m.config.EnableMetrics {
			m.metrics.RecordTransitionError(from, to, err, metadata)
		}
		
		return err
	}
	
	// Custom validation
	if m.validator != nil {
		if err := m.validator.ValidateTransition(from, to); err != nil {
			// Notify observers of failed transition
			m.notifyTransitionFailed(from, to, err, metadata)
			
			// Record error metric
			if m.metrics != nil && m.config.EnableMetrics {
				m.metrics.RecordTransitionError(from, to, err, metadata)
			}
			
			return fmt.Errorf("transition validation failed: %w", err)
		}
	}
	
	// Perform the transition
	now := time.Now()
	duration := now.Sub(m.stateStart)
	transitionID := generateTransitionID()
	
	// Update state atomically
	m.state.Store(to)
	m.stateStart = now
	
	// Create transition record
	transition := StateTransition[T]{
		From:         from,
		To:           to,
		Timestamp:    now,
		Duration:     duration,
		Metadata:     metadata,
		TransitionID: transitionID,
	}
	
	// Update history
	m.addToHistory(transition)
	
	// Notify observers
	m.notifyTransition(transition)
	
	// Record metrics
	if m.metrics != nil && m.config.EnableMetrics {
		m.metrics.RecordTransition(from, to, duration, metadata)
		m.metrics.RecordCurrentState(to)
		m.metrics.RecordStateDuration(from, duration)
	}
	
	// Auto-save state if persistence is enabled
	if m.persistenceEnabled && m.persistenceConfig.AutoSave && m.persistence != nil {
		go func() {
			// Save state asynchronously to avoid blocking transition
			if snapshot := m.createSnapshot(); snapshot != nil {
				m.persistence.SaveState(context.Background(), m.machineID, *snapshot)
			}
		}()
	}
	
	return nil
}

// CanTransition checks if a transition is valid.
func (m *machine[T]) CanTransition(to T) bool {
	current := m.state.Load()
	return m.config.TransitionRules.Contains(current, to)
}

// Subscribe adds a state change observer.
func (m *machine[T]) Subscribe(observer StateObserver[T]) {
	if observer == nil {
		return
	}
	
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.observers = append(m.observers, observer)
}

// Unsubscribe removes a state change observer.
func (m *machine[T]) Unsubscribe(observer StateObserver[T]) {
	if observer == nil {
		return
	}
	
	m.mu.Lock()
	defer m.mu.Unlock()
	
	for i, obs := range m.observers {
		// Compare function pointers (this is a simplified approach)
		// In practice, you might want to use unique IDs for observers
		if &obs == &observer {
			m.observers = append(m.observers[:i], m.observers[i+1:]...)
			break
		}
	}
}

// History returns recent state transition history.
func (m *machine[T]) History() []StateTransition[T] {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	// Return a copy of the history to prevent external modification
	history := make([]StateTransition[T], len(m.history))
	copy(history, m.history)
	return history
}

// Reset resets the state machine to initial state.
func (m *machine[T]) Reset() error {
	return m.TransitionWithContext(m.config.InitialState, map[string]interface{}{
		"reset": true,
	})
}

// ExportDOT generates a DOT format representation of the state machine for visualization.
// The output can be used with Graphviz tools to create visual diagrams of the state machine.
func (m *machine[T]) ExportDOT() string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var builder strings.Builder
	
	// DOT graph header
	graphName := m.config.Name
	if graphName == "" {
		graphName = "StateMachine"
	}
	
	builder.WriteString(fmt.Sprintf("digraph \"%s\" {\n", graphName))
	builder.WriteString("\trankdir=LR;\n")
	builder.WriteString("\tnode [shape=circle, style=filled];\n\n")
	
	// Current state highlighting
	currentState := m.state.Load()
	
	// Collect all states from transition rules
	statesMap := make(map[T]bool)
	statesMap[m.config.InitialState] = true
	statesMap[currentState] = true
	
	for from, targets := range m.config.TransitionRules {
		statesMap[from] = true
		for _, to := range targets {
			statesMap[to] = true
		}
	}
	
	// Define state nodes with styling
	for state := range statesMap {
		stateStr := fmt.Sprintf("%v", state)
		
		// Determine node styling based on state type
		var color, fillColor string
		switch {
		case state == currentState:
			color = "red"
			fillColor = "lightcoral"
		case state == m.config.InitialState:
			color = "green"
			fillColor = "lightgreen"
		default:
			color = "black"
			fillColor = "lightblue"
		}
		
		builder.WriteString(fmt.Sprintf("\t\"%s\" [color=%s, fillcolor=%s, label=\"%s\"];\n", 
			stateStr, color, fillColor, stateStr))
	}
	
	builder.WriteString("\n")
	
	// Define transitions (edges)
	for from, targets := range m.config.TransitionRules {
		fromStr := fmt.Sprintf("%v", from)
		for _, to := range targets {
			toStr := fmt.Sprintf("%v", to)
			builder.WriteString(fmt.Sprintf("\t\"%s\" -> \"%s\";\n", fromStr, toStr))
		}
	}
	
	// Add legend
	builder.WriteString("\n\t// Legend\n")
	builder.WriteString("\tsubgraph cluster_legend {\n")
	builder.WriteString("\t\tlabel=\"Legend\";\n")
	builder.WriteString("\t\tstyle=filled;\n")
	builder.WriteString("\t\tcolor=lightgrey;\n")
	builder.WriteString("\t\t\"Current State\" [color=red, fillcolor=lightcoral, shape=circle];\n")
	builder.WriteString("\t\t\"Initial State\" [color=green, fillcolor=lightgreen, shape=circle];\n")
	builder.WriteString("\t\t\"Other States\" [color=black, fillcolor=lightblue, shape=circle];\n")
	builder.WriteString("\t}\n")
	
	builder.WriteString("}")
	
	return builder.String()
}

// createSnapshot creates a snapshot of the current state machine state.
func (m *machine[T]) createSnapshot() *StateMachineSnapshot[T] {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	return &StateMachineSnapshot[T]{
		MachineID:     m.machineID,
		CurrentState:  m.state.Load(),
		InitialState:  m.config.InitialState,
		StateEnteredAt: m.stateStart,
		Config:        m.config,
		History:       m.history,
		SnapshotTime:  time.Now(),
		Version:       1, // Basic versioning
	}
}

// EnablePersistence enables state persistence for the state machine
func (m *machine[T]) EnablePersistence(persistence StatePersistence[T], machineID string, config PersistenceConfig) error {
	m.mu.Lock()
	m.persistence = persistence
	m.machineID = machineID
	m.persistenceConfig = config
	m.persistenceEnabled = true
	m.mu.Unlock()

	// Attempt to load and restore previous state if present
	if m.persistence != nil {
		if snapshot, err := m.persistence.LoadState(context.Background(), machineID); err == nil && snapshot != nil {
			m.restoreFromSnapshot(*snapshot)
		}
	}

	return nil
}

// DisablePersistence disables state persistence for the state machine
func (m *machine[T]) DisablePersistence() {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.persistence = nil
	m.machineID = ""
	m.persistenceConfig = PersistenceConfig{}
	m.persistenceEnabled = false
}

// CreateSnapshot creates and returns a snapshot of the current state machine state
func (m *machine[T]) CreateSnapshot() *StateMachineSnapshot[T] {
	return m.createSnapshot()
}

// restoreFromSnapshot restores the state machine from a snapshot.
func (m *machine[T]) restoreFromSnapshot(snapshot StateMachineSnapshot[T]) {
	// This is a simplified restore - a real implementation would need more logic
	// to handle potential inconsistencies, especially with observers and metrics.
	m.config = snapshot.Config
	m.state.Store(snapshot.CurrentState)
	m.stateStart = snapshot.StateEnteredAt
	m.history = snapshot.History
}

// addToHistory adds a transition to the history, maintaining size limit.
func (m *machine[T]) addToHistory(transition StateTransition[T]) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.history = append(m.history, transition)
	
	// Maintain history size limit
	if len(m.history) > m.config.MaxHistorySize {
		m.history = m.history[1:]
	}
}

// notifyTransition notifies all observers of a successful transition.
func (m *machine[T]) notifyTransition(transition StateTransition[T]) {
	m.mu.RLock()
	observers := make([]StateObserver[T], len(m.observers))
	copy(observers, m.observers)
	m.mu.RUnlock()
	
	// Notify observers in separate goroutines to prevent blocking
	for _, observer := range observers {
		go func(obs StateObserver[T]) {
			defer func() {
				// Recover from observer panics to prevent them from affecting the state machine
				if r := recover(); r != nil {
					// Log the panic if we had a logger available
					// For now, we silently recover
				}
			}()
			obs.OnTransition(transition)
		}(observer)
	}
}

// notifyTransitionFailed notifies all observers of a failed transition.
func (m *machine[T]) notifyTransitionFailed(from, to T, err error, metadata map[string]interface{}) {
	m.mu.RLock()
	observers := make([]StateObserver[T], len(m.observers))
	copy(observers, m.observers)
	m.mu.RUnlock()
	
	// Notify observers in separate goroutines to prevent blocking
	for _, observer := range observers {
		go func(obs StateObserver[T]) {
			defer func() {
				// Recover from observer panics to prevent them from affecting the state machine
				if r := recover(); r != nil {
					// Log the panic if we had a logger available
					// For now, we silently recover
				}
			}()
			obs.OnTransitionFailed(from, to, err, metadata)
		}(observer)
	}
}

// generateTransitionID creates a unique identifier for a transition.
func generateTransitionID() string {
	bytes := make([]byte, 4) // 8 character hex string
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// NewBuilder creates a state machine builder for fluent configuration.
func NewBuilder[T comparable](initialState T) *Builder[T] {
	return &Builder[T]{
		config: StateMachineConfig[T]{
			InitialState:    initialState,
			TransitionRules: make(TransitionRules[T]),
			MaxHistorySize:  50,
			EnableMetrics:   true,
		},
	}
}

// Builder provides a fluent interface for configuring state machines.
type Builder[T comparable] struct {
	config StateMachineConfig[T]
}

// Allow adds a valid transition from one state to another.
func (b *Builder[T]) Allow(from T, to ...T) *Builder[T] {
	if b.config.TransitionRules == nil {
		b.config.TransitionRules = make(TransitionRules[T])
	}
	
	b.config.TransitionRules[from] = append(b.config.TransitionRules[from], to...)
	return b
}

// WithValidator sets a custom validator for transitions.
func (b *Builder[T]) WithValidator(validator StateValidator[T]) *Builder[T] {
	b.config.Validator = validator
	return b
}

// WithMetrics sets a metrics collector.
func (b *Builder[T]) WithMetrics(metrics StateMetrics[T]) *Builder[T] {
	b.config.Metrics = metrics
	return b
}

// WithHistorySize sets the maximum history size.
func (b *Builder[T]) WithHistorySize(size int) *Builder[T] {
	b.config.MaxHistorySize = size
	return b
}

// WithName sets a name for the state machine.
func (b *Builder[T]) WithName(name string) *Builder[T] {
	b.config.Name = name
	return b
}

// EnableMetrics enables or disables metrics collection.
func (b *Builder[T]) EnableMetrics(enabled bool) *Builder[T] {
	b.config.EnableMetrics = enabled
	return b
}

// Build creates the configured state machine.
func (b *Builder[T]) Build() (StateMachine[T], error) {
	return New(b.config)
}
