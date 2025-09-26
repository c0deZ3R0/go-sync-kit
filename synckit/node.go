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

// Compile-time check: ensure SyncNode satisfies SyncManager.
// If SyncNode evolves into a struct wrapper later, missing method forwards will cause compile-time errors.
var _ SyncManager = (SyncNode)(nil)
