package payment

import (
	"context"
	"errors"
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/DesKaCash/internal/ledger"
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

	_, err := service.Create(context.Background(), "pay-1", "acct-1", "fake", "idem-1", ledger.FromDIDR(100))
	if err == nil {
		t.Fatal("expected provider error")
	}

	got, getErr := store.Get(context.Background(), "pay-1")
	if getErr != nil {
		t.Fatal(getErr)
	}
	if got.Status != StatusPending {
		t.Fatalf("expected pending payment after provider error, got %q", got.Status)
	}
}

func TestProviderCreateServiceRejectsAmountMismatch(t *testing.T) {
	store := NewMemoryPaymentStore()
	provider := &mismatchProvider{}
	service := NewProviderCreateService(store, provider)

	_, err := service.Create(context.Background(), "pay-1", "acct-1", "fake", "idem-1", ledger.FromDIDR(100))
	if !errors.Is(err, ErrProviderAmountMismatch) {
		t.Fatalf("expected amount mismatch, got %v", err)
	}

	got, getErr := store.Get(context.Background(), "pay-1")
	if getErr != nil {
		t.Fatal(getErr)
	}
	if got.Status != StatusPending {
		t.Fatalf("expected pending payment after amount mismatch, got %q", got.Status)
	}
}

func TestProviderCreateServiceIsIdempotent(t *testing.T) {
	store := NewMemoryPaymentStore()
	provider := &countingProvider{}
	service := NewProviderCreateService(store, provider)

	first, err := service.Create(context.Background(), "pay-1", "acct-1", "fake", "idem-1", ledger.FromDIDR(100))
	if err != nil {
		t.Fatal(err)
	}

	second, err := service.Create(context.Background(), "pay-2", "acct-1", "fake", "idem-1", ledger.FromDIDR(100))
	if err != nil {
		t.Fatal(err)
	}

	if first.ID != second.ID {
		t.Fatalf("expected same payment id, got %q and %q", first.ID, second.ID)
	}
	if provider.calls != 1 {
		t.Fatalf("expected provider to be called once, got %d", provider.calls)
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

type mismatchProvider struct{}

func (mismatchProvider) Name() string {
	return "fake"
}

func (mismatchProvider) CreatePayment(context.Context, Payment) (ProviderPayment, error) {
	return ProviderPayment{
		ID:     "provider-1",
		Status: StatusPending,
		Amount: ledger.FromDIDR(99),
	}, nil
}

func (mismatchProvider) GetPayment(context.Context, string) (ProviderPayment, error) {
	return ProviderPayment{}, context.DeadlineExceeded
}

type countingProvider struct {
	calls int
}

func (p *countingProvider) Name() string {
	return "fake"
}

func (p *countingProvider) CreatePayment(context.Context, Payment) (ProviderPayment, error) {
	p.calls++
	return ProviderPayment{
		ID:        "provider-1",
		Status:    StatusPending,
		Amount:    ledger.FromDIDR(100),
		Reference: "ref-1",
	}, nil
}

func (p *countingProvider) GetPayment(context.Context, string) (ProviderPayment, error) {
	return ProviderPayment{}, context.DeadlineExceeded
}
