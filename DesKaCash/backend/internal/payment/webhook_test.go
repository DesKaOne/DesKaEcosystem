package payment

import "testing"

func TestNewWebhookEvent(t *testing.T) {
	event, err := NewWebhookEvent(
		"event-1",
		"demo",
		"provider-1",
		StatusSucceeded,
		100_000,
	)
	if err != nil {
		t.Fatal(err)
	}

	if event.Provider != "demo" {
		t.Fatalf("expected provider demo, got %q", event.Provider)
	}
	if event.ProviderID != "provider-1" {
		t.Fatalf("expected provider id provider-1, got %q", event.ProviderID)
	}
	if event.Status != StatusSucceeded {
		t.Fatalf("expected succeeded status, got %s", event.Status)
	}
	if event.Amount != 100_000 {
		t.Fatalf("expected amount 100000, got %d", event.Amount)
	}
	if event.OccurredAt.IsZero() || event.ReceivedAt.IsZero() {
		t.Fatal("expected webhook timestamps to be set")
	}
}

func TestNewWebhookEventRejectsInvalidInput(t *testing.T) {
	if _, err := NewWebhookEvent("", "demo", "provider-1", StatusSucceeded, 100_000); err != ErrInvalidWebhookEvent {
		t.Fatalf("expected invalid webhook event error, got %v", err)
	}

	if _, err := NewWebhookEvent("event-1", "demo", "provider-1", StatusSucceeded, 0); err != ErrInvalidWebhookEvent {
		t.Fatalf("expected invalid webhook event error, got %v", err)
	}
}
