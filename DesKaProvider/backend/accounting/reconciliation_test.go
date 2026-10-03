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


type duplicateAuditReader struct {
	SettlementAuditReader
	allAudits []SettlementAudit
}

func (r duplicateAuditReader) AllSettlementAudits(_ context.Context) ([]SettlementAudit, error) {
	return append([]SettlementAudit(nil), r.allAudits...), nil
}

func TestSettlementReconcilerReportsDuplicateLedgerReferenceDeterministically(t *testing.T) {
	ctx := context.Background()
	txStore := routing.NewMemoryTransactionStore()
	ledger := NewMemoryStore()
	state := terminalPayment("duplicate-reference")
	if err := txStore.Put(state); err != nil { t.Fatal(err) }

	for _, id := range []string{"ledger-b", "ledger-a"} {
		if err := ledger.Append(LedgerTransaction{
			ID:id, ReferenceID:"duplicate-reference", SourceType:"PROVIDER_SETTLEMENT", SourceID:id,
			Currency:"IDR", Description:"duplicate", CreatedAt:time.Date(2026,10,3,13,0,0,0,time.UTC),
			Entries:settlementEntries(),
		}); err != nil { t.Fatal(err) }
	}

	reconciler, err := NewSettlementReconciler(txStore, ledger, ledger)
	if err != nil { t.Fatal(err) }
	report, err := reconciler.Reconcile(ctx)
	if err != nil { t.Fatal(err) }
	if len(report.Items) != 1 { t.Fatalf("got %d items: %#v", len(report.Items), report.Items) }
	item := report.Items[0]
	if item.Status != ReconciliationDuplicateReference {
		t.Fatalf("got %s", item.Status)
	}
	want := []string{"ledger-a", "ledger-b"}
	if len(item.LedgerTransactionIDs) != len(want) {
		t.Fatalf("got IDs %#v", item.LedgerTransactionIDs)
	}
	for i := range want {
		if item.LedgerTransactionIDs[i] != want[i] {
			t.Fatalf("got IDs %#v", item.LedgerTransactionIDs)
		}
	}
	if len(ledger.All()) != 2 { t.Fatal("reconciliation must not mutate ledger state") }
}

type duplicateProviderReferenceStore struct {
	*routing.MemoryTransactionStore
	duplicate routing.TransactionState
}

func (s duplicateProviderReferenceStore) AllContextE(ctx context.Context) ([]routing.TransactionState, error) {
	states, err := s.MemoryTransactionStore.AllContextE(ctx)
	if err != nil {
		return nil, err
	}
	return append(states, s.duplicate), nil
}

func TestSettlementReconcilerReportsDuplicateProviderReference(t *testing.T) {
	ctx := context.Background()
	base := terminalPayment("same-provider-reference")
	duplicate := terminalPayment("same-provider-reference")
	txStore := duplicateProviderReferenceStore{
		MemoryTransactionStore: routing.NewMemoryTransactionStore(),
		duplicate:              duplicate,
	}
	if err := txStore.Put(base); err != nil {
		t.Fatal(err)
	}
	ledger := NewMemoryStore()

	reconciler, err := NewSettlementReconciler(txStore, ledger, ledger)
	if err != nil {
		t.Fatal(err)
	}
	report, err := reconciler.Reconcile(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Items) != 2 {
		t.Fatalf("got %d items: %#v", len(report.Items), report.Items)
	}
	for _, item := range report.Items {
		if item.Status != ReconciliationDuplicateReference {
			t.Fatalf("got %s", item.Status)
		}
	}
}


func TestSettlementAuditIdentityConflictIsExplicit(t *testing.T) {
	ledger := NewMemoryStore()
	base := LedgerTransaction{
		ID:"audit-ledger-a", ReferenceID:"audit-ref-a", SourceType:"PROVIDER_SETTLEMENT", SourceID:"audit-source-a",
		Currency:"IDR", Description:"audit", CreatedAt:time.Date(2026,10,3,14,0,0,0,time.UTC), Entries:settlementEntries(),
	}
	first := SettlementAudit{EventID:"duplicate-event", TransactionID:base.ID, ReferenceID:base.ReferenceID, SourceType:base.SourceType, SourceID:base.SourceID, Status:ProviderStatusSuccess, CreatedAt:base.CreatedAt}
	if err := ledger.AppendSettlement(context.Background(), base, first); err != nil { t.Fatal(err) }
	second := base
	second.ID = "audit-ledger-b"
	second.ReferenceID = "audit-ref-b"
	second.SourceID = "audit-source-b"
	second.CreatedAt = base.CreatedAt.Add(time.Minute)
	secondAudit := first
	secondAudit.TransactionID = second.ID
	secondAudit.ReferenceID = second.ReferenceID
	secondAudit.SourceID = second.SourceID
	secondAudit.CreatedAt = second.CreatedAt
	if err := ledger.AppendSettlement(context.Background(), second, secondAudit); err != ErrSettlementAuditConflict {
		t.Fatalf("got %v, want %v", err, ErrSettlementAuditConflict)
	}
}

func TestSettlementReconcilerReportsDuplicateAuditIdentityDeterministically(t *testing.T) {
	ctx := context.Background()
	ledger := NewMemoryStore()
	base := terminalPayment("audit-duplicate-ref")
	txStore := routing.NewMemoryTransactionStore()
	if err := txStore.Put(base); err != nil { t.Fatal(err) }
	audits := []SettlementAudit{
		{EventID:"event-a", TransactionID:"ledger-b", ReferenceID:"audit-duplicate-ref", SourceType:"PROVIDER_SETTLEMENT", SourceID:"source-b", Status:ProviderStatusSuccess, CreatedAt:time.Date(2026,10,3,14,0,0,0,time.UTC)},
		{EventID:"event-a", TransactionID:"ledger-a", ReferenceID:"audit-duplicate-ref", SourceType:"PROVIDER_SETTLEMENT", SourceID:"source-a", Status:ProviderStatusSuccess, CreatedAt:time.Date(2026,10,3,14,1,0,0,time.UTC)},
	}
	reader := duplicateAuditReader{SettlementAuditReader:ledger, allAudits:audits}
	reconciler, err := NewSettlementReconciler(txStore, ledger, reader)
	if err != nil { t.Fatal(err) }
	report, err := reconciler.Reconcile(ctx)
	if err != nil { t.Fatal(err) }
	var found bool
	for _, item := range report.Items {
		if item.Status == ReconciliationDuplicateAuditIdentity && item.SettlementAuditEventID == "event-a" {
			found = true
			if len(item.LedgerTransactionIDs) != 2 || item.LedgerTransactionIDs[0] != "ledger-a" || item.LedgerTransactionIDs[1] != "ledger-b" {
				t.Fatalf("unexpected audit candidates: %#v", item.LedgerTransactionIDs)
			}
		}
	}
	if !found { t.Fatal("expected duplicate audit identity diagnostic") }
}


func TestSettlementReconcilerReportOrderingIsDeterministic(t *testing.T) {
	ctx := context.Background()
	txStore := routing.NewMemoryTransactionStore()
	ledger := NewMemoryStore()
	states := []string{"ref-c", "ref-a", "ref-b"}
	for _, ref := range states {
		if err := txStore.Put(terminalPayment(ref)); err != nil { t.Fatal(err) }
	}
	reconciler, err := NewSettlementReconciler(txStore, ledger, ledger)
	if err != nil { t.Fatal(err) }
	first, err := reconciler.Reconcile(ctx)
	if err != nil { t.Fatal(err) }
	second, err := reconciler.Reconcile(ctx)
	if err != nil { t.Fatal(err) }
	if len(first.Items) != len(second.Items) { t.Fatalf("report lengths differ: %d vs %d", len(first.Items), len(second.Items)) }
	for i := range first.Items {
		if first.Items[i].ReferenceID != second.Items[i].ReferenceID ||
			first.Items[i].Status != second.Items[i].Status {
			t.Fatalf("report ordering is unstable: %#v vs %#v", first.Items, second.Items)
		}
	}
	want := []string{"ref-a", "ref-b", "ref-c"}
	for i, ref := range want {
		if first.Items[i].ReferenceID != ref { t.Fatalf("got order %#v", first.Items) }
	}
}

func TestSettlementReconcilerMaterializesOneReadSnapshot(t *testing.T) {
	ctx := context.Background()
	txStore := routing.NewMemoryTransactionStore()
	ledger := NewMemoryStore()

	if err := txStore.Put(terminalPayment("snapshot-ref")); err != nil {
		t.Fatal(err)
	}
	if err := ledger.Append(LedgerTransaction{
		ID:"snapshot-ledger", ReferenceID:"snapshot-ref", SourceType:"PROVIDER_SETTLEMENT", SourceID:"snapshot-source",
		Currency:"IDR", Description:"snapshot", CreatedAt:time.Date(2026,10,3,15,0,0,0,time.UTC),
		Entries:settlementEntries(),
	}); err != nil {
		t.Fatal(err)
	}
	if err := ledger.AppendSettlement(ctx,
		LedgerTransaction{
			ID:"snapshot-ledger-audit", ReferenceID:"snapshot-audit-ref", SourceType:"PROVIDER_SETTLEMENT", SourceID:"snapshot-audit-source",
			Currency:"IDR", Description:"snapshot audit", CreatedAt:time.Date(2026,10,3,15,1,0,0,time.UTC),
			Entries:settlementEntries(),
		},
		SettlementAudit{
			EventID:"snapshot-audit", TransactionID:"snapshot-ledger-audit", ReferenceID:"snapshot-audit-ref",
			SourceType:"PROVIDER_SETTLEMENT", SourceID:"snapshot-audit-source", Status:ProviderStatusSuccess,
			CreatedAt:time.Date(2026,10,3,15,1,0,0,time.UTC),
		},
	); err != nil {
		t.Fatal(err)
	}

	reconciler, err := NewSettlementReconciler(txStore, ledger, ledger)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := reconciler.readSnapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.states) != 1 || len(snapshot.ledger) != 2 || len(snapshot.audits) != 1 {
		t.Fatalf("unexpected snapshot sizes: states=%d ledger=%d audits=%d", len(snapshot.states), len(snapshot.ledger), len(snapshot.audits))
	}

	// Mutate the stores after materialization; the snapshot must remain stable.
	snapshot.states[0].Payment.Status = payment.StatusFailed
	snapshot.ledger[0].ReferenceID = "mutated-reference"
	snapshot.audits[0].ReferenceID = "mutated-audit"

	report, err := reconciler.Reconcile(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Items) == 0 {
		t.Fatal("expected reconciliation items")
	}
	if report.Items[0].ReferenceID == "mutated-reference" || report.Items[0].ReferenceID == "mutated-audit" {
		t.Fatal("reconciliation must not consume a previously materialized mutable snapshot")
	}
}

func TestReconciliationItemKeyIsIndependentOfCandidateInputOrder(t *testing.T) {
	a := TransactionReconciliation{
		ReferenceID:"ref", ProviderStatus:ProviderStatusSuccess, Status:ReconciliationDuplicateReference,
		LedgerTransactionIDs:[]string{"ledger-b","ledger-a"}, SettlementAuditEventIDs:[]string{"event-b","event-a"},
	}
	b := TransactionReconciliation{
		ReferenceID:"ref", ProviderStatus:ProviderStatusSuccess, Status:ReconciliationDuplicateReference,
		LedgerTransactionIDs:[]string{"ledger-a","ledger-b"}, SettlementAuditEventIDs:[]string{"event-a","event-b"},
	}
	if reconciliationItemKey(a) != reconciliationItemKey(b) { t.Fatal("expected canonical item keys to match") }
}
