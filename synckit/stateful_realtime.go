package synckit

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/c0deZ3R0/go-sync-kit/synckit/statemachine"
)

// StatefulRealtimeNotifier extends RealtimeNotifier with state machine capabilities
type StatefulRealtimeNotifier interface {
	RealtimeNotifier
	statemachine.StatefulTransport
}

// StatefulRealtimeSyncManager enhances RealtimeSyncManager with transport state awareness
type StatefulRealtimeSyncManager struct {
	*realtimeSyncManager // embed existing functionality
	transportStateManager *statemachine.TransportStateManager
	mu                   sync.RWMutex
}

// StatefulRealtimeSyncOptions extends RealtimeSyncOptions with state machine configuration
type StatefulRealtimeSyncOptions struct {
	RealtimeSyncOptions
	
	// TransportStateConfig for configuring transport state management
	TransportStateConfig statemachine.TransportStateManagerConfig
	
	// EnableTransportStateLogging enables detailed transport state logging
	EnableTransportStateLogging bool
	
	// ConnectionTimeout for transport connection attempts
	ConnectionTimeout time.Duration
	
	// MaxConnectionAttempts before marking transport as failed
	MaxConnectionAttempts int
	
	// ConnectionRetryDelay between connection attempts
	ConnectionRetryDelay time.Duration
}

// NewStatefulRealtimeSyncManager creates a realtime sync manager with transport state awareness
func NewStatefulRealtimeSyncManager(store EventStore, transport Transport, options *StatefulRealtimeSyncOptions) (*StatefulRealtimeSyncManager, error) {
	if options == nil {
		return nil, fmt.Errorf("options cannot be nil")
	}
	
	// Set defaults
	if options.ConnectionTimeout == 0 {
		options.ConnectionTimeout = 30 * time.Second
	}
	if options.MaxConnectionAttempts == 0 {
		options.MaxConnectionAttempts = 5
	}
	if options.ConnectionRetryDelay == 0 {
		options.ConnectionRetryDelay = 2 * time.Second
	}
	
	// Create the base realtime sync manager
	baseManager := NewRealtimeSyncManager(store, transport, &options.RealtimeSyncOptions)
	baseRealtime, ok := baseManager.(*realtimeSyncManager)
	if !ok {
		return nil, fmt.Errorf("failed to cast to realtimeSyncManager")
	}
	
	// Setup transport state configuration
	stateConfig := options.TransportStateConfig
	if stateConfig.Logger == nil && baseRealtime.logger != nil {
		stateConfig.Logger = baseRealtime.logger
	}
	if stateConfig.ComponentName == "" {
		stateConfig.ComponentName = "realtime_transport"
	}
	if stateConfig.Endpoint == "" {
		stateConfig.Endpoint = "realtime_connection"
	}
	
	// Create transport state manager
	transportStateManager, err := statemachine.NewTransportStateManager(stateConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create transport state manager: %w", err)
	}
	
	statefulManager := &StatefulRealtimeSyncManager{
		realtimeSyncManager:   baseRealtime,
		transportStateManager: transportStateManager,
	}
	
	// Subscribe to transport state changes for enhanced connection monitoring
	transportStateManager.SubscribeToStateChanges(statefulManager.handleTransportStateChange)
	
	return statefulManager, nil
}

// handleTransportStateChange handles transport state transitions
func (srsm *StatefulRealtimeSyncManager) handleTransportStateChange(transition statemachine.StateTransition[statemachine.TransportState]) {
	srsm.logger.Info("Transport state changed",
		"from", transition.From.String(),
		"to", transition.To.String(),
		"duration", transition.Duration,
		"metadata", transition.Metadata)
	
	switch transition.To {
	case statemachine.TransportConnected:
		srsm.onTransportConnected(transition)
		
	case statemachine.TransportDisconnected:
		srsm.onTransportDisconnected(transition)
		
	case statemachine.TransportFailed:
		srsm.onTransportFailed(transition)
		
	case statemachine.TransportReconnecting:
		srsm.onTransportReconnecting(transition)
	}
}

// onTransportConnected handles successful transport connection
func (srsm *StatefulRealtimeSyncManager) onTransportConnected(transition statemachine.StateTransition[statemachine.TransportState]) {
	// Update internal connection status
	srsm.updateConnectionStatus(true, time.Now(), nil)
	
	// Stop fallback polling since we're now connected
	srsm.stopFallbackPolling()
	
	// Log successful connection with metadata
	srsm.logger.Info("Realtime transport connected successfully",
		"connection_time", transition.Metadata["connection_time"],
		"endpoint", transition.Metadata["endpoint"])
}

// onTransportDisconnected handles transport disconnection
func (srsm *StatefulRealtimeSyncManager) onTransportDisconnected(transition statemachine.StateTransition[statemachine.TransportState]) {
	// Update internal connection status
	reason := "unknown"
	if r, ok := transition.Metadata["disconnect_reason"].(string); ok {
		reason = r
	}
	
	srsm.updateConnectionStatus(false, time.Time{}, fmt.Errorf("disconnected: %s", reason))
	
	// Start fallback polling if not disabled
	if !srsm.realtimeOptions.DisablePolling {
		go func() {
			ctx := context.Background() // Use background context for fallback
			srsm.startFallbackPolling(ctx)
		}()
	}
	
	srsm.logger.Warn("Realtime transport disconnected",
		"reason", reason,
		"graceful", transition.Metadata["graceful"])
}

// onTransportFailed handles transport failure
func (srsm *StatefulRealtimeSyncManager) onTransportFailed(transition statemachine.StateTransition[statemachine.TransportState]) {
	// Update internal connection status with error
	errorMsg := "transport failed"
	if e, ok := transition.Metadata["error"].(string); ok {
		errorMsg = e
	}
	
	srsm.updateConnectionStatus(false, time.Time{}, fmt.Errorf("%s", errorMsg))
	
	// Start fallback polling if not disabled
	if !srsm.realtimeOptions.DisablePolling {
		go func() {
			ctx := context.Background() // Use background context for fallback
			srsm.startFallbackPolling(ctx)
		}()
	}
	
	srsm.logger.Error("Realtime transport failed",
		"error", errorMsg,
		"total_attempts", transition.Metadata["total_attempts"],
		"total_duration", transition.Metadata["total_duration"])
}

// onTransportReconnecting handles transport reconnection attempts
func (srsm *StatefulRealtimeSyncManager) onTransportReconnecting(transition statemachine.StateTransition[statemachine.TransportState]) {
	reason := "unknown"
	if r, ok := transition.Metadata["disconnect_reason"].(string); ok {
		reason = r
	}
	
	attempt := 0
	if a, ok := transition.Metadata["attempt"].(int); ok {
		attempt = a
	}
	
	nextRetry := time.Duration(0)
	if nr, ok := transition.Metadata["next_retry"].(time.Duration); ok {
		nextRetry = nr
	}
	
	srsm.logger.Info("Realtime transport reconnecting",
		"reason", reason,
		"attempt", attempt,
		"next_retry", nextRetry)
}

// EnhancedEnableRealtime starts real-time notifications with state machine integration
func (srsm *StatefulRealtimeSyncManager) EnhancedEnableRealtime(ctx context.Context) error {
	// Check if we can connect based on transport state
	currentState := srsm.transportStateManager.GetConnectionState()
	if !currentState.CanConnect() && currentState != statemachine.TransportDisconnected {
		return fmt.Errorf("cannot enable realtime from transport state: %s", currentState.String())
	}
	
	// Transition to connecting state
	if err := srsm.transportStateManager.TransitionToConnecting(30 * time.Second); err != nil {
		return fmt.Errorf("failed to transition to connecting state: %w", err)
	}
	
	// Attempt to enable realtime with state tracking
	start := time.Now()
	err := srsm.EnableRealtime(ctx)
	
	if err != nil {
		// Transition to failed state
		failErr := srsm.transportStateManager.TransitionToFailed(err, 1, time.Since(start))
		if failErr != nil {
			srsm.logger.Error("Failed to transition to failed state", "error", failErr)
		}
		return err
	}
	
	// Transition to connected state
	connErr := srsm.transportStateManager.TransitionToConnected(time.Since(start))
	if connErr != nil {
		srsm.logger.Error("Failed to transition to connected state", "error", connErr)
	}
	
	return nil
}

// EnhancedDisableRealtime stops real-time notifications with state machine integration
func (srsm *StatefulRealtimeSyncManager) EnhancedDisableRealtime() error {
	// Transition to shutting down state
	if err := srsm.transportStateManager.TransitionToShuttingDown("realtime_disabled"); err != nil {
		srsm.logger.Error("Failed to transition to shutting down state", "error", err)
	}
	
	// Call base disable realtime
	err := srsm.DisableRealtime()
	
	// After successful shutdown, transition to disconnected
	if err == nil {
		if transErr := srsm.transportStateManager.TransitionToDisconnected("realtime_disabled", true); transErr != nil {
			srsm.logger.Error("Failed to transition to disconnected state", "error", transErr)
		}
	}
	
	return err
}

// GetTransportConnectionState returns the current transport connection state
func (srsm *StatefulRealtimeSyncManager) GetTransportConnectionState() statemachine.TransportState {
	return srsm.transportStateManager.GetConnectionState()
}

// GetTransportConnectionMetadata returns metadata about the transport connection
func (srsm *StatefulRealtimeSyncManager) GetTransportConnectionMetadata() map[string]interface{} {
	return srsm.transportStateManager.GetConnectionMetadata()
}

// IsTransportHealthy returns true if the transport is in a healthy state
func (srsm *StatefulRealtimeSyncManager) IsTransportHealthy() bool {
	return srsm.transportStateManager.IsHealthy()
}

// CanSendRealtimeData returns true if the transport can send data in the current state
func (srsm *StatefulRealtimeSyncManager) CanSendRealtimeData() bool {
	return srsm.transportStateManager.CanSendData()
}

// CanReceiveRealtimeData returns true if the transport can receive data in the current state
func (srsm *StatefulRealtimeSyncManager) CanReceiveRealtimeData() bool {
	return srsm.transportStateManager.CanReceiveData()
}

// SubscribeToTransportStateChanges allows external components to listen to transport state changes
func (srsm *StatefulRealtimeSyncManager) SubscribeToTransportStateChanges(handler func(statemachine.StateTransition[statemachine.TransportState])) error {
	return srsm.transportStateManager.SubscribeToStateChanges(handler)
}

// EnhancedClose extends the base close with transport state management cleanup
func (srsm *StatefulRealtimeSyncManager) EnhancedClose() error {
	// Close transport state manager first
	if err := srsm.transportStateManager.Close(); err != nil {
		srsm.logger.Error("Error closing transport state manager", "error", err)
	}
	
	// Call base close
	return srsm.Close()
}

// MonitorTransportHealth continuously monitors transport health and manages reconnections
func (srsm *StatefulRealtimeSyncManager) MonitorTransportHealth(ctx context.Context, checkInterval time.Duration) {
	if checkInterval == 0 {
		checkInterval = 10 * time.Second
	}
	
	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
			
		case <-ticker.C:
			srsm.performHealthCheck(ctx)
		}
	}
}

// performHealthCheck checks transport health and handles reconnection logic
func (srsm *StatefulRealtimeSyncManager) performHealthCheck(ctx context.Context) {
	currentState := srsm.transportStateManager.GetConnectionState()
	
	// Skip health checks for terminal or transitioning states
	if currentState == statemachine.TransportShuttingDown || 
	   currentState == statemachine.TransportConnecting ||
	   currentState == statemachine.TransportReconnecting {
		return
	}
	
	// Check if realtime notifier is actually connected
	if srsm.realtimeOptions.RealtimeNotifier != nil {
		isConnected := srsm.realtimeOptions.RealtimeNotifier.IsConnected()
		
		// If state machine thinks we're connected but we're actually not, trigger reconnection
		if currentState == statemachine.TransportConnected && !isConnected {
			srsm.logger.Warn("Transport state mismatch detected, triggering reconnection")
			
			// Transition to reconnecting state
			if err := srsm.transportStateManager.TransitionToReconnecting(
				"health_check_failed", 1, 2*time.Second); err != nil {
				srsm.logger.Error("Failed to transition to reconnecting state", "error", err)
				return
			}
			
			// Attempt to reconnect
			go srsm.attemptReconnection(ctx)
		}
		
		// If we're in disconnected/failed state but should try to connect
		if (currentState == statemachine.TransportDisconnected || 
		    currentState == statemachine.TransportFailed) && 
		   srsm.IsRealtimeActive() {
			
			srsm.logger.Info("Attempting to reestablish transport connection")
			go srsm.attemptReconnection(ctx)
		}
	}
}

// attemptReconnection attempts to reestablish the transport connection
func (srsm *StatefulRealtimeSyncManager) attemptReconnection(ctx context.Context) {
	start := time.Now()
	
	// Transition to connecting state if not already reconnecting
	currentState := srsm.transportStateManager.GetConnectionState()
	if currentState.CanConnect() {
		if err := srsm.transportStateManager.TransitionToConnecting(30 * time.Second); err != nil {
			srsm.logger.Error("Failed to transition to connecting state during reconnection", "error", err)
			return
		}
	}
	
	// Attempt to resubscribe to notifications
	if srsm.realtimeOptions.RealtimeNotifier != nil {
		// First unsubscribe to clean up any existing connections
		srsm.realtimeOptions.RealtimeNotifier.Unsubscribe()
		
		// Attempt to subscribe again
		err := srsm.realtimeOptions.RealtimeNotifier.Subscribe(ctx, func(notification Notification) error {
			// Handle notification (simplified for this example)
			srsm.logger.Debug("Received notification during reconnection", "type", notification.Type)
			return nil
		})
		
		if err != nil {
			// Transition to failed state
			if failErr := srsm.transportStateManager.TransitionToFailed(err, 1, time.Since(start)); failErr != nil {
				srsm.logger.Error("Failed to transition to failed state", "error", failErr)
			}
			return
		}
		
		// Successful reconnection
		if connErr := srsm.transportStateManager.TransitionToConnected(time.Since(start)); connErr != nil {
			srsm.logger.Error("Failed to transition to connected state after reconnection", "error", connErr)
		}
	}
}

// GetEnhancedConnectionStatus returns enhanced connection status including transport state
func (srsm *StatefulRealtimeSyncManager) GetEnhancedConnectionStatus() map[string]interface{} {
	baseStatus := srsm.GetConnectionStatus()
	transportMeta := srsm.GetTransportConnectionMetadata()
	
	enhanced := map[string]interface{}{
		"base_connected":          baseStatus.Connected,
		"base_last_connected":     baseStatus.LastConnected,
		"base_reconnect_attempts": baseStatus.ReconnectAttempts,
		"base_error":             baseStatus.Error,
	}
	
	// Add transport state information
	for key, value := range transportMeta {
		enhanced["transport_"+key] = value
	}
	
	// Add health status
	enhanced["transport_healthy"] = srsm.IsTransportHealthy()
	enhanced["can_send_data"] = srsm.CanSendRealtimeData()
	enhanced["can_receive_data"] = srsm.CanReceiveRealtimeData()
	
	return enhanced
}
