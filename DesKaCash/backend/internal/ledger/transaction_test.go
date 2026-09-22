package ledger

import "testing"

func TestNewTransaction(t *testing.T) {
	tx, err := NewTransaction("tx-1", "acc-1", Money{BaseUnits: 1000}, "topup")
	if err != nil {
		t.Fatal(err)
	}

	if tx.Status != StatusPending {
		t.Fatalf("expected pending status, got %s", tx.Status)
	}
	if tx.Asset != AssetDIDR {
		t.Fatalf("expected dIDR asset, got %s", tx.Asset)
	}
}
