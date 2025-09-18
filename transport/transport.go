package transport

import (
	"context"
	"errors"
	"sync"
	"time"
)

// Common transport errors
var (
	ErrTransportClosed  = errors.New("transport is closed")
	ErrConnectionFailed = errors.New("connection failed")
	ErrInvalidPeer      = errors.New("invalid peer")
	ErrTransportTimeout = errors.New("transport operation timeout")
	ErrPeerNotFound     = errors.New("peer not found")
)

// Transport defines the interface for transport layers.
type Transport interface {
	// Connect establishes a connection to a peer.
	Connect(ctx context.Context, peer string) error

	// Disconnect closes the connection to a peer.
	Disconnect(ctx context.Context, peer string) error

	// Send sends data to a peer.
	Send(ctx context.Context, peer string, data []byte) error

	// Receive receives data from any connected peer.
	Receive(ctx context.Context) (peer string, data []byte, err error)

	// IsConnected checks if connected to a peer.
	IsConnected(peer string) bool

	// GetConnectedPeers returns a list of connected peers.
	GetConnectedPeers() []string

	// Close closes the transport layer.
	Close() error
}

// MockTransport is a simple mock transport implementation for testing.
type MockTransport struct {
	mu           sync.RWMutex
	peers        map[string]bool
	messageQueue []Message
	closed       bool
}

// Message represents a message in the transport layer.
type Message struct {
	Peer      string
	Data      []byte
	Timestamp time.Time
}

// NewMockTransport creates a new mock transport.
func NewMockTransport() *MockTransport {
	return &MockTransport{
		peers:        make(map[string]bool),
		messageQueue: make([]Message, 0),
	}
}

func (m *MockTransport) Connect(ctx context.Context, peer string) error {
	if peer == "" {
		return ErrInvalidPeer
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return ErrTransportClosed
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	m.peers[peer] = true
	return nil
}

func (m *MockTransport) Disconnect(ctx context.Context, peer string) error {
	if peer == "" {
		return ErrInvalidPeer
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return ErrTransportClosed
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	delete(m.peers, peer)
	return nil
}

func (m *MockTransport) Send(ctx context.Context, peer string, data []byte) error {
	if peer == "" {
		return ErrInvalidPeer
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return ErrTransportClosed
	}

	if !m.peers[peer] {
		return ErrPeerNotFound
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// In a mock transport, we just queue the message
	message := Message{
		Peer:      peer,
		Data:      append([]byte(nil), data...), // Copy data
		Timestamp: time.Now(),
	}
	m.messageQueue = append(m.messageQueue, message)
	return nil
}

func (m *MockTransport) Receive(ctx context.Context) (peer string, data []byte, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return "", nil, ErrTransportClosed
	}

	select {
	case <-ctx.Done():
		return "", nil, ctx.Err()
	default:
	}

	// Return the first message in queue if any
	if len(m.messageQueue) > 0 {
		message := m.messageQueue[0]
		m.messageQueue = m.messageQueue[1:]
		return message.Peer, message.Data, nil
	}

	// No messages available
	return "", nil, nil
}

func (m *MockTransport) IsConnected(peer string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.closed {
		return false
	}

	return m.peers[peer]
}

func (m *MockTransport) GetConnectedPeers() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.closed {
		return []string{}
	}

	peers := make([]string, 0, len(m.peers))
	for peer := range m.peers {
		peers = append(peers, peer)
	}
	return peers
}

func (m *MockTransport) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.closed = true
	m.peers = nil
	m.messageQueue = nil
	return nil
}
