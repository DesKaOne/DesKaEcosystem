package payment

import (
	"errors"
	"time"
)

var ErrInvalidWebhookEvent = errors.New("invalid payment webhook event")

type WebhookEvent struct {
	ID         string
	Provider   string
	ProviderID string
	Status     Status
	Amount     int64
	Reference  string
	OccurredAt time.Time
	ReceivedAt time.Time
}

func NewWebhookEvent(id, provider, providerID string, status Status, amount int64) (WebhookEvent, error) {
	if id == "" || provider == "" || providerID == "" || status == "" || amount <= 0 {
		return WebhookEvent{}, ErrInvalidWebhookEvent
	}

	now := time.Now().UTC()
	return WebhookEvent{
		ID:         id,
		Provider:   provider,
		ProviderID: providerID,
		Status:     status,
		Amount:     amount,
		OccurredAt: now,
		ReceivedAt: now,
	}, nil
}
