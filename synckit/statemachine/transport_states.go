package statemachine

import (
	"fmt"
	"time"
)

// TransportState represents the state of a transport connection
type TransportState int

const (
	// TransportDisconnected indicates the transport is not connected
	TransportDisconnected TransportState = iota
	
	// TransportConnecting indicates a connection attempt is in progress
	TransportConnecting
	
	// TransportConnected indicates the transport is connected and active
	TransportConnected
	
	// TransportReconnecting indicates the transport is attempting to reconnect after a failure
	TransportReconnecting
	
	// TransportFailed indicates the transport has failed and cannot connect
	TransportFailed
	
	// TransportShuttingDown indicates the transport is being shut down gracefully
	TransportShuttingDown
)

// String returns a string representation of the transport state
func (ts TransportState) String() string {
	switch ts {
	case TransportDisconnected:
		return "disconnected"
	case TransportConnecting:
		return "connecting"
	case TransportConnected:
		return "connected"
	case TransportReconnecting:
		return "reconnecting"
	case TransportFailed:
		return "failed"
	case TransportShuttingDown:
		return "shutting_down"
	default:
		return fmt.Sprintf("unknown_transport_state_%d", int(ts))
	}
}

// CanConnect returns true if the transport can attempt to connect from this state
func (ts TransportState) CanConnect() bool {
	switch ts {
	case TransportDisconnected, TransportFailed:
		return true
	default:
		return false
	}
}

// CanReconnect returns true if the transport can attempt to reconnect from this state
func (ts TransportState) CanReconnect() bool {
	switch ts {
	case TransportConnected, TransportDisconnected, TransportFailed:
		return true
	default:
		return false
	}
}

// CanSendData returns true if the transport can send data in this state
func (ts TransportState) CanSendData() bool {
	return ts == TransportConnected
}

// CanReceiveData returns true if the transport can receive data in this state
func (ts TransportState) CanReceiveData() bool {
	return ts == TransportConnected
}

// IsConnected returns true if the transport is in a connected state
func (ts TransportState) IsConnected() bool {
	return ts == TransportConnected
}

// IsTerminal returns true if the transport is in a terminal state that requires user intervention
func (ts TransportState) IsTerminal() bool {
	return ts == TransportFailed
}

// NewTransportStateMachine creates a new state machine for transport connections
func NewTransportStateMachine() (StateMachine[TransportState], error) {
	// Define the transition rules
	transitionRules := TransitionRules[TransportState]{
		// From Disconnected
		TransportDisconnected: {
			TransportConnecting,    // Start connecting
			TransportShuttingDown, // Direct shutdown without connecting
		},
		
		// From Connecting
		TransportConnecting: {
			TransportConnected,     // Connection successful
			TransportFailed,        // Connection failed
			TransportDisconnected,  // Connection cancelled
			TransportShuttingDown,  // Shutdown during connection
		},
		
		// From Connected
		TransportConnected: {
			TransportReconnecting,  // Connection lost, attempting to reconnect
			TransportDisconnected,  // Clean disconnection
			TransportFailed,        // Connection failed permanently
			TransportShuttingDown,  // Graceful shutdown
		},
		
		// From Reconnecting
		TransportReconnecting: {
			TransportConnected,     // Reconnection successful
			TransportFailed,        // Reconnection failed permanently
			TransportDisconnected,  // Gave up reconnecting
			TransportShuttingDown,  // Shutdown during reconnection
		},
		
		// From Failed
		TransportFailed: {
			TransportConnecting,    // Manual retry attempt
			TransportShuttingDown,  // Shutdown after failure
		},
		
		// From ShuttingDown - terminal state, no transitions allowed
		TransportShuttingDown: {},
	}
	
	// Create validator for additional validation logic
	validator := &transportStateValidator{}
	
	config := StateMachineConfig[TransportState]{
		InitialState:    TransportDisconnected,
		TransitionRules: transitionRules,
		Validator:       validator,
		MaxHistorySize:  50,
		EnableMetrics:   true,
		Name:           "transport_connection",
	}
	
	return New(config)
}

// transportStateValidator implements custom validation for transport state transitions
type transportStateValidator struct{}

func (v *transportStateValidator) ValidateTransition(from, to TransportState) error {
	// Additional validation logic
	switch {
	case from == TransportShuttingDown:
		return fmt.Errorf("cannot transition from shutting down state")
	}
	
	return nil
}

// TransportStateMetadata provides helper functions for creating transport state metadata
type TransportStateMetadata struct{}

// ConnectingMetadata creates metadata for connection attempts
func (TransportStateMetadata) ConnectingMetadata(endpoint string, timeout time.Duration) map[string]interface{} {
	return map[string]interface{}{
		"endpoint":     endpoint,
		"timeout":      timeout,
		"started_at":   time.Now().Format(time.RFC3339),
		"operation":    "connecting",
	}
}

// ConnectedMetadata creates metadata for successful connections
func (TransportStateMetadata) ConnectedMetadata(endpoint string, connectionTime time.Duration) map[string]interface{} {
	return map[string]interface{}{
		"endpoint":        endpoint,
		"connection_time": connectionTime,
		"connected_at":    time.Now().Format(time.RFC3339),
		"operation":       "connected",
	}
}

// ReconnectingMetadata creates metadata for reconnection attempts
func (TransportStateMetadata) ReconnectingMetadata(reason string, attempt int, nextRetry time.Duration) map[string]interface{} {
	return map[string]interface{}{
		"disconnect_reason": reason,
		"attempt":          attempt,
		"next_retry":       nextRetry,
		"started_at":       time.Now().Format(time.RFC3339),
		"operation":        "reconnecting",
	}
}

// FailedMetadata creates metadata for connection failures
func (TransportStateMetadata) FailedMetadata(err error, attempts int, duration time.Duration) map[string]interface{} {
	return map[string]interface{}{
		"error":          err.Error(),
		"total_attempts": attempts,
		"total_duration": duration,
		"failed_at":      time.Now().Format(time.RFC3339),
		"operation":      "failed",
	}
}

// DisconnectedMetadata creates metadata for disconnections
func (TransportStateMetadata) DisconnectedMetadata(reason string, graceful bool) map[string]interface{} {
	return map[string]interface{}{
		"disconnect_reason": reason,
		"graceful":         graceful,
		"disconnected_at":  time.Now().Format(time.RFC3339),
		"operation":        "disconnected",
	}
}

// ShuttingDownMetadata creates metadata for shutdown operations
func (TransportStateMetadata) ShuttingDownMetadata(reason string) map[string]interface{} {
	return map[string]interface{}{
		"shutdown_reason": reason,
		"started_at":      time.Now().Format(time.RFC3339),
		"operation":       "shutting_down",
	}
}

// Helper instance for easy access to metadata creation
var TransportMeta = TransportStateMetadata{}
