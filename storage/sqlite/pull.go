package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/c0deZ3R0/go-sync-kit/cursor"
)

type EventRow struct {
	Seq           uint64
	ID            string
	Type          string
	AggregateID   string
	DataJSON      string
	MetadataJSON  string
	OriginNode    sql.NullString
	OriginCounter sql.NullInt64
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

func PullWithCursor(ctx context.Context, db *sql.DB, since cursor.Cursor, limit int) ([]EventEnvelope, cursor.Cursor, error) {
	switch c := since.(type) {
	case nil:
		// treat as zero cursor; choose default strategy (integer or vector) depending on schema support
		return fetchFromZeroInteger(ctx, db, limit)
	case cursor.IntegerCursor:
		return fetchInteger(ctx, db, c, limit)
	case cursor.VectorCursor:
		return fetchVector(ctx, db, c, limit)
	default:
		return nil, nil, fmt.Errorf("unsupported cursor type: %T", since)
	}
}

// INTEGER MODE

func fetchFromZeroInteger(ctx context.Context, db *sql.DB, limit int) ([]EventEnvelope, cursor.Cursor, error) {
	return fetchInteger(ctx, db, cursor.IntegerCursor{Seq: 0}, limit)
}

func fetchInteger(ctx context.Context, db *sql.DB, c cursor.IntegerCursor, limit int) ([]EventEnvelope, cursor.Cursor, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT seq, id, event_type, aggregate_id, data, metadata, origin_node, origin_counter
		FROM events
		WHERE seq > ?
		ORDER BY seq ASC
		LIMIT ?`, c.Seq, limit)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	var out []EventEnvelope
	var maxSeq uint64
	for rows.Next() {
		var r EventRow
		if err := rows.Scan(&r.Seq, &r.ID, &r.Type, &r.AggregateID, &r.DataJSON, &r.MetadataJSON, &r.OriginNode, &r.OriginCounter); err != nil {
			return nil, nil, err
		}
		ev, err := toEnvelope(r)
		if err != nil {
			return nil, nil, err
		}
		out = append(out, ev)
		if r.Seq > maxSeq {
			maxSeq = r.Seq
		}
	}
	var next cursor.Cursor
	if len(out) > 0 {
		next = cursor.IntegerCursor{Seq: maxSeq}
	} else {
		next = c
	}
	return out, next, rows.Err()
}

// VECTOR MODE

// fetchVector uses a temp table ("since") to select events with origin counters greater
// than the caller's counters for each origin_node.
func fetchVector(ctx context.Context, db *sql.DB, c cursor.VectorCursor, limit int) ([]EventEnvelope, cursor.Cursor, error) {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, nil, err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	if _, err := tx.ExecContext(ctx, `CREATE TEMP TABLE since (node TEXT PRIMARY KEY, counter INTEGER)`); err != nil {
		return nil, nil, err
	}
	if len(c.Counters) > 0 {
		var sb strings.Builder
		args := make([]any, 0, len(c.Counters)*2)
		sb.WriteString("INSERT INTO since(node,counter) VALUES ")
		first := true
		for node, ctr := range c.Counters {
			if !first {
				sb.WriteString(",")
			}
			first = false
			sb.WriteString("(?,?)")
			args = append(args, node, int64(ctr))
		}
		if _, err := tx.ExecContext(ctx, sb.String(), args...); err != nil {
			return nil, nil, err
		}
	}

	rows, err := tx.QueryContext(ctx, `
		SELECT e.seq, e.id, e.event_type, e.aggregate_id, e.data, e.metadata, e.origin_node, e.origin_counter
		FROM events e
		LEFT JOIN since s ON s.node = e.origin_node
		WHERE COALESCE(s.counter, 0) < e.origin_counter
		ORDER BY e.seq ASC
		LIMIT ?`, limit)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	var out []EventEnvelope
	// Track max seen per-node to compute next cursor
	nextCounters := map[string]uint64{}
	for k, v := range c.Counters {
		nextCounters[k] = v
	}

	for rows.Next() {
		var r EventRow
		if err := rows.Scan(&r.Seq, &r.ID, &r.Type, &r.AggregateID, &r.DataJSON, &r.MetadataJSON, &r.OriginNode, &r.OriginCounter); err != nil {
			return nil, nil, err
		}
		ev, err := toEnvelope(r)
		if err != nil {
			return nil, nil, err
		}
		out = append(out, ev)
		if r.OriginNode.Valid && r.OriginCounter.Valid {
			node := r.OriginNode.String
			cnt := uint64(r.OriginCounter.Int64)
			if cur, ok := nextCounters[node]; !ok || cnt > cur {
				nextCounters[node] = cnt
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	// Commit the read-only tx (harmless but keeps pattern consistent)
	if err := tx.Commit(); err != nil {
		return nil, nil, err
	}
	var next cursor.Cursor
	if len(out) > 0 {
		next = cursor.VectorCursor{Counters: nextCounters}
	} else {
		next = c
	}
	return out, next, nil
}

func toEnvelope(r EventRow) (EventEnvelope, error) {
	ev := EventEnvelope{
		ID:          r.ID,
		Type:        r.Type,
		AggregateID: r.AggregateID,
		Seq:         r.Seq,
	}
	// Decode JSON payloads; for brevity, ignore errors on empty strings
	if r.DataJSON != "" {
		_ = jsonUnmarshalTo(r.DataJSON, &ev.Data)
	}
	if r.MetadataJSON != "" {
		_ = jsonUnmarshalTo(r.MetadataJSON, &ev.Metadata)
	}
	if r.OriginNode.Valid {
		ev.OriginNode = r.OriginNode.String
	}
	if r.OriginCounter.Valid && r.OriginCounter.Int64 > 0 {
		ev.OriginCounter = uint64(r.OriginCounter.Int64)
	}
	return ev, nil
}

// Small helper that tolerates null/empty strings
func jsonUnmarshalTo(s string, dst any) error {
	if s == "" || s == "null" {
		return nil
	}
	return json.Unmarshal([]byte(s), dst)
}
