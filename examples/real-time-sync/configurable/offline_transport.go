package main

import (
	"context"
	"errors"
	"sync"

	"github.com/c0deZ3R0/go-sync-kit/synckit"
)

var errOffline = errors.New("transport offline (simulated)")

type OfflineTransport struct {
	inner  synckit.Transport
	mu     sync.Mutex
	online bool
}

func NewOfflineTransport(inner synckit.Transport) *OfflineTransport {
	return &OfflineTransport{inner: inner, online: true}
}

func (t *OfflineTransport) SetOnline(v bool) {
	t.mu.Lock()
	t.online = v
	t.mu.Unlock()
}

func (t *OfflineTransport) isOnline() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.online
}

func (t *OfflineTransport) Push(ctx context.Context, events []synckit.EventWithVersion) error {
	if !t.isOnline() {
		return errOffline
	}
	return t.inner.Push(ctx, events)
}

func (t *OfflineTransport) Pull(ctx context.Context, since synckit.Version) ([]synckit.EventWithVersion, error) {
	if !t.isOnline() {
		return nil, errOffline
	}
	return t.inner.Pull(ctx, since)
}

func (t *OfflineTransport) GetLatestVersion(ctx context.Context) (synckit.Version, error) {
	if !t.isOnline() {
		return nil, errOffline
	}
	return t.inner.GetLatestVersion(ctx)
}

func (t *OfflineTransport) Subscribe(ctx context.Context, h func([]synckit.EventWithVersion) error) error {
	if !t.isOnline() {
		return errOffline
	}
	return t.inner.Subscribe(ctx, h)
}

func (t *OfflineTransport) Close() error { return t.inner.Close() }
