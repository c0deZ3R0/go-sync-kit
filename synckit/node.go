package synckit

// SyncNode is the preferred façade for creating and managing sync participants.
// It is currently a type alias for SyncManager but may evolve into a dedicated
// struct in future releases.
type SyncNode = SyncManager

// NewNode mirrors NewManager to construct a SyncNode.
// Use this instead of NewManager in new code.
func NewNode(opts ...ManagerOption) (SyncNode, error) {
	return NewManager(opts...)
}

// Compile-time check: ensure SyncNode always satisfies SyncManager.
// This guarantees that if SyncNode becomes a real struct wrapper later,
// missing method forwards will be caught by the compiler.
var _ SyncManager = (SyncNode)(nil)
