package payment

import (
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/DesKaCash/internal/ledger"
)

func TestNewPayment(t *testing.T) {
	payment, err := NewPayment(
		"pay-1",
		"acc-1",
		"demo",
		"idempotency-1",
		ledger.Money{BaseUnits: 100_000},
	)
	if err != nil {
		t.Fatal(err)
	}

	if payment.Status != StatusPending {
		t.Fatalf("expected pending status, got %s", payment.Status)
	}
	if payment.Provider != "demo" {
		t.Fatalf("expected provider demo, got %s", payment.Provider)
	}
	if payment.IdempotencyKey != "idempotency-1" {
		t.Fatalf("expected idempotency key, got %s", payment.IdempotencyKey)
	}
	if payment.Amount.BaseUnits != 100_000 {
		t.Fatalf("expected 100000 base units, got %d", payment.Amount.BaseUnits)
	}
	if payment.CreatedAt.IsZero() || payment.UpdatedAt.IsZero() {
		t.Fatal("expected payment timestamps to be set")
	}
}

func TestNewPaymentRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name           string
		id             string
		accountID      string
		provider       string
		idempotencyKey string
		amount         ledger.Money
	}{
		{"missing id", "", "acc-1", "demo", "key-1", ledger.Money{BaseUnits: 1}},
		{"missing account", "pay-1", "", "demo", "key-1", ledger.Money{BaseUnits: 1}},
		{"missing provider", "pay-1", "acc-1", "", "key-1", ledger.Money{BaseUnits: 1}},
		{"missing idempotency key", "pay-1", "acc-1", "demo", "", ledger.Money{BaseUnits: 1}},
		{"zero amount", "pay-1", "acc-1", "demo", "key-1", ledger.Money{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := NewPayment(tt.id, tt.accountID, tt.provider, tt.idempotencyKey, tt.amount); err != ErrInvalidPayment {
				t.Fatalf("expected invalid payment error, got %v", err)
			}
		})
	}
}
