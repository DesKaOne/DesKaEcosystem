package p2p

import (
	"bytes"
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/consensus"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/crypto"
)

func TestInMemoryTransportConsensusRuntimeIntegration(t *testing.T) {
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
	votingPower, err := consensus.NewVotingPowerSet([]consensus.ValidatorVotingPower{
		{ValidatorID: []byte("validator-a"), Power: 1},
		{ValidatorID: []byte("validator-b"), Power: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	config := consensus.RuntimeConfig{
		Rules: consensus.ValidationRules{
			ProtocolVersion: state.ProtocolVersion,
			ChainID:         state.ChainID,
			RequireSender:   true,
		},
		State:       state,
		Validators:  validators,
		VotingPower: votingPower,
		Threshold:   consensus.QuorumThreshold{Numerator: 2, Denominator: 3},
		Proposer:    consensus.RoundRobinProposer{},
	}

	nodeARuntime, err := consensus.NewValidatorRuntime(config)
	if err != nil {
		t.Fatal(err)
	}
	nodeBRuntime, err := consensus.NewValidatorRuntime(config)
	if err != nil {
		t.Fatal(err)
	}

	nodeA := NewInMemoryTransport(PeerID("node-a"), 4096)
	nodeB := NewInMemoryTransport(PeerID("node-b"), 4096)

	keyPair, err := crypto.NewEd25519KeyPair(bytes.Repeat([]byte{0x24}, 32))
	if err != nil {
		t.Fatal(err)
	}
	signer, err := crypto.NewEd25519Signer(keyPair.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	if err := nodeA.Connect(PeerID("node-b"), nodeB); err != nil {
		t.Fatal(err)
	}
	if err := nodeB.Connect(PeerID("node-a"), nodeA); err != nil {
		t.Fatal(err)
	}

	proposal := consensus.Message{
		ProtocolVersion: state.ProtocolVersion,
		ChainID:         state.ChainID,
		Epoch:           state.Epoch,
		Height:          state.Height,
		Round:           state.Round,
		Sender:          []byte("validator-a"),
		Type:            consensus.MessageTypeProposal,
		Payload:         []byte("deterministic-block-9"),
	}

	proposal, err = proposal.Sign(signer)
	if err != nil {
		t.Fatal(err)
	}
	if err := nodeARuntime.AcceptProposal(proposal); err != nil {
		t.Fatalf("node A local proposal processing: %v", err)
	}
	if err := nodeA.SendConsensus(PeerID("node-b"), proposal, config.Rules); err != nil {
		t.Fatal(err)
	}

	from, receivedProposal, err := nodeB.ReceiveConsensus(config.Rules)
	if err != nil {
		t.Fatal(err)
	}
	if from != PeerID("node-a") {
		t.Fatalf("proposal transport sender = %q, want node-a", from)
	}
	if err := nodeBRuntime.AcceptProposal(receivedProposal); err != nil {
		t.Fatalf("node B proposal processing: %v", err)
	}
	if nodeBRuntime.State().Phase != consensus.PhasePrevote {
		t.Fatalf("node B phase = %v, want prevote", nodeBRuntime.State().Phase)
	}

	voteA := consensus.Message{
		ProtocolVersion: state.ProtocolVersion,
		ChainID:         state.ChainID,
		Epoch:           state.Epoch,
		Height:          state.Height,
		Round:           state.Round,
		Sender:          []byte("validator-a"),
		Type:            consensus.MessageTypeVote,
		Payload:         append([]byte(nil), proposal.Payload...),
	}
	if err := nodeARuntime.AddVote(voteA); err != nil {
		t.Fatalf("node A local vote processing: %v", err)
	}

	voteB := consensus.Message{
		ProtocolVersion: state.ProtocolVersion,
		ChainID:         state.ChainID,
		Epoch:           state.Epoch,
		Height:          state.Height,
		Round:           state.Round,
		Sender:          []byte("validator-b"),
		Type:            consensus.MessageTypeVote,
		Payload:         append([]byte(nil), proposal.Payload...),
	}
	voteB, err = voteB.Sign(signer)
	if err != nil {
		t.Fatal(err)
	}
	if err := nodeBRuntime.AddVote(voteB); err != nil {
		t.Fatalf("node B local vote processing: %v", err)
	}
	if err := nodeB.SendConsensus(PeerID("node-a"), voteB, config.Rules); err != nil {
		t.Fatal(err)
	}

	from, receivedVote, err := nodeA.ReceiveConsensus(config.Rules)
	if err != nil {
		t.Fatal(err)
	}
	if from != PeerID("node-b") {
		t.Fatalf("vote transport sender = %q, want node-b", from)
	}
	if !bytes.Equal(receivedVote.Payload, proposal.Payload) {
		t.Fatalf("vote payload = %q, want proposal payload %q", receivedVote.Payload, proposal.Payload)
	}
	if err := nodeARuntime.AddVote(receivedVote); err != nil {
		t.Fatalf("node A remote vote processing: %v", err)
	}

	if nodeARuntime.State().Phase != consensus.PhasePrecommit {
		t.Fatalf("node A phase = %v, want precommit", nodeARuntime.State().Phase)
	}
	if nodeBRuntime.State().Phase != consensus.PhasePrevote {
		t.Fatalf("node B phase = %v, want prevote", nodeBRuntime.State().Phase)
	}

	certificate, err := nodeARuntime.FinalizeProposal()
	if err != nil {
		t.Fatalf("node A finalization: %v", err)
	}
	if nodeARuntime.State().Phase != consensus.PhaseFinalized {
		t.Fatalf("node A phase = %v, want finalized", nodeARuntime.State().Phase)
	}
	if !bytes.Equal(certificate.Payload, proposal.Payload) {
		t.Fatalf("certificate payload = %q, want %q", certificate.Payload, proposal.Payload)
	}
	if len(certificate.Votes) != 2 {
		t.Fatalf("certificate votes = %d, want 2", len(certificate.Votes))
	}
}
