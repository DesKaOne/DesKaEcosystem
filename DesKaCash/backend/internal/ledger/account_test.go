package ledger

import "testing"

func TestAccountCreditAndDebit(t *testing.T) {
	account, err := NewAccount("acc-1", "user-1")
	if err != nil {
		t.Fatal(err)
	}

	if err := account.Credit(Money{BaseUnits: 1000}); err != nil {
		t.Fatal(err)
	}
	if account.Balance.BaseUnits != 1000 {
		t.Fatalf("expected 1000 base units, got %d", account.Balance.BaseUnits)
	}

	if err := account.Debit(Money{BaseUnits: 250}); err != nil {
		t.Fatal(err)
	}
	if account.Balance.BaseUnits != 750 {
		t.Fatalf("expected 750 base units, got %d", account.Balance.BaseUnits)
	}
}

func TestAccountRejectsInsufficientFunds(t *testing.T) {
	account, err := NewAccount("acc-1", "user-1")
	if err != nil {
		t.Fatal(err)
	}

	err = account.Debit(Money{BaseUnits: 1})
	if err != ErrInsufficientFunds {
		t.Fatalf("expected ErrInsufficientFunds, got %v", err)
	}
}

func TestAccountRejectsNegativeMoney(t *testing.T) {
	account, err := NewAccount("acc-1", "user-1")
	if err != nil {
		t.Fatal(err)
	}

	if err := account.Credit(Money{BaseUnits: -1}); err != ErrInvalidAccount {
		t.Fatalf("expected ErrInvalidAccount, got %v", err)
	}
}
