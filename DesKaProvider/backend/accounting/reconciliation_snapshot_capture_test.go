package accounting

import (
	"context"
	"errors"
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/DesKaProvider/routing"
)

type mutatingProviderSnapshotReader struct {
	*routing.MemoryTransactionStore
}

func (s mutatingProviderSnapshotReader) CaptureReconciliationSnapshot(ctx context.Context) ([]routing.TransactionState, string, error) {
	states, token, err := s.MemoryTransactionStore.CaptureReconciliationSnapshot(ctx)
	if err != nil {
		return nil, "", err
	}
	mutated := routing.TransactionState{
		Kind: routing.TransactionKindPPOB,
		Request: routing.PurchaseRequest{ReferenceID: "snapshot-mutated"},
		Execution: routing.PurchaseExecution{ProviderName: "mock"},
		Version: 1,
	}
	if err := s.MemoryTransactionStore.Put(mutated); err != nil {
		return nil, "", err
	}
	return states, token, nil
}

func (s mutatingProviderSnapshotReader) VerifyReconciliationSnapshot(ctx context.Context, token string) error {
	return s.MemoryTransactionStore.VerifyReconciliationSnapshot(ctx, token)
}

func TestSettlementReconcilerFailsClosedWhenProviderSnapshotChangesDuringCapture(t *testing.T) {
	providerStore := routing.NewMemoryTransactionStore()
	reader := mutatingProviderSnapshotReader{MemoryTransactionStore: providerStore}

	reconciler, err := NewSettlementReconciler(reader, NewMemoryStore(), NewMemoryStore())
	if err != nil {
		t.Fatal(err)
	}

	report, err := reconciler.Reconcile(context.Background())
	if err == nil {
		t.Fatal("expected snapshot-change failure")
	}
	if !errors.Is(err, ErrReconciliationSnapshotChanged) {
		t.Fatalf("expected snapshot-change classification, got %v", err)
	}
	if len(report.Items) != 0 {
		t.Fatalf("snapshot-change failure must not expose reconciliation items: %#v", report.Items)
	}
}
