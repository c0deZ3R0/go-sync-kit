package synckit

import (
	"testing"
	"time"
)

// TestConfigValidate tests the Config.Validate() method.
func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid minimal config",
			config: Config{
				Store: &mockEventStore{},
			},
			wantErr: false,
		},
		{
			name: "valid full config",
			config: Config{
				Store:             &mockEventStore{},
				Transport:         &mockTransport{},
				Cursor:            CursorInteger,
				BatchSize:         50,
				Timeout:           30 * time.Second,
				SyncInterval:      1 * time.Minute,
				EnableValidation:  true,
				EnableCompression: true,
				Retry: RetryPolicy{
					Max:    3,
					Base:   100 * time.Millisecond,
					Cap:    5 * time.Second,
					Jitter: true,
				},
			},
			wantErr: false,
		},
		{
			name:    "missing store",
			config:  Config{},
			wantErr: true,
			errMsg:  "Store is required",
		},
		{
			name: "negative timeout",
			config: Config{
				Store:   &mockEventStore{},
				Timeout: -1 * time.Second,
			},
			wantErr: true,
			errMsg:  "Timeout must be non-negative",
		},
		{
			name: "negative batch size",
			config: Config{
				Store:     &mockEventStore{},
				BatchSize: -10,
			},
			wantErr: true,
			errMsg:  "BatchSize must be non-negative",
		},
		{
			name: "negative sync interval",
			config: Config{
				Store:        &mockEventStore{},
				SyncInterval: -1 * time.Minute,
			},
			wantErr: true,
			errMsg:  "SyncInterval must be non-negative",
		},
		{
			name: "pushonly and pullonly mutually exclusive",
			config: Config{
				Store:    &mockEventStore{},
				PushOnly: true,
				PullOnly: true,
			},
			wantErr: true,
			errMsg:  "PushOnly and PullOnly are mutually exclusive",
		},
		{
			name: "invalid retry max",
			config: Config{
				Store: &mockEventStore{},
				Retry: RetryPolicy{
					Max: -2,
				},
			},
			wantErr: true,
			errMsg:  "Retry.Max must be >= -1",
		},
		{
			name: "retry enabled but base zero",
			config: Config{
				Store: &mockEventStore{},
				Retry: RetryPolicy{
					Max:  3,
					Base: 0,
				},
			},
			wantErr: true,
			errMsg:  "Retry.Base must be > 0 when retries are enabled",
		},
		{
			name: "retry cap less than base",
			config: Config{
				Store: &mockEventStore{},
				Retry: RetryPolicy{
					Max:  3,
					Base: 5 * time.Second,
					Cap:  1 * time.Second,
				},
			},
			wantErr: true,
			errMsg:  "Retry.Cap",
		},
		{
			name: "invalid cursor mode",
			config: Config{
				Store:  &mockEventStore{},
				Cursor: CursorMode(99),
			},
			wantErr: true,
			errMsg:  "invalid CursorMode",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				if err == nil {
					t.Errorf("Validate() expected error containing %q, got nil", tt.errMsg)
				} else if tt.errMsg != "" && !containsSubstring(err.Error(), tt.errMsg) {
					t.Errorf("Validate() error = %v, want substring %q", err, tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("Validate() unexpected error = %v", err)
				}
			}
		})
	}
}

// TestNew tests the New(Config) constructor.
func TestNew(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		cfg := Config{
			Store:     &mockEventStore{},
			Transport: &mockTransport{}, // Transport is required by builder
		}
		mgr, err := New(cfg)
		if err != nil {
			t.Fatalf("New() unexpected error = %v", err)
		}
		if mgr == nil {
			t.Error("New() returned nil manager")
		}
	})

	t.Run("invalid config", func(t *testing.T) {
		cfg := Config{
			// Missing Store
		}
		_, err := New(cfg)
		if err == nil {
			t.Error("New() expected error for invalid config, got nil")
		}
	})

	t.Run("config with transport and options", func(t *testing.T) {
		cfg := Config{
			Store:             &mockEventStore{},
			Transport:         &mockTransport{},
			BatchSize:         50,
			Timeout:           10 * time.Second,
			EnableValidation:  true,
			EnableCompression: true,
		}
		mgr, err := New(cfg)
		if err != nil {
			t.Fatalf("New() unexpected error = %v", err)
		}
		if mgr == nil {
			t.Error("New() returned nil manager")
		}
	})
}

// TestNewManagerFromConfig tests the internal constructor.
func TestNewManagerFromConfig(t *testing.T) {
	cfg := Config{
		Store:     &mockEventStore{},
		Transport: &mockTransport{},
		BatchSize: 100,
		Timeout:   5 * time.Second,
	}

	mgr, err := newManagerFromConfig(cfg)
	if err != nil {
		t.Fatalf("newManagerFromConfig() unexpected error = %v", err)
	}
	if mgr == nil {
		t.Error("newManagerFromConfig() returned nil manager")
	}
}

// Helper function
func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || 
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
