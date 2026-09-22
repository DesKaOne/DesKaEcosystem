package ledger

import "testing"

func TestFromDIDR(t *testing.T) {
	m := FromDIDR(100)

	if m.BaseUnits != 100_000 {
		t.Fatalf("expected 100000 base units, got %d", m.BaseUnits)
	}

	if m.DIDR() != 100 {
		t.Fatalf("expected 100 dIDR, got %d", m.DIDR())
	}
}

func TestBaseUnitRepresentation(t *testing.T) {
	m := Money{BaseUnits: 1}

	if m.IsZero() {
		t.Fatal("1 base unit must not be zero")
	}
}
