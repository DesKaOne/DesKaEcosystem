package payment

import (
	"errors"
	"time"
)

var ErrInvalidWebhookEvent = errors.New("invalid payment webhook event")

// WebhookEvent is a normalized event delivered by DesKaProvider.
// It contains no provider-specific payload or routing information.
type WebhookEvent struct {
	ID               string
	ExternalReference string
	Status           Status
	Amount           int64
	Reference        string
	OccurredAt       time.Time
	ReceivedAt       time.Time
}

func NewWebhookEvent(id, externalReference string, status Status, amount int64) (WebhookEvent, error) {
	if id == "" || externalReference == "" || status == "" || amount <= 0 {
		return WebhookEvent{}, ErrInvalidWebhookEvent
	}

	now := time.Now().UTC()
	return WebhookEvent{
		ID:                id,
		ExternalReference: externalReference,
		Status:            status,
		Amount:            amount,
		OccurredAt:        now,
		ReceivedAt:        now,
	}, nil
}
