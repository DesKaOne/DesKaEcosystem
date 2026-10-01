package node

import (
	"errors"
	"reflect"
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/consensus"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/storage"
)

func buildFinalizedRuntimeForPublicationTest(t *testing.T, ctx consensus.BlockProductionContext, candidate block.Block, certificate consensus.FinalityCertificate, validatorResolver ValidatorAuthorityResolver) (*consensus.ValidatorRuntime, consensus.ValidatorSet, consensus.VotingPowerSet) {
	t.Helper()
	validators := mustValidatorSet(t, certificate)
	power := mustVotingPowerSet(t, certificate)
	runtime, err := consensus.NewValidatorRuntime(consensus.RuntimeConfig{
		Rules: consensus.ValidationRules{ProtocolVersion: ctx.State.ProtocolVersion, ChainID: ctx.State.ChainID, RequireSender: true},
		State: ctx.State, Validators: validators, VotingPower: power,
		Threshold: consensus.QuorumThreshold{Numerator: 1, Denominator: 1},
		Proposer: consensus.RoundRobinProposer{},
	})
	if err != nil { t.Fatal(err) }
	proposal, err := consensus.NewBlockProposal(ctx, candidate)
	if err != nil { t.Fatal(err) }
	if err := runtime.AcceptBlockProposal(proposal); err != nil { t.Fatal(err) }
	vote := consensus.Message{
		ProtocolVersion: ctx.State.ProtocolVersion, ChainID: ctx.State.ChainID,
		Epoch: ctx.State.Epoch, Height: ctx.State.Height, Round: ctx.State.Round,
		Sender: append([]byte(nil), certificate.Votes[0].Sender...),
		Type: consensus.MessageTypePrevote, Payload: append([]byte(nil), proposal.Payload[:]...),
	}
	if err := runtime.AddVote(vote); err != nil { t.Fatal(err) }
	precommit := vote
	precommit.Type = consensus.MessageTypePrecommit
	precommit, err = precommit.Sign(mustTestSigner(t, 23))
	if err != nil { t.Fatal(err) }
	if err := runtime.AddVote(precommit); err != nil { t.Fatal(err) }
	if _, err := runtime.FinalizeProposal(validatorResolver); err != nil { t.Fatal(err) }
	return runtime, validators, power
}

func TestCommitFinalityEvidenceAndPublishConsensusAdvancesNextHeight(t *testing.T) {
	n, ctx, candidate, certificate, validatorResolver, senderResolver, _ := finalizedBlockFixture(t, storage.NewMemoryStore())
	runtime, validators, power := buildFinalizedRuntimeForPublicationTest(t, ctx, candidate, certificate, validatorResolver)
	if err := n.CommitFinalityEvidenceAndPublishConsensus(ctx, candidate, certificate, validators, power, validatorResolver, senderResolver, runtime); err != nil {
		t.Fatal(err)
	}
	got := runtime.State()
	if got.Height != candidate.Header.Height || got.Round != 0 || got.Phase != consensus.PhaseProposal {
		t.Fatalf("unexpected next-height runtime: height=%d round=%d phase=%v", got.Height, got.Round, got.Phase)
	}
	if n.Head.Header.Height != candidate.Header.Height || n.HeadHash == candidate.Header.PreviousHash {
		t.Fatal("canonical node head did not advance")
	}
	if n.State.Root() != candidate.Header.StateRoot {
		t.Fatal("published state root does not match canonical candidate")
	}
}

func TestCommitFinalityEvidenceAndPublishConsensusDoesNotAdvanceOnStoreFailure(t *testing.T) {
	store := &failingCommitStore{MemoryStore: storage.NewMemoryStore()}
	n, ctx, candidate, certificate, validatorResolver, senderResolver, _ := finalizedBlockFixture(t, store)
	runtime, validators, power := buildFinalizedRuntimeForPublicationTest(t, ctx, candidate, certificate, validatorResolver)
	store.failCommit = true
	beforeHead, beforeHash, beforeRoot := n.Head, n.HeadHash, n.State.Root()
	err := n.CommitFinalityEvidenceAndPublishConsensus(ctx, candidate, certificate, validators, power, validatorResolver, senderResolver, runtime)
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
