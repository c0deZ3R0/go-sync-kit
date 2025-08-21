// Package transport defines the base transport interfaces used by go-sync-kit.
// Concrete implementations are in subpackages like transport/httptransport, transport/sse, etc.
package transport

import (
	synckit "github.com/c0deZ3R0/go-sync-kit/synckit"
)

// Transport is the base interface for network communication used by health checks.
// This is an alias to synckit.Transport for health check compatibility.
type Transport = synckit.Transport

// Additional transport-specific health check interfaces can be added here.
