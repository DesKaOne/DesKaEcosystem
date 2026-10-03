package accounting

import (
    "context"
    "database/sql"
    "errors"
    "testing"
    "time"

    "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/routing"
)

type legacyReconciliationAuditFailure struct {
    err error
}

func (s legacyReconciliationAuditFailure) GetSettlementAudit(context.Context, string) (SettlementAudit, bool, error) {
    return SettlementAudit{}, false, s.err
}

func TestReadLedgerTransactionsLegacyStoreContextCancellation(t *testing.T) {
    ledgerStore := NewMemoryStore()
    ctx, cancel := context.WithCancel(context.Background())
    cancel()
    _, err := readLedgerTransactions(ctx, ledgerStore)
    if !errors.Is(err, context.Canceled) {
        t.Fatalf("expected context.Canceled, got %v", err)
    }
}

func TestReadLedgerTransactionsLegacyStoreContextDeadline(t *testing.T) {
    ledgerStore := NewMemoryStore()
    ctx, cancel := context.WithDeadline(context.Background(), time.Unix(1, 0).UTC())
    defer cancel()
    _, err := readLedgerTransactions(ctx, ledgerStore)
    if !errors.Is(err, context.DeadlineExceeded) {
        t.Fatalf("expected context.DeadlineExceeded, got %v", err)
    }
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
    defer func() {
        if err := db.Close(); err != nil {
            t.Fatal(err)
        }
    }()

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

func TestSettlementReconcilerPropagatesLegacyPerLedgerAuditContextCancellation(t *testing.T) {
    ledgerStore := NewMemoryStore()
    auditErr := context.Canceled
    reconciler, err := NewSettlementReconciler(
        routing.NewMemoryTransactionStore(),
        ledgerStore,
        legacyReconciliationAuditFailure{err: auditErr},
    )
    if err != nil {
        t.Fatal(err)
    }
    ledger := LedgerTransaction{
        ID: "ledger-legacy-audit-canceled",
        ReferenceID: "ref-legacy-audit-canceled",
        SourceType: "provider_purchase",
        SourceID: "provider-source",
        Currency: "IDR",
        Description: "legacy audit cancellation test",
        CreatedAt: time.Unix(1, 0).UTC(),
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
        t.Fatal("expected legacy audit context cancellation")
    }
    if !errors.Is(err, ErrReconciliationAuditRead) {
        t.Fatalf("expected stable audit-read classification, got %v", err)
    }
    if !errors.Is(err, context.Canceled) {
        t.Fatalf("expected context.Canceled in audit error chain, got %v", err)
    }
    if len(report.Items) != 0 {
        t.Fatalf("canceled legacy audit read must not expose items: %#v", report.Items)
    }
    assertEmptySnapshotMetadata(t, report.Snapshot)
}

func TestSettlementReconcilerPropagatesLegacyPerLedgerAuditContextDeadline(t *testing.T) {
    ledgerStore := NewMemoryStore()
    auditErr := context.DeadlineExceeded
    reconciler, err := NewSettlementReconciler(
        routing.NewMemoryTransactionStore(),
        ledgerStore,
        legacyReconciliationAuditFailure{err: auditErr},
    )
    if err != nil {
        t.Fatal(err)
    }
    ledger := LedgerTransaction{
        ID: "ledger-legacy-audit-deadline",
        ReferenceID: "ref-legacy-audit-deadline",
        SourceType: "provider_purchase",
        SourceID: "provider-source",
        Currency: "IDR",
        Description: "legacy audit deadline test",
        CreatedAt: time.Unix(1, 0).UTC(),
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
        t.Fatal("expected legacy audit context deadline")
    }
    if !errors.Is(err, ErrReconciliationAuditRead) {
        t.Fatalf("expected stable audit-read classification, got %v", err)
    }
    if !errors.Is(err, context.DeadlineExceeded) {
        t.Fatalf("expected context.DeadlineExceeded in audit error chain, got %v", err)
    }
    if len(report.Items) != 0 {
        t.Fatalf("deadline legacy audit read must not expose items: %#v", report.Items)
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
        CreatedAt: time.Unix(1, 0).UTC(),
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
        CreatedAt: time.Unix(1, 0).UTC(),
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


func TestMemoryStoreGetContextPropagatesCancellation(t *testing.T) {
    store := NewMemoryStore()
    ctx, cancel := context.WithCancel(context.Background())
    cancel()

    _, ok, err := store.GetContext(ctx, "missing")
    if !errors.Is(err, context.Canceled) {
        t.Fatalf("expected context.Canceled, got %v", err)
    }
    if ok {
        t.Fatal("canceled point read must not report a result")
    }
}

func TestMemoryStoreGetContextPropagatesDeadline(t *testing.T) {
    store := NewMemoryStore()
    ctx, cancel := context.WithDeadline(context.Background(), time.Unix(1, 0).UTC())
    defer cancel()

    _, ok, err := store.GetContext(ctx, "missing")
    if !errors.Is(err, context.DeadlineExceeded) {
        t.Fatalf("expected context.DeadlineExceeded, got %v", err)
    }
    if ok {
        t.Fatal("expired point read must not report a result")
    }
}

func TestMemoryStoreAppendContextPropagatesCancellationWithoutMutation(t *testing.T) {
    store := NewMemoryStore()
    ctx, cancel := context.WithCancel(context.Background())
    cancel()

    ledger := LedgerTransaction{
        ID: "ledger-context-canceled",
        ReferenceID: "ref-context-canceled",
        SourceType: "provider_purchase",
        SourceID: "provider-source",
        Currency: "IDR",
        Description: "context cancellation write test",
        CreatedAt: time.Unix(1, 0).UTC(),
        Entries: []Entry{
            {LineID: 1, AccountID: "expense", Direction: Debit, Amount: 1000, Currency: "IDR"},
            {LineID: 2, AccountID: "cash", Direction: Credit, Amount: 1000, Currency: "IDR"},
        },
    }

    if err := store.AppendContext(ctx, ledger); !errors.Is(err, context.Canceled) {
        t.Fatalf("expected context.Canceled, got %v", err)
    }
    if _, ok := store.Get(ledger.ID); ok {
        t.Fatal("canceled append must not mutate memory ledger")
    }
}
