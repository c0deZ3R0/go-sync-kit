package tracing

import (
	"reflect"
	"strings"
	"testing"

	"go.opentelemetry.io/otel/attribute"
)

func TestSyncKitAttributes(t *testing.T) {
	// Test that core attribute keys are defined
	expectedKeys := []string{
		"synckit.component",
		"synckit.operation", 
		"synckit.sync.operation",
		"synckit.sync.phase",
		"synckit.events.count",
		"synckit.events.pushed",
		"synckit.events.pulled",
		"synckit.aggregates.ids",
		"synckit.conflicts.resolved",
		"synckit.conflict.strategy",
		"synckit.transport.operation",
		"synckit.transport.type",
		"synckit.storage.operation",
		"synckit.storage.type",
		"synckit.health.status",
	}

	// Create map of actual keys
	actualKeys := map[string]bool{
		string(ComponentKey):         true,
		string(OperationKey):         true,
		string(SyncOperationKey):     true,
		string(SyncPhaseKey):         true,
		string(EventCountKey):        true,
		string(EventsPushedKey):      true,
		string(EventsPulledKey):      true,
		string(AggregateIDsKey):      true,
		string(ConflictsResolvedKey): true,
		string(ConflictStrategyKey):  true,
		string(TransportOperationKey): true,
		string(TransportTypeKey):     true,
		string(StorageOperationKey):  true,
		string(StorageTypeKey):       true,
		string(HealthStatusKey):      true,
	}

	// Check all expected keys exist
	for _, key := range expectedKeys {
		if !actualKeys[key] {
			t.Errorf("Missing attribute key: %s", key)
		}
	}
}

func TestValidators(t *testing.T) {
	validators := DefaultValidators

	// Test sync operation validation
	if validators.ValidateSyncOperation("full") != "full" {
		t.Error("Failed to validate sync operation 'full'")
	}

	if validators.ValidateSyncOperation("invalid") != "full" {
		t.Error("Should return default 'full' for invalid sync operation")
	}

	// Test conflict strategy validation
	if validators.ValidateConflictStrategy("last_write_wins") != "last_write_wins" {
		t.Error("Failed to validate conflict strategy 'last_write_wins'")
	}

	if validators.ValidateConflictStrategy("invalid") != "custom" {
		t.Error("Should return default 'custom' for invalid conflict strategy")
	}

	// Test transport type validation
	if validators.ValidateTransportType("http") != "http" {
		t.Error("Failed to validate transport type 'http'")
	}

	if validators.ValidateTransportType("invalid") != "unknown" {
		t.Error("Should return 'unknown' for invalid transport type")
	}

	// Test storage type validation
	if validators.ValidateStorageType("sqlite") != "sqlite" {
		t.Error("Failed to validate storage type 'sqlite'")
	}

	if validators.ValidateStorageType("invalid") != "unknown" {
		t.Error("Should return 'unknown' for invalid storage type")
	}
}

func TestSanitizeStringAttribute(t *testing.T) {
	shortString := "short"
	result := SanitizeStringAttribute(shortString, 10)
	if result != shortString {
		t.Errorf("Expected '%s', got '%s'", shortString, result)
	}

	longString := "this is a very long string that exceeds the limit"
	result = SanitizeStringAttribute(longString, 10)
	if len(result) != 10 || !strings.HasSuffix(result, "...") {
		t.Errorf("Expected truncated string with '...', got '%s'", result)
	}
}

func TestSanitizeSliceAttribute(t *testing.T) {
	shortSlice := []string{"a", "b", "c"}
	result := SanitizeSliceAttribute(shortSlice, 5)
	if len(result) != 3 || !reflect.DeepEqual(result, shortSlice) {
		t.Errorf("Expected %v, got %v", shortSlice, result)
	}

	longSlice := []string{"a", "b", "c", "d", "e", "f", "g"}
	result = SanitizeSliceAttribute(longSlice, 3)
	if len(result) != 3 {
		t.Errorf("Expected slice of length 3, got length %d", len(result))
	}

	expected := []string{"a", "b", "c"}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Expected %v, got %v", expected, result)
	}
}

func TestAttributeHelpers(t *testing.T) {
	// Test SyncOperationAttributes
	attrs := SyncOperationAttributes("pull", 100)
	if len(attrs) != 3 {
		t.Errorf("Expected 3 attributes, got %d", len(attrs))
	}

	// Test TransportAttributes
	attrs = TransportAttributes("http", "push", "https://example.com/sync")
	if len(attrs) != 4 {
		t.Errorf("Expected 4 attributes, got %d", len(attrs))
	}

	// Test StorageAttributes
	attrs = StorageAttributes("sqlite", "store", "events")
	if len(attrs) != 4 {
		t.Errorf("Expected 4 attributes, got %d", len(attrs))
	}

	// Test ConflictAttributes
	attrs = ConflictAttributes("last_write_wins", "local_wins", "local version is newer")
	if len(attrs) != 4 {
		t.Errorf("Expected 4 attributes, got %d", len(attrs))
	}
}

func TestAttributeKeyTypes(t *testing.T) {
	// Test that core keys are of the correct type and not empty
	keys := []attribute.Key{
		ComponentKey,
		OperationKey,
		SyncOperationKey,
		SyncPhaseKey,
		EventCountKey,
		EventsPushedKey,
		EventsPulledKey,
		AggregateIDsKey,
		ConflictsResolvedKey,
		ConflictStrategyKey,
		TransportOperationKey,
		TransportTypeKey,
		StorageOperationKey,
		StorageTypeKey,
		HealthStatusKey,
	}

	for _, key := range keys {
		if string(key) == "" {
			t.Errorf("Attribute key should not be empty")
		}
	}
}
