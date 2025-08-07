package http

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"

    "github.com/c0deZ3R0/go-sync-kit/cursor"
    sync "github.com/c0deZ3R0/go-sync-kit"
)

// PullWithCursor fetches events using cursor-based pagination
func (c *Client) PullWithCursor(ctx context.Context, since cursor.Cursor, limit int) ([]sync.EventWithVersion, cursor.Cursor, error) {
    var sinceWire *cursor.WireCursor
    var err error
    if since != nil {
        sinceWire, err = cursor.MarshalWire(since)
        if err != nil {
            return nil, nil, fmt.Errorf("marshal cursor: %w", err)
        }
    }

    // Prepare request
    req := PullCursorRequest{
        Since: sinceWire,
        Limit: limit,
    }
    reqBody, err := json.Marshal(req)
    if err != nil {
        return nil, nil, fmt.Errorf("marshal request: %w", err)
    }

    // Create HTTP request
    httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/pull-cursor", bytes.NewReader(reqBody))
    if err != nil {
        return nil, nil, fmt.Errorf("create request: %w", err)
    }
    httpReq.Header.Set("Content-Type", "application/json")

    // Send request
    resp, err := c.http.Do(httpReq)
    if err != nil {
        return nil, nil, fmt.Errorf("send request: %w", err)
    }
    defer resp.Body.Close()

    // Check status
    if resp.StatusCode != http.StatusOK {
        body, _ := io.ReadAll(resp.Body)
        return nil, nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, body)
    }

    // Parse response
    var pullResp PullCursorResponse
    if err := json.NewDecoder(resp.Body).Decode(&pullResp); err != nil {
        return nil, nil, fmt.Errorf("decode response: %w", err)
    }

    // Convert events
    events := make([]sync.EventWithVersion, len(pullResp.Events))
    for i, je := range pullResp.Events {
        events[i] = fromJSONEventWithVersion(je)
    }

    // Parse next cursor
    var next cursor.Cursor
    if pullResp.Next != nil {
        next, err = cursor.UnmarshalWire(pullResp.Next)
        if err != nil {
            return nil, nil, fmt.Errorf("unmarshal next cursor: %w", err)
        }
    }

    return events, next, nil
}
