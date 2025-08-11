-- PostgreSQL EventStore Schema with LISTEN/NOTIFY support
-- This migration creates the events table with real-time notification capabilities

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

-- Comments for documentation
COMMENT ON TABLE events IS 'Event store table with LISTEN/NOTIFY capabilities for real-time synchronization';
COMMENT ON COLUMN events.version IS 'Auto-incrementing version number for ordering events';
COMMENT ON COLUMN events.id IS 'Unique event identifier provided by the application';
COMMENT ON COLUMN events.aggregate_id IS 'Identifier of the aggregate this event belongs to';
COMMENT ON COLUMN events.event_type IS 'Type of the event (e.g., UserCreated, OrderUpdated)';
COMMENT ON COLUMN events.data IS 'Event payload stored as JSONB for efficient querying';
COMMENT ON COLUMN events.metadata IS 'Event metadata stored as JSONB';
COMMENT ON COLUMN events.created_at IS 'Timestamp when the event was stored';
COMMENT ON COLUMN events.stream_name IS 'Generated stream name for LISTEN/NOTIFY channels';

COMMENT ON FUNCTION notify_event_inserted() IS 'Trigger function that sends PostgreSQL notifications when events are inserted';
COMMENT ON TRIGGER events_notify_trigger ON events IS 'Trigger that fires notifications for real-time event streaming';
