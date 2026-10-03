package accounting

import (
	"context"
	"testing"
	"time"

	"github.com/DesKaOne/DesKaEcosystem/DesKaProvider/routing"
	payment "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/internal/Payment"
	provider "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider"
)

func settlementEntries() []Entry {
	return []Entry{
		{LineID:1, AccountID:"provider-clearing", Direction:Debit, Amount:10000, Currency:"IDR"},
		{LineID:2, AccountID:"settlement-in", Direction:Credit, Amount:10000, Currency:"IDR"},
	}
}

func terminalPayment(reference string) routing.TransactionState {
	return routing.TransactionState{
		Kind:routing.TransactionKindPayment,
		Payment:&payment.Transaction{ReferenceID:reference,Amount:10000,Currency:"IDR",CustomerID:"cust",Status:payment.StatusSuccess},
		Execution:routing.PurchaseExecution{ProviderName:"mock",Result:provider.PurchaseResult{ReferenceID:reference,Status:provider.StatusSuccess}},
	}
}

func TestSettlementReconcilerReportsCorrelationStatesWithoutWriting(t *testing.T) {
	ctx:=context.Background()
	txStore:=routing.NewMemoryTransactionStore()
	ledger:=NewMemoryStore()
	poster,_:=NewSettlementPoster(ledger)
	state:=terminalPayment("recon-1")
	if err:=txStore.Put(state);err!=nil{t.Fatal(err)}

	reconciler,err:=NewSettlementReconciler(txStore,ledger,ledger)
	if err!=nil{t.Fatal(err)}
	report,err:=reconciler.Reconcile(ctx)
	if err!=nil{t.Fatal(err)}
	if report.Items[0].Status!=ReconciliationLedgerMissing{t.Fatalf("got %s",report.Items[0].Status)}

	if err:=poster.Post(ctx,SettlementPostingRequest{
		TransactionID:"ledger-recon-1",ReferenceID:"recon-1",SourceType:"PROVIDER_SETTLEMENT",SourceID:"recon-1",
		ProviderStatus:ProviderStatusSuccess,Currency:"IDR",Description:"reconciliation",
		CreatedAt:time.Date(2026,10,3,10,0,0,0,time.UTC),Entries:settlementEntries(),
	}); err!=nil{t.Fatal(err)}

	report,err=reconciler.Reconcile(ctx)
	if err!=nil{t.Fatal(err)}
	if report.Items[0].Status!=ReconciliationCorrelated{t.Fatalf("got %s",report.Items[0].Status)}
}

func TestSettlementReconcilerDoesNotRepairMissingSettlement(t *testing.T) {
	txStore:=routing.NewMemoryTransactionStore()
	ledger:=NewMemoryStore()
	state:=terminalPayment("recon-no-repair")
	if err:=txStore.Put(state);err!=nil{t.Fatal(err)}
	before:=len(ledger.All())
	reconciler,_:=NewSettlementReconciler(txStore,ledger,ledger)
	report,err:=reconciler.Reconcile(context.Background())
	if err!=nil{t.Fatal(err)}
	if report.Items[0].Status!=ReconciliationLedgerMissing{t.Fatal(report.Items[0].Status)}
	if len(ledger.All())!=before{t.Fatal("reconciliation must not create ledger entries")}
}


func TestSettlementReconcilerReportsOrphanedLedgerAndAudit(t *testing.T) {
	ctx := context.Background()
	txStore := routing.NewMemoryTransactionStore()
	ledger := NewMemoryStore()

	if err := ledger.Append(LedgerTransaction{
		ID:"orphan-ledger", ReferenceID:"orphan-reference", SourceType:"PROVIDER_SETTLEMENT", SourceID:"orphan-source",
		Currency:"IDR", Description:"orphan", CreatedAt:time.Date(2026,10,3,11,0,0,0,time.UTC),
		Entries:settlementEntries(),
	}); err != nil { t.Fatal(err) }

	audit := SettlementAudit{
		EventID:"orphan-audit", TransactionID:"missing-ledger", ReferenceID:"missing-reference",
		SourceType:"PROVIDER_SETTLEMENT", SourceID:"missing-source", Status:ProviderStatusSuccess,
		CreatedAt:time.Date(2026,10,3,11,1,0,0,time.UTC),
	}
	ledger.audits[audit.TransactionID] = audit

	reconciler, err := NewSettlementReconciler(txStore, ledger, ledger)
	if err != nil { t.Fatal(err) }
	report, err := reconciler.Reconcile(ctx)
	if err != nil { t.Fatal(err) }

	var orphanLedger, orphanAudit bool
	for _, item := range report.Items {
		switch item.Status {
		case ReconciliationOrphanedLedger:
			orphanLedger = item.LedgerTransactionID == "orphan-ledger"
		case ReconciliationOrphanedAudit:
			orphanAudit = item.SettlementAuditEventID == "orphan-audit"
		}
	}
	if !orphanLedger { t.Fatal("expected orphaned ledger diagnostic") }
	if !orphanAudit { t.Fatal("expected orphaned audit diagnostic") }
	if len(txStore.All()) != 0 { t.Fatal("reconciliation must not create provider state") }
}
