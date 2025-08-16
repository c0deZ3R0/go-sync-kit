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
	"log/slog"
	"strconv"
	"strings"
	stdSync "sync"
	"time"

	"github.com/c0deZ3R0/go-sync-kit/cursor"
	syncErrors "github.com/c0deZ3R0/go-sync-kit/errors"
	"github.com/c0deZ3R0/go-sync-kit/logging"
	"github.com/c0deZ3R0/go-sync-kit/synckit"

	// Go SQLite driver
	_ "github.com/mattn/go-sqlite3"
)

// Operation constants for consistent error reporting
const (
	opStore         = "sqlite.Store"
	opStoreBatch    = "sqlite.StoreBatch"
	opLoad          = "sqlite.Load"
	opLoadByAggregate = "sqlite.LoadByAggregate"
	opLatestVersion = "sqlite.LatestVersion"
	opParseVersion  = "sqlite.ParseVersion"
)

// Custom errors for better error handling
var (
	ErrIncompatibleVersion = errors.New("incompatible version type: expected cursor.IntegerCursor")
	ErrEventNotFound       = errors.New("event not found")
	ErrStoreClosed         = errors.New("store is closed")
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

// Config holds configuration options for the SQLiteEventStore.
//
// Production-ready defaults are applied by DefaultConfig() including:
//   - WAL mode enabled for better concurrency
//   - Connection pool with 25 max open, 5 max idle connections
//   - Connection lifetimes of 1 hour max, 5 minutes max idle
type Config struct {
	// DataSourceName is the connection string for the SQLite database.
	// For production use, consider enabling WAL mode for better concurrency.
	// Example: "file:events.db?_journal_mode=WAL"
	DataSourceName string

	// EnableWAL enables Write-Ahead Logging mode for better concurrency.
	// This is recommended for production use and is enabled by default.
	// When true, automatically appends "?_journal_mode=WAL" to DataSourceName.
	EnableWAL bool

	// Logger is an optional logger for logging internal operations and errors.
	// If nil, logging is disabled by default (logs to io.Discard).
	Logger *log.Logger

	// TableName is the name of the table to store events.
	// Defaults to "events" if empty.
	TableName string

	// Connection pool settings for production workloads.
	// Defaults: MaxOpen=25, MaxIdle=5, Lifetime=1h, IdleTime=5m
	MaxOpenConns    int           // Default: 25 - Maximum number of open connections
	MaxIdleConns    int           // Default: 5  - Maximum number of idle connections
	ConnMaxLifetime time.Duration // Default: 1h - Maximum lifetime of connections
	ConnMaxIdleTime time.Duration // Default: 5m - Maximum idle time before closing
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

// DefaultConfig returns a Config with production-ready defaults for SQLite.
//
// Default settings include:
//   - WAL mode enabled for better concurrency
//   - Connection pool: 25 max open, 5 max idle connections
//   - Connection lifetime: 1 hour max, 5 minutes max idle
//   - Table name: "events"
//   - Logging disabled (to io.Discard)
func DefaultConfig(dataSourceName string) *Config {
	config := &Config{
		DataSourceName: dataSourceName,
		// Enable WAL mode by default for production readiness
		EnableWAL: true,
	}
	config.setDefaults()
	return config
}

// SQLiteEventStore implements the sync.EventStore interface for SQLite.
type SQLiteEventStore struct {
	db        *sql.DB
	mu        stdSync.RWMutex
	closed    bool
	logger    *log.Logger
	tableName string
}

// Compile-time check to ensure SQLiteEventStore satisfies the EventStore interface
var _ synckit.EventStore = (*SQLiteEventStore)(nil)

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

	logger := logging.WithComponent(logging.Component("sqlite-store"))
	logger.InfoContext(context.Background(), "Opening SQLite database",
		slog.String("data_source", config.DataSourceName),
		slog.Bool("wal_enabled", config.EnableWAL),
	)

	db, err := sql.Open("sqlite3", config.DataSourceName)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database: %w", err)
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

	logger.InfoContext(context.Background(), "SQLite EventStore successfully initialized",
		slog.String("table_name", config.TableName),
	)
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
func (s *SQLiteEventStore) Store(ctx context.Context, event synckit.Event, version synckit.Version) error {
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
		return syncErrors.WrapOpComponent(err, opStore, "storage/sqlite")
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	dataJSON, err := json.Marshal(event.Data())
	if err != nil {
		return syncErrors.WrapOpComponent(err, opStore, "storage/sqlite")
	}

	metadataJSON, err := json.Marshal(event.Metadata())
	if err != nil {
		return syncErrors.WrapOpComponent(err, opStore, "storage/sqlite")
	}

	query := `INSERT INTO events (id, aggregate_id, event_type, data, metadata) VALUES (?, ?, ?, ?, ?)`
	_, err = tx.ExecContext(ctx, query, event.ID(), event.AggregateID(), event.Type(), string(dataJSON), string(metadataJSON))
	if err != nil {
		return syncErrors.WrapOpComponent(err, opStore, "storage/sqlite")
	}

	if err = tx.Commit(); err != nil {
		return syncErrors.WrapOpComponent(err, opStore, "storage/sqlite")
	}

	return nil
}

// Load retrieves all events since a given version.
func (s *SQLiteEventStore) Load(ctx context.Context, since synckit.Version) ([]synckit.EventWithVersion, error) {
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

	query := `SELECT version, id, aggregate_id, event_type, data, metadata FROM events WHERE version > ? ORDER BY version ASC`
	rows, err := s.db.QueryContext(ctx, query, int64(sinceCursor.Seq))
	if err != nil {
		return nil, syncErrors.WrapOpComponent(err, opLoad, "storage/sqlite")
	}
	defer rows.Close()

	return s.scanEvents(rows)
}

// LoadByAggregate retrieves events for a specific aggregate since a given version.
func (s *SQLiteEventStore) LoadByAggregate(ctx context.Context, aggregateID string, since synckit.Version) ([]synckit.EventWithVersion, error) {
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

	query := `SELECT version, id, aggregate_id, event_type, data, metadata FROM events WHERE aggregate_id = ? AND version > ? ORDER BY version ASC`
	rows, err := s.db.QueryContext(ctx, query, aggregateID, int64(sinceCursor.Seq))
	if err != nil {
		return nil, syncErrors.WrapOpComponent(err, opLoadByAggregate, "storage/sqlite")
	}
	defer rows.Close()

	return s.scanEvents(rows)
}

// LatestVersion returns the highest version number in the store.
func (s *SQLiteEventStore) LatestVersion(ctx context.Context) (synckit.Version, error) {
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
		return nil, syncErrors.WrapOpComponent(err, opLatestVersion, "storage/sqlite")
	}

	if !maxVersion.Valid {
		return cursor.IntegerCursor{Seq: 0}, nil // No events in store
	}

	return cursor.IntegerCursor{Seq: uint64(maxVersion.Int64)}, nil
}

// ParseVersion converts a string representation into a cursor.IntegerCursor.
// This allows external integrations to handle SQLite's integer versioning gracefully.
func (s *SQLiteEventStore) ParseVersion(ctx context.Context, versionStr string) (synckit.Version, error) {
	if versionStr == "" || versionStr == "0" {
		return cursor.IntegerCursor{Seq: 0}, nil
	}

	val, err := strconv.ParseInt(versionStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid integer version string '%s': %w", versionStr, err)
	}

	return cursor.IntegerCursor{Seq: uint64(val)}, nil
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

// StoreBatch stores multiple events in a single transaction for better performance.
func (s *SQLiteEventStore) StoreBatch(ctx context.Context, events []synckit.EventWithVersion) error {
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
		`INSERT INTO events (id, aggregate_id, event_type, data, metadata) VALUES (?, ?, ?, ?, ?)`)
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
			string(dataJSON),
			string(metadataJSON))
		if err != nil {
			return fmt.Errorf("failed to insert event: %w", err)
		}
	}

	// Commit the transaction
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit batch transaction: %w", err)
	}

	return nil
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
func (s *SQLiteEventStore) scanEvents(rows *sql.Rows) ([]synckit.EventWithVersion, error) {
	var events []synckit.EventWithVersion
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
