package consensus

import (
	"bytes"
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/block"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
)

func TestRoundRobinProposerIsDeterministicAcrossRounds(t *testing.T) {
	validators, err := NewValidatorSet([][]byte{[]byte("validator-a"), []byte("validator-b")})
	if err != nil { t.Fatal(err) }
	selector := RoundRobinProposer{}
	state, err := NewRoundState(1, 1001, 7, 0)
	if err != nil { t.Fatal(err) }

	p0, err := selector.Proposer(state, validators)
	if err != nil { t.Fatal(err) }
	if !bytes.Equal(p0, []byte("validator-a")) { t.Fatalf("round 0 proposer=%q", p0) }

	state1 := state
	state1.Round = 1
	p1, err := selector.Proposer(state1, validators)
	if err != nil { t.Fatal(err) }
	if !bytes.Equal(p1, []byte("validator-b")) { t.Fatalf("round 1 proposer=%q", p1) }

	state2 := state
	state2.Round = 2
	p2, err := selector.Proposer(state2, validators)
	if err != nil { t.Fatal(err) }
	if !bytes.Equal(p2, []byte("validator-a")) { t.Fatalf("round 2 proposer=%q", p2) }
}

func TestValidateProducedBlockRejectsConsensusContextMismatch(t *testing.T) {
	state, err := NewRoundState(1, 1001, 7, 2)
	if err != nil { t.Fatal(err) }
	ctx := BlockProductionContext{
		State: state,
		PreviousHash: types.Hash{1, 2, 3},
		Proposer: []byte("validator-a"),
	}
	candidate := block.Block{
		Header: block.Header{
			Version: state.ProtocolVersion,
			ChainID: state.ChainID,
			Height: state.Height + 1,
			PreviousHash: ctx.PreviousHash,
			Proposer: append([]byte(nil), ctx.Proposer...),
		},
	}
	if _, err := ValidateProducedBlock(ctx, candidate); err != nil {
		t.Fatal(err)
	}

	for name, mutate := range map[string]func(*block.Block){
		"height": func(b *block.Block) { b.Header.Height++ },
		"previous-hash": func(b *block.Block) { b.Header.PreviousHash[0] ^= 0xff },
		"proposer": func(b *block.Block) { b.Header.Proposer = []byte("validator-b") },
	} {
		t.Run(name, func(t *testing.T) {
			bad := candidate
			bad.Header.Proposer = append([]byte(nil), candidate.Header.Proposer...)
			bad.Header.PreviousHash = candidate.Header.PreviousHash
			mutate(&bad)
			if _, err := ValidateProducedBlock(ctx, bad); err == nil {
				t.Fatalf("expected %s mismatch rejection", name)
			}
		})
	}
}

func TestValidatorRuntimeRoundChangePreservesOnlyLockState(t *testing.T) {
	runtime, state, _, _ := runtimeFixture(t)
	proposal := []byte("round-0")
	if err := runtime.AcceptProposal(runtimeMessage(state, "validator-a", MessageTypeProposal, string(proposal))); err != nil {
		t.Fatal(err)
	}
	if err := runtime.AddVote(runtimeMessage(state, "validator-a", MessageTypePrevote, string(proposal))); err != nil {
		t.Fatal(err)
	}
	if err := runtime.AddVote(runtimeMessage(state, "validator-b", MessageTypePrevote, string(proposal))); err != nil {
		t.Fatal(err)
	}
	if len(runtime.Proposal()) == 0 || len(runtime.PrecommitVotes()) != 0 {
		// Precommit evidence is intentionally round-local and must not leak from
		// a prevote quorum before explicit precommit messages arrive.
	}
	if err := runtime.AdvanceRound(state.Round + 1); err != nil {
		t.Fatal(err)
	}
	if runtime.State().Round != state.Round+1 || runtime.State().Phase != PhaseProposal {
		t.Fatalf("unexpected round state: round=%d phase=%v", runtime.State().Round, runtime.State().Phase)
	}
	if len(runtime.Proposal()) != 0 {
		t.Fatal("round-local proposal leaked across round change")
	}
	if len(runtime.PrecommitVotes()) != 0 {
		t.Fatal("round-local precommit evidence leaked across round change")
	}
	if err := runtime.AcceptProposal(runtimeMessage(runtime.State(), "validator-b", MessageTypeProposal, string(proposal))); err != nil {
		t.Fatalf("preserved lock rejected matching proposal after round change: %v", err)
	}
	if err := runtime.AdvanceRound(state.Round + 2); err != nil {
		t.Fatal(err)
	}
	if err := runtime.AcceptProposal(runtimeMessage(runtime.State(), "validator-a", MessageTypeProposal, "conflicting-round-2")); err == nil {
		t.Fatal("preserved lock failed to reject conflicting proposal")
	}
}

func TestRuntimeDoesNotActAsProductionNetworkRoundDriver(t *testing.T) {
	runtime, _, _, _ := runtimeFixture(t)
	if runtime == nil || runtime.proposer == nil {
		t.Fatal("runtime must retain an explicit proposer policy boundary")
	}
	// The runtime exposes deterministic state transitions and evidence
	// validation; network scheduling and message delivery remain outside this
	// package. This test deliberately documents the boundary rather than
	// manufacturing a production scheduler.
}
