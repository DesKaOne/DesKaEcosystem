package node

import (
	"errors"
	"reflect"
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/block"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/state"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/storage"
)

type faultMatrixCandidateStore struct {
	storage.CandidateStore
	err error
}

func (s *faultMatrixCandidateStore) SaveCandidate(storage.CandidateKey, block.Block) error {
	return s.err
}

type faultMatrixCanonicalStore struct {
	storage.ChainStore
	err error
}

func (s *faultMatrixCanonicalStore) CommitBlockState(block.Block, types.Hash, *state.State) error {
	return s.err
}

func TestFinalityRecoveryFaultMatrixCandidateFailureStopsBeforeEvidence(t *testing.T) {
	n, ctx, candidate, certificate, validatorResolver, senderResolver, _ := finalizedBlockFixture(t, storage.NewMemoryStore())
	candidateStore := &faultMatrixCandidateStore{
		CandidateStore: storage.NewMemoryCandidateStore(),
		err:            errors.New("injected candidate persistence failure"),
	}
	evidenceStore := storage.NewMemoryConsensusEvidenceStore()
	validators := mustValidatorSet(t, certificate)
	power := mustVotingPowerSet(t, certificate)

	_, evidenceKey, err := n.PersistFinalizedCandidateAndEvidence(
		candidateStore,
		evidenceStore,
		ctx, candidate, certificate, validators, power,
		validatorResolver, senderResolver,
		finalizedPersistenceContext(ctx, certificate),
		mustTestSigner(t, 23), certificate.Votes[0].Sender,
	)
	if !errors.Is(err, candidateStore.err) {
		t.Fatalf("error = %v, want %v", err, candidateStore.err)
	}
	if evidenceKey != "" {
		t.Fatalf("evidence key = %q, want empty", evidenceKey)
	}
	records, err := evidenceStore.LoadConsensusEvidence()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 0 {
		t.Fatalf("evidence records = %d, want 0 after candidate failure", len(records))
	}
}

func TestFinalityRecoveryFaultMatrixEvidenceFailureRetainsCandidate(t *testing.T) {
	n, ctx, candidate, certificate, validatorResolver, senderResolver, _ := finalizedBlockFixture(t, storage.NewMemoryStore())
	candidateStore := storage.NewMemoryCandidateStore()
	evidenceStore := &failingFinalityEvidenceStore{
		MemoryConsensusEvidenceStore: storage.NewMemoryConsensusEvidenceStore(),
		err: errors.New("injected evidence persistence failure"),
	}
	validators := mustValidatorSet(t, certificate)
	power := mustVotingPowerSet(t, certificate)

	key, evidenceKey, err := n.PersistFinalizedCandidateAndEvidence(
		candidateStore,
		evidenceStore,
		ctx, candidate, certificate, validators, power,
		validatorResolver, senderResolver,
		finalizedPersistenceContext(ctx, certificate),
		mustTestSigner(t, 23), certificate.Votes[0].Sender,
	)
	if !errors.Is(err, evidenceStore.err) {
		t.Fatalf("error = %v, want %v", err, evidenceStore.err)
	}
	if evidenceKey != "" {
		t.Fatalf("evidence key = %q, want empty", evidenceKey)
	}
	if _, err := candidateStore.GetCandidate(key); err != nil {
		t.Fatalf("candidate = unavailable after evidence failure: %v", err)
	}
}

func TestFinalityRecoveryFaultMatrixCanonicalCommitFailureRetainsRecoveryArtifacts(t *testing.T) {
	n, ctx, candidate, certificate, validatorResolver, senderResolver, _ := finalizedBlockFixture(t, storage.NewMemoryStore())
	validators := mustValidatorSet(t, certificate)
	power := mustVotingPowerSet(t, certificate)
	candidateStore := storage.NewMemoryCandidateStore()
	evidenceStore := storage.NewMemoryConsensusEvidenceStore()

	key, evidenceKey, err := n.PersistFinalizedCandidateAndEvidence(
		candidateStore,
		evidenceStore,
		ctx, candidate, certificate, validators, power,
		validatorResolver, senderResolver,
		finalizedPersistenceContext(ctx, certificate),
		mustTestSigner(t, 23), certificate.Votes[0].Sender,
	)
	if err != nil {
		t.Fatal(err)
	}
	if evidenceKey == "" {
		t.Fatal("expected durable finality evidence key")
	}

	beforeHead := n.Head
	beforeHash := n.HeadHash
	beforeRoot := n.State.Root()
	canonicalStore := n.Store
	fault := errors.New("injected canonical commit failure")
	n.Store = &faultMatrixCanonicalStore{
		ChainStore: canonicalStore,
		err:        fault,
	}

	err = n.CommitFinalizedBlock(
		ctx, candidate, certificate, validators, power,
		validatorResolver, senderResolver,
	)
	if !errors.Is(err, fault) {
		t.Fatalf("error = %v, want canonical commit failure", err)
	}
	if !reflect.DeepEqual(n.Head, beforeHead) || n.HeadHash != beforeHash || n.State.Root() != beforeRoot {
		t.Fatal("canonical node state mutated after commit failure")
	}
	storedHead, storedHash, err := canonicalStore.Head()
	if err != nil {
		t.Fatal(err)
	}
	if storedHead.Header.Height != beforeHead.Header.Height || storedHash != beforeHash {
		t.Fatal("canonical store mutated after injected commit failure")
	}
	if _, err := candidateStore.GetCandidate(key); err != nil {
		t.Fatalf("candidate recovery artifact lost after commit failure: %v", err)
	}
	records, err := evidenceStore.LoadConsensusEvidence()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := records[evidenceKey]; !ok {
		t.Fatal("finality evidence recovery artifact lost after commit failure")
	}
}
