package payment

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestMemoryWebhookStoreIsIdempotent(t *testing.T) {
	store := NewMemoryWebhookStore()
	event := WebhookEvent{
		ID:         "event-1",
		ExternalReference: "provider-1",
		Status:     StatusSucceeded,
		Amount:     100_000,
		OccurredAt: time.Now().UTC(),
		ReceivedAt: time.Now().UTC(),
	}

	if err := store.Record(context.Background(), event); err != nil {
		t.Fatal(err)
	}

	if err := store.Record(context.Background(), event); !errors.Is(err, ErrDuplicateWebhook) {
		t.Fatalf("expected duplicate webhook error, got %v", err)
	}

	got, err := store.Get(context.Background(), event.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ExternalReference != event.ExternalReference {
		t.Fatalf("expected external reference %q, got %q", event.ExternalReference, got.ExternalReference)
	}
}

func TestMemoryWebhookStoreNotFound(t *testing.T) {
	store := NewMemoryWebhookStore()

	if _, err := store.Get(context.Background(), "missing"); !errors.Is(err, ErrWebhookNotFound) {
		t.Fatalf("expected webhook not found error, got %v", err)
	}
}
