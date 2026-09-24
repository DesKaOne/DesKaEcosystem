package operational

import (
	"context"
	"errors"
	"testing"
	"time"

	provider "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider"
)

type balanceStub struct {
	balance int64
	err     error
}

func (b balanceStub) GetProducts(context.Context, provider.ProductRequest) ([]provider.Product, error) {
	return nil, provider.ErrUnsupportedOperation
}
func (b balanceStub) Inquiry(context.Context, provider.InquiryRequest) (provider.InquiryResult, error) {
	return provider.InquiryResult{}, provider.ErrUnsupportedOperation
}
func (b balanceStub) Purchase(context.Context, provider.PurchaseRequest) (provider.PurchaseResult, error) {
	return provider.PurchaseResult{}, provider.ErrUnsupportedOperation
}
func (b balanceStub) GetStatus(context.Context, provider.StatusRequest) (provider.PurchaseStatus, error) {
	return provider.PurchaseStatus{}, provider.ErrUnsupportedOperation
}
func (b balanceStub) HandleWebhook(context.Context, provider.WebhookRequest) (provider.WebhookEvent, error) {
	return provider.WebhookEvent{}, provider.ErrUnsupportedOperation
}
func (b balanceStub) GetBalance(context.Context) (int64, error) {
	return b.balance, b.err
}

func TestSyncProviderSuccess(t *testing.T) {
	registry := provider.NewRegistry()
	if err := registry.Register("mock", balanceStub{balance: 1250000}); err != nil {
		t.Fatal(err)
	}
	store := NewMemoryStore()
	svc, err := NewSyncService(registry, store, "IDR", 3)
	if err != nil {
		t.Fatal(err)
	}
	fixed := time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	svc.Now = func() time.Time { return fixed }

	snapshot, err := svc.SyncProvider(context.Background(), "mock")
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Balance != 1250000 || snapshot.Health != HealthHealthy {
		t.Fatalf("unexpected snapshot: %#v", snapshot)
	}
	if !snapshot.LastCheckedAt.Equal(fixed) || !snapshot.LastSuccessAt.Equal(fixed) {
		t.Fatalf("unexpected timestamps: %#v", snapshot)
	}
}

func TestSyncProviderFailureEscalatesHealth(t *testing.T) {
	registry := provider.NewRegistry()
	if err := registry.Register("mock", balanceStub{err: errors.New("provider unavailable")}); err != nil {
		t.Fatal(err)
	}
	store := NewMemoryStore()
	svc, err := NewSyncService(registry, store, "IDR", 2)
	if err != nil {
		t.Fatal(err)
	}
	fixed := time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	svc.Now = func() time.Time { return fixed }

	if _, err := svc.SyncProvider(context.Background(), "mock"); err == nil {
		t.Fatal("expected first sync to fail")
	}
	first, _ := store.Get("mock")
	if first.Health != HealthDegraded || first.ConsecutiveFailures != 1 {
		t.Fatalf("unexpected first failure state: %#v", first)
	}

	if _, err := svc.SyncProvider(context.Background(), "mock"); err == nil {
		t.Fatal("expected second sync to fail")
	}
	second, _ := store.Get("mock")
	if second.Health != HealthUnhealthy || second.ConsecutiveFailures != 2 {
		t.Fatalf("unexpected second failure state: %#v", second)
	}
}

func TestSyncProviderUnsupported(t *testing.T) {
	registry := provider.NewRegistry()
	if err := registry.Register("mock", testProvider{}); err != nil {
		t.Fatal(err)
	}
	store := NewMemoryStore()
	svc, err := NewSyncService(registry, store, "IDR", 2)
	if err != nil {
		t.Fatal(err)
	}

	_, err = svc.SyncProvider(context.Background(), "mock")
	if !errors.Is(err, provider.ErrUnsupportedOperation) {
		t.Fatalf("expected unsupported operation, got %v", err)
	}
}

type testProvider struct{}

func (testProvider) GetProducts(context.Context, provider.ProductRequest) ([]provider.Product, error) {
	return nil, provider.ErrUnsupportedOperation
}
func (testProvider) Inquiry(context.Context, provider.InquiryRequest) (provider.InquiryResult, error) {
	return provider.InquiryResult{}, provider.ErrUnsupportedOperation
}
func (testProvider) Purchase(context.Context, provider.PurchaseRequest) (provider.PurchaseResult, error) {
	return provider.PurchaseResult{}, provider.ErrUnsupportedOperation
}
func (testProvider) GetStatus(context.Context, provider.StatusRequest) (provider.PurchaseStatus, error) {
	return provider.PurchaseStatus{}, provider.ErrUnsupportedOperation
}
func (testProvider) HandleWebhook(context.Context, provider.WebhookRequest) (provider.WebhookEvent, error) {
	return provider.WebhookEvent{}, provider.ErrUnsupportedOperation
}
