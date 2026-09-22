package ledger

import "testing"

func TestServiceApplyCreditIsIdempotentByTransactionID(t *testing.T) {
	account, err := NewAccount("acc-1", "user-1")
	if err != nil {
		t.Fatal(err)
	}

	service := NewService()
	if err := service.RegisterAccount(account); err != nil {
		t.Fatal(err)
	}

	tx, err := NewTransaction("topup-1", "acc-1", Money{BaseUnits: 100_000}, "topup")
	if err != nil {
		t.Fatal(err)
	}

	if err := service.ApplyCredit(tx, "provider:demo-1"); err != nil {
		t.Fatal(err)
	}

	if err := service.ApplyCredit(tx, "provider:demo-1"); err != ErrDuplicateTransaction {
		t.Fatalf("expected duplicate transaction error, got %v", err)
	}

	accountAfter, ok := service.GetAccount("acc-1")
	if !ok {
		t.Fatal("account not found")
	}

	if accountAfter.Balance.BaseUnits != 100_000 {
		t.Fatalf("expected 100000 base units, got %d", accountAfter.Balance.BaseUnits)
	}

	if len(service.Entries("acc-1")) != 1 {
		t.Fatalf("expected 1 ledger entry, got %d", len(service.Entries("acc-1")))
	}
}
