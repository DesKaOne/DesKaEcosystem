package node

import (
	"errors"
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/storage"
)

func TestCandidateRecoveryArtifactReconciliationClassifiesPendingAndRetains(t *testing.T) {
	n, _, candidate, _, _, _, _ := finalizedBlockFixture(t, storage.NewMemoryStore())
	store := storage.NewMemoryCandidateStore()
	key, err := n.PersistCandidateForFinality(store, candidate)
	if err != nil {
		t.Fatal(err)
	}

	report, err := n.ReconcileCandidateRecoveryArtifact(store, key)
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != RecoveryArtifactPending || report.Deleted {
		t.Fatalf("report = %+v, want retained pending artifact", report)
	}
	if _, err := store.GetCandidate(key); err != nil {
		t.Fatalf("pending candidate was unexpectedly removed: %v", err)
	}
}

func TestCandidateRecoveryArtifactReconciliationDeletesCommitted(t *testing.T) {
	n, ctx, candidate, certificate, validatorResolver, senderResolver, _ := finalizedBlockFixture(t, storage.NewMemoryStore())
	store := storage.NewMemoryCandidateStore()
	key, err := n.PersistCandidateForFinality(store, candidate)
	if err != nil {
		t.Fatal(err)
	}

	validators := mustValidatorSet(t, certificate)
	power := mustVotingPowerSet(t, certificate)
	if err := n.CommitFinalityEvidence(ctx, candidate, certificate, validators, power, validatorResolver, senderResolver); err != nil {
		t.Fatal(err)
	}

	report, err := n.ReconcileCandidateRecoveryArtifact(store, key)
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != RecoveryArtifactCommitted || !report.Deleted {
		t.Fatalf("report = %+v, want committed artifact deleted", report)
	}
	if _, err := store.GetCandidate(key); !errors.Is(err, storage.ErrCandidateNotFound) {
		t.Fatalf("candidate after committed cleanup = %v, want ErrCandidateNotFound", err)
	}

	report, err = n.ReconcileCandidateRecoveryArtifact(store, key)
	if !errors.Is(err, storage.ErrCandidateNotFound) {
		t.Fatalf("second reconciliation error = %v, want ErrCandidateNotFound for absent artifact", err)
	}
	_ = report
}

func TestCandidateRecoveryArtifactReconciliationDeletesStaleAndRetainsContextMismatch(t *testing.T) {
	n, _, candidate, _, _, _, _ := finalizedBlockFixture(t, storage.NewMemoryStore())
	store := storage.NewMemoryCandidateStore()

	stale := candidate
	stale.Header.Height = 0
	stale.Header.PreviousHash = candidate.Header.PreviousHash
	staleKey, err := CandidateRecoveryArtifactKey(stale)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveCandidate(staleKey, stale); err != nil {
		t.Fatal(err)
	}
	report, err := n.ReconcileCandidateRecoveryArtifact(store, staleKey)
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != RecoveryArtifactStale || !report.Deleted {
		t.Fatalf("stale report = %+v, want deleted stale artifact", report)
	}

	future := candidate
	future.Header.Height = n.Head.Header.Height + 2
	futureKey, err := CandidateRecoveryArtifactKey(future)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveCandidate(futureKey, future); err != nil {
		t.Fatal(err)
	}
	report, err = n.ReconcileCandidateRecoveryArtifact(store, futureKey)
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != RecoveryArtifactContextMismatch || report.Deleted {
		t.Fatalf("future report = %+v, want retained context mismatch", report)
	}
	if _, err := store.GetCandidate(futureKey); err != nil {
		t.Fatalf("context-mismatched artifact was unexpectedly removed: %v", err)
	}
}
