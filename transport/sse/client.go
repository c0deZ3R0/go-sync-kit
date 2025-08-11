package sse

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	synckit "github.com/c0deZ3R0/go-sync-kit/synckit"
	"github.com/c0deZ3R0/go-sync-kit/cursor"
	kiterr "github.com/c0deZ3R0/go-sync-kit/errors"
	"github.com/c0deZ3R0/go-sync-kit/logging"
)


type Client struct {
	BaseURL string
	Client  *http.Client
	logger  *slog.Logger
}

// NewClient creates a new SSE client
func NewClient(baseURL string, httpClient *http.Client) *Client {
	return NewClientWithLogger(baseURL, httpClient, logging.Default().Logger)
}

// NewClientWithLogger creates a new SSE client with structured logging
func NewClientWithLogger(baseURL string, httpClient *http.Client, logger *slog.Logger) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	if logger == nil {
		logger = logging.Default().Logger
	}
	return &Client{
		BaseURL: baseURL,
		Client:  httpClient,
		logger:  logger,
	}
}

func (c *Client) Subscribe(ctx context.Context, handler func([]synckit.EventWithVersion) error) error {
	c.logger.Debug("Starting SSE subscription",
		slog.String("base_url", c.BaseURL))

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/sse?cursor=", nil)
	resp, err := c.Client.Do(req)
	if err != nil {
		c.logger.Error("SSE subscription request failed",
			slog.String("error", err.Error()),
			slog.String("url", c.BaseURL+"/sse?cursor="))
		return kiterr.E(kiterr.Op("sse.Subscribe"), kiterr.Component("transport/sse"), err, "http request")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.logger.Error("SSE subscription returned error status",
			slog.Int("status_code", resp.StatusCode),
			slog.String("url", c.BaseURL+"/sse?cursor="))
		return kiterr.E(kiterr.Op("sse.Subscribe"), kiterr.Component("transport/sse"), 
			kiterr.KindInternal, "server error", resp.Status)
	}

	c.logger.Debug("SSE subscription established, starting to read events")

	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 0, 64<<10), 10<<20) // allow large lines
	for sc.Scan() {
		line := sc.Bytes()
		if !bytes.HasPrefix(line, []byte("data: ")) {
			continue
		}
		var payload struct {
			Events     []JSONEventWithVersion `json:"events"`
			NextCursor cursor.WireCursor       `json:"next_cursor"`
		}
		if err := json.Unmarshal(bytes.TrimPrefix(line, []byte("data: ")), &payload); err != nil {
			c.logger.Error("Failed to decode SSE payload",
				slog.String("error", err.Error()),
				slog.String("payload", string(line)))
			return kiterr.E(kiterr.Op("sse.Subscribe"), kiterr.Component("transport/sse"), kiterr.KindInvalid, err, "decode payload")
		}

		// Convert JSON events back to synckit.EventWithVersion
		events := make([]synckit.EventWithVersion, len(payload.Events))
		for i, jsonEv := range payload.Events {
			events[i] = fromJSONEventWithVersion(jsonEv)
		}

		c.logger.Debug("Received events from SSE",
			slog.Int("event_count", len(events)))

		if err := handler(events); err != nil {
			c.logger.Error("SSE event handler failed",
				slog.String("error", err.Error()),
				slog.Int("event_count", len(events)))
			return kiterr.E(kiterr.Op("sse.Subscribe"), kiterr.Component("transport/sse"), err, "handler")
		}
	}
	if err := sc.Err(); err != nil && !strings.Contains(err.Error(), "context canceled") {
		c.logger.Error("SSE scanner error",
			slog.String("error", err.Error()))
		return kiterr.E(kiterr.Op("sse.Subscribe"), kiterr.Component("transport/sse"), err, "scan")
	}
	c.logger.Debug("SSE subscription ended")
	return nil
}

// Implement other Transport methods as passthroughs or unimplemented if Subscribe-only for MVP.
func (c *Client) Push(ctx context.Context, _ []synckit.EventWithVersion) error { 
	return kiterr.E(kiterr.Op("sse.Push"), kiterr.Component("transport/sse"), kiterr.KindMethodNotAllowed, "not implemented") 
}

func (c *Client) Pull(ctx context.Context, _ synckit.Version) ([]synckit.EventWithVersion, error) { 
	return nil, kiterr.E(kiterr.Op("sse.Pull"), kiterr.Component("transport/sse"), kiterr.KindMethodNotAllowed, "not implemented") 
}

func (c *Client) GetLatestVersion(ctx context.Context) (synckit.Version, error) {
	return nil, kiterr.E(kiterr.Op("sse.GetLatestVersion"), kiterr.Component("transport/sse"), kiterr.KindMethodNotAllowed, "not implemented")
}

func (c *Client) Close() error { 
	return nil 
}
