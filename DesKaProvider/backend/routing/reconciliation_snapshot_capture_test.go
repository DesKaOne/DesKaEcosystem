package routing

import (
	"context"
	"errors"
	"testing"

	provider "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider"
)

func TestMemoryTransactionSnapshotVerificationDetectsMutation(t *testing.T) {
	store := NewMemoryTransactionStore()
	state := TransactionState{
		Kind: TransactionKindPPOB,
		Request: PurchaseRequest{ReferenceID: "snapshot-ref"},
		Execution: PurchaseExecution{
			ProviderName: "mock",
			Result: provider.PurchaseResult{ReferenceID: "snapshot-ref", Status: provider.StatusPending},
		},
		Version: 1,
	}
	if err := store.Put(state); err != nil {
		t.Fatal(err)
	}

	states, err := store.AllContextE(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	token, err := transactionReconciliationSnapshotFingerprint(states)
	if err != nil {
		t.Fatal(err)
	}

	next := state
	next.Version = 2
	next.Execution.Result.Status = provider.StatusSuccess
	if err := store.Put(next); err != nil {
		t.Fatal(err)
	}

	states, err = store.AllContextE(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := verifyTransactionReconciliationSnapshot(token, states); !errors.Is(err, ErrReconciliationSnapshotTokenMismatch) {
		t.Fatalf("expected deterministic snapshot mismatch after mutation, got %v", err)
	}
}
