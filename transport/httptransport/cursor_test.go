package httptransport

import (
	"log"
	"os"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"context"
	"github.com/c0deZ3R0/go-sync-kit/synckit"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/c0deZ3R0/go-sync-kit/cursor"
)

func TestCursorAPI_SizeLimits(t *testing.T) {
	// Create a logger
logger := log.New(os.Stdout, "TEST: ", log.Ldate|log.Ltime|log.Lshortfile)
	testLogger = logger
// Create a large request payload that exceeds the size limit
	longString := strings.Repeat("x", 2*1024) // 2KB 
	req := PullCursorRequest{
		Since: &cursor.WireCursor{Kind: "bytes", Data: []byte(longString)},
		Limit: 100,
	}

	// Serialize request to get actual size
	data, _ := json.Marshal(req)

	// Log test setup
	logger.Printf("Creating test payload size: %d bytes", len(data))
	logger.Printf("Server MaxRequestSize: %d bytes", 1*1024)

	// Create handler with adjusted options (set much lower for test)
	handler := NewSyncHandler(
		NewMockEventStore(),
		nil,
		nil,
&ServerOptions{MaxRequestSize: 1 * 1024 /* 1KB */, CompressionEnabled: false, CompressionThreshold: 0},
	)

	// Create test server
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler.handlePullCursor(w, r, NewCursorOptions())
	}))
	defer s.Close()

	// Send large request
	r, err := http.NewRequest(http.MethodPost, s.URL+"/pull-cursor", strings.NewReader(string(data)))
	if err != nil {
		t.Fatal(err)
	}
	r.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	// Should get a 413 Request Entity Too Large
	assert.Equal(t, http.StatusRequestEntityTooLarge, resp.StatusCode)
}

func TestCursorAPI_Compression(t *testing.T) {

	// Create test data
	longData := strings.Repeat("test data ", 150) // ~1.5KB
	testEvents := make([]synckit.EventWithVersion, 10)
	for i := range testEvents {
		testEvents[i] = synckit.EventWithVersion{
			Event: &MockEvent{
				id:   fmt.Sprintf("test-%d", i),
				data: longData,
			},
			Version: cursor.IntegerCursor{Seq: uint64(i)},
		}
	}

	// Create mock store with test data
	store := NewMockEventStore()
	for _, ev := range testEvents {
		store.Store(context.Background(), ev.Event, ev.Version)
	}

	// Create handler with compression enabled
	handler := NewSyncHandler(
		store,
		nil,
		nil,
		&ServerOptions{
MaxRequestSize:       1024, // Set to 1KB to ensure limit is hit during test
			CompressionEnabled:   true,
			CompressionThreshold: 1024, // 1KB
		},
	)

	// Create test server
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler.handlePullCursor(w, r, &CursorOptions{
			ServerOptions: &ServerOptions{
MaxRequestSize:       512,
				CompressionEnabled:   true,
				CompressionThreshold: 1024,
			},
		})
	}))
	defer s.Close()

	// Send request with Accept-Encoding: gzip
	r, err := http.NewRequest(http.MethodPost, s.URL+"/pull-cursor", strings.NewReader(`{"limit":100}`))
	if err != nil {
		t.Fatal(err)
	}
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Accept-Encoding", "gzip")

	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	// Response should be compressed
	assert.Equal(t, "gzip", resp.Header.Get("Content-Encoding"))

	// Should still be able to read and decode response
	var reader io.Reader = resp.Body
	if resp.Header.Get("Content-Encoding") == "gzip" {
		gzReader, err := gzip.NewReader(resp.Body)
		require.NoError(t, err)
		defer gzReader.Close()
		reader = gzReader
	}

	var result PullCursorResponse
	require.NoError(t, json.NewDecoder(reader).Decode(&result))
}
