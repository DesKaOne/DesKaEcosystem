package accounting

import (
	"testing"
	"time"
)

func validLedgerTransaction() LedgerTransaction {
	return LedgerTransaction{
		ID: "ledger-1", ReferenceID: "payment-1",
		SourceType: "PAYMENT", SourceID: "payment-1",
		Currency: "IDR", Description: "customer payment",
		CreatedAt: time.Date(2026, 10, 3, 4, 0, 0, 0, time.UTC),
		Entries: []Entry{
			{LineID: 1, AccountID: "customer-main", Direction: Debit, Amount: 10000, Currency: "IDR"},
			{LineID: 2, AccountID: "provider-clearing", Direction: Credit, Amount: 10000, Currency: "IDR"},
		},
	}
}

func TestLedgerTransactionRequiresBalancedDoubleEntry(t *testing.T) {
	tx := validLedgerTransaction()
	if err := tx.Validate(); err != nil {
		t.Fatal(err)
	}
	tx.Entries[1].Amount = 9999
	if err := tx.Validate(); err == nil {
		t.Fatal("unbalanced ledger transaction must be rejected")
	}
}

func TestLedgerTransactionRejectsMixedCurrency(t *testing.T) {
	tx := validLedgerTransaction()
	tx.Entries[1].Currency = "dIDR"
	if err := tx.Validate(); err == nil {
		t.Fatal("mixed-currency entries must be rejected")
	}
}

func TestLedgerTimestampIdentityUsesStoragePrecision(t *testing.T) {
	a := validLedgerTransaction()
	b := a
	b.CreatedAt = b.CreatedAt.Add(900 * time.Nanosecond)
	if !sameLedgerTransaction(a, b) {
		t.Fatal("sub-microsecond timestamp differences must not break ledger identity")
	}
}

func TestMemoryLedgerIsImmutableAndIdempotent(t *testing.T) {
	store := NewMemoryStore()
	tx := validLedgerTransaction()
	if err := store.Append(tx); err != nil {
		t.Fatal(err)
	}
	tx.Entries[0].Memo = "mutated after append"
	got, ok := store.Get(tx.ID)
	if !ok {
		t.Fatal("ledger transaction not found")
	}
	if got.Entries[0].Memo != "" {
		t.Fatal("stored ledger entry was mutated through caller memory")
	}
	if err := store.Append(validLedgerTransaction()); err != nil {
		t.Fatalf("identical append must be idempotent: %v", err)
	}
	conflict := validLedgerTransaction()
	conflict.Description = "different transaction"
	if err := store.Append(conflict); err != ErrLedgerConflict {
		t.Fatalf("expected immutable identity conflict, got %v", err)
	}
}

func TestAccountTypesAreExtensibleWithoutBalanceSemantics(t *testing.T) {
	store := NewMemoryAccountStore()
	account := Account{
		ID: "reserve-idr", Type: AccountTypeReserve,
		Currency: "IDR", Name: "Provider Reserve", Active: true,
	}
	got, created, err := store.CreateIfAbsent(account)
	if err != nil || !created || got != account {
		t.Fatalf("unexpected account creation result: %#v %v %v", got, created, err)
	}
	got, created, err = store.CreateIfAbsent(account)
	if err != nil || created || got != account {
		t.Fatalf("expected idempotent account creation: %#v %v %v", got, created, err)
	}
	conflict := account
	conflict.Type = AccountTypeFee
	if _, _, err := store.CreateIfAbsent(conflict); err != ErrAccountConflict {
		t.Fatalf("expected account conflict, got %v", err)
	}
}
