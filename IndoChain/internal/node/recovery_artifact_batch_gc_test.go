package node

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/consensus"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/block"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/storage"
)

func TestReconcileCandidateRecoveryArtifactsEnumeratesDeterministically(t *testing.T) {
	n, _, candidate, _, _, _, _ := finalizedBlockFixture(t, storage.NewMemoryStore())
	store := storage.NewMemoryCandidateStore()

	pendingKey, err := n.PersistCandidateForFinality(store, candidate)
	if err != nil {
		t.Fatal(err)
	}

	stale := candidate
	stale.Header.Height = 0
	staleKey, err := CandidateRecoveryArtifactKey(stale)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveCandidate(staleKey, stale); err != nil {
		t.Fatal(err)
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

	reports, err := n.ReconcileCandidateRecoveryArtifacts(store)
	if err != nil {
		t.Fatal(err)
	}
	if len(reports) != 3 {
		t.Fatalf("reports = %d, want 3", len(reports))
	}
	for i := 1; i < len(reports); i++ {
		prev, cur := reports[i-1].Key, reports[i].Key
		if prev.Height > cur.Height ||
			(prev.Height == cur.Height && string(prev.Hash[:]) > string(cur.Hash[:])) {
			t.Fatalf("reports are not deterministically ordered: %+v", reports)
		}
	}
	if _, err := store.GetCandidate(pendingKey); err != nil {
		t.Fatalf("pending candidate removed: %v", err)
	}
	if _, err := store.GetCandidate(staleKey); !errors.Is(err, storage.ErrCandidateNotFound) {
		t.Fatalf("stale candidate retained: %v", err)
	}
	if _, err := store.GetCandidate(futureKey); err != nil {
		t.Fatalf("future candidate removed: %v", err)
	}
}

func TestReconcileCandidateRecoveryArtifactsFileStoreSurvivesCleanup(t *testing.T) {
	n, _, candidate, _, _, _, _ := finalizedBlockFixture(t, storage.NewMemoryStore())
	path := filepath.Join(t.TempDir(), "candidates.gob")
	store, err := storage.NewFileCandidateStore(path)
	if err != nil {
		t.Fatal(err)
	}

	stale := candidate
	stale.Header.Height = 0
	key, err := CandidateRecoveryArtifactKey(stale)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveCandidate(key, stale); err != nil {
		t.Fatal(err)
	}
	reports, err := n.ReconcileCandidateRecoveryArtifacts(store)
	if err != nil {
		t.Fatal(err)
	}
	if len(reports) != 1 || reports[0].Status != RecoveryArtifactStale || !reports[0].Deleted {
		t.Fatalf("reports = %+v", reports)
	}
	reopened, err := storage.NewFileCandidateStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := reopened.GetCandidate(key); !errors.Is(err, storage.ErrCandidateNotFound) {
		t.Fatalf("stale candidate survived reopen: %v", err)
	}
}

func TestApplyCoordinatedArtifactGCKeepsUnlistedEvidence(t *testing.T) {
	n := testNodeWithCanonicalHeight(t, 10)
	candidate := block.Block{Header: block.Header{Height: 7, Timestamp: 1}}
	hash, err := block.Hash(candidate)
	if err != nil {
		t.Fatal(err)
	}
	candidates := storage.NewMemoryCandidateStore()
	key := storage.CandidateKey{Height: 7, Hash: hash}
	if err := candidates.SaveCandidate(key, candidate); err != nil {
		t.Fatal(err)
	}
	evidenceStore := storage.NewMemoryConsensusEvidenceStore()
	if err := evidenceStore.PutConsensusEvidence("keep-me", []byte("needed")); err != nil {
		t.Fatal(err)
	}

	plan := consensus.ArtifactGCPlan{
		ProtocolVersion: n.Config.ProtocolVersion,
		ChainID: n.Config.ChainID,
		CurrentEpoch: 5,
		CanonicalHeight: 10,
		CandidateHeight: 7,
		CandidateHash: hash,
		Policy: consensus.ArtifactRetentionPolicy{KeepRecentHeights: 3},
	}
	decision, validators, power, authority := testCoordinatedGCDecision(t, plan)
	if _, err := n.ApplyCoordinatedArtifactGC(
		decision, validators, power, authority, candidates, evidenceStore, nil,
	); err != nil {
		t.Fatal(err)
	}
	records, err := evidenceStore.LoadConsensusEvidence()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := records["keep-me"]; !ok {
		t.Fatal("unlisted evidence was deleted by coordinated GC")
	}
}
