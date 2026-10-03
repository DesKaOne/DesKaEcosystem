package accounting

import (
    "context"
    "database/sql"
    "errors"
    "testing"
	"time"

    "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/routing"
)

type nonContextReconciliationProviderReader struct {
    *routing.MemoryTransactionStore
}

func (s nonContextReconciliationProviderReader) AllContextE(context.Context) ([]routing.TransactionState, error) {
    return s.MemoryTransactionStore.All(), nil
}

type nonContextReconciliationLedgerReader struct {
    *MemoryStore
}

func (s nonContextReconciliationLedgerReader) AllContext(context.Context) ([]LedgerTransaction, error) {
    return s.MemoryStore.All(), nil
}

type contextAwareReconciliationProviderReader struct {
    *routing.MemoryTransactionStore
}

func (s contextAwareReconciliationProviderReader) AllContextE(ctx context.Context) ([]routing.TransactionState, error) {
    if err := ctx.Err(); err != nil {
        return nil, err
    }
    return s.MemoryTransactionStore.AllContextE(ctx)
}

type contextAwareReconciliationLedgerReader struct {
    *MemoryStore
}

func (s contextAwareReconciliationLedgerReader) AllContext(ctx context.Context) ([]LedgerTransaction, error) {
    if err := ctx.Err(); err != nil {
        return nil, err
    }
    return s.MemoryStore.AllContext(ctx)
}

type contextAwareReconciliationAuditReader struct {
    *MemoryStore
}

func (s contextAwareReconciliationAuditReader) AllSettlementAudits(ctx context.Context) ([]SettlementAudit, error) {
    if err := ctx.Err(); err != nil {
        return nil, err
    }
    return s.MemoryStore.AllSettlementAudits(ctx)
}

func assertReconciliationContextFailure(
    t *testing.T,
    reconciler *SettlementReconciler,
    ctx context.Context,
    expected error,
    classification error,
) {
    t.Helper()
    report, err := reconciler.Reconcile(ctx)
    if err == nil {
        t.Fatal("expected reconciliation context failure")
    }
    if !errors.Is(err, classification) {
        t.Fatalf("expected stable classification %v, got %v", classification, err)
    }
    if !errors.Is(err, expected) {
        t.Fatalf("expected original context error %v, got %v", expected, err)
    }
    if len(report.Items) != 0 {
        t.Fatalf("context failure must not expose reconciliation items: %#v", report.Items)
    }
    assertEmptySnapshotMetadata(t, report.Snapshot)
}

func TestSettlementReconcilerPropagatesContextCancellationAtEachSnapshotReader(t *testing.T) {
    cases := []struct {
        name string
        build func() (*SettlementReconciler, error)
        classification error
    }{
        {
            name: "provider",
            build: func() (*SettlementReconciler, error) {
                return NewSettlementReconciler(
                    contextAwareReconciliationProviderReader{MemoryTransactionStore: routing.NewMemoryTransactionStore()},
                    NewMemoryStore(),
                    NewMemoryStore(),
                )
            },
            classification: ErrReconciliationProviderRead,
        },
        {
            name: "ledger",
            build: func() (*SettlementReconciler, error) {
                return NewSettlementReconciler(
                    nonContextReconciliationProviderReader{MemoryTransactionStore: routing.NewMemoryTransactionStore()},
                    contextAwareReconciliationLedgerReader{MemoryStore: NewMemoryStore()},
                    NewMemoryStore(),
                )
            },
            classification: ErrReconciliationLedgerRead,
        },
        {
            name: "bulk-audit",
            build: func() (*SettlementReconciler, error) {
                return NewSettlementReconciler(
                    nonContextReconciliationProviderReader{MemoryTransactionStore: routing.NewMemoryTransactionStore()},
                    nonContextReconciliationLedgerReader{MemoryStore: NewMemoryStore()},
                    contextAwareReconciliationAuditReader{MemoryStore: NewMemoryStore()},
                )
            },
            classification: ErrReconciliationAuditRead,
        },
    }

    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {
            reconciler, err := tc.build()
            if err != nil {
                t.Fatal(err)
            }
            ctx, cancel := context.WithCancel(context.Background())
            cancel()
            assertReconciliationContextFailure(t, reconciler, ctx, context.Canceled, tc.classification)
        })
    }
}

func TestSettlementReconcilerPropagatesContextDeadlineAtEachSnapshotReader(t *testing.T) {
    cases := []struct {
        name string
        build func() (*SettlementReconciler, error)
        classification error
    }{
        {
            name: "provider",
            build: func() (*SettlementReconciler, error) {
                return NewSettlementReconciler(
                    contextAwareReconciliationProviderReader{MemoryTransactionStore: routing.NewMemoryTransactionStore()},
                    NewMemoryStore(),
                    NewMemoryStore(),
                )
            },
            classification: ErrReconciliationProviderRead,
        },
        {
            name: "ledger",
            build: func() (*SettlementReconciler, error) {
                return NewSettlementReconciler(
                    routing.NewMemoryTransactionStore(),
                    contextAwareReconciliationLedgerReader{MemoryStore: NewMemoryStore()},
                    NewMemoryStore(),
                )
            },
            classification: ErrReconciliationLedgerRead,
        },
        {
            name: "bulk-audit",
            build: func() (*SettlementReconciler, error) {
                return NewSettlementReconciler(
                    routing.NewMemoryTransactionStore(),
                    NewMemoryStore(),
                    contextAwareReconciliationAuditReader{MemoryStore: NewMemoryStore()},
                )
            },
            classification: ErrReconciliationAuditRead,
        },
    }

    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {
            reconciler, err := tc.build()
            if err != nil {
                t.Fatal(err)
            }
            ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
            defer cancel()
            assertReconciliationContextFailure(t, reconciler, ctx, context.DeadlineExceeded, tc.classification)
        })
    }
}

func TestSettlementReconcilerPostgreSQLReaderFailuresRemainClassifiedAndFailClosed(t *testing.T) {
    db, err := sql.Open("pgx", "postgres://invalid")
    if err != nil {
        t.Fatal(err)
    }
    if err := db.Close(); err != nil {
        t.Fatal(err)
    }

    pgLedger, err := NewPostgresStore(db)
    if err != nil {
        t.Fatal(err)
    }

    t.Run("ledger", func(t *testing.T) {
        reconciler, err := NewSettlementReconciler(
            routing.NewMemoryTransactionStore(),
            pgLedger,
            NewMemoryStore(),
        )
        if err != nil {
            t.Fatal(err)
        }
        report, err := reconciler.Reconcile(context.Background())
        if err == nil {
            t.Fatal("expected PostgreSQL ledger read failure")
        }
        if !errors.Is(err, ErrReconciliationLedgerRead) {
            t.Fatalf("expected stable ledger-read classification, got %v", err)
        }
        if len(report.Items) != 0 {
            t.Fatalf("PostgreSQL ledger read failure must not expose items: %#v", report.Items)
        }
        assertEmptySnapshotMetadata(t, report.Snapshot)
    })

    t.Run("audit", func(t *testing.T) {
        reconciler, err := NewSettlementReconciler(
            routing.NewMemoryTransactionStore(),
            NewMemoryStore(),
            pgLedger,
        )
        if err != nil {
            t.Fatal(err)
        }
        report, err := reconciler.Reconcile(context.Background())
        if err == nil {
            t.Fatal("expected PostgreSQL audit read failure")
        }
        if !errors.Is(err, ErrReconciliationAuditRead) {
            t.Fatalf("expected stable audit-read classification, got %v", err)
        }
        if len(report.Items) != 0 {
            t.Fatalf("PostgreSQL audit read failure must not expose items: %#v", report.Items)
        }
        assertEmptySnapshotMetadata(t, report.Snapshot)
    })
}
