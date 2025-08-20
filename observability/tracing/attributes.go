package tracing

import (
	"go.opentelemetry.io/otel/attribute"
)

// Semantic attribute keys following OpenTelemetry conventions for sync-kit operations.
// These attributes provide structured metadata for spans and enable consistent
// querying and analysis across different observability backends.

var (
	// General component and operation attributes
	ComponentKey = attribute.Key("synckit.component")
	OperationKey = attribute.Key("synckit.operation")
	
	// Sync operation attributes  
	SyncOperationKey     = attribute.Key("synckit.sync.operation")      // "full", "push", "pull"
	SyncPhaseKey         = attribute.Key("synckit.sync.phase")          // "start", "pull", "push", "complete"
	SyncStrategyKey      = attribute.Key("synckit.sync.strategy")       // "bidirectional", "push_only", "pull_only"
	
	// Event attributes
	EventCountKey        = attribute.Key("synckit.events.count")        // Number of events processed
	EventsPushedKey      = attribute.Key("synckit.events.pushed")       // Events sent to remote
	EventsPulledKey      = attribute.Key("synckit.events.pulled")       // Events received from remote
	EventTypeKey         = attribute.Key("synckit.event.type")          // Type of event
	AggregateIDsKey      = attribute.Key("synckit.aggregates.ids")      // List of aggregate IDs
	AggregateTypeKey     = attribute.Key("synckit.aggregate.type")      // Type of aggregate
	
	// Conflict resolution attributes
	ConflictsResolvedKey = attribute.Key("synckit.conflicts.resolved")  // Number of conflicts resolved
	ConflictStrategyKey  = attribute.Key("synckit.conflict.strategy")   // "lww", "fww", "additive", "manual"
	ConflictDecisionKey  = attribute.Key("synckit.conflict.decision")   // Resolution decision made
	ConflictReasonKey    = attribute.Key("synckit.conflict.reason")     // Reason for conflict resolution
	
	// Transport attributes
	TransportOperationKey = attribute.Key("synckit.transport.operation") // "push", "pull", "get_version"
	TransportTypeKey      = attribute.Key("synckit.transport.type")      // "http", "grpc", "nats", "null"
	TransportEndpointKey  = attribute.Key("synckit.transport.endpoint")  // Remote endpoint URL
	TransportMethodKey    = attribute.Key("synckit.transport.method")    // HTTP method, gRPC method, etc.
	
	// Storage attributes  
	StorageOperationKey = attribute.Key("synckit.storage.operation")   // "store", "load", "latest_version"
	StorageTypeKey      = attribute.Key("synckit.storage.type")        // "sqlite", "postgresql", "memory"
	StorageTableKey     = attribute.Key("synckit.storage.table")       // Database table name
	StorageQueryKey     = attribute.Key("synckit.storage.query")       // SQL query or operation
	
	// Performance attributes
	BatchSizeKey        = attribute.Key("synckit.batch.size")          // Batch size for operations
	RetryCountKey       = attribute.Key("synckit.retry.count")         // Number of retry attempts
	TimeoutKey          = attribute.Key("synckit.timeout")             // Operation timeout
	
	// Version attributes
	LocalVersionKey     = attribute.Key("synckit.version.local")       // Local version
	RemoteVersionKey    = attribute.Key("synckit.version.remote")      // Remote version
	VersionTypeKey      = attribute.Key("synckit.version.type")        // "integer", "vector_clock", "timestamp"
	
	// Filter attributes
	FilterAppliedKey    = attribute.Key("synckit.filter.applied")      // Whether filtering was applied
	FilterMatchedKey    = attribute.Key("synckit.filter.matched")      // Number of events that matched filter
	FilterTotalKey      = attribute.Key("synckit.filter.total")        // Total events before filtering
	
	// Error attributes (extending standard OpenTelemetry error attributes)
	ErrorCodeKey        = attribute.Key("synckit.error.code")          // Sync-kit specific error code
	ErrorComponentKey   = attribute.Key("synckit.error.component")     // Component where error occurred
	ErrorRetryableKey   = attribute.Key("synckit.error.retryable")     // Whether error is retryable
	
	// Health check attributes
	HealthCheckTypeKey  = attribute.Key("synckit.health.check_type")   // "liveness", "readiness", "startup"
	HealthStatusKey     = attribute.Key("synckit.health.status")       // "up", "down", "degraded"
	HealthComponentKey  = attribute.Key("synckit.health.component")    // Component being checked
	
	// Real-time sync attributes
	RealtimeEnabledKey     = attribute.Key("synckit.realtime.enabled")     // Whether real-time sync is enabled
	RealtimeNotificationKey = attribute.Key("synckit.realtime.notification") // Type of real-time notification
	RealtimeConnectionKey   = attribute.Key("synckit.realtime.connection")  // Connection status
)

// Standard attribute values for consistent reporting
const (
	// Sync operation values
	SyncOperationFull = "full"
	SyncOperationPush = "push" 
	SyncOperationPull = "pull"
	
	// Sync phase values
	SyncPhaseStart    = "start"
	SyncPhasePull     = "pull"
	SyncPhasePush     = "push" 
	SyncPhaseComplete = "complete"
	
	// Conflict strategy values
	ConflictStrategyLWW       = "last_write_wins"
	ConflictStrategyFWW       = "first_write_wins"
	ConflictStrategyAdditive  = "additive_merge"
	ConflictStrategyManual    = "manual"
	ConflictStrategyCustom    = "custom"
	
	// Transport type values
	TransportTypeHTTP = "http"
	TransportTypeGRPC = "grpc"
	TransportTypeNATS = "nats"
	TransportTypeNull = "null"
	
	// Storage type values  
	StorageTypeSQLite     = "sqlite"
	StorageTypePostgreSQL = "postgresql"
	StorageTypeMemory     = "memory"
	
	// Component values
	ComponentSyncKit         = "synckit"
	ComponentTransport       = "transport"
	ComponentStorage         = "storage"
	ComponentConflictResolver = "conflict-resolver"
	ComponentHealthCheck     = "health-check"
	
	// Health status values
	HealthStatusUp       = "up"
	HealthStatusDown     = "down" 
	HealthStatusDegraded = "degraded"
	HealthStatusUnknown  = "unknown"
	
	// Health check type values
	HealthCheckTypeLiveness  = "liveness"
	HealthCheckTypeReadiness = "readiness"
	HealthCheckTypeStartup   = "startup"
)

// AttributeValidators provides validation functions for attribute values
type AttributeValidators struct{}

// ValidateSyncOperation validates sync operation values
func (av AttributeValidators) ValidateSyncOperation(operation string) string {
	switch operation {
	case SyncOperationFull, SyncOperationPush, SyncOperationPull:
		return operation
	default:
		return SyncOperationFull // Default fallback
	}
}

// ValidateConflictStrategy validates conflict strategy values
func (av AttributeValidators) ValidateConflictStrategy(strategy string) string {
	switch strategy {
	case ConflictStrategyLWW, ConflictStrategyFWW, ConflictStrategyAdditive, ConflictStrategyManual, ConflictStrategyCustom:
		return strategy
	default:
		return ConflictStrategyCustom // Default fallback
	}
}

// ValidateTransportType validates transport type values
func (av AttributeValidators) ValidateTransportType(transportType string) string {
	switch transportType {
	case TransportTypeHTTP, TransportTypeGRPC, TransportTypeNATS, TransportTypeNull:
		return transportType
	default:
		return "unknown"
	}
}

// ValidateStorageType validates storage type values
func (av AttributeValidators) ValidateStorageType(storageType string) string {
	switch storageType {
	case StorageTypeSQLite, StorageTypePostgreSQL, StorageTypeMemory:
		return storageType
	default:
		return "unknown"
	}
}

// SanitizeStringAttribute ensures string attributes don't exceed reasonable limits
func SanitizeStringAttribute(value string, maxLength int) string {
	if len(value) <= maxLength {
		return value
	}
	return value[:maxLength-3] + "..."
}

// SanitizeSliceAttribute ensures slice attributes don't exceed reasonable limits  
func SanitizeSliceAttribute(values []string, maxItems int) []string {
	if len(values) <= maxItems {
		return values
	}
	return values[:maxItems]
}

// DefaultValidators provides a default instance of attribute validators
var DefaultValidators = AttributeValidators{}

// Common attribute combinations for convenience
func SyncOperationAttributes(operation string, batchSize int) []attribute.KeyValue {
	return []attribute.KeyValue{
		SyncOperationKey.String(DefaultValidators.ValidateSyncOperation(operation)),
		BatchSizeKey.Int(batchSize),
		ComponentKey.String(ComponentSyncKit),
	}
}

func TransportAttributes(transportType, operation, endpoint string) []attribute.KeyValue {
	return []attribute.KeyValue{
		TransportTypeKey.String(DefaultValidators.ValidateTransportType(transportType)),
		TransportOperationKey.String(operation),
		TransportEndpointKey.String(SanitizeStringAttribute(endpoint, 100)),
		ComponentKey.String(ComponentTransport),
	}
}

func StorageAttributes(storageType, operation, table string) []attribute.KeyValue {
	return []attribute.KeyValue{
		StorageTypeKey.String(DefaultValidators.ValidateStorageType(storageType)),
		StorageOperationKey.String(operation),
		StorageTableKey.String(table),
		ComponentKey.String(ComponentStorage),
	}
}

func ConflictAttributes(strategy, decision, reason string) []attribute.KeyValue {
	return []attribute.KeyValue{
		ConflictStrategyKey.String(DefaultValidators.ValidateConflictStrategy(strategy)),
		ConflictDecisionKey.String(decision),
		ConflictReasonKey.String(SanitizeStringAttribute(reason, 200)),
		ComponentKey.String(ComponentConflictResolver),
	}
}
