package p2p

import (
	"bytes"
	"errors"
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/consensus"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/crypto"
)

type adversarialNode struct {
	id        PeerID
	transport *InMemoryTransport
	runtime   *consensus.ValidatorRuntime
}

func TestAdversarialRoundDriverStaleRoundMessageRejectedAcrossNodes(t *testing.T) {
	state, validators, power, rules, authority, signerA, _ := adversarialRuntimeFixture(t, 0)
	nodeA, nodeB := adversarialNodes(t, state, validators, power, rules)
	proposal := signedAdversarialProposal(t, state, []byte("validator-a"), []byte("block-r0"), signerA)

	if err := nodeA.runtime.AcceptProposal(proposal); err != nil {
		t.Fatal(err)
	}
	if err := nodeA.transport.SendConsensus(nodeB.id, proposal, rules); err != nil {
		t.Fatal(err)
	}
	if _, received, err := nodeB.transport.ReceiveConsensus(rules); err != nil {
		t.Fatal(err)
	} else if err := nodeB.runtime.AcceptProposal(received); err != nil {
		t.Fatal(err)
	}

	timeoutA, err := consensus.NewTimeoutMessage(state, []byte("validator-a"), 1, signerA)
	if err != nil {
		t.Fatal(err)
	}
	_, _, _, _, _, signerB := adversarialRuntimeFixture(t, 0)
	timeoutB, err := consensus.NewTimeoutMessage(state, []byte("validator-b"), 1, signerB)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := nodeB.runtime.AdvanceRoundWithTimeoutEvidence([]consensus.Message{timeoutA, timeoutB}, authority); err != nil {
		t.Fatal(err)
	}
	if nodeB.runtime.State().Round != 1 || nodeB.runtime.State().Phase != consensus.PhaseProposal {
		t.Fatalf("node B state = %+v, want round 1 proposal", nodeB.runtime.State())
	}

	if err := nodeA.transport.SendConsensus(nodeB.id, proposal, rules); err != nil {
		t.Fatal(err)
	}
	_, stale, err := nodeB.transport.ReceiveConsensus(rules)
	if err != nil {
		t.Fatal(err)
	}
	if err := nodeB.runtime.AcceptProposal(stale); err == nil {
		t.Fatal("stale round-0 proposal was accepted after round change")
	}
	if nodeB.runtime.State().Round != 1 || nodeB.runtime.State().Phase != consensus.PhaseProposal {
		t.Fatalf("node B state changed after stale proposal: %+v", nodeB.runtime.State())
	}
}

func TestAdversarialTimeoutLockConflictRejectedAcrossNodes(t *testing.T) {
	state, validators, power, rules, authority, signerA, signerB := adversarialRuntimeFixture(t, 0)
	nodeA, nodeB := adversarialNodes(t, state, validators, power, rules)

	timeoutA, err := consensus.NewTimeoutMessageWithLock(state, []byte("validator-a"), 1, []byte("lock-A"), signerA)
	if err != nil {
		t.Fatal(err)
	}
	timeoutB, err := consensus.NewTimeoutMessageWithLock(state, []byte("validator-b"), 1, []byte("lock-B"), signerB)
	if err != nil {
		t.Fatal(err)
	}
	for _, msg := range []consensus.Message{timeoutA, timeoutB} {
		if err := nodeA.transport.SendConsensus(nodeB.id, msg, rules); err != nil {
			t.Fatal(err)
		}
	}
	var messages []consensus.Message
	for range 2 {
		_, msg, err := nodeB.transport.ReceiveConsensus(rules)
		if err != nil {
			t.Fatal(err)
		}
		messages = append(messages, msg)
	}
	if _, err := nodeB.runtime.AdvanceRoundWithTimeoutEvidence(messages, authority); !errors.Is(err, consensus.ErrConflictingTimeoutLock) {
		t.Fatalf("conflicting timeout locks error = %v, want %v", err, consensus.ErrConflictingTimeoutLock)
	}
	if nodeB.runtime.State().Round != 0 || nodeB.runtime.State().Phase != consensus.PhaseProposal {
		t.Fatalf("node B state mutated after conflicting timeout locks: %+v", nodeB.runtime.State())
	}
}

func TestAdversarialTimeoutReplayRejectedAfterRoundAdvance(t *testing.T) {
	state, validators, power, rules, authority, signerA, signerB := adversarialRuntimeFixture(t, 0)
	nodeA, nodeB := adversarialNodes(t, state, validators, power, rules)

	timeoutA, err := consensus.NewTimeoutMessage(state, []byte("validator-a"), 1, signerA)
	if err != nil {
		t.Fatal(err)
	}
	timeoutB, err := consensus.NewTimeoutMessage(state, []byte("validator-b"), 1, signerB)
	if err != nil {
		t.Fatal(err)
	}
	for _, msg := range []consensus.Message{timeoutA, timeoutB} {
		if err := nodeA.transport.SendConsensus(nodeB.id, msg, rules); err != nil {
			t.Fatal(err)
		}
	}
	var firstBatch []consensus.Message
	for range 2 {
		_, msg, err := nodeB.transport.ReceiveConsensus(rules)
		if err != nil {
			t.Fatal(err)
		}
		firstBatch = append(firstBatch, msg)
	}
	if _, err := nodeB.runtime.AdvanceRoundWithTimeoutEvidence(firstBatch, authority); err != nil {
		t.Fatal(err)
	}

	for _, msg := range []consensus.Message{timeoutA, timeoutB} {
		if err := nodeA.transport.SendConsensus(nodeB.id, msg, rules); err != nil {
			t.Fatal(err)
		}
	}
	var replayBatch []consensus.Message
	for range 2 {
		_, msg, err := nodeB.transport.ReceiveConsensus(rules)
		if err != nil {
			t.Fatal(err)
		}
		replayBatch = append(replayBatch, msg)
	}
	if _, err := nodeB.runtime.AdvanceRoundWithTimeoutEvidence(replayBatch, authority); err == nil {
		t.Fatal("replayed round-0 timeout certificate was accepted at round 1")
	}
	if nodeB.runtime.State().Round != 1 || nodeB.runtime.State().Phase != consensus.PhaseProposal {
		t.Fatalf("node B state changed after timeout replay: %+v", nodeB.runtime.State())
	}
}

func TestAdversarialCrossHeightConsensusMessageRejected(t *testing.T) {
	state0, validators, power, rules0, _, signerA, _ := adversarialRuntimeFixture(t, 0)
	state1, _, _, rules1, _, _, _ := adversarialRuntimeFixture(t, 1)
	nodeA, nodeB := adversarialNodes(t, state1, validators, power, rules1)

	msg := signedAdversarialProposal(t, state0, []byte("validator-a"), []byte("height-0"), signerA)
	if err := nodeA.transport.SendConsensus(nodeB.id, msg, rules0); err != nil {
		t.Fatal(err)
	}
	_, received, err := nodeB.transport.ReceiveConsensus(rules1)
	if err != nil {
		t.Fatal(err)
	}
	if err := nodeB.runtime.AcceptProposal(received); err == nil {
		t.Fatal("height-0 proposal was accepted by height-1 runtime")
	}
	if nodeB.runtime.State().Height != 1 || nodeB.runtime.State().Phase != consensus.PhaseProposal {
		t.Fatalf("node B state changed after cross-height message: %+v", nodeB.runtime.State())
	}
}

func adversarialNodes(
	t *testing.T,
	state consensus.RoundState,
	validators consensus.ValidatorSet,
	power consensus.VotingPowerSet,
	rules consensus.ValidationRules,
) (*adversarialNode, *adversarialNode) {
	t.Helper()
	newRuntime := func() *consensus.ValidatorRuntime {
		runtime, err := consensus.NewValidatorRuntime(consensus.RuntimeConfig{
			Rules: rules, State: state, Validators: validators, VotingPower: power,
			Threshold: consensus.QuorumThreshold{Numerator: 2, Denominator: 3},
			Proposer: consensus.RoundRobinProposer{},
		})
		if err != nil {
			t.Fatal(err)
		}
		return runtime
	}
	a := &adversarialNode{id: PeerID("node-a"), transport: NewInMemoryTransport(PeerID("node-a"), 4096), runtime: newRuntime()}
	b := &adversarialNode{id: PeerID("node-b"), transport: NewInMemoryTransport(PeerID("node-b"), 4096), runtime: newRuntime()}
	if err := a.transport.Connect(b.id, b.transport); err != nil {
		t.Fatal(err)
	}
	if err := b.transport.Connect(a.id, a.transport); err != nil {
		t.Fatal(err)
	}
	return a, b
}

func adversarialRuntimeFixture(
	t *testing.T,
	height uint64,
) (
	consensus.RoundState,
	consensus.ValidatorSet,
	consensus.VotingPowerSet,
	consensus.ValidationRules,
	consensus.StaticValidatorAuthority,
	*crypto.Ed25519Signer,
	*crypto.Ed25519Signer,
) {
	t.Helper()
	state, err := consensus.NewRoundState(1, 1001, 1, height)
	if err != nil {
		t.Fatal(err)
	}
	ids := [][]byte{[]byte("validator-a"), []byte("validator-b")}
	validators, err := consensus.NewValidatorSet(ids)
	if err != nil {
		t.Fatal(err)
	}
	power, err := consensus.NewVotingPowerSet([]consensus.ValidatorVotingPower{
		{ValidatorID: append([]byte(nil), ids[0]...), Power: 1},
		{ValidatorID: append([]byte(nil), ids[1]...), Power: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	keyA, err := crypto.NewEd25519KeyPair(bytes.Repeat([]byte{0x61}, 32))
	if err != nil {
		t.Fatal(err)
	}
	keyB, err := crypto.NewEd25519KeyPair(bytes.Repeat([]byte{0x62}, 32))
	if err != nil {
		t.Fatal(err)
	}
	signerA, err := crypto.NewEd25519Signer(keyA.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	signerB, err := crypto.NewEd25519Signer(keyB.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	authority, err := consensus.NewStaticValidatorAuthority(map[string][]byte{
		string(ids[0]): signerA.PublicKey(),
		string(ids[1]): signerB.PublicKey(),
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
	return state, validators, power, rules, authority, signerA, signerB
}

func signedAdversarialProposal(
	t *testing.T,
	state consensus.RoundState,
	sender []byte,
	payload []byte,
	signer *crypto.Ed25519Signer,
) consensus.Message {
	t.Helper()
	msg := consensus.Message{
		ProtocolVersion: state.ProtocolVersion,
		ChainID: state.ChainID,
		Epoch: state.Epoch,
		Height: state.Height,
		Round: state.Round,
		Sender: append([]byte(nil), sender...),
		Type: consensus.MessageTypeProposal,
		Payload: append([]byte(nil), payload...),
	}
	signed, err := msg.Sign(signer)
	if err != nil {
		t.Fatal(err)
	}
	return signed
}
