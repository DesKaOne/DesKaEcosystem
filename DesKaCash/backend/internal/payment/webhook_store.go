package payment

import (
	"context"
	"errors"
	"sync"
)

var (
	ErrDuplicateWebhook = errors.New("duplicate payment webhook")
	ErrWebhookNotFound  = errors.New("payment webhook not found")
)

type WebhookStore interface {
	Record(ctx context.Context, event WebhookEvent) error
	Get(ctx context.Context, id string) (WebhookEvent, error)
}

type MemoryWebhookStore struct {
	mu     sync.RWMutex
	events map[string]WebhookEvent
}

func NewMemoryWebhookStore() *MemoryWebhookStore {
	return &MemoryWebhookStore{
		events: make(map[string]WebhookEvent),
	}
}

func (s *MemoryWebhookStore) Record(ctx context.Context, event WebhookEvent) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.events[event.ID]; exists {
		return ErrDuplicateWebhook
	}

	s.events[event.ID] = event
	return nil
}

func (s *MemoryWebhookStore) Get(ctx context.Context, id string) (WebhookEvent, error) {
	if err := ctx.Err(); err != nil {
		return WebhookEvent{}, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	event, ok := s.events[id]
	if !ok {
		return WebhookEvent{}, ErrWebhookNotFound
	}
	return event, nil
}
