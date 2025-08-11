// Package postgres provides a PostgreSQL implementation of the go-sync-kit EventStore
// with real-time LISTEN/NOTIFY capabilities for event streaming.
package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"log/slog"
	"strconv"
	"strings"
	stdSync "sync"
	"time"

	"github.com/c0deZ3R0/go-sync-kit/cursor"
	syncErrors "github.com/c0deZ3R0/go-sync-kit/errors"
	"github.com/c0deZ3R0/go-sync-kit/logging"
	"github.com/c0deZ3R0/go-sync-kit/synckit"

	// PostgreSQL driver
	_ "github.com/lib/pq"
)

// Custom errors for better error handling
var (
	ErrIncompatibleVersion = errors.New("incompatible version type: expected cursor.IntegerCursor")
	ErrEventNotFound       = errors.New("event not found")
	ErrStoreClosed         = errors.New("store is closed")
	ErrInvalidConnection   = errors.New("invalid database connection")
)

// ParseVersion parses a version string into a cursor.IntegerCursor.
// This is useful for HTTP transport and other external integrations.
func ParseVersion(s string) (cursor.IntegerCursor, error) {
	if s == "" || s == "0" {
		return cursor.IntegerCursor{Seq: 0}, nil
	}

	val, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return cursor.IntegerCursor{}, fmt.Errorf("invalid version string '%s': %w", s, err)
	}

	return cursor.IntegerCursor{Seq: uint64(val)}, nil
}

// StoredEvent is a concrete implementation of synckit.Event used for retrieving
// events from the database. It holds data and metadata as JSONB.
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

// Config holds configuration options for the PostgresEventStore.
//
// Production-ready defaults are applied by DefaultConfig() including:
//   - Connection pool with 25 max open, 10 max idle connections
//   - Connection lifetimes of 1 hour max, 15 minutes max idle
//   - LISTEN/NOTIFY timeout of 30 seconds
//   - Reconnection with exponential backoff
type Config struct {
	// ConnectionString is the PostgreSQL connection string.
	// Example: "postgres://user:password@localhost/dbname?sslmode=require"
	ConnectionString string

	// Logger is an optional logger for logging internal operations and errors.
	// If nil, logging is disabled by default (logs to io.Discard).
	Logger *log.Logger

	// TableName is the name of the table to store events.
	// Defaults to "events" if empty.
	TableName string

	// Connection pool settings for production workloads.
	// Defaults: MaxOpen=25, MaxIdle=10, Lifetime=1h, IdleTime=15m
	MaxOpenConns    int           // Default: 25 - Maximum number of open connections
	MaxIdleConns    int           // Default: 10 - Maximum number of idle connections
	ConnMaxLifetime time.Duration // Default: 1h - Maximum lifetime of connections
	ConnMaxIdleTime time.Duration // Default: 15m - Maximum idle time before closing

	// LISTEN/NOTIFY settings for real-time capabilities
	NotificationTimeout    time.Duration // Default: 30s - Timeout for waiting on notifications
	ReconnectInterval      time.Duration // Default: 5s - Interval between reconnection attempts
	MaxReconnectAttempts   int           // Default: 10 - Maximum reconnection attempts before giving up

	// Performance tuning
	BatchSize           int  // Default: 1000 - Batch size for bulk operations
	EnablePreparedStmts bool // Default: true - Enable prepared statements for better performance

	// Monitoring and observability
	EnableMetrics bool // Default: false - Enable metrics collection
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
		c.MaxIdleConns = 10
	}
	if c.ConnMaxLifetime == 0 {
		c.ConnMaxLifetime = time.Hour
	}
	if c.ConnMaxIdleTime == 0 {
		c.ConnMaxIdleTime = 15 * time.Minute
	}
	if c.NotificationTimeout == 0 {
		c.NotificationTimeout = 30 * time.Second
	}
	if c.ReconnectInterval == 0 {
		c.ReconnectInterval = 5 * time.Second
	}
	if c.MaxReconnectAttempts == 0 {
		c.MaxReconnectAttempts = 10
	}
	if c.BatchSize == 0 {
		c.BatchSize = 1000
	}
	if !c.EnablePreparedStmts {
		c.EnablePreparedStmts = true // Enable by default
	}
}

// NewWithConnectionString is a convenience constructor
func NewWithConnectionString(connectionString string) (*PostgresEventStore, error) {
	config := DefaultConfig(connectionString)
	return New(config)
}

// DefaultConfig returns a Config with production-ready defaults for PostgreSQL.
//
// Default settings include:
//   - Connection pool: 25 max open, 10 max idle connections
//   - Connection lifetime: 1 hour max, 15 minutes max idle
//   - LISTEN/NOTIFY timeout: 30 seconds
//   - Table name: "events"
//   - Logging disabled (to io.Discard)
//   - Prepared statements enabled
func DefaultConfig(connectionString string) *Config {
	config := &Config{
		ConnectionString: connectionString,
	}
	config.setDefaults()
	return config
}

// PostgresEventStore implements the synckit.EventStore interface for PostgreSQL
// with additional LISTEN/NOTIFY capabilities for real-time event streaming.
type PostgresEventStore struct {
	db        *sql.DB
	mu        stdSync.RWMutex
	closed    bool
	logger    *log.Logger
	tableName string
	config    *Config

	// LISTEN/NOTIFY components
	listener *NotificationListener
}

// Compile-time check to ensure PostgresEventStore satisfies the EventStore interface
var _ synckit.EventStore = (*PostgresEventStore)(nil)

// New creates a new PostgresEventStore from a Config.
// If config is nil, returns an error.
func New(config *Config) (*PostgresEventStore, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	// Apply defaults
	config.setDefaults()

	if config.ConnectionString == "" {
		return nil, fmt.Errorf("ConnectionString is required")
	}

	logger := logging.WithComponent(logging.Component("postgres-store"))
	logger.InfoContext(context.Background(), "Opening PostgreSQL database",
		slog.String("data_source", maskConnectionString(config.ConnectionString)),
		slog.String("table_name", config.TableName),
	)

	db, err := sql.Open("postgres", config.ConnectionString)
	if err != nil {
		return nil, fmt.Errorf("failed to open postgres database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(config.MaxOpenConns)
	db.SetMaxIdleConns(config.MaxIdleConns)
	db.SetConnMaxLifetime(config.ConnMaxLifetime)
	db.SetConnMaxIdleTime(config.ConnMaxIdleTime)

	logger.InfoContext(context.Background(), "Connection pool configured",
		slog.Int("max_open_conns", config.MaxOpenConns),
		slog.Int("max_idle_conns", config.MaxIdleConns),
		slog.Duration("conn_max_lifetime", config.ConnMaxLifetime),
		slog.Duration("conn_max_idle_time", config.ConnMaxIdleTime),
	)

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to connect to postgres database: %w", err)
	}

	store := &PostgresEventStore{
		db:        db,
		logger:    config.Logger,
		tableName: config.TableName,
		config:    config,
	}

	if err := store.setupSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to setup database schema: %w", err)
	}

	// Initialize the notification listener
	store.listener, err = NewNotificationListener(config.ConnectionString, config.Logger)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create notification listener: %w", err)
	}

	logger.InfoContext(context.Background(), "PostgreSQL EventStore successfully initialized",
		slog.String("table_name", config.TableName),
		slog.Bool("listen_notify_enabled", true),
	)
	return store, nil
}

// maskConnectionString masks sensitive information in connection strings for logging
func maskConnectionString(connStr string) string {
	// Simple masking - in production, use a more robust approach
	if strings.Contains(connStr, "password=") {
		parts := strings.Split(connStr, " ")
		for i, part := range parts {
			if strings.HasPrefix(part, "password=") {
				parts[i] = "password=***"
			}
		}
		return strings.Join(parts, " ")
	}
	return connStr
}

// setupSchema creates the events table and related objects if they don't exist.
func (s *PostgresEventStore) setupSchema() error {
	// Read the migration script
	migrationSQL := `
-- Create events table with enhanced features for PostgreSQL
CREATE TABLE IF NOT EXISTS events (
    version         BIGSERIAL PRIMARY KEY,
    id              TEXT NOT NULL UNIQUE,
    aggregate_id    TEXT NOT NULL,
    event_type      TEXT NOT NULL,
    data            JSONB,
    metadata        JSONB,
    created_at      TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    stream_name     TEXT GENERATED ALWAYS AS ('stream_' || aggregate_id) STORED
);

-- Indexes for optimal query performance
CREATE INDEX IF NOT EXISTS idx_events_aggregate_id ON events (aggregate_id);
CREATE INDEX IF NOT EXISTS idx_events_version ON events (version);
CREATE INDEX IF NOT EXISTS idx_events_created_at ON events (created_at);
CREATE INDEX IF NOT EXISTS idx_events_stream_name ON events (stream_name);
CREATE INDEX IF NOT EXISTS idx_events_type ON events (event_type);
CREATE INDEX IF NOT EXISTS idx_events_data_gin ON events USING GIN (data);
CREATE INDEX IF NOT EXISTS idx_events_metadata_gin ON events USING GIN (metadata);

-- Function to send notifications when events are inserted
CREATE OR REPLACE FUNCTION notify_event_inserted()
RETURNS TRIGGER AS $$
BEGIN
    -- Notify on the specific stream channel for aggregate-level subscriptions
    PERFORM pg_notify(
        NEW.stream_name, 
        json_build_object(
            'version', NEW.version,
            'id', NEW.id,
            'aggregate_id', NEW.aggregate_id,
            'event_type', NEW.event_type,
            'created_at', NEW.created_at
        )::text
    );
    
    -- Notify on the global events channel for system-wide subscriptions
    PERFORM pg_notify(
        'events_global', 
        json_build_object(
            'version', NEW.version,
            'id', NEW.id,
            'aggregate_id', NEW.aggregate_id,
            'event_type', NEW.event_type,
            'stream_name', NEW.stream_name,
            'created_at', NEW.created_at
        )::text
    );
    
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Trigger to fire notifications after event insertion
DROP TRIGGER IF EXISTS events_notify_trigger ON events;
CREATE TRIGGER events_notify_trigger
    AFTER INSERT ON events
    FOR EACH ROW
    EXECUTE FUNCTION notify_event_inserted();
`

	_, err := s.db.Exec(migrationSQL)
	return err
}

// Store saves an event to the PostgreSQL database.
// The version parameter is ignored as PostgreSQL uses BIGSERIAL for auto-incrementing versions.
// Upon successful insert, this triggers PostgreSQL LISTEN/NOTIFY for real-time subscribers.
func (s *PostgresEventStore) Store(ctx context.Context, event synckit.Event, version synckit.Version) error {
	// Check for context cancellation at start
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Check context deadline
	if deadline, ok := ctx.Deadline(); ok {
		if time.Until(deadline) < time.Second {
			return fmt.Errorf("context deadline too close: %v remaining", time.Until(deadline))
		}
	}

	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return ErrStoreClosed
	}
	s.mu.RUnlock()

	// Begin transaction for atomicity
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return syncErrors.NewWithComponent(syncErrors.OpStore, "postgres", fmt.Errorf("failed to begin transaction: %w", err))
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	dataJSON, err := json.Marshal(event.Data())
	if err != nil {
		return syncErrors.NewWithComponent(syncErrors.OpStore, "postgres", fmt.Errorf("failed to marshal event data: %w", err))
	}

	metadataJSON, err := json.Marshal(event.Metadata())
	if err != nil {
		return syncErrors.NewWithComponent(syncErrors.OpStore, "postgres", fmt.Errorf("failed to marshal event metadata: %w", err))
	}

	query := `INSERT INTO events (id, aggregate_id, event_type, data, metadata) VALUES ($1, $2, $3, $4, $5)`
	_, err = tx.ExecContext(ctx, query, event.ID(), event.AggregateID(), event.Type(), dataJSON, metadataJSON)
	if err != nil {
		return syncErrors.NewWithComponent(syncErrors.OpStore, "postgres", fmt.Errorf("failed to insert event: %w", err))
	}

	if err = tx.Commit(); err != nil {
		return syncErrors.NewWithComponent(syncErrors.OpStore, "postgres", fmt.Errorf("failed to commit transaction: %w", err))
	}

	s.logger.Printf("[Postgres EventStore] Stored event %s for aggregate %s", event.ID(), event.AggregateID())
	return nil
}

// Load retrieves all events since a given version.
func (s *PostgresEventStore) Load(ctx context.Context, since synckit.Version) ([]synckit.EventWithVersion, error) {
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return nil, ErrStoreClosed
	}
	s.mu.RUnlock()

	sinceCursor, ok := since.(cursor.IntegerCursor)
	if !ok && !since.IsZero() {
		return nil, ErrIncompatibleVersion
	}

	query := `SELECT version, id, aggregate_id, event_type, data, metadata FROM events WHERE version > $1 ORDER BY version ASC`
	rows, err := s.db.QueryContext(ctx, query, int64(sinceCursor.Seq))
	if err != nil {
		return nil, syncErrors.NewWithComponent(syncErrors.OpLoad, "postgres", fmt.Errorf("failed to query events: %w", err))
	}
	defer rows.Close()

	return s.scanEvents(rows)
}

// LoadByAggregate retrieves events for a specific aggregate since a given version.
func (s *PostgresEventStore) LoadByAggregate(ctx context.Context, aggregateID string, since synckit.Version) ([]synckit.EventWithVersion, error) {
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return nil, ErrStoreClosed
	}
	s.mu.RUnlock()

	sinceCursor, ok := since.(cursor.IntegerCursor)
	if !ok && !since.IsZero() {
		return nil, ErrIncompatibleVersion
	}

	query := `SELECT version, id, aggregate_id, event_type, data, metadata FROM events WHERE aggregate_id = $1 AND version > $2 ORDER BY version ASC`
	rows, err := s.db.QueryContext(ctx, query, aggregateID, int64(sinceCursor.Seq))
	if err != nil {
		return nil, syncErrors.NewWithComponent(syncErrors.OpLoad, "postgres", fmt.Errorf("failed to query events by aggregate: %w", err))
	}
	defer rows.Close()

	return s.scanEvents(rows)
}

// LatestVersion returns the highest version number in the store.
func (s *PostgresEventStore) LatestVersion(ctx context.Context) (synckit.Version, error) {
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
		return nil, syncErrors.NewWithComponent(syncErrors.OpLoad, "postgres", fmt.Errorf("failed to query latest version: %w", err))
	}

	if !maxVersion.Valid {
		return cursor.IntegerCursor{Seq: 0}, nil // No events in store
	}

	return cursor.IntegerCursor{Seq: uint64(maxVersion.Int64)}, nil
}

// ParseVersion converts a string representation into a cursor.IntegerCursor.
// This allows external integrations to handle PostgreSQL's integer versioning gracefully.
func (s *PostgresEventStore) ParseVersion(ctx context.Context, versionStr string) (synckit.Version, error) {
	if versionStr == "" || versionStr == "0" {
		return cursor.IntegerCursor{Seq: 0}, nil
	}

	val, err := strconv.ParseInt(versionStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid integer version string '%s': %w", versionStr, err)
	}

	return cursor.IntegerCursor{Seq: uint64(val)}, nil
}

// Close closes the database connection and stops the notification listener.
func (s *PostgresEventStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil
	}

	s.closed = true

	// Close the notification listener first
	if s.listener != nil {
		if err := s.listener.Close(); err != nil {
			s.logger.Printf("[Postgres EventStore] Error closing notification listener: %v", err)
		}
	}

	return s.db.Close()
}

// StoreBatch stores multiple events in a single transaction for better performance.
func (s *PostgresEventStore) StoreBatch(ctx context.Context, events []synckit.EventWithVersion) error {
	// Check context deadline
	if deadline, ok := ctx.Deadline(); ok {
		if time.Until(deadline) < time.Second {
			return fmt.Errorf("context deadline too close: %v remaining", time.Until(deadline))
		}
	}

	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return ErrStoreClosed
	}
	s.mu.RUnlock()

	if len(events) == 0 {
		return nil // Nothing to store
	}

	// Begin transaction for atomicity
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin batch transaction: %w", err)
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	// Prepare statement for batch inserts
	stmt, err := tx.PrepareContext(ctx,
		`INSERT INTO events (id, aggregate_id, event_type, data, metadata) VALUES ($1, $2, $3, $4, $5)`)
	if err != nil {
		return fmt.Errorf("failed to prepare batch statement: %w", err)
	}
	defer stmt.Close()

	// Process each event
	for _, ev := range events {
		dataJSON, err := json.Marshal(ev.Event.Data())
		if err != nil {
			return fmt.Errorf("failed to marshal event data: %w", err)
		}

		metadataJSON, err := json.Marshal(ev.Event.Metadata())
		if err != nil {
			return fmt.Errorf("failed to marshal event metadata: %w", err)
		}

		_, err = stmt.ExecContext(ctx,
			ev.Event.ID(),
			ev.Event.AggregateID(),
			ev.Event.Type(),
			dataJSON,
			metadataJSON)
		if err != nil {
			return fmt.Errorf("failed to insert event: %w", err)
		}
	}

	// Commit the transaction
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit batch transaction: %w", err)
	}

	s.logger.Printf("[Postgres EventStore] Stored batch of %d events", len(events))
	return nil
}

// Stats returns database statistics for monitoring
func (s *PostgresEventStore) Stats() sql.DBStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return sql.DBStats{}
	}

	return s.db.Stats()
}

// scanEvents is a helper to scan sql.Rows into a slice of EventWithVersion.
func (s *PostgresEventStore) scanEvents(rows *sql.Rows) ([]synckit.EventWithVersion, error) {
	var events []synckit.EventWithVersion
	for rows.Next() {
		var version int64
		var data, metadata []byte
		evt := &StoredEvent{}

		if err := rows.Scan(&version, &evt.id, &evt.aggregateID, &evt.eventType, &data, &metadata); err != nil {
			return nil, fmt.Errorf("failed to scan event row: %w", err)
		}

		if data != nil {
			evt.data = data
		}
		if metadata != nil {
			evt.metadata = metadata
		}

		events = append(events, synckit.EventWithVersion{
			Event:   evt,
			Version: cursor.IntegerCursor{Seq: uint64(version)},
		})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error during row iteration: %w", err)
	}

	return events, nil
}
