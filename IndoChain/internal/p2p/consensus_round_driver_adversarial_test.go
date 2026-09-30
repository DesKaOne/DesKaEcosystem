package p2p

import (
	"errors"
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/consensus"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/crypto"
)

func TestAdversarialRoundDriverRejectsStaleRoundAcrossNodes(t *testing.T) {
	state, err := consensus.NewRoundState(1, 1001, 2, 9)
	if err != nil {
		t.Fatal(err)
	}
	validators, err := consensus.NewValidatorSet([][]byte{
		[]byte("validator-a"),
		[]byte("validator-b"),
	})
	if err != nil {
		t.Fatal(err)
	}
	power, err := consensus.NewVotingPowerSet([]consensus.ValidatorVotingPower{
		{ValidatorID: []byte("validator-a"), Power: 1},
		{ValidatorID: []byte("validator-b"), Power: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	rules := consensus.ValidationRules{
		ProtocolVersion: state.ProtocolVersion,
		ChainID: state.ChainID,
		RequireSender: true,
		RequireSignature: true,
	}
	runtimeA, err := consensus.NewValidatorRuntime(consensus.RuntimeConfig{
		Rules: rules, State: state, Validators: validators, VotingPower: power,
		Threshold: consensus.QuorumThreshold{Numerator: 2, Denominator: 3},
		Proposer: consensus.RoundRobinProposer{},
	})
	if err != nil {
		t.Fatal(err)
	}
	runtimeB, err := consensus.NewValidatorRuntime(consensus.RuntimeConfig{
		Rules: rules, State: state, Validators: validators, VotingPower: power,
		Threshold: consensus.QuorumThreshold{Numerator: 2, Denominator: 3},
		Proposer: consensus.RoundRobinProposer{},
	})
	if err != nil {
		t.Fatal(err)
	}
	transportA := NewInMemoryTransport(PeerID("node-a"), 4096)
	transportB := NewInMemoryTransport(PeerID("node-b"), 4096)
	if err := transportA.Connect(PeerID("node-b"), transportB); err != nil {
		t.Fatal(err)
	}
	if err := transportB.Connect(PeerID("node-a"), transportA); err != nil {
		t.Fatal(err)
	}

	signer, err := crypto.GenerateEd25519Signer()
	if err != nil {
		t.Fatal(err)
	}
	proposal := consensus.Message{
		ProtocolVersion: state.ProtocolVersion,
		ChainID: state.ChainID,
		Epoch: state.Epoch,
		Height: state.Height,
		Round: state.Round,
		Sender: []byte("validator-a"),
		Type: consensus.MessageTypeProposal,
		Payload: []byte("proposal-round-0"),
	}
	proposal, err = proposal.Sign(signer)
	if err != nil {
		t.Fatal(err)
	}
	if err := runtimeA.AcceptProposal(proposal); err != nil {
		t.Fatal(err)
	}
	if err := transportA.SendConsensus(PeerID("node-b"), proposal, rules); err != nil {
		t.Fatal(err)
	}
	if _, received, err := transportB.ReceiveConsensus(rules); err != nil {
		t.Fatal(err)
	} else if err := runtimeB.AcceptProposal(received); err != nil {
		t.Fatal(err)
	}

	authority := runtimeAuthorityForSigner(t, signer)
	timeoutA, err := consensus.NewTimeoutMessage(state, []byte("validator-a"), 1, signer)
	if err != nil {
		t.Fatal(err)
	}
	timeoutB, err := consensus.NewTimeoutMessage(state, []byte("validator-b"), 1, signer)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runtimeB.AdvanceRoundWithTimeoutEvidence([]consensus.Message{timeoutA, timeoutB}, authority); err != nil {
		t.Fatal(err)
	}
	if runtimeB.State().Round != 1 || runtimeB.State().Phase != consensus.PhaseProposal {
		t.Fatalf("node B state = %+v, want round 1 proposal", runtimeB.State())
	}

	if err := transportA.SendConsensus(PeerID("node-b"), proposal, rules); err != nil {
		t.Fatal(err)
	}
	_, stale, err := transportB.ReceiveConsensus(rules)
	if err != nil {
		t.Fatal(err)
	}
	if err := runtimeB.AcceptProposal(stale); err == nil {
		t.Fatal("round-0 proposal was accepted after node B advanced to round 1")
	}
	if runtimeB.State().Round != 1 || runtimeB.State().Phase != consensus.PhaseProposal {
		t.Fatalf("node B state changed after stale proposal: %+v", runtimeB.State())
	}
}

func TestAdversarialRoundDriverRejectsConflictingTimeoutLocks(t *testing.T) {
	state, err := consensus.NewRoundState(1, 1001, 2, 9)
	if err != nil {
		t.Fatal(err)
	}
	validators, err := consensus.NewValidatorSet([][]byte{
		[]byte("validator-a"),
		[]byte("validator-b"),
	})
	if err != nil {
		t.Fatal(err)
	}
	power, err := consensus.NewVotingPowerSet([]consensus.ValidatorVotingPower{
		{ValidatorID: []byte("validator-a"), Power: 1},
		{ValidatorID: []byte("validator-b"), Power: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	rules := consensus.ValidationRules{
		ProtocolVersion: state.ProtocolVersion,
		ChainID: state.ChainID,
		RequireSender: true,
		RequireSignature: true,
	}
	runtime, err := consensus.NewValidatorRuntime(consensus.RuntimeConfig{
		Rules: rules, State: state, Validators: validators, VotingPower: power,
		Threshold: consensus.QuorumThreshold{Numerator: 2, Denominator: 3},
		Proposer: consensus.RoundRobinProposer{},
	})
	if err != nil {
		t.Fatal(err)
	}
	transportA := NewInMemoryTransport(PeerID("node-a"), 4096)
	transportB := NewInMemoryTransport(PeerID("node-b"), 4096)
	if err := transportA.Connect(PeerID("node-b"), transportB); err != nil {
		t.Fatal(err)
	}
	signer, err := crypto.GenerateEd25519Signer()
	if err != nil {
		t.Fatal(err)
	}
	timeoutA, err := consensus.NewTimeoutMessageWithLock(state, []byte("validator-a"), 1, []byte("lock-A"), signer)
	if err != nil {
		t.Fatal(err)
	}
	timeoutB, err := consensus.NewTimeoutMessageWithLock(state, []byte("validator-b"), 1, []byte("lock-B"), signer)
	if err != nil {
		t.Fatal(err)
	}
	for _, msg := range []consensus.Message{timeoutA, timeoutB} {
		if err := transportA.SendConsensus(PeerID("node-b"), msg, rules); err != nil {
			t.Fatal(err)
		}
	}
	var messages []consensus.Message
	for range 2 {
		_, msg, err := transportB.ReceiveConsensus(rules)
		if err != nil {
			t.Fatal(err)
		}
		messages = append(messages, msg)
	}
	authority := runtimeAuthorityForSigner(t, signer)
	if _, err := runtime.AdvanceRoundWithTimeoutEvidence(messages, authority); !errors.Is(err, consensus.ErrConflictingTimeoutLock) {
		t.Fatalf("conflicting timeout lock error = %v, want %v", err, consensus.ErrConflictingTimeoutLock)
	}
	if runtime.State().Round != 0 || runtime.State().Phase != consensus.PhaseProposal {
		t.Fatalf("runtime state changed after conflicting locks: %+v", runtime.State())
	}
}

func TestAdversarialRoundDriverRejectsReplayedTimeoutAfterRoundChange(t *testing.T) {
	state, err := consensus.NewRoundState(1, 1001, 2, 9)
	if err != nil {
		t.Fatal(err)
	}
	validators, err := consensus.NewValidatorSet([][]byte{
		[]byte("validator-a"),
		[]byte("validator-b"),
	})
	if err != nil {
		t.Fatal(err)
	}
	power, err := consensus.NewVotingPowerSet([]consensus.ValidatorVotingPower{
		{ValidatorID: []byte("validator-a"), Power: 1},
		{ValidatorID: []byte("validator-b"), Power: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	rules := consensus.ValidationRules{
		ProtocolVersion: state.ProtocolVersion,
		ChainID: state.ChainID,
		RequireSender: true,
		RequireSignature: true,
	}
	runtime, err := consensus.NewValidatorRuntime(consensus.RuntimeConfig{
		Rules: rules, State: state, Validators: validators, VotingPower: power,
		Threshold: consensus.QuorumThreshold{Numerator: 2, Denominator: 3},
		Proposer: consensus.RoundRobinProposer{},
	})
	if err != nil {
		t.Fatal(err)
	}
	signer, err := crypto.GenerateEd25519Signer()
	if err != nil {
		t.Fatal(err)
	}
	timeoutA, err := consensus.NewTimeoutMessage(state, []byte("validator-a"), 1, signer)
	if err != nil {
		t.Fatal(err)
	}
	timeoutB, err := consensus.NewTimeoutMessage(state, []byte("validator-b"), 1, signer)
	if err != nil {
		t.Fatal(err)
	}
	authority := runtimeAuthorityForSigner(t, signer)
	if _, err := runtime.AdvanceRoundWithTimeoutEvidence([]consensus.Message{timeoutA, timeoutB}, authority); err != nil {
		t.Fatal(err)
	}
	if runtime.State().Round != 1 {
		t.Fatalf("runtime round = %d, want 1", runtime.State().Round)
	}
	if _, err := runtime.AdvanceRoundWithTimeoutEvidence([]consensus.Message{timeoutA, timeoutB}, authority); err == nil {
		t.Fatal("replayed round-0 timeout evidence was accepted at round 1")
	}
	if runtime.State().Round != 1 || runtime.State().Phase != consensus.PhaseProposal {
		t.Fatalf("runtime state changed after timeout replay: %+v", runtime.State())
	}
}
