package sse

	import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	synckit "github.com/c0deZ3R0/go-sync-kit/synckit"
	"github.com/c0deZ3R0/go-sync-kit/cursor"
	"github.com/c0deZ3R0/go-sync-kit/logging"
)


type Server struct {
	Store     synckit.EventStore
	Logger    *slog.Logger
	BatchSize int
}

// NewServer creates a new SSE server with default settings
func NewServer(store synckit.EventStore, logger *slog.Logger) *Server {
	return NewServerWithLogger(store, logger)
}

// NewServerWithLogger creates a new SSE server with structured logging
func NewServerWithLogger(store synckit.EventStore, logger *slog.Logger) *Server {
	if logger == nil {
		logger = logging.Default().Logger
	}
	return &Server{
		Store:     store,
		Logger:    logger,
		BatchSize: 100, // reasonable default
	}
}

func (s *Server) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.Logger.Debug("SSE client connected",
			slog.String("remote_addr", r.RemoteAddr),
			slog.String("user_agent", r.UserAgent()))

		flusher, ok := w.(http.Flusher)
		if !ok {
			s.Logger.Error("Client does not support streaming",
				slog.String("remote_addr", r.RemoteAddr))
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		ctx := r.Context()
		curParam := r.URL.Query().Get("cursor")
		var cur cursor.Cursor
		if curParam != "" {
			s.Logger.Debug("Parsing cursor from client",
				slog.String("cursor_param", curParam),
				slog.String("remote_addr", r.RemoteAddr))
			// Use existing WireCursor parser
			var wc cursor.WireCursor
			if err := json.Unmarshal([]byte(curParam), &wc); err != nil {
				s.Logger.Warn("Invalid cursor format from SSE client",
					slog.String("cursor_param", curParam),
					slog.String("error", err.Error()),
					slog.String("remote_addr", r.RemoteAddr))
				http.Error(w, "bad cursor format", http.StatusBadRequest)
				return
			}
			parsed, err := cursor.UnmarshalWire(&wc)
			if err != nil {
				s.Logger.Warn("Failed to parse cursor from SSE client",
					slog.String("cursor_param", curParam),
					slog.String("error", err.Error()),
					slog.String("remote_addr", r.RemoteAddr))
				http.Error(w, "bad cursor", http.StatusBadRequest)
				return
			}
			cur = parsed
		} else {
			s.Logger.Debug("SSE client starting from beginning (no cursor provided)",
				slog.String("remote_addr", r.RemoteAddr))
		}

		for {
			select {
			case <-ctx.Done():
				s.Logger.Debug("SSE client disconnected",
					slog.String("remote_addr", r.RemoteAddr))
				return
			default:
			}

			events, nextCur, err := loadNext(ctx, s.Store, cur, s.BatchSize)
			if err != nil {
				s.Logger.Error("Failed to load events for SSE client",
					slog.String("error", err.Error()),
					slog.String("remote_addr", r.RemoteAddr),
					slog.String("cursor", fmt.Sprintf("%v", cur)))
				return
			}
			if len(events) == 0 {
				time.Sleep(200 * time.Millisecond)
				continue
			}

		// Convert events to JSON serializable format
		jsonEvents := make([]JSONEventWithVersion, len(events))
		for i, ev := range events {
			jsonEvents[i] = toJSONEventWithVersion(ev)
		}

		payload := struct {
			Events     []JSONEventWithVersion `json:"events"`
			NextCursor cursor.WireCursor       `json:"next_cursor"`
		}{
			Events:     jsonEvents,
			NextCursor: cursor.MustMarshalWire(nextCur),
		}
			b, _ := json.Marshal(payload)
			s.Logger.Debug("Sending events to SSE client",
				slog.Int("event_count", len(events)),
				slog.String("remote_addr", r.RemoteAddr),
				slog.String("next_cursor", fmt.Sprintf("%v", nextCur)))
			fmt.Fprintf(w, "data: %s\n\n", b)
			flusher.Flush()

			cur = nextCur
		}
	})
}

// loadNext uses your store's cursor pagination (adapt to your store/load API).
func loadNext(ctx context.Context, store synckit.EventStore, cur cursor.Cursor, batch int) ([]synckit.EventWithVersion, cursor.Cursor, error) {
	// For initial MVP, call Load since version or a cursor-aware LoadNext if available.
	// Return events and a new cursor (e.g., last seen version).
	
	// Convert cursor to version for store.Load
	var sinceVersion synckit.Version
	if cur != nil {
		// Try to cast cursor to version types
		if ic, ok := cur.(cursor.IntegerCursor); ok {
			sinceVersion = ic
		} else if vc, ok := cur.(cursor.VectorCursor); ok {
			sinceVersion = vc
		}
	}
	
	evs, err := store.Load(ctx, sinceVersion)
	if err != nil {
		return nil, nil, err
	}
	if len(evs) == 0 {
		return nil, cur, nil
	}

	// Truncate to batch size
	if len(evs) > batch {
		evs = evs[:batch]
	}

	// Compute next cursor from the last event's version
	next := cur // fallback to current cursor
	if len(evs) > 0 {
		lastEvent := evs[len(evs)-1]
		// Try to convert the version to a cursor
		if ic, ok := lastEvent.Version.(cursor.IntegerCursor); ok {
			next = cursor.NewInteger(ic.Seq)
		} else if vc, ok := lastEvent.Version.(cursor.VectorCursor); ok {
			next = cursor.NewVector(vc.Counters)
		} else {
			// For other version types, try to create an IntegerCursor based on a hash or sequence
			// This is a fallback - in a real implementation, you'd want proper cursor derivation
			next = cursor.NewInteger(uint64(len(evs)))
		}
	}

	return evs, next, nil
}
