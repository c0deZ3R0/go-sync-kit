package synckit

// SyncNode is the preferred public API.
// Internally, it just wraps SyncManager.
type SyncNode = SyncManager

// NewNode mirrors NewManager.
func NewNode(opts ...ManagerOption) (SyncNode, error) {
	return NewManager(opts...)
}