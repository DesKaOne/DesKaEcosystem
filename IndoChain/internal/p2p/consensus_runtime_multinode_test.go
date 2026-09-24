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


type runtimeValidatorAuthorityResolver struct{}
func (runtimeValidatorAuthorityResolver) PublicKeyForValidator([]byte) ([]byte, error) {
	return []byte("validator-public-key"), nil
}

type runtimeSenderAuthorityResolver struct{}
func (runtimeSenderAuthorityResolver) PublicKeyForSender([]byte) ([]byte, error) {
	return nil, nil
}

func TestInMemoryTransportRuntimeFinalizedBlockHandoff(t *testing.T) {
	store := storage.NewMemoryStore()
	n, err := node.NewDevnet(store)
	if err != nil {
		t.Fatal(err)
	}

	state, err := consensus.NewRoundState(devnet.ProtocolVersion, devnet.ChainID, 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	validatorID := []byte("validator-a")
	validators, err := consensus.NewValidatorSet([][]byte{validatorID})
	if err != nil {
		t.Fatal(err)
	}
	power, err := consensus.NewVotingPowerSet([]consensus.ValidatorVotingPower{{ValidatorID: validatorID, Power: 1}})
	if err != nil {
		t.Fatal(err)
	}
	rules := consensus.ValidationRules{
		ProtocolVersion: state.ProtocolVersion,
		ChainID: state.ChainID,
		RequireSender: true,
	}
	runtimeA, err := consensus.NewValidatorRuntime(consensus.RuntimeConfig{
		Rules: rules, State: state, Validators: validators, VotingPower: power,
		Threshold: consensus.QuorumThreshold{Numerator: 1, Denominator: 1},
		Proposer: consensus.RoundRobinProposer{},
	})
	if err != nil {
		t.Fatal(err)
	}
	runtimeB, err := consensus.NewValidatorRuntime(consensus.RuntimeConfig{
		Rules: rules, State: state, Validators: validators, VotingPower: power,
		Threshold: consensus.QuorumThreshold{Numerator: 1, Denominator: 1},
		Proposer: consensus.RoundRobinProposer{},
	})
	if err != nil {
		t.Fatal(err)
	}

	ctx := consensus.BlockProductionContext{
		State: state,
		PreviousHash: n.HeadHash,
		Proposer: append([]byte(nil), validatorID...),
	}
	txsRoot, err := block.TransactionsRoot([]any{})
	if err != nil {
		t.Fatal(err)
	}
	candidate := block.Block{
		Header: block.Header{
			Version: devnet.ProtocolVersion,
			ChainID: devnet.ChainID,
			Height: 1,
			Timestamp: n.Head.Header.Timestamp + 1,
			PreviousHash: n.HeadHash,
			TransactionsRoot: txsRoot,
			StateRoot: n.State.Root(),
			Proposer: append([]byte(nil), validatorID...),
		},
		Transactions: []any{},
	}
	proposal, err := consensus.NewBlockProposal(ctx, candidate)
	if err != nil {
		t.Fatal(err)
	}
	proposalMsg := consensus.Message{
		ProtocolVersion: state.ProtocolVersion,
		ChainID: state.ChainID,
		Epoch: state.Epoch,
		Height: state.Height,
		Round: state.Round,
		Sender: append([]byte(nil), validatorID...),
		Type: consensus.MessageTypeProposal,
		Payload: proposal.MessagePayload(),
	}

	nodeA := NewInMemoryTransport(PeerID("node-a"), 4096)
	nodeB := NewInMemoryTransport(PeerID("node-b"), 4096)
	if err := nodeA.Connect(PeerID("node-b"), nodeB); err != nil {
		t.Fatal(err)
	}
	if err := nodeB.Connect(PeerID("node-a"), nodeA); err != nil {
		t.Fatal(err)
	}
	if err := nodeA.SendConsensus(PeerID("node-b"), proposalMsg, rules); err != nil {
		t.Fatal(err)
	}
	from, received, err := nodeB.ReceiveConsensus(rules)
	if err != nil {
		t.Fatal(err)
	}
	if from != PeerID("node-a") {
		t.Fatalf("proposal sender = %q, want node-a", from)
	}
	if err := runtimeB.AcceptProposal(received); err != nil {
		t.Fatal(err)
	}

	vote := consensus.Message{
		ProtocolVersion: state.ProtocolVersion,
		ChainID: state.ChainID,
		Epoch: state.Epoch,
		Height: state.Height,
		Round: state.Round,
		Sender: append([]byte(nil), validatorID...),
		Type: consensus.MessageTypeVote,
		Payload: proposal.MessagePayload(),
	}
	if err := runtimeB.AddVote(vote); err != nil {
		t.Fatal(err)
	}
	if err := nodeB.SendConsensus(PeerID("node-a"), vote, rules); err != nil {
		t.Fatal(err)
	}
	from, received, err = nodeA.ReceiveConsensus(rules)
	if err != nil {
		t.Fatal(err)
	}
	if from != PeerID("node-b") {
		t.Fatalf("vote sender = %q, want node-b", from)
	}
	if err := runtimeA.AcceptProposal(proposalMsg); err != nil {
		t.Fatal(err)
	}
	if err := runtimeA.AddVote(received); err != nil {
		t.Fatal(err)
	}
	certificate, err := runtimeA.FinalizeProposal()
	if err != nil {
		t.Fatal(err)
	}
	if runtimeA.State().Phase != consensus.PhaseFinalized {
		t.Fatal("runtime A did not reach finalized phase")
	}

	validatorResolver := runtimeValidatorAuthorityResolver{}
	senderResolver := runtimeSenderAuthorityResolver{}
	if err := n.CommitFinalizedBlock(ctx, candidate, certificate, validators, power, validatorResolver, senderResolver); err != nil {
		t.Fatal(err)
	}
	if n.Head.Header.Height != 1 {
		t.Fatalf("node head height = %d, want 1", n.Head.Header.Height)
	}
	if n.HeadHash == (types.Hash{}) {
		t.Fatal("node head hash is zero after finalized handoff")
	}
	stored, storedHash, err := store.Head()
	if err != nil {
		t.Fatal(err)
	}
	if stored.Header.Height != 1 || storedHash != n.HeadHash {
		t.Fatal("canonical store head does not match finalized handoff")
	}
}
