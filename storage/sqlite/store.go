// Package sqlite provides a SQLite implementation of the go-sync-kit EventStore.
package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"strconv"
	"strings"
	stdSync "sync"
	"time"

	// Import the go-sync-kit interfaces. Use relative import.
	sync "github.com/c0deZ3R0/go-sync-kit"

	// Go SQLite driver
	_ "github.com/mattn/go-sqlite3"
)

// Custom errors for better error handling
var (
	ErrIncompatibleVersion = errors.New("incompatible version type: expected IntegerVersion")
	ErrEventNotFound       = errors.New("event not found")
	ErrStoreClosed         = errors.New("store is closed")
)

// --- Concrete Version Implementation ---

// IntegerVersion implements the sync.Version interface using a simple integer.
type IntegerVersion int64

// Compile-time check to ensure IntegerVersion satisfies the Version interface
var _ sync.Version = IntegerVersion(0)

// Compare compares this version with another.
// Returns -1 if this version is less than other, 1 if greater, and 0 if equal.
func (v IntegerVersion) Compare(other sync.Version) int {
	ov, ok := other.(IntegerVersion)
	if !ok {
		// Cannot compare with a different version type, treat as less.
		return -1
	}
	if v < ov {
		return -1
	}
	if v > ov {
		return 1
	}
	return 0
}

// String returns the string representation of the version.
func (v IntegerVersion) String() string {
	return strconv.FormatInt(int64(v), 10)
}

// IsZero checks if the version is the zero value.
func (v IntegerVersion) IsZero() bool {
	return v == 0
}

// ParseVersion parses a version string into an IntegerVersion.
// This is useful for HTTP transport and other external integrations.
func ParseVersion(s string) (IntegerVersion, error) {
	if s == "" || s == "0" {
		return IntegerVersion(0), nil
	}
	
	val, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return IntegerVersion(0), fmt.Errorf("invalid version string '%s': %w", s, err)
	}
	
	return IntegerVersion(val), nil
}

// --- Concrete Event Implementation for Storage/Retrieval ---

// StoredEvent is a concrete implementation of sync.Event used for retrieving
// events from the database. It holds data and metadata as raw JSON.
type StoredEvent struct {
	id          string
	eventType   string
	aggregateID string
	data        json.RawMessage
	metadata    json.RawMessage
}

func (e *StoredEvent) ID() string          { return e.id }
func (e *StoredEvent) Type() string        { return e.eventType }
func (e *StoredEvent) AggregateID() string { return e.aggregateID }
func (e *StoredEvent) Data() interface{}   { return e.data }
func (e *StoredEvent) Metadata() map[string]interface{} {
	if e.metadata == nil {
		return nil
	}
	var m map[string]interface{}
	// It's acceptable to ignore the error here for the interface contract.
	// The consumer can handle the raw json.RawMessage from Data().
	_ = json.Unmarshal(e.metadata, &m)
	return m
}

// --- Configuration ---

// Config holds configuration options for the SQLiteEventStore.
type Config struct {
	// DataSourceName is the connection string for the SQLite database.
	// For production use, consider enabling WAL mode for better concurrency.
	// Example: "file:events.db?_journal_mode=WAL"
	DataSourceName string

	// EnableWAL enables Write-Ahead Logging mode for better concurrency.
	// This is recommended for production use.
	EnableWAL bool

	// Logger is an optional logger for logging internal operations and errors.
	// If nil, logging is disabled.
	Logger *log.Logger

	// TableName is the name of the table to store events.
	// Defaults to "events" if empty.
	TableName string

	// Connection pool settings
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

// setDefaults applies default values to the config
func (c *Config) setDefaults() {
	if c.TableName == "" {
		c.TableName = "events"
	}
	if c.Logger == nil {
		c.Logger = log.New(io.Discard, "", 0) // Disable logging by default
	}
	if c.MaxOpenConns == 0 {
		c.MaxOpenConns = 25
	}
	if c.MaxIdleConns == 0 {
		c.MaxIdleConns = 5
	}
	if c.ConnMaxLifetime == 0 {
		c.ConnMaxLifetime = time.Hour
	}
	if c.ConnMaxIdleTime == 0 {
		c.ConnMaxIdleTime = 5 * time.Minute
	}
	if c.EnableWAL {
		if !strings.Contains(c.DataSourceName, "?_journal_mode=") {
			c.DataSourceName += "?_journal_mode=WAL"
		}
	}
}

// NewWithDataSource is a convenience constructor
func NewWithDataSource(dataSourceName string) (*SQLiteEventStore, error) {
	config := DefaultConfig(dataSourceName)
	return New(config)
}

// DefaultConfig returns a Config with sensible defaults for SQLite
func DefaultConfig(dataSourceName string) *Config {
	config := &Config{
		DataSourceName: dataSourceName,
	}
	config.setDefaults()
	return config
}

// --- SQLite EventStore Implementation ---

// SQLiteEventStore implements the sync.EventStore interface for SQLite.
type SQLiteEventStore struct {
	db        *sql.DB
	mu        stdSync.RWMutex
	closed    bool
	logger    *log.Logger
	tableName string
}

// Compile-time check to ensure SQLiteEventStore satisfies the EventStore interface
var _ sync.EventStore = (*SQLiteEventStore)(nil)

// New creates a new SQLiteEventStore from a Config.
// If config is nil, DefaultConfig will be used with an empty DataSourceName.
func New(config *Config) (*SQLiteEventStore, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	// Apply defaults
	config.setDefaults()

	if config.DataSourceName == "" {
		return nil, fmt.Errorf("DataSourceName is required")
	}

	config.Logger.Printf("[SQLite EventStore] Opening database: %s", config.DataSourceName)

	db, err := sql.Open("sqlite3", config.DataSourceName)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(config.MaxOpenConns)
	db.SetMaxIdleConns(config.MaxIdleConns)
	db.SetConnMaxLifetime(config.ConnMaxLifetime)
	db.SetConnMaxIdleTime(config.ConnMaxIdleTime)

	config.Logger.Printf("[SQLite EventStore] Connection pool configured: MaxOpen=%d, MaxIdle=%d", 
		config.MaxOpenConns, config.MaxIdleConns)

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to connect to sqlite database: %w", err)
	}

	store := &SQLiteEventStore{
		db:        db,
		logger:    config.Logger,
		tableName: config.TableName,
	}

	if err := store.setupSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to setup database schema: %w", err)
	}

	config.Logger.Printf("[SQLite EventStore] Successfully initialized with table: %s", config.TableName)
	return store, nil
}


// setupSchema creates the 'events' table if it doesn't exist.
func (s *SQLiteEventStore) setupSchema() error {
	query := `
    CREATE TABLE IF NOT EXISTS events (
        version         INTEGER PRIMARY KEY AUTOINCREMENT,
        id              TEXT NOT NULL UNIQUE,
        aggregate_id    TEXT NOT NULL,
        event_type      TEXT NOT NULL,
        data            TEXT,
        metadata        TEXT,
        created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    );
    CREATE INDEX IF NOT EXISTS idx_aggregate_id ON events (aggregate_id);
    CREATE INDEX IF NOT EXISTS idx_version ON events (version);
    CREATE INDEX IF NOT EXISTS idx_created_at ON events (created_at);
    `
	_, err := s.db.Exec(query)
	return err
}

// Store saves an event to the SQLite database.
// Note: This implementation ignores the 'version' parameter and relies on
// SQLite's AUTOINCREMENT to assign a new, sequential version.
func (s *SQLiteEventStore) Store(ctx context.Context, event sync.Event, version sync.Version) error {
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return ErrStoreClosed
	}
	s.mu.RUnlock()

	// Begin transaction for atomicity
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	dataJSON, err := json.Marshal(event.Data())
	if err != nil {
		return fmt.Errorf("failed to marshal event data: %w", err)
	}

	metadataJSON, err := json.Marshal(event.Metadata())
	if err != nil {
		return fmt.Errorf("failed to marshal event metadata: %w", err)
	}

	query := `INSERT INTO events (id, aggregate_id, event_type, data, metadata) VALUES (?, ?, ?, ?, ?)`
	_, err = tx.ExecContext(ctx, query, event.ID(), event.AggregateID(), event.Type(), string(dataJSON), string(metadataJSON))
	if err != nil {
		return fmt.Errorf("failed to insert event: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// Load retrieves all events since a given version.
func (s *SQLiteEventStore) Load(ctx context.Context, since sync.Version) ([]sync.EventWithVersion, error) {
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return nil, ErrStoreClosed
	}
	s.mu.RUnlock()

	sinceVersion, ok := since.(IntegerVersion)
	if !ok && !since.IsZero() {
		return nil, ErrIncompatibleVersion
	}

	query := `SELECT version, id, aggregate_id, event_type, data, metadata FROM events WHERE version > ? ORDER BY version ASC`
	rows, err := s.db.QueryContext(ctx, query, int64(sinceVersion))
	if err != nil {
		return nil, fmt.Errorf("failed to query events: %w", err)
	}
	defer rows.Close()

	return s.scanEvents(rows)
}

// LoadByAggregate retrieves events for a specific aggregate since a given version.
func (s *SQLiteEventStore) LoadByAggregate(ctx context.Context, aggregateID string, since sync.Version) ([]sync.EventWithVersion, error) {
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return nil, ErrStoreClosed
	}
	s.mu.RUnlock()

	sinceVersion, ok := since.(IntegerVersion)
	if !ok && !since.IsZero() {
		return nil, ErrIncompatibleVersion
	}

	query := `SELECT version, id, aggregate_id, event_type, data, metadata FROM events WHERE aggregate_id = ? AND version > ? ORDER BY version ASC`
	rows, err := s.db.QueryContext(ctx, query, aggregateID, int64(sinceVersion))
	if err != nil {
		return nil, fmt.Errorf("failed to query events by aggregate: %w", err)
	}
	defer rows.Close()

	return s.scanEvents(rows)
}

// LatestVersion returns the highest version number in the store.
func (s *SQLiteEventStore) LatestVersion(ctx context.Context) (sync.Version, error) {
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return nil, ErrStoreClosed
	}
	s.mu.RUnlock()

	var maxVersion sql.NullInt64
	query := `SELECT MAX(version) FROM events`
	err := s.db.QueryRowContext(ctx, query).Scan(&maxVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to query latest version: %w", err)
	}

	if !maxVersion.Valid {
		return IntegerVersion(0), nil // No events in store
	}

	return IntegerVersion(maxVersion.Int64), nil
}

// ParseVersion converts a string representation into an IntegerVersion.
// This allows external integrations to handle SQLite's integer versioning gracefully.
func (s *SQLiteEventStore) ParseVersion(ctx context.Context, versionStr string) (sync.Version, error) {
	if versionStr == "" || versionStr == "0" {
		return IntegerVersion(0), nil
	}
	
	val, err := strconv.ParseInt(versionStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid integer version string '%s': %w", versionStr, err)
	}
	
	return IntegerVersion(val), nil
}

// Close closes the database connection.
func (s *SQLiteEventStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil
	}

	s.closed = true
	return s.db.Close()
}

// Stats returns database statistics for monitoring
func (s *SQLiteEventStore) Stats() sql.DBStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return sql.DBStats{}
	}

	return s.db.Stats()
}

// scanEvents is a helper to scan sql.Rows into a slice of EventWithVersion.
func (s *SQLiteEventStore) scanEvents(rows *sql.Rows) ([]sync.EventWithVersion, error) {
	var events []sync.EventWithVersion
	for rows.Next() {
		var version int64
		var data, metadata sql.NullString
		evt := &StoredEvent{}

		if err := rows.Scan(&version, &evt.id, &evt.aggregateID, &evt.eventType, &data, &metadata); err != nil {
			return nil, fmt.Errorf("failed to scan event row: %w", err)
		}

		if data.Valid {
			evt.data = []byte(data.String)
		}
		if metadata.Valid {
			evt.metadata = []byte(metadata.String)
		}

		events = append(events, sync.EventWithVersion{
			Event:   evt,
			Version: IntegerVersion(version),
		})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error during row iteration: %w", err)
	}

	return events, nil
}
