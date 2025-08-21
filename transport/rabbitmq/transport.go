package rabbitmq

import (
	"context"
	"fmt"

	synckit "github.com/c0deZ3R0/go-sync-kit/synckit"
)

// Transport is a minimal skeleton to satisfy compiler once wired.
// Full implementation will be added per RABBITMQ_ROADMAP.md.
type Transport struct {
	cfg *Config
}

// NewTransport creates a RabbitMQ transport instance (not connected yet).
func NewTransport(cfg *Config) *Transport {
	return &Transport{cfg: cfg}
}

func (t *Transport) Push(ctx context.Context, events []synckit.EventWithVersion) error {
	return fmt.Errorf("rabbitmq transport Push not yet implemented")
}

func (t *Transport) Pull(ctx context.Context, since synckit.Version) ([]synckit.EventWithVersion, error) {
	return nil, fmt.Errorf("rabbitmq transport Pull not yet implemented; consider HTTP Pull + RabbitMQ Subscribe hybrid")
}

func (t *Transport) GetLatestVersion(ctx context.Context) (synckit.Version, error) {
	return nil, fmt.Errorf("rabbitmq transport GetLatestVersion not yet implemented; use store/HTTP path")
}

func (t *Transport) Subscribe(ctx context.Context, handler func([]synckit.EventWithVersion) error) error {
	return fmt.Errorf("rabbitmq transport Subscribe not yet implemented")
}

func (t *Transport) Close() error {
	return nil
}
