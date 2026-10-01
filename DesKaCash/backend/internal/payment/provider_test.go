package payment

import (
	"context"
	"errors"
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/DesKaCash/internal/ledger"
)

type fakeProvider struct{}

func (fakeProvider) CreatePayment(context.Context, Payment) (ProviderPayment, error) {
	return ProviderPayment{
		ExternalReference: "provider-1",
		Status:    StatusPending,
		Amount:    ledger.Money{BaseUnits: 100_000},
		Reference: "ref-1",
	}, nil
}

func (fakeProvider) GetPayment(context.Context, string) (ProviderPayment, error) {
	return ProviderPayment{}, errors.New("not implemented")
}

func TestFakeProviderSatisfiesProvider(t *testing.T) {
	var provider DesKaProviderClient = fakeProvider{}

	got, err := provider.CreatePayment(context.Background(), Payment{
		ID:     "pay-1",
		Amount: ledger.Money{BaseUnits: 100_000},
	})
	if err != nil {
		t.Fatal(err)
	}

	if got.ExternalReference != "provider-1" {
		t.Fatalf("expected provider payment id provider-1, got %q", got.ExternalReference)
	}
	if got.Status != StatusPending {
		t.Fatalf("expected pending status, got %s", got.Status)
	}
	if got.Reference != "ref-1" {
		t.Fatalf("expected reference ref-1, got %q", got.Reference)
	}
}
