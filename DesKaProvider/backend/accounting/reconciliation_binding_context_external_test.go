package accounting_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/DesKaProvider/backend/accounting"
	"github.com/DesKaOne/DesKaEcosystem/DesKaProvider/routing"
)

func TestSnapshotBindingCapabilityDoesNotSurviveSerialization(t *testing.T) {
	store := accounting.NewMemoryStore()
	reconciler, err := accounting.NewSettlementReconciler(
		routing.NewMemoryTransactionStore(),
		store,
		store,
	)
	if err != nil {
		t.Fatal(err)
	}

	report, err := reconciler.Reconcile(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	contextValue := report.SnapshotBindingContext()

	original := (accounting.ReconciliationPersistenceEvidence{
		Resolution: accounting.ReconciliationPersistenceConfirmedApplied,
		Outcome:    "applied",
		Observed:   true,
	}).BindToSnapshot(contextValue)
	if original.ObservationScope != accounting.ReconciliationPersistenceEvidenceScopeSnapshotBound {
		t.Fatalf("real reconciliation context did not bind evidence: %+v", original)
	}
	if original.SnapshotFingerprint == "" {
		t.Fatal("real reconciliation context produced an empty snapshot fingerprint")
	}

	encoded, err := json.Marshal(contextValue)
	if err != nil {
		t.Fatal(err)
	}

	var reconstructed accounting.ReconciliationSnapshotBindingContext
	if err := json.Unmarshal(encoded, &reconstructed); err != nil {
		t.Fatal(err)
	}

	replayed := (accounting.ReconciliationPersistenceEvidence{
		Resolution: accounting.ReconciliationPersistenceConfirmedApplied,
		Outcome:    "applied",
		Observed:   true,
	}).BindToSnapshot(reconstructed)

	if replayed.ObservationScope != accounting.ReconciliationPersistenceEvidenceScopeUnspecified ||
		replayed.SnapshotFingerprint != "" {
		t.Fatalf("serialized binding context recreated snapshot authority: encoded=%s evidence=%+v", encoded, replayed)
	}
}
