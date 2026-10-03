package accounting

import (\n\t"context"\n\t"testing"\n\t"time"\n\n\t"github.com/DesKaOne/DesKaEcosystem/DesKaProvider/routing"
	"context"
	"testing"
	"time"
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
