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


func TestFinalizedCrashRecoveryAcrossFileStoresCommitsExactlyOnce(t *testing.T) {
	tempDir := t.TempDir()
	chainPath := tempDir + "/chain.gob"
	candidatePath := tempDir + "/candidates.gob"
	evidencePath := tempDir + "/evidence.gob"

	chainStore, err := storage.NewFileStore(chainPath)
	if err != nil {
		t.Fatal(err)
	}
	n, err := NewDevnet(chainStore)
	if err != nil {
		t.Fatal(err)
	}

	signer := testEd25519Signer(t, 23)
	validatorID := []byte("fixture-validator")
	validators, err := consensus.NewValidatorSet([][]byte{validatorID})
	if err != nil {
		t.Fatal(err)
	}
	power, err := consensus.NewVotingPowerSet([]consensus.ValidatorVotingPower{{ValidatorID: validatorID, Power: 1}})
	if err != nil {
		t.Fatal(err)
	}
	validatorAuthority := validatorAuthorityResolver{publicKey: signer.PublicKey()}
	senderAuthority := senderAuthorityResolver{publicKey: signer.PublicKey()}
	ctx := consensus.BlockProductionContext{
		State: consensus.RoundState{
			ProtocolVersion: n.Config.ProtocolVersion,
			ChainID:         n.Config.ChainID,
			Epoch:           1,
			Height:          0,
			Round:           0,
			Phase:           consensus.PhaseProposal,
		},
		PreviousHash: n.HeadHash,
		Proposer:     validatorID,
	}
	candidate := block.Block{
		Header: block.Header{
			Version:       n.Config.ProtocolVersion,
			ChainID:       n.Config.ChainID,
			Height:        1,
			Timestamp:     n.Head.Header.Timestamp + 1,
			PreviousHash:  n.HeadHash,
			StateRoot:     n.State.Root(),
			Proposer:      validatorID,
		},
	}
	candidate.Header.TransactionsRoot, err = block.TransactionsRoot(candidate.Transactions)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := consensus.ValidateProducedBlock(ctx, candidate)
	if err != nil {
		t.Fatal(err)
	}
	vote := consensus.Message{
		ProtocolVersion: ctx.State.ProtocolVersion,
		ChainID:         ctx.State.ChainID,
		Epoch:           ctx.State.Epoch,
		Height:          ctx.State.Height,
		Round:           ctx.State.Round,
		Sender:          validatorID,
		Type:            consensus.MessageTypePrecommit,
		Payload:         payload[:],
	}
	vote, err = vote.Sign(signer)
	if err != nil {
		t.Fatal(err)
	}
	certificate, err := consensus.NewFinalityCertificate(
		ctx.State, validators, power,
		consensus.QuorumThreshold{Numerator: 1, Denominator: 1},
		payload[:], []consensus.Message{vote},
	)
	if err != nil {
		t.Fatal(err)
	}

	candidateStore, err := storage.NewFileCandidateStore(candidatePath)
	if err != nil {
		t.Fatal(err)
	}
	evidenceStore, err := storage.NewFileConsensusEvidenceStore(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	candidateKey, evidenceKey, err := n.PersistFinalizedCandidateAndEvidence(
		candidateStore, evidenceStore, ctx, candidate, certificate,
		validators, power, validatorAuthority, senderAuthority,
		finalizedPersistenceContext(ctx, certificate), signer, validatorID,
	)
	if err != nil {
		t.Fatal(err)
	}
	if evidenceKey == "" {
		t.Fatal("finality evidence key is empty")
	}

	// Simulate process restart by reopening all three independent durable stores.
	reopenedChainStore, err := storage.NewFileStore(chainPath)
	if err != nil {
		t.Fatal(err)
	}
	restartedNode, err := OpenDevnet(reopenedChainStore)
	if err != nil {
		t.Fatal(err)
	}
	reopenedCandidateStore, err := storage.NewFileCandidateStore(candidatePath)
	if err != nil {
		t.Fatal(err)
	}
	reopenedEvidenceStore, err := storage.NewFileConsensusEvidenceStore(evidencePath)
	if err != nil {
		t.Fatal(err)
	}

	recoveredCertificate, recoveredEvidenceKey, err := consensus.RecoverFinalityCertificateWithContext(
		reopenedEvidenceStore, ctx.State, validators, power,
		validatorAuthority, finalizedPersistenceContext(ctx, certificate),
	)
	if err != nil {
		t.Fatal(err)
	}
	if recoveredEvidenceKey != evidenceKey {
		t.Fatalf("recovered evidence key = %q, want %q", recoveredEvidenceKey, evidenceKey)
	}
	recoveredCandidate, err := reopenedCandidateStore.GetCandidate(candidateKey)
	if err != nil {
		t.Fatal(err)
	}
	recoveredHash, err := block.Hash(recoveredCandidate)
	if err != nil {
		t.Fatal(err)
	}
	if recoveredHash != candidateKey.Hash {
		t.Fatal("recovered candidate hash does not match durable candidate identity")
	}
	if recoveredCandidate.Header.StateRoot != restartedNode.State.Root() {
		t.Fatal("recovered candidate state root does not match restarted canonical state")
	}

	if err := restartedNode.CommitFinalizedBlock(
		ctx, recoveredCandidate, recoveredCertificate,
		validators, power, validatorAuthority, senderAuthority,
	); err != nil {
		t.Fatal(err)
	}
	if restartedNode.Head.Header.Height != candidate.Header.Height {
		t.Fatalf("restarted head height = %d, want %d", restartedNode.Head.Header.Height, candidate.Header.Height)
	}
	if restartedNode.HeadHash != candidateKey.Hash {
		t.Fatal("restarted canonical head hash does not match recovered candidate")
	}
	if restartedNode.State.Root() != candidate.Header.StateRoot {
		t.Fatal("restarted canonical state root changed unexpectedly")
	}

	if err := reopenedCandidateStore.DeleteCandidate(candidateKey); err != nil {
		t.Fatal(err)
	}
	if _, err := reopenedCandidateStore.GetCandidate(candidateKey); !errors.Is(err, storage.ErrCandidateNotFound) {
		t.Fatalf("candidate cleanup error = %v, want %v", err, storage.ErrCandidateNotFound)
	}

	if err := restartedNode.CommitFinalizedBlock(
		ctx, recoveredCandidate, recoveredCertificate,
		validators, power, validatorAuthority, senderAuthority,
	); !errors.Is(err, ErrFinalizedBlockAlreadyCommitted) {
		t.Fatalf("second canonical recovery error = %v, want %v", err, ErrFinalizedBlockAlreadyCommitted)
	}

	finalStore, err := storage.NewFileStore(chainPath)
	if err != nil {
		t.Fatal(err)
	}
	finalNode, err := OpenDevnet(finalStore)
	if err != nil {
		t.Fatal(err)
	}
	finalHead, finalHash, err := finalStore.Head()
	if err != nil {
		t.Fatal(err)
	}
	finalState, err := finalStore.LoadState()
	if err != nil {
		t.Fatal(err)
	}
	if finalHead.Header.Height != candidate.Header.Height || finalHash != candidateKey.Hash {
		t.Fatalf("reopened canonical head = height %d hash %x, want height %d hash %x",
			finalHead.Header.Height, finalHash, candidate.Header.Height, candidateKey.Hash)
	}
	if finalHead.Header.StateRoot != candidate.Header.StateRoot || finalState.Root() != candidate.Header.StateRoot {
		t.Fatal("reopened canonical state root does not match finalized candidate")
	}
	if finalNode.HeadHash != candidateKey.Hash || finalNode.State.Root() != candidate.Header.StateRoot {
		t.Fatal("reopened node does not match canonical final commit")
	}
}
