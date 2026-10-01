package node

import (
	"errors"
	"reflect"
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/consensus"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/storage"
)

func TestCommitFinalityEvidenceAndPublishConsensusAdvancesNextHeight(t *testing.T) {
	n, ctx, candidate, certificate, validatorResolver, senderResolver, _ := finalizedBlockFixture(t, storage.NewMemoryStore())
	validators := mustValidatorSet(t, certificate)
	power := mustVotingPowerSet(t, certificate)
	runtime, err := consensus.NewValidatorRuntime(consensus.RuntimeConfig{
		Rules: consensus.ValidationRules{
			ProtocolVersion: ctx.State.ProtocolVersion,
			ChainID:        ctx.State.ChainID,
			RequireSender:  true,
		},
		State: ctx.State, Validators: validators, VotingPower: power,
		Threshold: consensus.QuorumThreshold{Numerator: 1, Denominator: 1},
		Proposer: consensus.RoundRobinProposer{},
	})
	if err != nil {
		t.Fatal(err)
	}
	proposal, err := consensus.NewBlockProposal(ctx, candidate)
	if err != nil {
		t.Fatal(err)
	}
	if err := runtime.AcceptBlockProposal(proposal); err != nil {
		t.Fatal(err)
	}
	prevote := consensus.Message{
		ProtocolVersion: ctx.State.ProtocolVersion, ChainID: ctx.State.ChainID,
		Epoch: ctx.State.Epoch, Height: ctx.State.Height, Round: ctx.State.Round,
		Sender: append([]byte(nil), certificate.Votes[0].Sender...),
		Type: consensus.MessageTypePrevote, Payload: append([]byte(nil), certificate.Payload...),
	}
	if err := runtime.AddVote(prevote); err != nil {
		t.Fatal(err)
	}
	if err := runtime.AddVote(certificate.Votes[0]); err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.FinalizeProposal(validatorResolver); err != nil {
		t.Fatal(err)
	}

	if err := n.CommitFinalityEvidenceAndPublishConsensus(
		ctx, candidate, certificate, validators, power,
		validatorResolver, senderResolver, runtime,
	); err != nil {
		t.Fatal(err)
	}
	if got := runtime.State(); got.Height != candidate.Header.Height || got.Round != 0 || got.Phase != consensus.PhaseProposal {
		t.Fatalf("runtime did not enter next height: height=%d round=%d phase=%v", got.Height, got.Round, got.Phase)
	}
	last, err := runtime.LastCanonicalCommit()
	if err != nil {
		t.Fatal(err)
	}
	if last.Height != n.Head.Header.Height || last.BlockHash != n.HeadHash || last.StateRoot != n.State.Root() {
		t.Fatal("runtime publication does not match canonical node state")
	}
}

func TestCommitFinalityEvidenceAndPublishConsensusDoesNotAdvanceOnStoreFailure(t *testing.T) {
	store := &failingCommitStore{MemoryStore: storage.NewMemoryStore(), failCommit: false}
	n, ctx, candidate, certificate, validatorResolver, senderResolver, _ := finalizedBlockFixture(t, store)
	validators := mustValidatorSet(t, certificate)
	power := mustVotingPowerSet(t, certificate)
	runtime, err := consensus.NewValidatorRuntime(consensus.RuntimeConfig{
		Rules: consensus.ValidationRules{ProtocolVersion: ctx.State.ProtocolVersion, ChainID: ctx.State.ChainID, RequireSender: true},
		State: ctx.State, Validators: validators, VotingPower: power,
		Threshold: consensus.QuorumThreshold{Numerator: 1, Denominator: 1},
		Proposer: consensus.RoundRobinProposer{},
	})
	if err != nil {
		t.Fatal(err)
	}
	proposal, err := consensus.NewBlockProposal(ctx, candidate)
	if err != nil {
		t.Fatal(err)
	}
	if err := runtime.AcceptBlockProposal(proposal); err != nil {
		t.Fatal(err)
	}
	prevote := consensus.Message{
		ProtocolVersion: ctx.State.ProtocolVersion, ChainID: ctx.State.ChainID,
		Epoch: ctx.State.Epoch, Height: ctx.State.Height, Round: ctx.State.Round,
		Sender: append([]byte(nil), certificate.Votes[0].Sender...),
		Type: consensus.MessageTypePrevote, Payload: append([]byte(nil), certificate.Payload...),
	}
	if err := runtime.AddVote(prevote); err != nil {
		t.Fatal(err)
	}
	if err := runtime.AddVote(certificate.Votes[0]); err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.FinalizeProposal(validatorResolver); err != nil {
		t.Fatal(err)
	}

	store.failCommit = true
	if err := n.CommitFinalityEvidenceAndPublishConsensus(
		ctx, candidate, certificate, validators, power,
		validatorResolver, senderResolver, runtime,
	); !errors.Is(err, errCommitFailed) {
		t.Fatalf("error = %v, want %v", err, errCommitFailed)
	}
	if got := runtime.State(); got.Height != ctx.State.Height || got.Phase != consensus.PhaseFinalized {
		t.Fatal("runtime advanced despite canonical commit failure")
	}
	if _, err := runtime.FinalizedCertificate(); err != nil {
		t.Fatalf("finality evidence disappeared after failed commit: %v", err)
	}
	if n.Head.Header.Height != ctx.State.Height || n.HeadHash != ctx.PreviousHash {
		t.Fatal("node advanced despite canonical commit failure")
	}
}

func TestCommitFinalityEvidenceAndPublishConsensusRejectsDuplicateAfterAdvance(t *testing.T) {
	n, ctx, candidate, certificate, validatorResolver, senderResolver, _ := finalizedBlockFixture(t, storage.NewMemoryStore())
	validators := mustValidatorSet(t, certificate)
	power := mustVotingPowerSet(t, certificate)
	runtime, err := consensus.NewValidatorRuntime(consensus.RuntimeConfig{
		Rules: consensus.ValidationRules{ProtocolVersion: ctx.State.ProtocolVersion, ChainID: ctx.State.ChainID, RequireSender: true},
		State: ctx.State, Validators: validators, VotingPower: power,
		Threshold: consensus.QuorumThreshold{Numerator: 1, Denominator: 1},
		Proposer: consensus.RoundRobinProposer{},
	})
	if err != nil {
		t.Fatal(err)
	}
	proposal, err := consensus.NewBlockProposal(ctx, candidate)
	if err != nil {
		t.Fatal(err)
	}
	if err := runtime.AcceptBlockProposal(proposal); err != nil {
		t.Fatal(err)
	}
	prevote := consensus.Message{
		ProtocolVersion: ctx.State.ProtocolVersion, ChainID: ctx.State.ChainID,
		Epoch: ctx.State.Epoch, Height: ctx.State.Height, Round: ctx.State.Round,
		Sender: append([]byte(nil), certificate.Votes[0].Sender...),
		Type: consensus.MessageTypePrevote, Payload: append([]byte(nil), certificate.Payload...),
	}
	if err := runtime.AddVote(prevote); err != nil {
		t.Fatal(err)
	}
	if err := runtime.AddVote(certificate.Votes[0]); err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.FinalizeProposal(validatorResolver); err != nil {
		t.Fatal(err)
	}
	if err := n.CommitFinalityEvidenceAndPublishConsensus(ctx, candidate, certificate, validators, power, validatorResolver, senderResolver, runtime); err != nil {
		t.Fatal(err)
	}
	beforeHead, beforeHash, beforeRoot := n.Head, n.HeadHash, n.State.Root()
	if err := n.CommitFinalityEvidenceAndPublishConsensus(ctx, candidate, certificate, validators, power, validatorResolver, senderResolver, runtime); err == nil {
		t.Fatal("expected duplicate handoff rejection")
	}
	if !reflect.DeepEqual(n.Head, beforeHead) || n.HeadHash != beforeHash || n.State.Root() != beforeRoot {
		t.Fatal("duplicate handoff mutated canonical node state")
	}
}
