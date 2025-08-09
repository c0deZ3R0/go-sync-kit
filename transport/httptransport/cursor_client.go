package httptransport

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/c0deZ3R0/go-sync-kit/cursor"
	"github.com/c0deZ3R0/go-sync-kit/synckit"
)

// PullWithCursor POSTs /pull-cursor and returns events + the next cursor.
func (t *HTTPTransport) PullWithCursor(ctx context.Context, since cursor.Cursor, limit int) ([]synckit.EventWithVersion, cursor.Cursor, error) {
	var sinceWire *cursor.WireCursor
	var err error
	if since != nil {
		sinceWire, err = cursor.MarshalWire(since)
		if err != nil {
			return nil, nil, err
		}
	}
	if limit <= 0 || limit > 1000 {
		limit = 200
	}

	reqBody, _ := json.Marshal(PullCursorRequest{Since: sinceWire, Limit: limit})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.baseURL+"/pull-cursor", bytes.NewReader(reqBody))
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, nil, fmt.Errorf("pull-cursor failed: %s: %s", resp.Status, string(b))
	}

	// Handle compressed response and enforce size limits
	reader, cleanup, err := createSafeResponseReader(resp, t.options)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create safe response reader: %w", err)
	}
	defer cleanup()

	var pr PullCursorResponse
	if err := json.NewDecoder(reader).Decode(&pr); err != nil {
		// Check if this is a size limit violation
		if errors.Is(err, errResponseDecompressedTooLarge) {
			return nil, nil, fmt.Errorf("response decompressed size exceeds limit: %w", err)
		}
		return nil, nil, err
	}

	// Convert JSONEventWithVersion -> EventWithVersion
	events := make([]synckit.EventWithVersion, len(pr.Events))
	for i, jev := range pr.Events {
		// Use version parser to decode version
		version, err := t.versionParser(ctx, jev.Version)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid version: %w", err)
		}
		ev := &SimpleEvent{
			IDValue:          jev.Event.ID,
			TypeValue:        jev.Event.Type,
			AggregateIDValue: jev.Event.AggregateID,
			DataValue:        jev.Event.Data,
			MetadataValue:    jev.Event.Metadata,
		}
		events[i] = synckit.EventWithVersion{Event: ev, Version: version}
	}

	var next cursor.Cursor
	if pr.Next != nil {
		next, err = cursor.UnmarshalWire(pr.Next)
		if err != nil {
			return nil, nil, fmt.Errorf("bad next cursor: %w", err)
		}
	}

	return events, next, nil
}
