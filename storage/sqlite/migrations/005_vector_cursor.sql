-- Add vector cursor support

ALTER TABLE events ADD COLUMN origin_node TEXT;
ALTER TABLE events ADD COLUMN origin_counter INTEGER;
CREATE INDEX IF NOT EXISTS idx_origin ON events(origin_node, origin_counter);
