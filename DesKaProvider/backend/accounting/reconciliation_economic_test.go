package accounting

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DesKaOne/DesKaEcosystem/DesKaProvider/routing"
)

func TestSettlementReconcilerDoesNotCorrelateWhenLedgerAmountDisagrees(t *testing.T) {
	ctx := context.Background()
	txStore := routing.NewMemoryTransactionStore()
	ledger := NewMemoryStore()
	state := terminalPayment("economic-amount-conflict")
	if err := txStore.Put(state); err != nil {
		t.Fatal(err)
	}

	entries := []Entry{
		{LineID: 1, AccountID: "provider-clearing", Direction: Debit, Amount: 9999, Currency: "IDR"},
		{LineID: 2, AccountID: "settlement-in", Direction: Credit, Amount: 9999, Currency: "IDR"},
	}
	ledgerTx := LedgerTransaction{
		ID: "economic-amount-ledger", ReferenceID: state.Payment.ReferenceID,
		SourceType: "PROVIDER_SETTLEMENT", SourceID: "economic-amount-source",
		Currency: "IDR", Description: "amount conflict",
		CreatedAt: time.Date(2026, 10, 4, 11, 0, 0, 0, time.UTC), Entries: entries,
	}
	audit := SettlementAudit{
		EventID: "economic-amount-event", TransactionID: ledgerTx.ID,
		ReferenceID: ledgerTx.ReferenceID, SourceType: ledgerTx.SourceType,
		SourceID: ledgerTx.SourceID, Status: ProviderStatusSuccess, CreatedAt: ledgerTx.CreatedAt,
	}
	if err := ledger.AppendSettlement(ctx, ledgerTx, audit); err != nil {
		t.Fatal(err)
	}

	reconciler, err := NewSettlementReconciler(txStore, ledger, ledger)
	if err != nil {
		t.Fatal(err)
	}
	report, err := reconciler.Reconcile(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Items) != 1 || report.Items[0].Status != ReconciliationCorrelationConflict {
		t.Fatalf("expected amount conflict, got %#v", report.Items)
	}
	if len(ledger.All()) != 1 {
		t.Fatal("reconciliation must remain read-only")
	}
}

func TestSettlementReconcilerDoesNotCorrelateWhenLedgerCurrencyDisagrees(t *testing.T) {
	ctx := context.Background()
	txStore := routing.NewMemoryTransactionStore()
	ledger := NewMemoryStore()
	state := terminalPayment("economic-currency-conflict")
	if err := txStore.Put(state); err != nil {
		t.Fatal(err)
	}

	entries := []Entry{
		{LineID: 1, AccountID: "provider-clearing", Direction: Debit, Amount: 10000, Currency: "USD"},
		{LineID: 2, AccountID: "settlement-in", Direction: Credit, Amount: 10000, Currency: "USD"},
	}
	ledgerTx := LedgerTransaction{
		ID: "economic-currency-ledger", ReferenceID: state.Payment.ReferenceID,
		SourceType: "PROVIDER_SETTLEMENT", SourceID: "economic-currency-source",
		Currency: "USD", Description: "currency conflict",
		CreatedAt: time.Date(2026, 10, 4, 11, 1, 0, 0, time.UTC), Entries: entries,
	}
	audit := SettlementAudit{
		EventID: "economic-currency-event", TransactionID: ledgerTx.ID,
		ReferenceID: ledgerTx.ReferenceID, SourceType: ledgerTx.SourceType,
		SourceID: ledgerTx.SourceID, Status: ProviderStatusSuccess, CreatedAt: ledgerTx.CreatedAt,
	}
	if err := ledger.AppendSettlement(ctx, ledgerTx, audit); err != nil {
		t.Fatal(err)
	}

	reconciler, err := NewSettlementReconciler(txStore, ledger, ledger)
	if err != nil {
		t.Fatal(err)
	}
	report, err := reconciler.Reconcile(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Items) != 1 || report.Items[0].Status != ReconciliationCorrelationConflict {
		t.Fatalf("expected currency conflict, got %#v", report.Items)
	}
	if len(ledger.All()) != 1 {
		t.Fatal("reconciliation must remain read-only")
	}
}

func TestLedgerTransactionValidateRejectsAmountOverflow(t *testing.T) {
	tx := LedgerTransaction{
		ID: "overflow-ledger", ReferenceID: "overflow-ref",
		SourceType: "PROVIDER_SETTLEMENT", SourceID: "overflow-source",
		Currency: "IDR", CreatedAt: time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC),
		Entries: []Entry{
			{LineID: 1, AccountID: "a", Direction: Debit, Amount: 9223372036854775807, Currency: "IDR"},
			{LineID: 2, AccountID: "b", Direction: Debit, Amount: 1, Currency: "IDR"},
			{LineID: 3, AccountID: "c", Direction: Credit, Amount: 9223372036854775807, Currency: "IDR"},
		},
	}
	if err := tx.Validate(); err == nil {
		t.Fatal("expected amount overflow to be rejected")
	} else if !errors.Is(err, ErrInvalidLedgerTransaction) {
		t.Fatalf("expected ErrInvalidLedgerTransaction, got %v", err)
	}
}

func TestReconciliationEconomicAgreementRejectsOverflow(t *testing.T) {
	state := terminalPayment("economic-overflow-conflict")
	tx := LedgerTransaction{
		ID: "economic-overflow-ledger", ReferenceID: state.Payment.ReferenceID,
		SourceType: "PROVIDER_SETTLEMENT", SourceID: "economic-overflow-source",
		Currency: state.Payment.Currency,
		CreatedAt: time.Date(2026, 10, 4, 12, 1, 0, 0, time.UTC),
		Entries: []Entry{
			{LineID: 1, AccountID: "a", Direction: Debit, Amount: 9223372036854775807, Currency: state.Payment.Currency},
			{LineID: 2, AccountID: "b", Direction: Debit, Amount: 1, Currency: state.Payment.Currency},
			{LineID: 3, AccountID: "c", Direction: Credit, Amount: 9223372036854775807, Currency: state.Payment.Currency},
		},
	}
	if reconciliationEconomicAgreement(state, tx) {
		t.Fatal("economic agreement must fail closed when persisted debit aggregation overflows")
	}
}
