package node

import (
	"errors"
	"reflect"
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/consensus"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/block"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/storage"
)

func TestCommitFinalityEvidenceCommitsCanonicalBlock(t *testing.T) {
	n, ctx, candidate, certificate, validatorResolver, senderResolver, recipient := finalizedBlockFixture(t, storage.NewMemoryStore())
	if err := n.CommitFinalityEvidence(
		ctx, candidate, certificate,
		mustValidatorSet(t, certificate), mustVotingPowerSet(t, certificate),
		validatorResolver, senderResolver,
	); err != nil {
		t.Fatal(err)
	}
	if n.Head.Header.Height != 1 {
		t.Fatalf("head height = %d, want 1", n.Head.Header.Height)
	}
	if got, ok := n.State.Get(recipient); !ok || got.Balance != 20 {
		t.Fatalf("recipient = %+v, want balance 20", got)
	}
}

func TestCommitFinalityEvidenceRejectsTamperedEvidenceWithoutMutation(t *testing.T) {
	n, ctx, candidate, certificate, validatorResolver, senderResolver, _ := finalizedBlockFixture(t, storage.NewMemoryStore())
	certificate.Votes[0].Signature[0] ^= 0xff
	beforeHead, beforeHash, beforeRoot := n.Head, n.HeadHash, n.State.Root()
	_, storedHash, err := n.Store.Head()
	if err != nil {
		t.Fatal(err)
	}
	if err := n.CommitFinalityEvidence(
		ctx, candidate, certificate,
		mustValidatorSet(t, certificate), mustVotingPowerSet(t, certificate),
		validatorResolver, senderResolver,
	); err == nil {
		t.Fatal("expected tampered evidence rejection")
	}
	if !reflect.DeepEqual(n.Head, beforeHead) || n.HeadHash != beforeHash || n.State.Root() != beforeRoot {
		t.Fatal("node mutated after tampered evidence rejection")
	}
	_, afterHash, err := n.Store.Head()
	if err != nil {
		t.Fatal(err)
	}
	if afterHash != storedHash {
		t.Fatal("storage mutated after tampered evidence rejection")
	}
}

func TestCommitFinalityEvidenceRejectsStaleCanonicalContextWithoutMutation(t *testing.T) {
	n, ctx, candidate, certificate, validatorResolver, senderResolver, _ := finalizedBlockFixture(t, storage.NewMemoryStore())
	stale := ctx
	stale.State.Height++
	stale.PreviousHash = candidate.Header.PreviousHash
	beforeHead, beforeHash, beforeRoot := n.Head, n.HeadHash, n.State.Root()
	err := n.CommitFinalityEvidence(
		stale, candidate, certificate,
		mustValidatorSet(t, certificate), mustVotingPowerSet(t, certificate),
		validatorResolver, senderResolver,
	)
	if !errors.Is(err, ErrConsensusContextMismatch) {
		t.Fatalf("error = %v, want %v", err, ErrConsensusContextMismatch)
	}
	if !reflect.DeepEqual(n.Head, beforeHead) || n.HeadHash != beforeHash || n.State.Root() != beforeRoot {
		t.Fatal("node mutated after stale context rejection")
	}
}

func TestCommitFinalityEvidenceReplayIsIdempotentlyRejected(t *testing.T) {
	n, ctx, candidate, certificate, validatorResolver, senderResolver, _ := finalizedBlockFixture(t, storage.NewMemoryStore())
	validators := mustValidatorSet(t, certificate)
	power := mustVotingPowerSet(t, certificate)
	if err := n.CommitFinalityEvidence(ctx, candidate, certificate, validators, power, validatorResolver, senderResolver); err != nil {
		t.Fatal(err)
	}
	beforeHead, beforeHash, beforeRoot := n.Head, n.HeadHash, n.State.Root()
	err := n.CommitFinalityEvidence(ctx, candidate, certificate, validators, power, validatorResolver, senderResolver)
	if err != ErrConsensusContextMismatch && err != ErrFinalizedBlockAlreadyCommitted {
		t.Fatalf("error = %v, want replay rejection", err)
	}
	if !reflect.DeepEqual(n.Head, beforeHead) || n.HeadHash != beforeHash || n.State.Root() != beforeRoot {
		t.Fatal("node mutated after replay rejection")
	}
}


type failingFinalityEvidenceStore struct {
	*storage.MemoryConsensusEvidenceStore
	err error
}

func (s *failingFinalityEvidenceStore) PutConsensusEvidence(string, []byte) error {
	return s.err
}

func finalizedPersistenceContext(ctx consensus.BlockProductionContext, certificate consensus.FinalityCertificate) consensus.PersistenceContext {
	return consensus.PersistenceContext{
		ProtocolVersion: uint64(ctx.State.ProtocolVersion),
		ChainID: uint64(ctx.State.ChainID),
		Epoch: certificate.Epoch,
		Height: uint64(certificate.Height),
		Round: certificate.Round,
		Phase: uint8(consensus.PhaseFinalized),
		ThresholdNumerator: certificate.Threshold.Numerator,
		ThresholdDenominator: certificate.Threshold.Denominator,
	}
}

func TestPersistFinalizedCandidateAndEvidenceOrdersCandidateBeforeEvidence(t *testing.T) {
	n, ctx, candidate, certificate, validatorResolver, senderResolver, _ := finalizedBlockFixture(t, storage.NewMemoryStore())
	validators := mustValidatorSet(t, certificate)
	power := mustVotingPowerSet(t, certificate)
	evidenceStore := storage.NewMemoryConsensusEvidenceStore()

	key, evidenceKey, err := n.PersistFinalizedCandidateAndEvidence(
		storage.NewMemoryCandidateStore(),
		evidenceStore,
		ctx, candidate, certificate, validators, power,
		validatorResolver, senderResolver,
		finalizedPersistenceContext(ctx, certificate),
		mustTestSigner(t, 23), certificate.Votes[0].Sender,
	)
	if err != nil {
		t.Fatal(err)
	}
	if key.Height != candidate.Header.Height || evidenceKey == "" {
		t.Fatalf("unexpected persistence identities: candidate=%+v evidence=%q", key, evidenceKey)
	}
}

func TestPersistFinalizedCandidateAndEvidenceRetainsCandidateWhenEvidenceFails(t *testing.T) {
	n, ctx, candidate, certificate, validatorResolver, senderResolver, _ := finalizedBlockFixture(t, storage.NewMemoryStore())
	validators := mustValidatorSet(t, certificate)
	power := mustVotingPowerSet(t, certificate)
	candidateStore := storage.NewMemoryCandidateStore()
	evidenceStore := &failingFinalityEvidenceStore{
		MemoryConsensusEvidenceStore: storage.NewMemoryConsensusEvidenceStore(),
		err: errors.New("injected evidence persistence failure"),
	}

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
	got, err := candidateStore.GetCandidate(key)
	if err != nil {
		t.Fatalf("candidate was not retained after evidence failure: %v", err)
	}
	gotHash, err := block.Hash(got)
	if err != nil {
		t.Fatal(err)
	}
	if gotHash != key.Hash {
		t.Fatal("retained candidate hash does not match persistence key")
	}
}
