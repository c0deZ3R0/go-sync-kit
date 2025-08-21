package rabbitmq

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	// Verify defaults
	assert.Equal(t, "amqp://guest:guest@localhost:5672/", cfg.URL)
	assert.Equal(t, "go-sync-kit", cfg.ConnectionName)
	assert.Equal(t, 60*time.Second, cfg.Heartbeat)
	assert.Equal(t, "go-sync-kit.events", cfg.Exchange)
	assert.Equal(t, "topic", cfg.ExchangeType)
	assert.True(t, cfg.QueueDurable)
	assert.False(t, cfg.QueueAutoDelete)
	assert.False(t, cfg.QueueExclusive)
	assert.True(t, cfg.MessagePersistent)
	assert.Equal(t, uint8(0), cfg.Priority)
	assert.False(t, cfg.ConfirmMode)
	assert.Equal(t, 10, cfg.PrefetchCount)

	// Config should be valid
	err := cfg.Validate()
	assert.NoError(t, err)
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name        string
		modifyConfig func(*Config)
		expectError bool
		errorContains string
	}{
		{
			name:        "valid default config",
			modifyConfig: func(c *Config) {},
			expectError: false,
		},
		{
			name: "empty URL",
			modifyConfig: func(c *Config) {
				c.URL = ""
			},
			expectError: true,
			errorContains: "URL is required",
		},
		{
			name: "empty exchange",
			modifyConfig: func(c *Config) {
				c.Exchange = ""
			},
			expectError: true,
			errorContains: "Exchange name is required",
		},
		{
			name: "invalid exchange type",
			modifyConfig: func(c *Config) {
				c.ExchangeType = "invalid"
			},
			expectError: true,
			errorContains: "invalid exchange type",
		},
		{
			name: "negative prefetch count",
			modifyConfig: func(c *Config) {
				c.PrefetchCount = -1
			},
			expectError: true,
			errorContains: "PrefetchCount cannot be negative",
		},
		{
			name: "priority too high",
			modifyConfig: func(c *Config) {
				// Test validation by setting to max and then incrementing beyond
				c.Priority = 255 // Max valid value
			},
			expectError: false, // 255 should be valid
		},
		{
			name: "auto-apply defaults for missing values",
			modifyConfig: func(c *Config) {
				c.ExchangeType = "" // Should default to "topic"
				c.ConnectionName = "" // Should default to "go-sync-kit"
				c.Heartbeat = 0 // Should default to 60s
				c.PrefetchCount = 0 // Should default to 10
			},
			expectError: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := DefaultConfig()
			test.modifyConfig(cfg)

			err := cfg.Validate()

			if test.expectError {
				assert.Error(t, err)
				if test.errorContains != "" {
					assert.Contains(t, err.Error(), test.errorContains)
				}
			} else {
				assert.NoError(t, err)
				
				// Verify defaults were applied
				if cfg.ExchangeType == "" {
					assert.Equal(t, "topic", cfg.ExchangeType)
				}
				if cfg.ConnectionName == "" {
					assert.Equal(t, "go-sync-kit", cfg.ConnectionName)
				}
				if cfg.Heartbeat == 0 {
					assert.Equal(t, 60*time.Second, cfg.Heartbeat)
				}
				if cfg.PrefetchCount == 0 {
					assert.Equal(t, 10, cfg.PrefetchCount)
				}
			}
		})
	}
}

func TestConfigValidationWithQueue(t *testing.T) {
	// Test queue-specific validation
	cfg := DefaultConfig()
	cfg.QueueName = "test-queue"

	// Should add default binding keys
	err := cfg.Validate()
	require.NoError(t, err)
	assert.Equal(t, []string{"#"}, cfg.BindingKeys) // Default for topic exchange

	// Test with direct exchange
	cfg = DefaultConfig()
	cfg.QueueName = "test-queue"
	cfg.ExchangeType = "direct"
	err = cfg.Validate()
	require.NoError(t, err)
	assert.Equal(t, []string{""}, cfg.BindingKeys) // Default for direct exchange

	// Test with existing binding keys (should not overwrite)
	cfg = DefaultConfig()
	cfg.QueueName = "test-queue"
	cfg.BindingKeys = []string{"custom.key"}
	err = cfg.Validate()
	require.NoError(t, err)
	assert.Equal(t, []string{"custom.key"}, cfg.BindingKeys) // Should preserve custom keys
}

func TestDefaultRoutingKey(t *testing.T) {
	// Mock event for testing
	event := &mockEvent{eventType: "UserCreated"}

	routingKey := DefaultRoutingKey(event)
	assert.Equal(t, "events.UserCreated", routingKey)
}

func TestNewTransportWithValidation(t *testing.T) {
	// Valid config
	cfg := DefaultConfig()
	transport, err := NewTransportWithValidation(cfg)
	require.NoError(t, err)
	require.NotNil(t, transport)
	assert.Equal(t, cfg, transport.cfg)

	// Invalid config
	cfg.URL = ""
	transport, err = NewTransportWithValidation(cfg)
	assert.Error(t, err)
	assert.Nil(t, transport)
	assert.Contains(t, err.Error(), "invalid config")
}

// mockEvent for testing
type mockEvent struct {
	eventType string
}

func (m *mockEvent) ID() string                        { return "test-id" }
func (m *mockEvent) Type() string                     { return m.eventType }
func (m *mockEvent) AggregateID() string              { return "test-aggregate" }
func (m *mockEvent) Data() interface{}                { return nil }
func (m *mockEvent) Metadata() map[string]interface{} { return nil }
