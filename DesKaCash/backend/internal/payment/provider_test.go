package payment

import (
	"context"
	"errors"
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/DesKaCash/backend/internal/ledger"
)

type fakeProvider struct{}

func (fakeProvider) Name() string {
	return "fake"
}

func (fakeProvider) CreatePayment(context.Context, Payment) (ProviderPayment, error) {
	return ProviderPayment{
		ID:     "provider-1",
		Status: StatusPending,
		Amount: ledger.Money{BaseUnits: 100_000},
	}, nil
}

func (fakeProvider) GetPayment(context.Context, string) (ProviderPayment, error) {
	return ProviderPayment{}, errors.New("not implemented")
}

func TestFakeProviderSatisfiesProvider(t *testing.T) {
	var provider Provider = fakeProvider{}

	if provider.Name() != "fake" {
		t.Fatalf("expected fake provider name, got %q", provider.Name())
	}

	got, err := provider.CreatePayment(context.Background(), Payment{
		ID:     "pay-1",
		Amount: ledger.Money{BaseUnits: 100_000},
	})
	if err != nil {
		t.Fatal(err)
	}

	if got.ID != "provider-1" {
		t.Fatalf("expected provider payment id provider-1, got %q", got.ID)
	}
	if got.Status != StatusPending {
		t.Fatalf("expected pending status, got %s", got.Status)
	}
}
