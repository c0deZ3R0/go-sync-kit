package httptransport

import (
    "encoding/json"
    "net/http"

    "github.com/c0deZ3R0/go-sync-kit/cursor"
    "github.com/c0deZ3R0/go-sync-kit/synckit"
    "github.com/c0deZ3R0/go-sync-kit/storage/sqlite"
)

type PullCursorRequest struct {
    Since *cursor.WireCursor `json:"since,omitempty"`
    Limit int                `json:"limit,omitempty"`
}

type PullCursorResponse struct {
    Events []JSONEventWithVersion `json:"events"`
    Next   *cursor.WireCursor     `json:"next,omitempty"`
}

func (h *SyncHandler) handlePullCursor(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        respondWithError(w, http.StatusMethodNotAllowed, "method not allowed")
        return
    }

    var req PullCursorRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        respondWithError(w, http.StatusBadRequest, "bad request: "+err.Error())
        return
    }

    // Integer mode first
    since := synckit.Version(sqlite.IntegerVersion(0))
    if req.Since != nil {
        c, err := cursor.UnmarshalWire(req.Since)
        if err != nil {
            respondWithError(w, http.StatusBadRequest, "bad cursor: "+err.Error())
            return
        }
        ic, ok := c.(cursor.IntegerCursor)
        if !ok {
            respondWithError(w, http.StatusBadRequest, "cursor kind not supported by this store")
            return
        }
        since = sqlite.IntegerVersion(ic.Seq)
    }

    // Load events since
    events, err := h.store.Load(r.Context(), since)
    if err != nil {
        respondWithError(w, http.StatusInternalServerError, "could not load events")
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
        if iv, ok := ev.Version.(sqlite.IntegerVersion); ok && int64(iv) > maxSeq {
            maxSeq = int64(iv)
        }
    }

    // Next cursor
    var nextWire *cursor.WireCursor
    if maxSeq > 0 {
        next, _ := cursor.MarshalWire(cursor.IntegerCursor{Seq: uint64(maxSeq)})
        nextWire = next
    }

    respondWithJSON(w, http.StatusOK, PullCursorResponse{
        Events: jsonEvents,
        Next:   nextWire,
    })
}
