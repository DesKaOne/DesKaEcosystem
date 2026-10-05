package node

import (
	"errors"
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/consensus"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/block"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/storage"
)

type cleanupFaultCandidateStore struct {
	storage.CandidateStore
	deleteFailures int
	err            error
}

func (s *cleanupFaultCandidateStore) DeleteCandidate(key storage.CandidateKey) error {
	if s.deleteFailures > 0 {
		s.deleteFailures--
		return s.err
	}
	return s.CandidateStore.DeleteCandidate(key)
}

func TestResumeFinalityCommitCleanupFailureRetainsCanonicalAndCandidate(t *testing.T) {
	n, _, candidate, certificate, validatorResolver, senderResolver, _ := finalizedBlockFixture(t, storage.NewMemoryStore())
	validators := mustValidatorSet(t, certificate)
	power := mustVotingPowerSet(t, certificate)
	recovery, err := n.ReconstructConsensusRuntime(1, validators, power, certificate.Threshold, consensus.RoundRobinProposer{})
	if err != nil {
		t.Fatal(err)
	}

	base := storage.NewMemoryCandidateStore()
	key, err := n.PersistCandidateForFinality(base, candidate)
	if err != nil {
		t.Fatal(err)
	}
	cleanupErr := errors.New("injected candidate cleanup failure")
	store := &cleanupFaultCandidateStore{
		CandidateStore: base,
		deleteFailures: 1,
		err:            cleanupErr,
	}

	result, err := n.ResumeFinalityCommitFromCandidateStore(
		recovery, store, certificate, validators, power,
		validatorResolver, senderResolver, validatorResolver,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Committed || result.AlreadyCommitted {
		t.Fatalf("unexpected recovery result: %+v", result)
	}
	if n.Head.Header.Height != candidate.Header.Height || n.HeadHash != key.Hash {
		t.Fatal("canonical commit was not preserved after cleanup failure")
	}
	if _, err := store.GetCandidate(key); err != nil {
		t.Fatalf("candidate should remain recoverable after cleanup failure: %v", err)
	}

	replay, err := n.ResumeFinalityCommit(
		recovery, candidate, certificate, validators, power,
		validatorResolver, senderResolver, validatorResolver,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !replay.AlreadyCommitted || replay.Committed {
		t.Fatalf("unexpected replay result: %+v", replay)
	}
	if n.Head.Header.Height != candidate.Header.Height || n.HeadHash != key.Hash {
		t.Fatal("canonical state changed during post-cleanup replay")
	}
}

func TestResumeFinalityCommitCleanupRetryIsIdempotent(t *testing.T) {
	n, _, candidate, certificate, validatorResolver, senderResolver, _ := finalizedBlockFixture(t, storage.NewMemoryStore())
	base := storage.NewMemoryCandidateStore()
	key, err := n.PersistCandidateForFinality(base, candidate)
	if err != nil {
		t.Fatal(err)
	}

	cleanupErr := errors.New("injected one-shot cleanup failure")
	store := &cleanupFaultCandidateStore{
		CandidateStore: base,
		deleteFailures: 1,
		err:            cleanupErr,
	}

	validators := mustValidatorSet(t, certificate)
	power := mustVotingPowerSet(t, certificate)
	recovery, err := n.ReconstructConsensusRuntime(1, validators, power, certificate.Threshold, consensus.RoundRobinProposer{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := n.ResumeFinalityCommitFromCandidateStore(
		recovery, store, certificate, validators, power,
		validatorResolver, senderResolver, validatorResolver,
	); err != nil {
		t.Fatal(err)
	}

	if err := store.DeleteCandidate(key); err != nil {
		t.Fatalf("cleanup retry failed: %v", err)
	}
	if _, err := store.GetCandidate(key); !errors.Is(err, storage.ErrCandidateNotFound) {
		t.Fatalf("candidate after cleanup retry = %v, want ErrCandidateNotFound", err)
	}

	if err := store.DeleteCandidate(key); err != nil {
		t.Fatalf("idempotent cleanup retry failed: %v", err)
	}
}

func TestFinalityEvidenceCleanupIsIdempotent(t *testing.T) {
	store := storage.NewMemoryConsensusEvidenceStore()
	_, ctx, _, certificate, validatorResolver, _, _ := finalizedBlockFixture(t, storage.NewMemoryStore())
	signer := mustTestSigner(t, 23)
	validators := mustValidatorSet(t, certificate)
	power := mustVotingPowerSet(t, certificate)
	key, err := consensus.PersistFinalityCertificateWithContext(
		store, certificate, ctx.State, validators, power, validatorResolver,
		finalizedPersistenceContext(ctx, certificate), signer, certificate.Votes[0].Sender,
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := consensus.DeleteConsensusEvidence(store, key); err != nil {
		t.Fatal(err)
	}
	if err := consensus.DeleteConsensusEvidence(store, key); err != nil {
		t.Fatal(err)
	}
}
