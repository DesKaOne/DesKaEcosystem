package node

import (
	"errors"
	"testing"
	"path/filepath"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/block"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/consensus"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/storage"
)

func TestPersistCandidateForFinalityDoesNotAdvanceCanonicalHead(t *testing.T) {
	n, _, candidate, _, _, _, _ := finalizedBlockFixture(t, storage.NewMemoryStore())
	store := storage.NewMemoryCandidateStore()
	before := n.HeadHash
	key, err := n.PersistCandidateForFinality(store, candidate)
	if err != nil { t.Fatal(err) }
	if n.HeadHash != before || n.Head.Header.Height != 0 { t.Fatal("candidate persistence advanced canonical head") }
	got, err := store.GetCandidate(key)
	if err != nil { t.Fatal(err) }
	h, err := block.Hash(got)
	if err != nil { t.Fatal(err) }
	if h != key.Hash { t.Fatal("stored candidate hash mismatch") }
}

func TestResumeFinalityCommitFromCandidateStoreAfterRestart(t *testing.T) {
	n, _, candidate, certificate, validatorResolver, senderResolver, _ := finalizedBlockFixture(t, storage.NewMemoryStore())
	validators := mustValidatorSet(t, certificate)
	power := mustVotingPowerSet(t, certificate)
	candidateStore := storage.NewMemoryCandidateStore()
	if _, err := n.PersistCandidateForFinality(candidateStore, candidate); err != nil { t.Fatal(err) }

	recovery, err := n.ReconstructConsensusRuntime(1, validators, power, certificate.Threshold, consensus.RoundRobinProposer{})
	if err != nil { t.Fatal(err) }
	result, err := n.ResumeFinalityCommitFromCandidateStore(recovery, candidateStore, certificate, validators, power, validatorResolver, senderResolver, validatorResolver)
	if err != nil { t.Fatal(err) }
	if !result.Committed { t.Fatalf("result=%+v", result) }
}

func TestResumeFinalityCommitFromCandidateStoreMissingCandidate(t *testing.T) {
	n, _, _, certificate, validatorResolver, senderResolver, _ := finalizedBlockFixture(t, storage.NewMemoryStore())
	validators := mustValidatorSet(t, certificate)
	power := mustVotingPowerSet(t, certificate)
	recovery, err := n.ReconstructConsensusRuntime(1, validators, power, certificate.Threshold, consensus.RoundRobinProposer{})
	if err != nil { t.Fatal(err) }
	_, err = n.ResumeFinalityCommitFromCandidateStore(recovery, storage.NewMemoryCandidateStore(), certificate, validators, power, validatorResolver, senderResolver, validatorResolver)
	if !errors.Is(err, ErrFinalityRecoveryCandidateRequired) { t.Fatalf("error=%v", err) }
}



func TestResumeFinalityCommitFromFileCandidateStoreSurvivesRestartAndCleansUp(t *testing.T) {
	n, _, candidate, certificate, validatorResolver, senderResolver, _ := finalizedBlockFixture(t, storage.NewMemoryStore())
	validators := mustValidatorSet(t, certificate)
	power := mustVotingPowerSet(t, certificate)
	path := filepath.Join(t.TempDir(), "pending-candidates.gob")
	candidateStore, err := storage.NewFileCandidateStore(path)
	if err != nil { t.Fatal(err) }
	if _, err := n.PersistCandidateForFinality(candidateStore, candidate); err != nil { t.Fatal(err) }

	// Simulate process restart by reopening the independent candidate store.
	reopened, err := storage.NewFileCandidateStore(path)
	if err != nil { t.Fatal(err) }
	recovery, err := n.ReconstructConsensusRuntime(1, validators, power, certificate.Threshold, consensus.RoundRobinProposer{})
	if err != nil { t.Fatal(err) }

	result, err := n.ResumeFinalityCommitFromCandidateStore(
		recovery, reopened, certificate, validators, power,
		validatorResolver, senderResolver, validatorResolver,
	)
	if err != nil { t.Fatal(err) }
	if !result.Committed || result.AlreadyCommitted { t.Fatalf("result=%+v", result) }
	if _, err := reopened.GetCandidate(storage.CandidateKey{Height:candidate.Header.Height, Hash:result.BlockHash}); !errors.Is(err, storage.ErrCandidateNotFound) {
		t.Fatalf("candidate should be cleaned after successful canonical commit, got %v", err)
	}

	// Reopen again: cleanup must be durable, while canonical commit remains
	// the authoritative source of truth.
	reopenedAgain, err := storage.NewFileCandidateStore(path)
	if err != nil { t.Fatal(err) }
	if _, err := reopenedAgain.GetCandidate(storage.CandidateKey{Height:candidate.Header.Height, Hash:result.BlockHash}); !errors.Is(err, storage.ErrCandidateNotFound) {
		t.Fatalf("candidate cleanup did not survive restart, got %v", err)
	}
}

func TestResumeFinalityCommitFromCandidateStoreDoesNotDeleteBeforeCommit(t *testing.T) {
	store := &failingCommitStore{MemoryStore: storage.NewMemoryStore()}
	n, _, candidate, certificate, validatorResolver, senderResolver, _ := finalizedBlockFixture(t, store)
	validators := mustValidatorSet(t, certificate)
	power := mustVotingPowerSet(t, certificate)
	candidateStore := storage.NewMemoryCandidateStore()
	key, err := n.PersistCandidateForFinality(candidateStore, candidate)
	if err != nil { t.Fatal(err) }
	recovery, err := n.ReconstructConsensusRuntime(1, validators, power, certificate.Threshold, consensus.RoundRobinProposer{})
	if err != nil { t.Fatal(err) }
	store.failCommit = true

	_, err = n.ResumeFinalityCommitFromCandidateStore(
		recovery, candidateStore, certificate, validators, power,
		validatorResolver, senderResolver, validatorResolver,
	)
	if !errors.Is(err, errCommitFailed) { t.Fatalf("error=%v", err) }
	if _, err := candidateStore.GetCandidate(key); err != nil {
		t.Fatalf("candidate was removed before successful canonical commit: %v", err)
	}
}
