package httptransport

import (
    "encoding/json"
    "net/http"

    "github.com/c0deZ3R0/go-sync-kit/synckit"
)

// SyncHandler handles HTTP sync requests
type SyncHandler struct {
    store synckit.EventStore
}

// NewHandler creates a new sync HTTP handler
func NewHandler(store synckit.EventStore) *SyncHandler {
    return &SyncHandler{store: store}
}

func (h *SyncHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    path := r.URL.Path
    if p := "/sync"; len(path) >= len(p) && path[:len(p)] == p {
        path = path[len(p):]
    }
    switch path {
    case "/push":
        h.handlePush(w, r)
    case "/pull":
        h.handlePull(w, r) // legacy
    case "/pull-cursor":
        h.handlePullCursor(w, r) // new
    case "/latest-version":
        h.handleLatestVersion(w, r)
    default:
        http.NotFound(w, r)
    }
}

func (h *SyncHandler) handlePush(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        respondWithError(w, http.StatusMethodNotAllowed, "method not allowed")
        return
    }
    // TODO: Implement push handler
    http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (h *SyncHandler) handlePull(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        respondWithError(w, http.StatusMethodNotAllowed, "method not allowed")
        return
    }
    // TODO: Implement legacy pull handler
    http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (h *SyncHandler) handleLatestVersion(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodGet {
        respondWithError(w, http.StatusMethodNotAllowed, "method not allowed")
        return
    }
    // TODO: Implement latest version handler
    http.Error(w, "not implemented", http.StatusNotImplemented)
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
