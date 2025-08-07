package http

import (
    "encoding/json"
    "net/http"

    sync "github.com/c0deZ3R0/go-sync-kit"
)

// SyncHandler handles HTTP sync requests
type SyncHandler struct {
    store sync.EventStore
}

// NewHandler creates a new sync HTTP handler
func NewHandler(store sync.EventStore) *SyncHandler {
    return &SyncHandler{store: store}
}

func (h *SyncHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    // Handle cursor-based pull endpoint
    if r.URL.Path == "/pull-cursor" {
        h.handlePullCursor(w, r)
        return
    }

    // Handle other endpoints...
    respondWithError(w, http.StatusNotFound, "endpoint not found")
}

func respondWithError(w http.ResponseWriter, code int, message string) {
    respondWithJSON(w, code, map[string]string{"error": message})
}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
    response, _ := json.Marshal(payload)
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(code)
    w.Write(response)
}
