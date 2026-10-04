package accounting

import (
	"errors"
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


func TestSettlementReconcilerReportsOrphanLedgerAuditIdentityConflict(t *testing.T) {
	ctx := context.Background()
	txStore := routing.NewMemoryTransactionStore()
	ledger := NewMemoryStore()
	ledgerTx := LedgerTransaction{
		ID: "orphan-identity-ledger", ReferenceID: "orphan-identity-reference",
		SourceType: "PROVIDER_SETTLEMENT", SourceID: "orphan-identity-source",
		Currency: "IDR", Description: "orphan identity", CreatedAt: time.Date(2026, 10, 4, 13, 0, 0, 0, time.UTC),
		Entries: settlementEntries(),
	}
	if err := ledger.Append(ledgerTx); err != nil {
		t.Fatal(err)
	}
	ledger.audits[ledgerTx.ID] = SettlementAudit{
		EventID: "orphan-identity-audit", TransactionID: ledgerTx.ID,
		ReferenceID: "wrong-reference", SourceType: ledgerTx.SourceType,
		SourceID: ledgerTx.SourceID, Status: ProviderStatusSuccess, CreatedAt: ledgerTx.CreatedAt,
	}

	reconciler, err := NewSettlementReconciler(txStore, ledger, ledger)
	if err != nil {
		t.Fatal(err)
	}
	report, err := reconciler.Reconcile(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Items) != 1 {
		t.Fatalf("got %d items: %#v", len(report.Items), report.Items)
	}
	item := report.Items[0]
	if item.Status != ReconciliationCorrelationConflict {
		t.Fatalf("got %s", item.Status)
	}
	if item.LedgerTransactionID != ledgerTx.ID || item.SettlementAuditEventID != "orphan-identity-audit" {
		t.Fatalf("expected conflicting ledger/audit identity to remain visible: %#v", item)
	}
	if len(ledger.All()) != 1 {
		t.Fatal("reconciliation must remain read-only")
	}
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

func TestSettlementReconcilerSnapshotMetadataReportsCaptureProvenance(t *testing.T) {
	ctx := context.Background()
	txStore := routing.NewMemoryTransactionStore()
	ledger := NewMemoryStore()
	if err := txStore.Put(terminalPayment("metadata-ref")); err != nil {
		t.Fatal(err)
	}
	reconciler, err := NewSettlementReconciler(txStore, ledger, ledger)
	if err != nil {
		t.Fatal(err)
	}
	before := time.Now().UTC()
	report, err := reconciler.Reconcile(ctx)
	if err != nil {
		t.Fatal(err)
	}
	after := time.Now().UTC()
	if report.Snapshot.CaptureStartedAt.Before(before) || report.Snapshot.CaptureStartedAt.After(after) {
		t.Fatalf("unexpected capture start: %s", report.Snapshot.CaptureStartedAt)
	}
	if report.Snapshot.CaptureCompletedAt.Before(report.Snapshot.CaptureStartedAt) || report.Snapshot.CaptureCompletedAt.After(after) {
		t.Fatalf("unexpected capture completion: start=%s completed=%s", report.Snapshot.CaptureStartedAt, report.Snapshot.CaptureCompletedAt)
	}
	if !report.Snapshot.CapturedAt.Equal(report.Snapshot.CaptureCompletedAt) {
		t.Fatalf("CapturedAt must represent completed capture: captured=%s completed=%s", report.Snapshot.CapturedAt, report.Snapshot.CaptureCompletedAt)
	}
	if report.Snapshot.ProviderTransactionCount != 1 || report.Snapshot.LedgerTransactionCount != 0 || report.Snapshot.SettlementAuditCount != 0 {
		t.Fatalf("unexpected snapshot counts: %#v", report.Snapshot)
	}
	if report.Snapshot.ProviderReader != "context-all" {
		t.Fatalf("unexpected provider reader: %q", report.Snapshot.ProviderReader)
	}
	if report.Snapshot.LedgerReader != "context-ledger" {
		t.Fatalf("unexpected ledger reader: %q", report.Snapshot.LedgerReader)
	}
	if report.Snapshot.SettlementAuditReader != "context-bulk" {
		t.Fatalf("unexpected audit reader: %q", report.Snapshot.SettlementAuditReader)
	}
}

func TestReconciliationSnapshotFingerprintIsDeterministic(t *testing.T) {
	statesA := []routing.TransactionState{terminalPayment("fingerprint-b"), terminalPayment("fingerprint-a")}
	statesB := []routing.TransactionState{statesA[1], statesA[0]}
	ledgerA := []LedgerTransaction{
		{ID:"ledger-b", ReferenceID:"fingerprint-b", SourceType:"PROVIDER_SETTLEMENT", SourceID:"b", Currency:"IDR", Description:"b", CreatedAt:time.Date(2026,10,3,16,0,0,0,time.UTC), Entries:settlementEntries()},
		{ID:"ledger-a", ReferenceID:"fingerprint-a", SourceType:"PROVIDER_SETTLEMENT", SourceID:"a", Currency:"IDR", Description:"a", CreatedAt:time.Date(2026,10,3,16,1,0,0,time.UTC), Entries:settlementEntries()},
	}
	ledgerB := []LedgerTransaction{ledgerA[1], ledgerA[0]}
	auditsA := []SettlementAudit{
		{EventID:"event-b", TransactionID:"ledger-b", ReferenceID:"fingerprint-b", SourceType:"PROVIDER_SETTLEMENT", SourceID:"b", Status:ProviderStatusSuccess, CreatedAt:time.Date(2026,10,3,16,0,0,0,time.UTC)},
		{EventID:"event-a", TransactionID:"ledger-a", ReferenceID:"fingerprint-a", SourceType:"PROVIDER_SETTLEMENT", SourceID:"a", Status:ProviderStatusSuccess, CreatedAt:time.Date(2026,10,3,16,1,0,0,time.UTC)},
	}
	auditsB := []SettlementAudit{auditsA[1], auditsA[0]}

	first, err := reconciliationSnapshotFingerprint(statesA, ledgerA, auditsA)
	if err != nil { t.Fatal(err) }
	second, err := reconciliationSnapshotFingerprint(statesB, ledgerB, auditsB)
	if err != nil { t.Fatal(err) }
	if first != second { t.Fatalf("fingerprint depends on input order: %s != %s", first, second) }

	changed := append([]routing.TransactionState(nil), statesA...)
	changed[0].Payment.Amount++
	third, err := reconciliationSnapshotFingerprint(changed, ledgerA, auditsA)
	if err != nil { t.Fatal(err) }
	if first == third { t.Fatal("fingerprint must change when snapshot content changes") }
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


type failingProviderSnapshotStore struct {
	*routing.MemoryTransactionStore
	err error
}

func (s failingProviderSnapshotStore) AllContextE(context.Context) ([]routing.TransactionState, error) {
	return nil, s.err
}

type failingLedgerSnapshotReader struct {
	*MemoryStore
	err error
}

func (s failingLedgerSnapshotReader) AllContext(context.Context) ([]LedgerTransaction, error) {
	return nil, s.err
}

type failingAuditSnapshotReader struct {
	*MemoryStore
	err error
}

func (s failingAuditSnapshotReader) AllSettlementAudits(context.Context) ([]SettlementAudit, error) {
	return nil, s.err
}

func assertEmptySnapshotMetadata(t *testing.T, metadata ReconciliationSnapshotMetadata) {
	t.Helper()
	if !metadata.CaptureStartedAt.IsZero() || !metadata.CaptureCompletedAt.IsZero() || !metadata.CapturedAt.IsZero() {
		t.Fatalf("failed capture must not expose partial lifecycle metadata: %#v", metadata)
	}
	if metadata.ProviderTransactionCount != 0 || metadata.LedgerTransactionCount != 0 || metadata.SettlementAuditCount != 0 {
		t.Fatalf("failed capture must not expose partial dataset counts: %#v", metadata)
	}
	if metadata.ProviderReader != "" || metadata.LedgerReader != "" || metadata.SettlementAuditReader != "" || metadata.SnapshotFingerprint != "" {
		t.Fatalf("failed capture must not expose partial provenance/fingerprint: %#v", metadata)
	}
}

func TestSettlementReconcilerSnapshotMetadataContractOnCaptureFailures(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name         string
		transactions routing.ContextReadTransactionStore
		ledger       interface{}
		audit        SettlementAuditReader
		classification error
	}{
		{
			name:         "provider-read",
			transactions: failingProviderSnapshotStore{MemoryTransactionStore: routing.NewMemoryTransactionStore(), err: context.Canceled},
			ledger:       NewMemoryStore(),
			audit:        NewMemoryStore(),
			classification: ErrReconciliationProviderRead,
		},
		{
			name:         "ledger-read",
			transactions: routing.NewMemoryTransactionStore(),
			ledger:       failingLedgerSnapshotReader{MemoryStore: NewMemoryStore(), err: context.Canceled},
			audit:        NewMemoryStore(),
			classification: ErrReconciliationLedgerRead,
		},
		{
			name:         "audit-read",
			transactions: routing.NewMemoryTransactionStore(),
			ledger:       NewMemoryStore(),
			audit:        failingAuditSnapshotReader{MemoryStore: NewMemoryStore(), err: context.Canceled},
			classification: ErrReconciliationAuditRead,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reconciler, err := NewSettlementReconciler(tc.transactions, tc.ledger, tc.audit)
			if err != nil {
				t.Fatal(err)
			}
			report, err := reconciler.Reconcile(ctx)
			if err == nil {
				t.Fatal("expected snapshot capture failure")
			}
			if !errors.Is(err, tc.classification) {
				t.Fatalf("expected stable snapshot classification %v, got %v", tc.classification, err)
			}
			if len(report.Items) != 0 {
				t.Fatalf("failed capture must not return reconciliation items: %#v", report.Items)
			}
			assertEmptySnapshotMetadata(t, report.Snapshot)
		})
	}
}

func TestSettlementReconcilerFailureClassificationPreservesUnderlyingCause(t *testing.T) {
	ctx := context.Background()
	storageErr := errors.New("database connection unavailable")

	cases := []struct {
		name           string
		transactions   routing.ContextReadTransactionStore
		ledger         interface{}
		audit          SettlementAuditReader
		classification error
	}{
		{
			name: "provider-storage-error",
			transactions: failingProviderSnapshotStore{MemoryTransactionStore: routing.NewMemoryTransactionStore(), err: storageErr},
			ledger: NewMemoryStore(),
			audit: NewMemoryStore(),
			classification: ErrReconciliationProviderRead,
		},
		{
			name: "ledger-storage-error",
			transactions: routing.NewMemoryTransactionStore(),
			ledger: failingLedgerSnapshotReader{MemoryStore: NewMemoryStore(), err: storageErr},
			audit: NewMemoryStore(),
			classification: ErrReconciliationLedgerRead,
		},
		{
			name: "legacy-audit-storage-error",
			transactions: routing.NewMemoryTransactionStore(),
			ledger: NewMemoryStore(),
			audit: failingAuditSnapshotReader{MemoryStore: NewMemoryStore(), err: storageErr},
			classification: ErrReconciliationAuditRead,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reconciler, err := NewSettlementReconciler(tc.transactions, tc.ledger, tc.audit)
			if err != nil {
				t.Fatal(err)
			}
			report, err := reconciler.Reconcile(ctx)
			if err == nil {
				t.Fatal("expected snapshot capture failure")
			}
			if !errors.Is(err, tc.classification) {
				t.Fatalf("expected stable classification %v, got %v", tc.classification, err)
			}
			if !errors.Is(err, storageErr) {
				t.Fatalf("expected underlying storage error to remain inspectable, got %v", err)
			}
			if len(report.Items) != 0 {
				t.Fatalf("failed capture must not expose reconciliation items: %#v", report.Items)
			}
			assertEmptySnapshotMetadata(t, report.Snapshot)
		})
	}
}

func TestSettlementReconcilerUnsupportedLedgerReaderFailsClosedWithStableClassification(t *testing.T) {
	ctx := context.Background()
	txStore := routing.NewMemoryTransactionStore()
	reconciler, err := NewSettlementReconciler(txStore, struct{}{}, NewMemoryStore())
	if err != nil {
		t.Fatal(err)
	}

	report, err := reconciler.Reconcile(ctx)
	if err == nil {
		t.Fatal("expected unsupported ledger reader failure")
	}
	if !errors.Is(err, ErrReconciliationLedgerRead) {
		t.Fatalf("expected stable ledger-read classification, got %v", err)
	}
	if len(report.Items) != 0 {
		t.Fatalf("unsupported ledger reader must not expose reconciliation items: %#v", report.Items)
	}
	assertEmptySnapshotMetadata(t, report.Snapshot)
}

func TestSettlementReconcilerSnapshotMetadataFingerprintAndLifecycleContract(t *testing.T) {
	ctx := context.Background()
	txStore := routing.NewMemoryTransactionStore()
	ledger := NewMemoryStore()
	if err := txStore.Put(terminalPayment("metadata-contract")); err != nil {
		t.Fatal(err)
	}
	reconciler, err := NewSettlementReconciler(txStore, ledger, ledger)
	if err != nil {
		t.Fatal(err)
	}

	first, err := reconciler.Reconcile(ctx)
	if err != nil {
		t.Fatal(err)
	}
	second, err := reconciler.Reconcile(ctx)
	if err != nil {
		t.Fatal(err)
	}

	for name, report := range map[string]ReconciliationReport{"first": first, "second": second} {
		metadata := report.Snapshot
		if metadata.CaptureStartedAt.IsZero() || metadata.CaptureCompletedAt.IsZero() || metadata.CapturedAt.IsZero() {
			t.Fatalf("%s capture lifecycle metadata must be populated: %#v", name, metadata)
		}
		if metadata.CaptureCompletedAt.Before(metadata.CaptureStartedAt) {
			t.Fatalf("%s capture completion precedes start: %#v", name, metadata)
		}
		if !metadata.CapturedAt.Equal(metadata.CaptureCompletedAt) {
			t.Fatalf("%s CapturedAt must alias CaptureCompletedAt: %#v", name, metadata)
		}
		if metadata.ProviderTransactionCount != 1 || metadata.LedgerTransactionCount != 0 || metadata.SettlementAuditCount != 0 {
			t.Fatalf("%s unexpected counts: %#v", name, metadata)
		}
		if metadata.ProviderReader != "context-all" || metadata.LedgerReader != "context-ledger" || metadata.SettlementAuditReader != "context-bulk" {
			t.Fatalf("%s unexpected reader provenance: %#v", name, metadata)
		}
		if metadata.SnapshotFingerprint == "" {
			t.Fatalf("%s snapshot fingerprint must be populated", name)
		}
	}
	if first.Snapshot.SnapshotFingerprint != second.Snapshot.SnapshotFingerprint {
		t.Fatal("equivalent captured datasets must retain the same fingerprint across repeated reconciliation runs")
	}
	if first.Snapshot.CaptureStartedAt.Equal(second.Snapshot.CaptureStartedAt) && first.Snapshot.CaptureCompletedAt.Equal(second.Snapshot.CaptureCompletedAt) {
		t.Fatal("separate captures must not be represented as the same lifecycle observation")
	}
}


func TestReconciliationSnapshotFingerprintUsesExplicitSchemaVersion(t *testing.T) {
	if reconciliationSnapshotFingerprintVersion != "v1" {
		t.Fatalf("unexpected fingerprint schema version: %q", reconciliationSnapshotFingerprintVersion)
	}
	fingerprint, err := reconciliationSnapshotFingerprint(nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(fingerprint) != 64 {
		t.Fatalf("expected SHA-256 hex fingerprint, got %q", fingerprint)
	}
}

func TestReconciliationSnapshotFingerprintPreservesDatasetDomainSeparation(t *testing.T) {
	state := terminalPayment("domain-separated")
	ledger := LedgerTransaction{
		ID:"domain-ledger", ReferenceID:"domain-separated", SourceType:"PROVIDER_SETTLEMENT", SourceID:"domain-source",
		Currency:"IDR", Description:"domain", CreatedAt:time.Date(2026,10,3,17,0,0,0,time.UTC), Entries:settlementEntries(),
	}
	audit := SettlementAudit{
		EventID:"domain-event", TransactionID:ledger.ID, ReferenceID:ledger.ReferenceID, SourceType:ledger.SourceType,
		SourceID:ledger.SourceID, Status:ProviderStatusSuccess, CreatedAt:ledger.CreatedAt,
	}
	first, err := reconciliationSnapshotFingerprint([]routing.TransactionState{state}, []LedgerTransaction{ledger}, []SettlementAudit{audit})
	if err != nil { t.Fatal(err) }
	second, err := reconciliationSnapshotFingerprint([]routing.TransactionState{state}, []LedgerTransaction{ledger}, nil)
	if err != nil { t.Fatal(err) }
	third, err := reconciliationSnapshotFingerprint(nil, []LedgerTransaction{ledger}, []SettlementAudit{audit})
	if err != nil { t.Fatal(err) }
	if first == second || first == third || second == third {
		t.Fatal("fingerprint must preserve dataset-domain boundaries")
	}
}


func TestSettlementReconcilerFailsClosedWhenSnapshotFingerprintFails(t *testing.T) {
	ctx := context.Background()
	txStore := routing.NewMemoryTransactionStore()
	ledger := NewMemoryStore()
	if err := txStore.Put(terminalPayment("fingerprint-failure")); err != nil {
		t.Fatal(err)
	}
	fingerprintErr := errors.New("fingerprint unavailable")
	reconciler, err := NewSettlementReconciler(txStore, ledger, ledger)
	if err != nil {
		t.Fatal(err)
	}
	reconciler.fingerprint = func([]routing.TransactionState, []LedgerTransaction, []SettlementAudit) (string, error) {
		return "", fingerprintErr
	}

	report, err := reconciler.Reconcile(ctx)
	if !errors.Is(err, fingerprintErr) {
		t.Fatalf("expected fingerprint error to propagate, got %v", err)
	}
	if !errors.Is(err, ErrReconciliationFingerprint) {
		t.Fatalf("expected stable fingerprint classification, got %v", err)
	}
	if len(report.Items) != 0 {
		t.Fatalf("failed fingerprint must not expose reconciliation items: %#v", report.Items)
	}
	assertEmptySnapshotMetadata(t, report.Snapshot)
}

func TestSettlementReconcilerUsesDefaultFingerprintWhenDependencyUnset(t *testing.T) {
	ctx := context.Background()
	txStore := routing.NewMemoryTransactionStore()
	ledger := NewMemoryStore()
	if err := txStore.Put(terminalPayment("fingerprint-default")); err != nil {
		t.Fatal(err)
	}
	reconciler, err := NewSettlementReconciler(txStore, ledger, ledger)
	if err != nil {
		t.Fatal(err)
	}
	reconciler.fingerprint = nil

	report, err := reconciler.Reconcile(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if report.Snapshot.SnapshotFingerprint == "" {
		t.Fatal("default fingerprint implementation must populate snapshot fingerprint")
	}
}


func TestSettlementReconcilerDoesNotCorrelateWhenTransactionHasDuplicateAudits(t *testing.T) {
	ctx := context.Background()
	txStore := routing.NewMemoryTransactionStore()
	ledger := NewMemoryStore()
	state := terminalPayment("duplicate-audit-transaction")
	if err := txStore.Put(state); err != nil { t.Fatal(err) }
	ledgerTx := LedgerTransaction{
		ID: "duplicate-audit-ledger", ReferenceID: "duplicate-audit-transaction",
		SourceType: "PROVIDER_SETTLEMENT", SourceID: "duplicate-audit-source",
		Currency: "IDR", Description: "duplicate audit", CreatedAt: time.Date(2026,10,3,18,0,0,0,time.UTC),
		Entries: settlementEntries(),
	}
	if err := ledger.Append(ledgerTx); err != nil { t.Fatal(err) }

	audits := []SettlementAudit{
		{EventID:"event-a", TransactionID:ledgerTx.ID, ReferenceID:ledgerTx.ReferenceID, SourceType:ledgerTx.SourceType, SourceID:ledgerTx.SourceID, Status:ProviderStatusSuccess, CreatedAt:ledgerTx.CreatedAt},
		{EventID:"event-b", TransactionID:ledgerTx.ID, ReferenceID:ledgerTx.ReferenceID, SourceType:ledgerTx.SourceType, SourceID:ledgerTx.SourceID, Status:ProviderStatusSuccess, CreatedAt:ledgerTx.CreatedAt.Add(time.Minute)},
	}
	reader := duplicateAuditReader{SettlementAuditReader: ledger, allAudits: audits}
	reconciler, err := NewSettlementReconciler(txStore, ledger, reader)
	if err != nil { t.Fatal(err) }
	report, err := reconciler.Reconcile(ctx)
	if err != nil { t.Fatal(err) }

	for _, item := range report.Items {
		if item.Status == ReconciliationCorrelated {
			t.Fatal("duplicate audit identity must never be reported as correlated")
		}
	}
	found := false
	for _, item := range report.Items {
		if item.Status == ReconciliationDuplicateAuditIdentity && item.LedgerTransactionID == ledgerTx.ID {
			found = true
			want := []string{"event-a","event-b"}
			if len(item.SettlementAuditEventIDs) != len(want) {
				t.Fatalf("got audit IDs %#v", item.SettlementAuditEventIDs)
			}
			for i := range want {
				if item.SettlementAuditEventIDs[i] != want[i] {
					t.Fatalf("got audit IDs %#v", item.SettlementAuditEventIDs)
				}
			}
		}
	}
	if !found { t.Fatal("expected duplicate audit identity diagnostic") }
}

func TestSettlementReconcilerDoesNotCorrelateWhenEventIdentityIsDuplicatedAcrossTransactions(t *testing.T) {
	ctx := context.Background()
	txStore := routing.NewMemoryTransactionStore()
	ledger := NewMemoryStore()
	state := terminalPayment("duplicate-event-reference")
	if err := txStore.Put(state); err != nil { t.Fatal(err) }
	ledgerTx := LedgerTransaction{
		ID: "duplicate-event-ledger", ReferenceID: "duplicate-event-reference",
		SourceType: "PROVIDER_SETTLEMENT", SourceID: "duplicate-event-source",
		Currency: "IDR", Description: "duplicate event", CreatedAt: time.Date(2026,10,3,18,0,0,0,time.UTC),
		Entries: settlementEntries(),
	}
	orphanTx := ledgerTx
	orphanTx.ID = "duplicate-event-orphan"
	orphanTx.ReferenceID = "orphan-reference"
	orphanTx.SourceID = "orphan-source"
	if err := ledger.Append(ledgerTx); err != nil { t.Fatal(err) }
	if err := ledger.Append(orphanTx); err != nil { t.Fatal(err) }

	audits := []SettlementAudit{
		{EventID:"shared-event", TransactionID:ledgerTx.ID, ReferenceID:ledgerTx.ReferenceID, SourceType:ledgerTx.SourceType, SourceID:ledgerTx.SourceID, Status:ProviderStatusSuccess, CreatedAt:ledgerTx.CreatedAt},
		{EventID:"shared-event", TransactionID:orphanTx.ID, ReferenceID:orphanTx.ReferenceID, SourceType:orphanTx.SourceType, SourceID:orphanTx.SourceID, Status:ProviderStatusSuccess, CreatedAt:orphanTx.CreatedAt.Add(time.Minute)},
	}
	reader := duplicateAuditReader{SettlementAuditReader: ledger, allAudits: audits}
	reconciler, err := NewSettlementReconciler(txStore, ledger, reader)
	if err != nil { t.Fatal(err) }
	report, err := reconciler.Reconcile(ctx)
	if err != nil { t.Fatal(err) }

	for _, item := range report.Items {
		if item.LedgerTransactionID == ledgerTx.ID && item.Status == ReconciliationCorrelated {
			t.Fatal("globally duplicated audit event identity must not correlate")
		}
	}
	found := false
	for _, item := range report.Items {
		if item.LedgerTransactionID == ledgerTx.ID && item.Status == ReconciliationDuplicateAuditIdentity {
			found = true
		}
	}
	if !found { t.Fatal("expected duplicate audit event diagnostic") }
}


func TestSettlementReconcilerDoesNotCorrelateWhenAuditStatusIsNonSuccess(t *testing.T) {
	ctx := context.Background()
	txStore := routing.NewMemoryTransactionStore()
	ledger := NewMemoryStore()
	state := terminalPayment("audit-status-conflict")
	if err := txStore.Put(state); err != nil { t.Fatal(err) }

	ledgerTx := LedgerTransaction{
		ID: "audit-status-ledger", ReferenceID: "audit-status-conflict",
		SourceType: "PROVIDER_SETTLEMENT", SourceID: "audit-status-source",
		Currency: "IDR", Description: "audit status conflict", CreatedAt: time.Date(2026,10,4,10,0,0,0,time.UTC),
		Entries: settlementEntries(),
	}
	audit := SettlementAudit{
		EventID: "audit-status-event", TransactionID: ledgerTx.ID, ReferenceID: ledgerTx.ReferenceID,
		SourceType: ledgerTx.SourceType, SourceID: ledgerTx.SourceID, Status: string(provider.StatusFailed),
		CreatedAt: ledgerTx.CreatedAt,
	}
	if err := ledger.Append(ledgerTx); err != nil { t.Fatal(err) }

	// Inject the non-success audit through the read-model seam. A real SettlementStore
	// rejects non-success settlement audits at its mutation boundary; reconciliation
	// still must fail closed if a legacy/external reader exposes one.
	auditReader := duplicateAuditReader{SettlementAuditReader: ledger, allAudits: []SettlementAudit{audit}}
	reconciler, err := NewSettlementReconciler(txStore, ledger, auditReader)
	if err != nil { t.Fatal(err) }
	report, err := reconciler.Reconcile(ctx)
	if err != nil { t.Fatal(err) }
	if len(report.Items) != 1 { t.Fatalf("got %d items: %#v", len(report.Items), report.Items) }
	item := report.Items[0]
	if item.Status != ReconciliationCorrelationConflict {
		t.Fatalf("got %s", item.Status)
	}
	if item.LedgerTransactionID != ledgerTx.ID || item.SettlementAuditEventID != audit.EventID {
		t.Fatalf("expected conflicting identities to remain visible: %#v", item)
	}
	if len(ledger.All()) != 1 { t.Fatal("reconciliation must remain read-only") }
}


func TestSettlementReconcilerSurfacesFinancialRecordsForNonSuccessProviderState(t *testing.T) {
	ctx := context.Background()
	txStore := routing.NewMemoryTransactionStore()
	ledger := NewMemoryStore()
	state := terminalPayment("provider-failed-with-ledger")
	state.Payment.Status = payment.StatusFailed
	state.Execution.Result.Status = provider.StatusFailed
	if err := txStore.Put(state); err != nil { t.Fatal(err) }

	ledgerTx := LedgerTransaction{
		ID: "provider-failed-ledger", ReferenceID: "provider-failed-with-ledger",
		SourceType: "PROVIDER_SETTLEMENT", SourceID: "provider-failed-source",
		Currency: "IDR", Description: "unexpected financial record", CreatedAt: time.Date(2026,10,4,9,0,0,0,time.UTC),
		Entries: settlementEntries(),
	}
	audit := SettlementAudit{
		EventID: "provider-failed-audit", TransactionID: ledgerTx.ID, ReferenceID: ledgerTx.ReferenceID,
		SourceType: ledgerTx.SourceType, SourceID: ledgerTx.SourceID, Status: ProviderStatusSuccess,
		CreatedAt: ledgerTx.CreatedAt,
	}
	if err := ledger.AppendSettlement(ctx, ledgerTx, audit); err != nil { t.Fatal(err) }

	reconciler, err := NewSettlementReconciler(txStore, ledger, ledger)
	if err != nil { t.Fatal(err) }
	report, err := reconciler.Reconcile(ctx)
	if err != nil { t.Fatal(err) }

	if len(report.Items) != 1 { t.Fatalf("got %d items: %#v", len(report.Items), report.Items) }
	item := report.Items[0]
	if item.Status != ReconciliationNotSettleable { t.Fatalf("got %s", item.Status) }
	if len(item.LedgerTransactionIDs) != 1 || item.LedgerTransactionIDs[0] != ledgerTx.ID {
		t.Fatalf("expected financial record identity, got %#v", item.LedgerTransactionIDs)
	}
	if len(item.SettlementAuditEventIDs) != 1 || item.SettlementAuditEventIDs[0] != audit.EventID {
		t.Fatalf("expected audit identity, got %#v", item.SettlementAuditEventIDs)
	}
	if len(ledger.All()) != 1 { t.Fatal("reconciliation must remain read-only") }
}

type legacyPerLedgerAuditReader struct {
	audits map[string]SettlementAudit
}

func (s legacyPerLedgerAuditReader) GetSettlementAudit(_ context.Context, transactionID string) (SettlementAudit, bool, error) {
	audit, ok := s.audits[transactionID]
	return audit, ok, nil
}

func TestSettlementReconcilerLabelsLegacyMixedSnapshot(t *testing.T) {
	ledgerStore := NewMemoryStore()
	ledger := validLedgerTransaction()
	ledger.ID = "legacy-mixed-snapshot"
	ledger.ReferenceID = "legacy-mixed-reference"
	ledger.Entries[0].Amount = 1000
	ledger.Entries[1].Amount = 1000
	if err := ledgerStore.Append(ledger); err != nil {
		t.Fatal(err)
	}
	reconciler, err := NewSettlementReconciler(
		routing.NewMemoryTransactionStore(),
		ledgerStore,
		legacyPerLedgerAuditReader{audits: map[string]SettlementAudit{}},
	)
	if err != nil {
		t.Fatal(err)
	}
	report, err := reconciler.Reconcile(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if report.Snapshot.SnapshotConsistency != ReconciliationSnapshotConsistencyLegacyMixed {
		t.Fatalf("expected legacy mixed snapshot classification, got %q", report.Snapshot.SnapshotConsistency)
	}
}

func TestSettlementReconcilerSnapshotMetadataExposesCaptureWindow(t *testing.T) {
	ctx := context.Background()
	txStore := routing.NewMemoryTransactionStore()
	if err := txStore.Put(terminalPayment("snapshot-window")); err != nil {
		t.Fatal(err)
	}
	reconciler, err := NewSettlementReconciler(txStore, NewMemoryStore(), NewMemoryStore())
	if err != nil {
		t.Fatal(err)
	}

	report, err := reconciler.Reconcile(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if report.Snapshot.CaptureCompletedAt.Before(report.Snapshot.CaptureStartedAt) {
		t.Fatalf("capture completion precedes start: %#v", report.Snapshot)
	}
	if report.Snapshot.SnapshotWindowMillis < 0 {
		t.Fatalf("capture window must never be negative: %#v", report.Snapshot)
	}
}
