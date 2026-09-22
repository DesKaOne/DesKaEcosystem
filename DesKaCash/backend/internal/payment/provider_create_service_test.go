package payment

import (
	"context"
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/DesKaCash/backend/internal/ledger"
)

func TestProviderCreateServiceStoresProviderResult(t *testing.T) {
	store := NewMemoryPaymentStore()
	provider := &fakeProvider{}
	service := NewProviderCreateService(store, provider)

	payment, err := service.Create(context.Background(), "pay-1", "acct-1", "fake", "idem-1", ledger.FromDIDR(100))
	if err != nil {
		t.Fatal(err)
	}

	if payment.ProviderID != "provider-1" {
		t.Fatalf("expected provider-1, got %q", payment.ProviderID)
	}
	if payment.Reference != "ref-1" {
		t.Fatalf("expected ref-1, got %q", payment.Reference)
	}
	if payment.Status != StatusPending {
		t.Fatalf("expected pending status, got %q", payment.Status)
	}

	got, err := store.Get(context.Background(), "pay-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.ProviderID != "provider-1" {
		t.Fatalf("expected stored provider id provider-1, got %q", got.ProviderID)
	}
}

func TestProviderCreateServicePropagatesProviderError(t *testing.T) {
	store := NewMemoryPaymentStore()
	provider := &failingProvider{}
	service := NewProviderCreateService(store, provider)

	if _, err := service.Create(context.Background(), "pay-1", "acct-1", "fake", "idem-1", ledger.FromDIDR(100)); err == nil {
		t.Fatal("expected provider error")
	}
}

type failingProvider struct{}

func (f *failingProvider) Name() string {
	return "fake"
}

func (f *failingProvider) CreatePayment(context.Context, Payment) (ProviderPayment, error) {
	return ProviderPayment{}, context.DeadlineExceeded
}

func (f *failingProvider) GetPayment(context.Context, string) (ProviderPayment, error) {
	return ProviderPayment{}, context.DeadlineExceeded
}
