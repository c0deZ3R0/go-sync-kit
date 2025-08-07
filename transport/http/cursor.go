package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/c0deZ3R0/go-sync-kit/cursor"
)

// Request/response for PullWithCursor
type PullRequest struct {
	Since *cursor.WireCursor `json:"since,omitempty"`
	Limit int               `json:"limit,omitempty"`
}

type EventEnvelope struct {
	ID            string                 `json:"id"`
	Type          string                 `json:"type"`
	AggregateID   string                 `json:"aggregate_id"`
	Data          map[string]any         `json:"data,omitempty"`
	Metadata      map[string]any         `json:"metadata,omitempty"`
	OriginNode    string                 `json:"origin_node,omitempty"`
	OriginCounter uint64                 `json:"origin_counter,omitempty"`
	Seq           uint64                 `json:"seq,omitempty"`
}

type PullResponse struct {
	Events []EventEnvelope    `json:"events"`
	Next   *cursor.WireCursor `json:"next,omitempty"`
}

// Client implements HTTP transport
type Client struct {
	baseURL string
	http    *http.Client
}

// NewClient creates a new HTTP transport client
func NewClient(baseURL string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{
		baseURL: baseURL,
		http:    httpClient,
	}
}

// PullWithCursor fetches events using cursor-based pagination
func (c *Client) PullWithCursor(ctx context.Context, since cursor.Cursor, limit int) ([]EventEnvelope, cursor.Cursor, error) {
	var sinceWire *cursor.WireCursor
	var err error
	if since != nil {
		sinceWire, err = cursor.MarshalWire(since)
		if err != nil {
			return nil, nil, err
		}
	}
	reqBody, _ := json.Marshal(PullRequest{Since: sinceWire, Limit: limit})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/pull", bytes.NewReader(reqBody))
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, nil, fmt.Errorf("pull failed: %s: %s", resp.Status, string(b))
	}
	var pr PullResponse
	if err := json.NewDecoder(resp.Body).Decode(&pr); err != nil {
		return nil, nil, err
	}
	var next cursor.Cursor
	if pr.Next != nil {
		next, err = cursor.UnmarshalWire(pr.Next)
		if err != nil {
			return nil, nil, err
		}
	}
	return pr.Events, next, nil
}

// Store provides cursor-based event access
type Store interface {
	PullWithCursor(ctx context.Context, since cursor.Cursor, limit int) ([]EventEnvelope, cursor.Cursor, error)
}

// Server handles HTTP transport requests
type Server struct {
	store Store
}

// NewServer creates a new HTTP transport server
func NewServer(store Store) *Server {
	return &Server{store: store}
}

// HandlePullWithCursor handles cursor-based pull requests
func (s *Server) HandlePullWithCursor(w http.ResponseWriter, r *http.Request) {
	var req PullRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	var since cursor.Cursor
	if req.Since != nil {
		c, err := cursor.UnmarshalWire(req.Since)
		if err != nil {
			http.Error(w, "bad cursor", http.StatusBadRequest)
			return
		}
		since = c
	}
	limit := req.Limit
	if limit <= 0 || limit > 1000 {
		limit = 200
	}
	
	events, next, err := s.store.PullWithCursor(r.Context(), since, limit)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	
	var nextWire *cursor.WireCursor
	if next != nil {
		if wcur, err := cursor.MarshalWire(next); err == nil {
			nextWire = wcur
		}
	}
	resp := PullResponse{Events: events, Next: nextWire}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
