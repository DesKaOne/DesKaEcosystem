package ledger

import "testing"

func TestFromIDR(t *testing.T) {
	m := FromIDR(100_000)

	if m.BaseUnits != 100_000 {
		t.Fatalf("expected 100000 IDR base units, got %d", m.BaseUnits)
	}

	if m.IDR() != 100_000 {
		t.Fatalf("expected 100000 IDR, got %d", m.IDR())
}

func TestFromDIDR(t *testing.T) {
	m := FromDIDR(100)

	if m.BaseUnits != 100_000 {
		t.Fatalf("expected 100000 dIDR base units, got %d", m.BaseUnits)
	}

	if m.DIDR() != 100 {
		t.Fatalf("expected 100 dIDR, got %d", m.DIDR())
}

func TestBaseUnitRepresentation(t *testing.T) {
	m := Money{BaseUnits: 1}

	if m.IsZero() {
		t.Fatal("1 base unit must not be zero")
	}
}
