package accounting

import (
    "context"
    "database/sql"
    "errors"
    "testing"

    "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/routing"
)

type legacyReconciliationAuditFailure struct {
    err error
}

func (s legacyReconciliationAuditFailure) GetSettlementAudit(context.Context, string) (SettlementAudit, bool, error) {
    return SettlementAudit{}, false, s.err
}

func TestSettlementReconcilerClassifiesPostgreSQLProviderTransactionReadFailure(t *testing.T) {
    db, err := sql.Open("pgx", "postgres://invalid")
    if err != nil {
        t.Fatal(err)
    }
    if err := db.Close(); err != nil {
        t.Fatal(err)
    }

    providerStore, err := routing.NewPostgresTransactionStore(db)
    if err != nil {
        t.Fatal(err)
    }

    reconciler, err := NewSettlementReconciler(
        providerStore,
        NewMemoryStore(),
        NewMemoryStore(),
    )
    if err != nil {
        t.Fatal(err)
    }

    report, err := reconciler.Reconcile(context.Background())
    if err == nil {
        t.Fatal("expected PostgreSQL provider transaction read failure")
    }
    if !errors.Is(err, ErrReconciliationProviderRead) {
        t.Fatalf("expected stable provider-read classification, got %v", err)
    }
    if len(report.Items) != 0 {
        t.Fatalf("provider PostgreSQL read failure must not expose items: %#v", report.Items)
    }
    assertEmptySnapshotMetadata(t, report.Snapshot)
}

func TestSettlementReconcilerPreservesPostgreSQLProviderContextCancellation(t *testing.T) {
    db, err := sql.Open("pgx", "postgres://invalid")
    if err != nil {
        t.Fatal(err)
    }
    if err := db.Close(); err != nil {
        t.Fatal(err)
    }

    providerStore, err := routing.NewPostgresTransactionStore(db)
    if err != nil {
        t.Fatal(err)
    }

    reconciler, err := NewSettlementReconciler(
        providerStore,
        NewMemoryStore(),
        NewMemoryStore(),
    )
    if err != nil {
        t.Fatal(err)
    }

    ctx, cancel := context.WithCancel(context.Background())
    cancel()

    report, err := reconciler.Reconcile(ctx)
    if err == nil {
        t.Fatal("expected canceled PostgreSQL provider read")
    }
    if !errors.Is(err, ErrReconciliationProviderRead) {
        t.Fatalf("expected stable provider-read classification, got %v", err)
    }
    if !errors.Is(err, context.Canceled) {
        t.Fatalf("expected context.Canceled in provider read error chain, got %v", err)
    }
    if len(report.Items) != 0 {
        t.Fatalf("canceled provider read must not expose items: %#v", report.Items)
    }
    assertEmptySnapshotMetadata(t, report.Snapshot)
}

func TestSettlementReconcilerClassifiesLegacyPerLedgerAuditReadFailure(t *testing.T) {
    auditErr := errors.New("legacy audit store unavailable")
    ledgerStore := NewMemoryStore()
    reconciler, err := NewSettlementReconciler(
        routing.NewMemoryTransactionStore(),
        ledgerStore,
        legacyReconciliationAuditFailure{err: auditErr},
    )
    if err != nil {
        t.Fatal(err)
    }

    ledger := LedgerTransaction{
        ID: "ledger-legacy-audit",
        ReferenceID: "ref-legacy-audit",
        SourceType: "provider_purchase",
        SourceID: "provider-source",
        Currency: "IDR",
        Description: "legacy audit failure test",
        CreatedAt: testLedgerCreatedAt(),
        Entries: []Entry{
            {LineID: 1, AccountID: "expense", Direction: Debit, Amount: 1000, Currency: "IDR"},
            {LineID: 2, AccountID: "cash", Direction: Credit, Amount: 1000, Currency: "IDR"},
        },
    }
    if err := ledgerStore.Append(ledger); err != nil {
        t.Fatal(err)
    }

    report, err := reconciler.Reconcile(context.Background())
    if err == nil {
        t.Fatal("expected legacy audit reader failure")
    }
    if !errors.Is(err, ErrReconciliationAuditRead) {
        t.Fatalf("expected stable audit-read classification, got %v", err)
    }
    if !errors.Is(err, auditErr) {
        t.Fatalf("expected original legacy audit error in chain, got %v", err)
    }
    if len(report.Items) != 0 {
        t.Fatalf("legacy audit read failure must not expose items: %#v", report.Items)
    }
    assertEmptySnapshotMetadata(t, report.Snapshot)
}

func TestSettlementReconcilerLegacyAuditReaderDoesNotFallbackToEmptyDataset(t *testing.T) {
    auditErr := errors.New("legacy audit query canceled")
    ledgerStore := NewMemoryStore()
    reconciler, err := NewSettlementReconciler(
        routing.NewMemoryTransactionStore(),
        ledgerStore,
        legacyReconciliationAuditFailure{err: auditErr},
    )
    if err != nil {
        t.Fatal(err)
    }

    ledger := LedgerTransaction{
        ID: "ledger-no-fallback",
        ReferenceID: "ref-no-fallback",
        SourceType: "provider_purchase",
        SourceID: "provider-source",
        Currency: "IDR",
        Description: "legacy fallback guard",
        CreatedAt: testLedgerCreatedAt(),
        Entries: []Entry{
            {LineID: 1, AccountID: "expense", Direction: Debit, Amount: 1000, Currency: "IDR"},
            {LineID: 2, AccountID: "cash", Direction: Credit, Amount: 1000, Currency: "IDR"},
        },
    }
    if err := ledgerStore.Append(ledger); err != nil {
        t.Fatal(err)
    }

    report, err := reconciler.Reconcile(context.Background())
    if err == nil {
        t.Fatal("expected legacy audit reader failure")
    }
    if !errors.Is(err, auditErr) {
        t.Fatalf("expected original legacy audit error, got %v", err)
    }
    if len(report.Items) != 0 {
        t.Fatalf("legacy audit failure must not degrade to partial/empty reconciliation: %#v", report.Items)
    }
    assertEmptySnapshotMetadata(t, report.Snapshot)
}
