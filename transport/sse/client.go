package sse

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"

	synckit "github.com/c0deZ3R0/go-sync-kit/synckit"
	"github.com/c0deZ3R0/go-sync-kit/cursor"
	kiterr "github.com/c0deZ3R0/go-sync-kit/errors"
)


type Client struct {
	BaseURL string
	Client  *http.Client
}

// NewClient creates a new SSE client
func NewClient(baseURL string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{
		BaseURL: baseURL,
		Client:  httpClient,
	}
}

func (c *Client) Subscribe(ctx context.Context, handler func([]synckit.EventWithVersion) error) error {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/sse?cursor=", nil)
	resp, err := c.Client.Do(req)
	if err != nil {
		return kiterr.E(kiterr.Op("sse.Subscribe"), kiterr.Component("transport/sse"), err, "http request")
	}
	defer resp.Body.Close()

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
			return kiterr.E(kiterr.Op("sse.Subscribe"), kiterr.Component("transport/sse"), kiterr.KindInvalid, err, "decode payload")
		}

		// Convert JSON events back to synckit.EventWithVersion
		events := make([]synckit.EventWithVersion, len(payload.Events))
		for i, jsonEv := range payload.Events {
			events[i] = fromJSONEventWithVersion(jsonEv)
		}

		if err := handler(events); err != nil {
			return kiterr.E(kiterr.Op("sse.Subscribe"), kiterr.Component("transport/sse"), err, "handler")
		}
	}
	if err := sc.Err(); err != nil && !strings.Contains(err.Error(), "context canceled") {
		return kiterr.E(kiterr.Op("sse.Subscribe"), kiterr.Component("transport/sse"), err, "scan")
	}
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
