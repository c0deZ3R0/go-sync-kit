package statemachine

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// Transport interface - basic transport interface that we extend
// This should match the synckit.Transport interface
type Transport interface {
	// Push sends events to the remote endpoint
	Push(ctx context.Context, events []interface{}) error
	
	// Pull retrieves events from the remote endpoint
	Pull(ctx context.Context, since interface{}) ([]interface{}, error)
	
	// GetLatestVersion efficiently retrieves the latest version from remote
	GetLatestVersion(ctx context.Context) (interface{}, error)
	
	// Subscribe listens for real-time updates (optional)
	Subscribe(ctx context.Context, handler func([]interface{}) error) error
	
	// Close closes the transport connection
	Close() error
}

// StatefulTransport extends the basic Transport interface with state machine capabilities
type StatefulTransport interface {
	// GetConnectionState returns the current connection state
	GetConnectionState() TransportState
	
	// SubscribeToStateChanges allows listening to transport state changes
	SubscribeToStateChanges(handler TransportStateHandler) error
	
	// Connect attempts to establish a connection with state tracking
	Connect(ctx context.Context) error
	
	// Disconnect gracefully closes the connection with state tracking
	Disconnect(ctx context.Context) error
	
	// IsHealthy returns true if the transport is in a healthy state for operations
	IsHealthy() bool
	
	// GetLastError returns the last error that occurred, if any
	GetLastError() error
	
	// GetConnectionMetadata returns metadata about the current connection
	GetConnectionMetadata() map[string]interface{}
}

// TransportStateHandler handles transport state change events
type TransportStateHandler func(transition StateTransition[TransportState])

// TransportStateManager manages the state of transport connections with observability integration
type TransportStateManager struct {
	stateMachine     StateMachine[TransportState]
	logger           *slog.Logger
	mu               sync.RWMutex
	
	// Connection tracking
	endpoint         string
	lastError        error
	connectionTime   time.Time
	totalAttempts    int
	
	// Observability hooks
	observabilityHooks *ObservabilityHooks[TransportState]
	
	// External state change handlers
	stateHandlers    []TransportStateHandler
}

// TransportStateManagerConfig configures the transport state manager
type TransportStateManagerConfig struct {
	Endpoint         string
	Logger           *slog.Logger
	MetricsCollector StateMetricsCollector
	Tracer           StateTracer
	HealthUpdater    StateHealthUpdater
	ComponentName    string
}

// NewTransportStateManager creates a new transport state manager with observability integration
func NewTransportStateManager(config TransportStateManagerConfig) (*TransportStateManager, error) {
	// Create the transport state machine
	stateMachine, err := NewTransportStateMachine()
	if err != nil {
		return nil, fmt.Errorf("failed to create transport state machine: %w", err)
	}
	
	logger := config.Logger
	if logger == nil {
		logger = slog.Default()
	}
	
	componentName := config.ComponentName
	if componentName == "" {
		componentName = "transport_manager"
	}
	
	manager := &TransportStateManager{
		stateMachine:   stateMachine,
		logger:        logger,
		endpoint:      config.Endpoint,
		stateHandlers: make([]TransportStateHandler, 0),
	}
	
	// Setup observability integration if components are provided
	if config.MetricsCollector != nil || config.Tracer != nil || config.HealthUpdater != nil {
		hooks := NewObservabilityHooks[TransportState](
			config.MetricsCollector,
			config.Tracer,
			config.HealthUpdater,
			logger,
			componentName,
		)
		
		manager.observabilityHooks = hooks
		stateMachine.Subscribe(hooks)
	}
	
	// Subscribe to state changes for internal tracking
	stateMachine.Subscribe(manager)
	
	return manager, nil
}

// OnTransition handles state machine transitions for internal tracking and external notifications
func (tsm *TransportStateManager) OnTransition(transition StateTransition[TransportState]) {
	tsm.mu.Lock()
	defer tsm.mu.Unlock()
	
	// Update internal tracking based on transition
	switch transition.To {
	case TransportConnecting:
		tsm.totalAttempts++
		tsm.lastError = nil
		
	case TransportConnected:
		tsm.connectionTime = transition.Timestamp
		tsm.lastError = nil
		
	case TransportFailed:
		if errorMsg, ok := transition.Metadata["error"]; ok {
			if errorStr, ok := errorMsg.(string); ok {
				tsm.lastError = fmt.Errorf("%s", errorStr)
			}
		}
		
	case TransportDisconnected:
		tsm.lastError = nil
	}
	
	// Notify external handlers
	for _, handler := range tsm.stateHandlers {
		// Run handlers in goroutines to avoid blocking
		go func(h TransportStateHandler) {
			defer func() {
				if r := recover(); r != nil {
					tsm.logger.Error("Transport state handler panicked",
						"panic", r,
						"from_state", transition.From.String(),
						"to_state", transition.To.String())
				}
			}()
			h(transition)
		}(handler)
	}
	
	tsm.logger.Debug("Transport state transition",
		"from", transition.From.String(),
		"to", transition.To.String(),
		"duration", transition.Duration,
		"endpoint", tsm.endpoint)
}

// OnTransitionFailed handles failed state transitions
func (tsm *TransportStateManager) OnTransitionFailed(from, to TransportState, err error, metadata map[string]interface{}) {
	tsm.logger.Error("Transport state transition failed",
		"from", from.String(),
		"to", to.String(),
		"error", err,
		"endpoint", tsm.endpoint)
}

// GetConnectionState returns the current connection state
func (tsm *TransportStateManager) GetConnectionState() TransportState {
	return tsm.stateMachine.Current()
}

// SubscribeToStateChanges adds a handler for transport state changes
func (tsm *TransportStateManager) SubscribeToStateChanges(handler TransportStateHandler) error {
	if handler == nil {
		return fmt.Errorf("handler cannot be nil")
	}
	
	tsm.mu.Lock()
	defer tsm.mu.Unlock()
	
	tsm.stateHandlers = append(tsm.stateHandlers, handler)
	return nil
}

// TransitionToConnecting transitions the transport to connecting state
func (tsm *TransportStateManager) TransitionToConnecting(timeout time.Duration) error {
	currentState := tsm.stateMachine.Current()
	if !currentState.CanConnect() {
		return fmt.Errorf("cannot connect from state: %s", currentState.String())
	}
	
	metadata := TransportMeta.ConnectingMetadata(tsm.endpoint, timeout)
	return tsm.stateMachine.TransitionWithContext(TransportConnecting, metadata)
}

// TransitionToConnected transitions the transport to connected state
func (tsm *TransportStateManager) TransitionToConnected(connectionTime time.Duration) error {
	metadata := TransportMeta.ConnectedMetadata(tsm.endpoint, connectionTime)
	return tsm.stateMachine.TransitionWithContext(TransportConnected, metadata)
}

// TransitionToReconnecting transitions the transport to reconnecting state
func (tsm *TransportStateManager) TransitionToReconnecting(reason string, attempt int, nextRetry time.Duration) error {
	currentState := tsm.stateMachine.Current()
	if !currentState.CanReconnect() {
		return fmt.Errorf("cannot reconnect from state: %s", currentState.String())
	}
	
	metadata := TransportMeta.ReconnectingMetadata(reason, attempt, nextRetry)
	return tsm.stateMachine.TransitionWithContext(TransportReconnecting, metadata)
}

// TransitionToFailed transitions the transport to failed state
func (tsm *TransportStateManager) TransitionToFailed(err error, attempts int, duration time.Duration) error {
	metadata := TransportMeta.FailedMetadata(err, attempts, duration)
	return tsm.stateMachine.TransitionWithContext(TransportFailed, metadata)
}

// TransitionToDisconnected transitions the transport to disconnected state
func (tsm *TransportStateManager) TransitionToDisconnected(reason string, graceful bool) error {
	metadata := TransportMeta.DisconnectedMetadata(reason, graceful)
	return tsm.stateMachine.TransitionWithContext(TransportDisconnected, metadata)
}

// TransitionToShuttingDown transitions the transport to shutting down state
func (tsm *TransportStateManager) TransitionToShuttingDown(reason string) error {
	metadata := TransportMeta.ShuttingDownMetadata(reason)
	return tsm.stateMachine.TransitionWithContext(TransportShuttingDown, metadata)
}

// IsHealthy returns true if the transport is in a healthy state for operations
func (tsm *TransportStateManager) IsHealthy() bool {
	state := tsm.stateMachine.Current()
	return state.CanSendData() || state.CanReceiveData()
}

// CanSendData returns true if the transport can send data in the current state
func (tsm *TransportStateManager) CanSendData() bool {
	return tsm.stateMachine.Current().CanSendData()
}

// CanReceiveData returns true if the transport can receive data in the current state
func (tsm *TransportStateManager) CanReceiveData() bool {
	return tsm.stateMachine.Current().CanReceiveData()
}

// GetLastError returns the last error that occurred, if any
func (tsm *TransportStateManager) GetLastError() error {
	tsm.mu.RLock()
	defer tsm.mu.RUnlock()
	return tsm.lastError
}

// GetConnectionMetadata returns metadata about the current connection
func (tsm *TransportStateManager) GetConnectionMetadata() map[string]interface{} {
	tsm.mu.RLock()
	defer tsm.mu.RUnlock()
	
	state := tsm.stateMachine.Current()
	metadata := map[string]interface{}{
		"current_state":   state.String(),
		"endpoint":       tsm.endpoint,
		"total_attempts": tsm.totalAttempts,
		"is_connected":   state.IsConnected(),
		"can_send_data":  state.CanSendData(),
		"can_receive_data": state.CanReceiveData(),
	}
	
	if !tsm.connectionTime.IsZero() {
		metadata["connected_at"] = tsm.connectionTime.Format(time.RFC3339)
		metadata["connection_duration"] = time.Since(tsm.connectionTime).String()
	}
	
	if tsm.lastError != nil {
		metadata["last_error"] = tsm.lastError.Error()
	}
	
	return metadata
}

// GetStateMachine returns the underlying state machine for advanced usage
func (tsm *TransportStateManager) GetStateMachine() StateMachine[TransportState] {
	return tsm.stateMachine
}

// Close gracefully shuts down the transport state manager
func (tsm *TransportStateManager) Close() error {
	// Transition to shutting down state if not already there
	currentState := tsm.stateMachine.Current()
	if currentState != TransportShuttingDown {
		if err := tsm.TransitionToShuttingDown("manager_close"); err != nil {
			tsm.logger.Error("Failed to transition to shutting down state during close", "error", err)
		}
	}
	
	tsm.mu.Lock()
	defer tsm.mu.Unlock()
	
	// Clear handlers
	tsm.stateHandlers = nil
	
	return nil
}

// StatefulTransportWrapper wraps an existing transport with state management capabilities
type StatefulTransportWrapper struct {
	transport Transport
	stateManager *TransportStateManager
}

// Transport interface - all existing transport methods should be available
// The transport field will handle the actual implementation

// NewStatefulTransportWrapper creates a stateful wrapper around an existing transport
func NewStatefulTransportWrapper(transport Transport, config TransportStateManagerConfig) (*StatefulTransportWrapper, error) {
	stateManager, err := NewTransportStateManager(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create transport state manager: %w", err)
	}
	
	return &StatefulTransportWrapper{
		transport:    transport,
		stateManager: stateManager,
	}, nil
}

// GetConnectionState returns the current connection state
func (stw *StatefulTransportWrapper) GetConnectionState() TransportState {
	return stw.stateManager.GetConnectionState()
}

// SubscribeToStateChanges allows listening to transport state changes
func (stw *StatefulTransportWrapper) SubscribeToStateChanges(handler TransportStateHandler) error {
	return stw.stateManager.SubscribeToStateChanges(handler)
}

// IsHealthy returns true if the transport is in a healthy state for operations
func (stw *StatefulTransportWrapper) IsHealthy() bool {
	return stw.stateManager.IsHealthy()
}

// GetLastError returns the last error that occurred, if any
func (stw *StatefulTransportWrapper) GetLastError() error {
	return stw.stateManager.GetLastError()
}

// GetConnectionMetadata returns metadata about the current connection
func (stw *StatefulTransportWrapper) GetConnectionMetadata() map[string]interface{} {
	return stw.stateManager.GetConnectionMetadata()
}

// Close closes both the state manager and the underlying transport
func (stw *StatefulTransportWrapper) Close() error {
	var errs []error
	
	// Close state manager first
	if err := stw.stateManager.Close(); err != nil {
		errs = append(errs, fmt.Errorf("state manager close error: %w", err))
	}
	
	// Close underlying transport
	if err := stw.transport.Close(); err != nil {
		errs = append(errs, fmt.Errorf("transport close error: %w", err))
	}
	
	if len(errs) > 0 {
		return fmt.Errorf("multiple close errors: %v", errs)
	}
	
	return nil
}
