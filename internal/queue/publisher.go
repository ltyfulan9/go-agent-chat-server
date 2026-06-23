package queue

import (
	"context"
	"encoding/json"
	"time"
)

type Event struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`
	UserID    string                 `json:"user_id,omitempty"`
	SessionID string                 `json:"session_id,omitempty"`
	CreatedAt time.Time              `json:"created_at"`
	Payload   map[string]interface{} `json:"payload,omitempty"`
}

type Publisher interface {
	Publish(ctx context.Context, event Event) error
	Close() error
}

type NoopPublisher struct{}

func NewNoopPublisher() *NoopPublisher {
	return &NoopPublisher{}
}

func (p *NoopPublisher) Publish(ctx context.Context, event Event) error {
	return nil
}

func (p *NoopPublisher) Close() error {
	return nil
}

func EncodeEvent(event Event) ([]byte, error) {
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now()
	}
	return json.Marshal(event)
}
