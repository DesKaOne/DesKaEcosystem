package node

import (
	"errors"
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/consensus"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/crypto"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/block"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/storage"
)

type faultedRestartFixture struct {
	chainPath    string
	candidatePath string
	evidencePath string
	node         *Node
	ctx          consensus.BlockProductionContext
	candidate   block.Block
	certificate consensus.FinalityCertificate
	validators  consensus.ValidatorSet
	power       consensus.VotingPowerSet
	validatorAuthority validatorAuthorityResolver
	senderAuthority senderAuthorityResolver
	signer       crypto.Signer
	validatorID  []byte
}

func newFaultedRestartFixture(t *testing.T) faultedRestartFixture {
	t.Helper()
	dir := t.TempDir()
	chainPath := dir + "/chain.gob"
	candidatePath := dir + "/candidates.gob"
	evidencePath := dir + "/evidence.gob"

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
			ChainID: n.Config.ChainID,
			Epoch: 1,
			Height: 0,
			Round: 0,
			Phase: consensus.PhaseProposal,
		},
		PreviousHash: n.HeadHash,
		Proposer: validatorID,
	}
	candidate := block.Block{
		Header: block.Header{
			Version: n.Config.ProtocolVersion,
			ChainID: n.Config.ChainID,
			Height: 1,
			Timestamp: n.Head.Header.Timestamp + 1,
			PreviousHash: n.HeadHash,
			StateRoot: n.State.Root(),
			Proposer: validatorID,
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
		ChainID: ctx.State.ChainID,
		Epoch: ctx.State.Epoch,
		Height: ctx.State.Height,
		Round: ctx.State.Round,
		Sender: validatorID,
		Type: consensus.MessageTypePrecommit,
		Payload: payload[:],
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

	return faultedRestartFixture{
		chainPath: chainPath, candidatePath: candidatePath, evidencePath: evidencePath,
		node: n, ctx: ctx, candidate: candidate, certificate: certificate,
		validators: validators, power: power,
		validatorAuthority: validatorAuthority, senderAuthority: senderAuthority,
		signer: signer, validatorID: validatorID,
	}
}

func (f faultedRestartFixture) reopen(t *testing.T) (*Node, *storage.FileCandidateStore, *storage.FileConsensusEvidenceStore) {
	t.Helper()
	chainStore, err := storage.NewFileStore(f.chainPath)
	if err != nil {
		t.Fatal(err)
	}
	n, err := OpenDevnet(chainStore)
	if err != nil {
		t.Fatal(err)
	}
	candidates, err := storage.NewFileCandidateStore(f.candidatePath)
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := storage.NewFileConsensusEvidenceStore(f.evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	return n, candidates, evidence
}

func TestFinalityFaultedRestartMatrix(t *testing.T) {
	t.Run("crash_before_candidate_persistence", func(t *testing.T) {
		f := newFaultedRestartFixture(t)
		restarted, candidates, evidence := f.reopen(t)
		if restarted.Head.Header.Height != 0 {
			t.Fatalf("restarted height = %d, want 0", restarted.Head.Header.Height)
		}
		key, err := block.Hash(f.candidate)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := candidates.GetCandidate(storage.CandidateKey{Height: 1, Hash: key}); !errors.Is(err, storage.ErrCandidateNotFound) {
			t.Fatalf("candidate lookup error = %v, want ErrCandidateNotFound", err)
		}
		records, err := evidence.LoadConsensusEvidence()
		if err != nil {
			t.Fatal(err)
		}
		if len(records) != 0 {
			t.Fatalf("evidence records = %d, want 0", len(records))
		}
	})

	t.Run("crash_after_candidate_before_evidence", func(t *testing.T) {
		f := newFaultedRestartFixture(t)
		candidates, err := storage.NewFileCandidateStore(f.candidatePath)
		if err != nil {
			t.Fatal(err)
		}
		key, err := f.node.PersistCandidateForFinality(candidates, f.candidate)
		if err != nil {
			t.Fatal(err)
		}

		restarted, candidates, evidence := f.reopen(t)
		if restarted.Head.Header.Height != 0 {
			t.Fatalf("restarted height = %d, want 0", restarted.Head.Header.Height)
		}
		if _, err := candidates.GetCandidate(key); err != nil {
			t.Fatalf("candidate after restart = %v", err)
		}
		records, err := evidence.LoadConsensusEvidence()
		if err != nil {
			t.Fatal(err)
		}
		if len(records) != 0 {
			t.Fatalf("evidence records = %d, want 0", len(records))
		}
	})

	t.Run("crash_after_evidence_before_canonical_commit", func(t *testing.T) {
		f := newFaultedRestartFixture(t)
		candidates, err := storage.NewFileCandidateStore(f.candidatePath)
		if err != nil {
			t.Fatal(err)
		}
		evidence, err := storage.NewFileConsensusEvidenceStore(f.evidencePath)
		if err != nil {
			t.Fatal(err)
		}
		key, evidenceKey, err := f.node.PersistFinalizedCandidateAndEvidence(
			candidates, evidence, f.ctx, f.candidate, f.certificate,
			f.validators, f.power, f.validatorAuthority, f.senderAuthority,
			finalizedPersistenceContext(f.ctx, f.certificate), f.signer, f.validatorID,
		)
		if err != nil {
			t.Fatal(err)
		}

		restarted, candidates, evidence := f.reopen(t)
		recoveredCertificate, recoveredKey, err := consensus.RecoverFinalityCertificateWithContext(
			evidence, f.ctx.State, f.validators, f.power,
			f.validatorAuthority, finalizedPersistenceContext(f.ctx, f.certificate),
		)
		if err != nil {
			t.Fatal(err)
		}
		if recoveredKey != evidenceKey {
			t.Fatalf("recovered evidence key = %q, want %q", recoveredKey, evidenceKey)
		}
		recoveredCandidate, err := candidates.GetCandidate(key)
		if err != nil {
			t.Fatal(err)
		}
		if err := restarted.CommitFinalizedBlock(
			f.ctx, recoveredCandidate, recoveredCertificate,
			f.validators, f.power, f.validatorAuthority, f.senderAuthority,
		); err != nil {
			t.Fatal(err)
		}
		if restarted.HeadHash != key.Hash {
			t.Fatal("restart recovery did not commit recovered candidate")
		}
	})

	t.Run("crash_after_canonical_commit_before_cleanup", func(t *testing.T) {
		f := newFaultedRestartFixture(t)
		candidates, err := storage.NewFileCandidateStore(f.candidatePath)
		if err != nil {
			t.Fatal(err)
		}
		evidence, err := storage.NewFileConsensusEvidenceStore(f.evidencePath)
		if err != nil {
			t.Fatal(err)
		}
		key, _, err := f.node.PersistFinalizedCandidateAndEvidence(
			candidates, evidence, f.ctx, f.candidate, f.certificate,
			f.validators, f.power, f.validatorAuthority, f.senderAuthority,
			finalizedPersistenceContext(f.ctx, f.certificate), f.signer, f.validatorID,
		)
		if err != nil {
			t.Fatal(err)
		}
		if err := f.node.CommitFinalizedBlock(
			f.ctx, f.candidate, f.certificate,
			f.validators, f.power, f.validatorAuthority, f.senderAuthority,
		); err != nil {
			t.Fatal(err)
		}

		restarted, candidates, evidence := f.reopen(t)
		recoveredCertificate, _, err := consensus.RecoverFinalityCertificateWithContext(
			evidence, f.ctx.State, f.validators, f.power,
			f.validatorAuthority, finalizedPersistenceContext(f.ctx, f.certificate),
		)
		if err != nil {
			t.Fatal(err)
		}
		recoveredCandidate, err := candidates.GetCandidate(key)
		if err != nil {
			t.Fatal(err)
		}
		if err := restarted.CommitFinalizedBlock(
			f.ctx, recoveredCandidate, recoveredCertificate,
			f.validators, f.power, f.validatorAuthority, f.senderAuthority,
		); !errors.Is(err, ErrFinalizedBlockAlreadyCommitted) {
			t.Fatalf("post-commit replay error = %v, want ErrFinalizedBlockAlreadyCommitted", err)
		}
		if err := candidates.DeleteCandidate(key); err != nil {
			t.Fatal(err)
		}
		if _, err := candidates.GetCandidate(key); !errors.Is(err, storage.ErrCandidateNotFound) {
			t.Fatalf("cleanup lookup error = %v, want ErrCandidateNotFound", err)
		}
	})
}
