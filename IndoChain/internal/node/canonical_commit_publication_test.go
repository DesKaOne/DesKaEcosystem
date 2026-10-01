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
	prevote := certificate.Votes[0]
	prevote.Type = consensus.MessageTypePrevote
	prevote.Signature = nil
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
	got := runtime.State()
	if got.Height != candidate.Header.Height || got.Round != 0 || got.Phase != consensus.PhaseProposal {
		t.Fatalf("unexpected next-height runtime: height=%d round=%d phase=%v", got.Height, got.Round, got.Phase)
	}
	if n.Head.Header.Height != candidate.Header.Height || n.HeadHash == (candidate.Header.PreviousHash) {
		t.Fatal("canonical node head did not advance")
	}
	if n.State.Root() != candidate.Header.StateRoot {
		t.Fatal("published state root does not match canonical candidate")
	}
}

func TestCommitFinalityEvidenceAndPublishConsensusDoesNotAdvanceOnStoreFailure(t *testing.T) {
	store := &failingCommitStore{MemoryStore: storage.NewMemoryStore(), failCommit: true}
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
	prevote := certificate.Votes[0]
	prevote.Type = consensus.MessageTypePrevote
	prevote.Signature = nil
	if err := runtime.AddVote(prevote); err != nil {
		t.Fatal(err)
	}
	if err := runtime.AddVote(certificate.Votes[0]); err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.FinalizeProposal(validatorResolver); err != nil {
		t.Fatal(err)
	}
	beforeHead, beforeHash, beforeRoot := n.Head, n.HeadHash, n.State.Root()
	err = n.CommitFinalityEvidenceAndPublishConsensus(ctx, candidate, certificate, validators, power, validatorResolver, senderResolver, runtime)
	if !errors.Is(err, errCommitFailed) {
		t.Fatalf("error = %v, want %v", err, errCommitFailed)
	}
	if !reflect.DeepEqual(n.Head, beforeHead) || n.HeadHash != beforeHash || n.State.Root() != beforeRoot {
		t.Fatal("node advanced despite failed canonical commit")
	}
	if got := runtime.State(); got.Height != ctx.State.Height || got.Phase != consensus.PhaseFinalized {
		t.Fatal("consensus advanced despite failed canonical commit")
	}
}
