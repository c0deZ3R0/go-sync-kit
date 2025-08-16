package sse

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

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
	c.logger.Debug("Starting SSE subscription with reconnection",
		slog.String("base_url", c.BaseURL))

	// Exponential backoff configuration
	delay := 250 * time.Millisecond
	maxDelay := 10 * time.Second

	for {
		// Check for context cancellation before attempting connection
		if ctx.Err() != nil {
			c.logger.Debug("Context cancelled, exiting SSE subscription")
			return ctx.Err()
		}

		c.logger.Debug("Attempting SSE connection",
			slog.Duration("backoff_delay", delay))

		// Create HTTP request with context
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/sse", nil)
		if err != nil {
			c.logger.Error("Failed to create SSE request",
				slog.String("error", err.Error()))
			return kiterr.E(kiterr.Op("sse.Subscribe"), kiterr.Component("transport/sse"), err, "create request")
		}

		resp, err := c.Client.Do(req)
		if err != nil {
			c.logger.Warn("SSE connection failed, will retry",
				slog.String("error", err.Error()),
				slog.Duration("retry_delay", delay))

			// Wait with backoff or context cancellation
			select {
			case <-time.After(delay):
				// Continue to retry
			case <-ctx.Done():
				c.logger.Debug("Context cancelled during backoff")
				return ctx.Err()
			}

			// Exponential backoff with cap
			if delay < maxDelay {
				delay *= 2
			}
			continue
		}

		// Check HTTP status
		if resp.StatusCode != http.StatusOK {
			c.logger.Warn("SSE returned error status, will retry",
				slog.Int("status_code", resp.StatusCode),
				slog.Duration("retry_delay", delay))
			resp.Body.Close()

			// Wait with backoff or context cancellation
			select {
			case <-time.After(delay):
				// Continue to retry
			case <-ctx.Done():
				c.logger.Debug("Context cancelled during backoff")
				return ctx.Err()
			}

			// Exponential backoff with cap
			if delay < maxDelay {
				delay *= 2
			}
			continue
		}

		// Reset delay on successful connection
		delay = 250 * time.Millisecond
		c.logger.Debug("SSE connection established, consuming stream")

		// Consume the stream
		err = c.consumeStream(ctx, resp.Body, handler)
		resp.Body.Close()

		// Check if error is due to context cancellation
		if errors.Is(err, context.Canceled) || ctx.Err() != nil {
			c.logger.Debug("SSE stream ended due to context cancellation")
			return ctx.Err()
		}

		// Check if error is a handler error - don't reconnect for handler errors
		if err != nil && strings.Contains(err.Error(), "handler") {
			c.logger.Error("SSE handler error, stopping subscription",
				slog.String("error", err.Error()))
			return err
		}

		// Log the disconnection and prepare for reconnect
		if err != nil {
			c.logger.Warn("SSE stream ended, will reconnect",
				slog.String("error", err.Error()))
		} else {
			c.logger.Info("SSE stream ended normally, reconnecting")
		}

		// Brief pause before reconnecting (even on normal disconnection)
		select {
		case <-time.After(150 * time.Millisecond):
			// Continue to next iteration
		case <-ctx.Done():
			c.logger.Debug("Context cancelled during reconnect pause")
			return ctx.Err()
		}
	}
}

// consumeStream processes the SSE stream from the response body
func (c *Client) consumeStream(ctx context.Context, body io.ReadCloser, handler func([]synckit.EventWithVersion) error) error {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64<<10), 10<<20) // allow large lines

	for scanner.Scan() {
		// Check for context cancellation during stream processing
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			// Continue processing
		}

		line := scanner.Bytes()
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
			// Don't return error for decode failures - continue processing
			continue
		}

		// Convert JSON events back to synckit.EventWithVersion
		events := make([]synckit.EventWithVersion, len(payload.Events))
		for i, jsonEv := range payload.Events {
			events[i] = fromJSONEventWithVersion(jsonEv)
		}

		if len(events) > 0 {
			c.logger.Debug("Received events from SSE",
				slog.Int("event_count", len(events)))

			if err := handler(events); err != nil {
				c.logger.Error("SSE event handler failed",
					slog.String("error", err.Error()),
					slog.Int("event_count", len(events)))
				// Handler error should bubble up and potentially break the stream
				return kiterr.E(kiterr.Op("sse.consumeStream"), kiterr.Component("transport/sse"), err, "handler")
			}
		}
	}

	// Check scanner error
	if err := scanner.Err(); err != nil {
		// Don't treat context cancellation as an error
		if !strings.Contains(err.Error(), "context canceled") {
			c.logger.Error("SSE scanner error",
				slog.String("error", err.Error()))
			return kiterr.E(kiterr.Op("sse.consumeStream"), kiterr.Component("transport/sse"), err, "scan")
		}
		return context.Canceled
	}

	c.logger.Debug("SSE stream ended normally")
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
