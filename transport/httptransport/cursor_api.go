package httptransport

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/c0deZ3R0/go-sync-kit/cursor"
)

type PullCursorRequest struct {
	Since *cursor.WireCursor `json:"since,omitempty"`
	Limit int                `json:"limit,omitempty"`
}

type PullCursorResponse struct {
	Events []JSONEventWithVersion `json:"events"`
	Next   *cursor.WireCursor     `json:"next,omitempty"`
}

// CursorOptions configures the cursor handler
type CursorOptions struct {
	*ServerOptions
}

// NewCursorOptions returns default cursor options
func NewCursorOptions() *CursorOptions {
	return &CursorOptions{
		ServerOptions: DefaultServerOptions(),
	}
}

// testLogger is used for debugging in tests
var testLogger *log.Logger

func (h *SyncHandler) handlePullCursor(w http.ResponseWriter, r *http.Request, options *CursorOptions) {
	if r.Method != http.MethodPost {
		h.respondErr(w, r, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Validate Content-Type for JSON endpoints
	if !validateContentType(w, r, options.ServerOptions) {
		return // validateContentType already sent the response
	}

	// Log request details if logger is available
	if testLogger != nil {
		testLogger.Printf("Request ContentLength: %d, MaxRequestSize: %d", r.ContentLength, options.MaxRequestSize)
	}

	// Create safe reader that handles both compressed and decompressed size limits
	safeReader, cleanup, err := createSafeRequestReader(w, r, h.options)
	if err != nil {
		respondWithError(w, r, http.StatusBadRequest, "invalid request body", h.options)
		return
	}
	defer cleanup()

	var req PullCursorRequest
	if err := json.NewDecoder(safeReader).Decode(&req); err != nil {
		if errors.Is(err, errDecompressedTooLarge) {
			respondWithError(w, r, http.StatusRequestEntityTooLarge, "request entity too large", h.options)
			return
		}
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			respondWithError(w, r, http.StatusRequestEntityTooLarge, "request entity too large", h.options)
			return
		}
		respondWithError(w, r, http.StatusBadRequest, "bad request", h.options)
	return
	}

	// Integer mode first
	since := cursor.IntegerCursor{Seq: 0}
	if req.Since != nil {
		c, err := cursor.UnmarshalWire(req.Since)
		if err != nil {
			h.respondErr(w, r, http.StatusBadRequest, "bad cursor: "+err.Error())
			return
		}
		ic, ok := c.(cursor.IntegerCursor)
		if !ok {
			h.respondErr(w, r, http.StatusBadRequest, "cursor kind not supported by this store")
			return
		}
		since = cursor.IntegerCursor{Seq: ic.Seq}
	}

	// Load events since
	events, err := h.store.Load(r.Context(), since)
	if err != nil {
		h.respondErr(w, r, http.StatusInternalServerError, "could not load events")
		return
	}

	// Apply limit if specified
	limit := req.Limit
	if limit <= 0 || limit > 1000 {
		limit = 200 // reasonable default
	}
	if len(events) > limit {
		events = events[:limit]
	}

	// Map to JSON
	jsonEvents := make([]JSONEventWithVersion, len(events))
	var maxSeq int64
	for i, ev := range events {
		jsonEvents[i] = toJSONEventWithVersion(ev)
		if ic, ok := ev.Version.(cursor.IntegerCursor); ok && int64(ic.Seq) > maxSeq {
			maxSeq = int64(ic.Seq)
		}
	}

	// Next cursor
	var nextWire *cursor.WireCursor
	// Use ParseVersion here to ensure synckit.Version compatibility
	if maxSeq > 0 {
		nextCursor, _ := cursor.MarshalWire(cursor.IntegerCursor{Seq: uint64(maxSeq)})
		nextWire = nextCursor
	}

	respondWithJSON(w, r, http.StatusOK, PullCursorResponse{
		Events: jsonEvents,
		Next:   nextWire,
	}, options.ServerOptions)
}
