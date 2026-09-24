package p2p

import (
	"bytes"
	"errors"
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/genesis/devnet"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/consensus"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/block"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/node"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/storage"
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
	keyPair, err := crypto.NewEd25519KeyPair(bytes.Repeat([]byte{0x35}, 32))
	if err != nil { t.Fatal(err) }
	signer, err := crypto.NewEd25519Signer(keyPair.PrivateKey)
	if err != nil { t.Fatal(err) }
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
		RequireSignature: true,
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
	proposalMsg, err = proposalMsg.Sign(signer)
	if err != nil { t.Fatal(err) }

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
	vote, err = vote.Sign(signer)
	if err != nil { t.Fatal(err) }
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


func TestInMemoryTransportRuntimeFinalizedBlockMultiHeight(t *testing.T) {
	store := storage.NewMemoryStore()
	n, err := node.NewDevnet(store)
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
	keyPair, err := crypto.NewEd25519KeyPair(bytes.Repeat([]byte{0x45}, 32))
	if err != nil {
		t.Fatal(err)
	}
	signer, err := crypto.NewEd25519Signer(keyPair.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	validatorResolver := runtimeValidatorAuthorityResolver{}
	senderResolver := runtimeSenderAuthorityResolver{}

	var previousHash = n.HeadHash
	for height := types.Height(1); height <= 3; height++ {
		state, err := consensus.NewRoundState(
			devnet.ProtocolVersion,
			devnet.ChainID,
			0,
			n.Head.Header.Height,
		)
		if err != nil {
			t.Fatal(err)
		}
		ctx := consensus.BlockProductionContext{
			State: state, PreviousHash: previousHash, Proposer: append([]byte(nil), validatorID...),
		}
		rules, err := n.Config.BlockRules(nil)
		if err != nil {
			t.Fatal(err)
		}
		candidate, err := consensus.BuildBlockCandidate(consensus.BlockCandidateInput{
			Context: ctx, Timestamp: n.Head.Header.Timestamp + 1, Transactions: []any{}, Rules: rules,
		}, n.State)
		if err != nil {
			t.Fatal(err)
		}
		proposal, err := consensus.NewBlockProposal(ctx, candidate)
		if err != nil {
			t.Fatal(err)
		}
		messageRules := consensus.ValidationRules{
			ProtocolVersion: state.ProtocolVersion, ChainID: state.ChainID,
			RequireSender: true, RequireSignature: true,
		}
		runtimeA, err := consensus.NewValidatorRuntime(consensus.RuntimeConfig{
			Rules: messageRules, State: state, Validators: validators, VotingPower: power,
			Threshold: consensus.QuorumThreshold{Numerator: 1, Denominator: 1},
			Proposer: consensus.RoundRobinProposer{},
		})
		if err != nil {
			t.Fatal(err)
		}
		runtimeB, err := consensus.NewValidatorRuntime(consensus.RuntimeConfig{
			Rules: messageRules, State: state, Validators: validators, VotingPower: power,
			Threshold: consensus.QuorumThreshold{Numerator: 1, Denominator: 1},
			Proposer: consensus.RoundRobinProposer{},
		})
		if err != nil {
			t.Fatal(err)
		}
		proposalMsg := consensus.Message{
			ProtocolVersion: state.ProtocolVersion, ChainID: state.ChainID,
			Epoch: state.Epoch, Height: state.Height, Round: state.Round,
			Sender: append([]byte(nil), validatorID...), Type: consensus.MessageTypeProposal,
			Payload: proposal.MessagePayload(),
		}
		proposalMsg, err = proposalMsg.Sign(signer)
		if err != nil {
			t.Fatal(err)
		}
		nodeA := NewInMemoryTransport(PeerID("node-a"), 4096)
		nodeB := NewInMemoryTransport(PeerID("node-b"), 4096)
		if err := nodeA.Connect(PeerID("node-b"), nodeB); err != nil {
			t.Fatal(err)
		}
		if err := nodeB.Connect(PeerID("node-a"), nodeA); err != nil {
			t.Fatal(err)
		}
		if err := nodeA.SendConsensus(PeerID("node-b"), proposalMsg, messageRules); err != nil {
			t.Fatal(err)
		}
		if _, received, err := nodeB.ReceiveConsensus(messageRules); err != nil {
			t.Fatal(err)
		} else if err := runtimeB.AcceptProposal(received); err != nil {
			t.Fatal(err)
		}
		vote := consensus.Message{
			ProtocolVersion: state.ProtocolVersion, ChainID: state.ChainID,
			Epoch: state.Epoch, Height: state.Height, Round: state.Round,
			Sender: append([]byte(nil), validatorID...), Type: consensus.MessageTypeVote,
			Payload: proposal.MessagePayload(),
		}
		vote, err = vote.Sign(signer)
		if err != nil {
			t.Fatal(err)
		}
		if err := runtimeB.AddVote(vote); err != nil {
			t.Fatal(err)
		}
		if err := nodeB.SendConsensus(PeerID("node-a"), vote, messageRules); err != nil {
			t.Fatal(err)
		}
		if _, received, err := nodeA.ReceiveConsensus(messageRules); err != nil {
			t.Fatal(err)
		} else if err := runtimeA.AcceptProposal(proposalMsg); err != nil {
			t.Fatal(err)
		} else if err := runtimeA.AddVote(received); err != nil {
			t.Fatal(err)
		}
		certificate, err := runtimeA.FinalizeProposal()
		if err != nil {
			t.Fatal(err)
		}
		if err := n.CommitFinalizedBlock(ctx, candidate, certificate, validators, power, validatorResolver, senderResolver); err != nil {
			t.Fatalf("height %d finalized commit: %v", height, err)
		}
		if n.Head.Header.Height != height {
			t.Fatalf("node head height = %d, want %d", n.Head.Header.Height, height)
		}
		stored, storedHash, err := store.Head()
		if err != nil {
			t.Fatal(err)
		}
		if stored.Header.Height != height || storedHash != n.HeadHash {
			t.Fatalf("height %d store head mismatch", height)
		}
		previousHash = n.HeadHash
	}
}

func TestConsensusRuntimeNegativeCrossHeightInvalidFinalityEvidence(t *testing.T) {
	n, candidate1, certificate1, validators, power, ctx1, validatorResolver, senderResolver := finalizedHandoffFixture(t)
	if err := n.CommitFinalizedBlock(ctx1, candidate1, certificate1, validators, power, validatorResolver, senderResolver); err != nil {
		t.Fatal(err)
	}
	canonicalHeight1Hash := n.HeadHash

	state2, err := consensus.NewRoundState(devnet.ProtocolVersion, devnet.ChainID, 0, n.Head.Header.Height)
	if err != nil {
		t.Fatal(err)
	}
	ctx2 := consensus.BlockProductionContext{State: state2, PreviousHash: canonicalHeight1Hash, Proposer: append([]byte(nil), candidate1.Header.Proposer...)}
	rules, err := n.Config.BlockRules(nil)
	if err != nil {
		t.Fatal(err)
	}
	candidate2, err := consensus.BuildBlockCandidate(consensus.BlockCandidateInput{Context: ctx2, Timestamp: n.Head.Header.Timestamp + 1, Transactions: []any{}, Rules: rules}, n.State)
	if err != nil {
		t.Fatal(err)
	}
	messageRules := consensus.ValidationRules{ProtocolVersion: state2.ProtocolVersion, ChainID: state2.ChainID, RequireSender: true, RequireSignature: true}
	runtime, err := consensus.NewValidatorRuntime(consensus.RuntimeConfig{Rules: messageRules, State: state2, Validators: validators, VotingPower: power, Threshold: consensus.QuorumThreshold{Numerator: 1, Denominator: 1}, Proposer: consensus.RoundRobinProposer{}})
	if err != nil {
		t.Fatal(err)
	}
	proposal, err := consensus.NewBlockProposal(ctx2, candidate2)
	if err != nil {
		t.Fatal(err)
	}
	keyPair, err := crypto.NewEd25519KeyPair(bytes.Repeat([]byte{0x58}, 32))
	if err != nil {
		t.Fatal(err)
	}
	signer, err := crypto.NewEd25519Signer(keyPair.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	proposalMsg := consensus.Message{ProtocolVersion: state2.ProtocolVersion, ChainID: state2.ChainID, Epoch: state2.Epoch, Height: state2.Height, Round: state2.Round, Sender: append([]byte(nil), candidate2.Header.Proposer...), Type: consensus.MessageTypeProposal, Payload: proposal.MessagePayload()}
	proposalMsg, err = proposalMsg.Sign(signer)
	if err != nil {
		t.Fatal(err)
	}
	if err := runtime.AcceptProposal(proposalMsg); err != nil {
		t.Fatal(err)
	}
	vote := consensus.Message{ProtocolVersion: state2.ProtocolVersion, ChainID: state2.ChainID, Epoch: state2.Epoch, Height: state2.Height, Round: state2.Round, Sender: append([]byte(nil), candidate2.Header.Proposer...), Type: consensus.MessageTypeVote, Payload: proposal.MessagePayload()}
	vote, err = vote.Sign(signer)
	if err != nil {
		t.Fatal(err)
	}
	if err := runtime.AddVote(vote); err != nil {
		t.Fatal(err)
	}
	certificate2, err := runtime.FinalizeProposal()
	if err != nil {
		t.Fatal(err)
	}
	certificate2.Height = certificate2.Height - 1
	if err := n.CommitFinalizedBlock(ctx2, candidate2, certificate2, validators, power, validatorResolver, senderResolver); !errors.Is(err, consensus.ErrStateContextMismatch) {
		t.Fatalf("cross-height finality context error = %v, want %v", err, consensus.ErrStateContextMismatch)
	}
	if n.Head.Header.Height != 1 || n.HeadHash != canonicalHeight1Hash {
		t.Fatal("canonical head changed after cross-height finality evidence rejection")
	}
}

func TestConsensusRuntimeNegativeCrossHeightVoteContextMismatch(t *testing.T) {
	n, candidate1, certificate1, validators, power, ctx1, validatorResolver, senderResolver := finalizedHandoffFixture(t)
	if err := n.CommitFinalizedBlock(ctx1, candidate1, certificate1, validators, power, validatorResolver, senderResolver); err != nil {
		t.Fatal(err)
	}
	canonicalHeight1Hash := n.HeadHash

	state2, err := consensus.NewRoundState(devnet.ProtocolVersion, devnet.ChainID, 0, n.Head.Header.Height)
	if err != nil {
		t.Fatal(err)
	}
	ctx2 := consensus.BlockProductionContext{
		State: state2,
		PreviousHash: n.HeadHash,
		Proposer: append([]byte(nil), candidate1.Header.Proposer...),
	}
	rules, err := n.Config.BlockRules(nil)
	if err != nil {
		t.Fatal(err)
	}
	candidate2, err := consensus.BuildBlockCandidate(consensus.BlockCandidateInput{
		Context: ctx2,
		Timestamp: n.Head.Header.Timestamp + 1,
		Transactions: []any{},
		Rules: rules,
	}, n.State)
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := consensus.NewValidatorRuntime(consensus.RuntimeConfig{
		Rules: consensus.ValidationRules{
			ProtocolVersion: state2.ProtocolVersion,
			ChainID: state2.ChainID,
			RequireSender: true,
			RequireSignature: true,
		},
		State: state2,
		Validators: validators,
		VotingPower: power,
		Threshold: consensus.QuorumThreshold{Numerator: 1, Denominator: 1},
		Proposer: consensus.RoundRobinProposer{},
	})
	if err != nil {
		t.Fatal(err)
	}
	proposal, err := consensus.NewBlockProposal(ctx2, candidate2)
	if err != nil {
		t.Fatal(err)
	}
	keyPair, err := crypto.NewEd25519KeyPair(bytes.Repeat([]byte{0x59}, 32))
	if err != nil {
		t.Fatal(err)
	}
	signer, err := crypto.NewEd25519Signer(keyPair.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	proposalMsg := consensus.Message{
		ProtocolVersion: state2.ProtocolVersion,
		ChainID: state2.ChainID,
		Epoch: state2.Epoch,
		Height: state2.Height,
		Round: state2.Round,
		Sender: append([]byte(nil), candidate2.Header.Proposer...),
		Type: consensus.MessageTypeProposal,
		Payload: proposal.MessagePayload(),
	}
	proposalMsg, err = proposalMsg.Sign(signer)
	if err != nil {
		t.Fatal(err)
	}
	if err := runtime.AcceptProposal(proposalMsg); err != nil {
		t.Fatal(err)
	}
	vote := consensus.Message{
		ProtocolVersion: state2.ProtocolVersion,
		ChainID: state2.ChainID,
		Epoch: state2.Epoch,
		Height: state2.Height,
		Round: state2.Round,
		Sender: append([]byte(nil), candidate2.Header.Proposer...),
		Type: consensus.MessageTypeVote,
		Payload: proposal.MessagePayload(),
	}
	vote, err = vote.Sign(signer)
	if err != nil {
		t.Fatal(err)
	}
	if err := runtime.AddVote(vote); err != nil {
		t.Fatal(err)
	}
	certificate2, err := runtime.FinalizeProposal()
	if err != nil {
		t.Fatal(err)
	}
	certificate2.Votes[0].Height--

	if err := n.CommitFinalizedBlock(ctx2, candidate2, certificate2, validators, power, validatorResolver, senderResolver); !errors.Is(err, consensus.ErrConsensusMessageContextMismatch) {
		t.Fatalf("cross-height vote context error = %v, want %v", err, consensus.ErrConsensusMessageContextMismatch)
	}
	if n.Head.Header.Height != 1 || n.HeadHash != canonicalHeight1Hash {
		t.Fatal("canonical head changed after cross-height vote context rejection")
	}

	// Restore the vote context so the next mutation isolates certificate-level context.
	certificate2.Votes[0].Height = state2.Height
	certificate2.Round++
	if err := n.CommitFinalizedBlock(ctx2, candidate2, certificate2, validators, power, validatorResolver, senderResolver); !errors.Is(err, consensus.ErrStateContextMismatch) {
		t.Fatalf("cross-height certificate round context error = %v, want %v", err, consensus.ErrStateContextMismatch)
	}
	if n.Head.Header.Height != 1 || n.HeadHash != canonicalHeight1Hash {
		t.Fatal("canonical head changed after cross-height certificate round rejection")
	}

	certificate2.Round = state2.Round
	certificate2.Epoch++
	if err := n.CommitFinalizedBlock(ctx2, candidate2, certificate2, validators, power, validatorResolver, senderResolver); !errors.Is(err, consensus.ErrStateContextMismatch) {
		t.Fatalf("cross-height certificate epoch context error = %v, want %v", err, consensus.ErrStateContextMismatch)
	}
	if n.Head.Header.Height != 1 || n.HeadHash != canonicalHeight1Hash {
		t.Fatal("canonical head changed after cross-height certificate epoch rejection")
	}

	certificate2.Epoch = state2.Epoch
	certificate2.ProtocolVersion++
	if err := n.CommitFinalizedBlock(ctx2, candidate2, certificate2, validators, power, validatorResolver, senderResolver); !errors.Is(err, consensus.ErrStateContextMismatch) {
		t.Fatalf("cross-height certificate protocol version context error = %v, want %v", err, consensus.ErrStateContextMismatch)
	}
	if n.Head.Header.Height != 1 || n.HeadHash != canonicalHeight1Hash {
		t.Fatal("canonical head changed after cross-height certificate protocol version rejection")
	}

	certificate2.ProtocolVersion = state2.ProtocolVersion
	certificate2.ChainID++
	if err := n.CommitFinalizedBlock(ctx2, candidate2, certificate2, validators, power, validatorResolver, senderResolver); !errors.Is(err, consensus.ErrStateContextMismatch) {
		t.Fatalf("cross-height certificate chain ID context error = %v, want %v", err, consensus.ErrStateContextMismatch)
	}
	if n.Head.Header.Height != 1 || n.HeadHash != canonicalHeight1Hash {
		t.Fatal("canonical head changed after cross-height certificate chain ID rejection")
	}

	certificate2.ChainID = state2.ChainID
	certificate2.Height++
	if err := n.CommitFinalizedBlock(ctx2, candidate2, certificate2, validators, power, validatorResolver, senderResolver); !errors.Is(err, consensus.ErrStateContextMismatch) {
		t.Fatalf("cross-height certificate height context error = %v, want %v", err, consensus.ErrStateContextMismatch)
	}
	if n.Head.Header.Height != 1 || n.HeadHash != canonicalHeight1Hash {
		t.Fatal("canonical head changed after cross-height certificate height rejection")
	}

	certificate2.Height = state2.Height
	certificate2.Threshold = consensus.QuorumThreshold{Numerator: 2, Denominator: 1}
	if err := n.CommitFinalizedBlock(ctx2, candidate2, certificate2, validators, power, validatorResolver, senderResolver); !errors.Is(err, consensus.ErrInvalidQuorumThreshold) {
		t.Fatalf("cross-height certificate invalid threshold error = %v, want %v", err, consensus.ErrInvalidQuorumThreshold)
	}
	if n.Head.Header.Height != 1 || n.HeadHash != canonicalHeight1Hash {
		t.Fatal("canonical head changed after invalid certificate threshold rejection")
	}
}

func TestConsensusRuntimeNegativeInvalidFinalityCertificateStructure(t *testing.T) {
	n, candidate, certificate, validators, power, ctx, validatorResolver, senderResolver := finalizedHandoffFixture(t)
	canonicalHead := n.HeadHash

	certificate.Payload = nil
	if err := n.CommitFinalizedBlock(ctx, candidate, certificate, validators, power, validatorResolver, senderResolver); !errors.Is(err, consensus.ErrInvalidFinalityCertificate) {
		t.Fatalf("empty certificate payload error = %v, want %v", err, consensus.ErrInvalidFinalityCertificate)
	}
	if n.Head.Header.Height != 0 || n.HeadHash != canonicalHead {
		t.Fatal("canonical head changed after empty certificate payload rejection")
	}

	certificate.Payload = []byte("restored-finality-payload")
	certificate.Votes = nil
	if err := n.CommitFinalizedBlock(ctx, candidate, certificate, validators, power, validatorResolver, senderResolver); !errors.Is(err, consensus.ErrInvalidFinalityCertificate) {
		t.Fatalf("empty certificate votes error = %v, want %v", err, consensus.ErrInvalidFinalityCertificate)
	}
	if n.Head.Header.Height != 0 || n.HeadHash != canonicalHead {
		t.Fatal("canonical head changed after empty certificate votes rejection")
	}
}

func TestConsensusRuntimeNegativeFinalityQuorumNotReached(t *testing.T) {
	n, candidate, certificate, validators, power, ctx, validatorResolver, senderResolver := finalizedHandoffFixture(t)
	canonicalHead := n.HeadHash

	// Keep the certificate structurally valid but make one vote insufficient:
	// validator-a has 1/2 of total voting power while the threshold is 2/3.
	validators, err := consensus.NewValidatorSet([][]byte{[]byte("validator-a"), []byte("validator-b")})
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
	certificate.Threshold = consensus.QuorumThreshold{Numerator: 2, Denominator: 3}
	if err := n.CommitFinalizedBlock(ctx, candidate, certificate, validators, power, validatorResolver, senderResolver); !errors.Is(err, consensus.ErrFinalityQuorumNotReached) {
		t.Fatalf("unreached finality quorum error = %v, want %v", err, consensus.ErrFinalityQuorumNotReached)
	}
	if n.Head.Header.Height != 0 || n.HeadHash != canonicalHead {
		t.Fatal("canonical head changed after unreached finality quorum rejection")
	}
}

func TestConsensusRuntimeNegativeCrossHeightReplayedCandidate(t *testing.T) {
	n, candidate1, certificate1, validators, power, ctx1, validatorResolver, senderResolver := finalizedHandoffFixture(t)
	if err := n.CommitFinalizedBlock(ctx1, candidate1, certificate1, validators, power, validatorResolver, senderResolver); err != nil {
		t.Fatal(err)
	}
	canonicalHeight1Hash := n.HeadHash

	state2, err := consensus.NewRoundState(devnet.ProtocolVersion, devnet.ChainID, 0, n.Head.Header.Height)
	if err != nil {
		t.Fatal(err)
	}
	ctx2 := consensus.BlockProductionContext{
		State:        state2,
		PreviousHash: canonicalHeight1Hash,
		Proposer:     append([]byte(nil), candidate1.Header.Proposer...),
	}
	rules, err := n.Config.BlockRules(nil)
	if err != nil {
		t.Fatal(err)
	}
	candidate2, err := consensus.BuildBlockCandidate(consensus.BlockCandidateInput{
		Context: ctx2,
		Timestamp: n.Head.Header.Timestamp + 1,
		Transactions: []any{},
		Rules: rules,
	}, n.State)
	if err != nil {
		t.Fatal(err)
	}

	consensusRules := consensus.ValidationRules{
		ProtocolVersion: state2.ProtocolVersion,
		ChainID:         state2.ChainID,
		RequireSender:   true,
		RequireSignature: true,
	}
	runtime, err := consensus.NewValidatorRuntime(consensus.RuntimeConfig{
		Rules: consensusRules,
		State: state2,
		Validators: validators,
		VotingPower: power,
		Threshold: consensus.QuorumThreshold{Numerator: 1, Denominator: 1},
		Proposer: consensus.RoundRobinProposer{},
	})
	if err != nil {
		t.Fatal(err)
	}
	proposal, err := consensus.NewBlockProposal(ctx2, candidate2)
	if err != nil {
		t.Fatal(err)
	}
	keyPair, err := crypto.NewEd25519KeyPair(bytes.Repeat([]byte{0x57}, 32))
	if err != nil {
		t.Fatal(err)
	}
	signer, err := crypto.NewEd25519Signer(keyPair.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	proposalMsg := consensus.Message{
		ProtocolVersion: state2.ProtocolVersion,
		ChainID: state2.ChainID,
		Epoch: state2.Epoch,
		Height: state2.Height,
		Round: state2.Round,
		Sender: append([]byte(nil), candidate2.Header.Proposer...),
		Type: consensus.MessageTypeProposal,
		Payload: proposal.MessagePayload(),
	}
	proposalMsg, err = proposalMsg.Sign(signer)
	if err != nil {
		t.Fatal(err)
	}
	if err := runtime.AcceptProposal(proposalMsg); err != nil {
		t.Fatal(err)
	}
	vote := consensus.Message{
		ProtocolVersion: state2.ProtocolVersion,
		ChainID:         state2.ChainID,
		Epoch:           state2.Epoch,
		Height:          state2.Height,
		Round:           state2.Round,
		Sender:          append([]byte(nil), candidate2.Header.Proposer...),
		Type:            consensus.MessageTypeVote,
		Payload:         proposal.MessagePayload(),
	}
	vote, err = vote.Sign(signer)
	if err != nil {
		t.Fatal(err)
	}
	if err := runtime.AddVote(vote); err != nil {
		t.Fatal(err)
	}
	certificate2, err := runtime.FinalizeProposal()
	if err != nil {
		t.Fatal(err)
	}
	if err := n.CommitFinalizedBlock(ctx2, candidate2, certificate2, validators, power, validatorResolver, senderResolver); err != nil {
		t.Fatal(err)
	}
	canonicalHeight2Hash := n.HeadHash

	if err := n.CommitFinalizedBlock(ctx1, candidate1, certificate1, validators, power, validatorResolver, senderResolver); !errors.Is(err, node.ErrFinalizedBlockAlreadyCommitted) {
		t.Fatalf("cross-height replay error = %v, want %v", err, node.ErrFinalizedBlockAlreadyCommitted)
	}
	if n.Head.Header.Height != 2 || n.HeadHash != canonicalHeight2Hash {
		t.Fatal("canonical head changed after lower-height replay rejection")
	}
}

func TestConsensusRuntimeNegativeCrossHeightDifferentCandidate(t *testing.T) {
	n, candidate1, certificate1, validators, power, ctx1, validatorResolver, senderResolver := finalizedHandoffFixture(t)
	if err := n.CommitFinalizedBlock(ctx1, candidate1, certificate1, validators, power, validatorResolver, senderResolver); err != nil {
		t.Fatal(err)
	}
	canonicalHeight1Hash := n.HeadHash

	state2, err := consensus.NewRoundState(devnet.ProtocolVersion, devnet.ChainID, 0, n.Head.Header.Height)
	if err != nil {
		t.Fatal(err)
	}
	ctx2 := consensus.BlockProductionContext{
		State:        state2,
		PreviousHash: canonicalHeight1Hash,
		Proposer:     append([]byte(nil), candidate1.Header.Proposer...),
	}
	rules, err := n.Config.BlockRules(nil)
	if err != nil {
		t.Fatal(err)
	}
	candidate2, err := consensus.BuildBlockCandidate(consensus.BlockCandidateInput{
		Context: ctx2,
		Timestamp: n.Head.Header.Timestamp + 1,
		Transactions: []any{},
		Rules: rules,
	}, n.State)
	if err != nil {
		t.Fatal(err)
	}
	consensusRules := consensus.ValidationRules{
		ProtocolVersion: state2.ProtocolVersion,
		ChainID:         state2.ChainID,
		RequireSender:   true,
		RequireSignature: true,
	}
	runtime2, err := consensus.NewValidatorRuntime(consensus.RuntimeConfig{
		Rules: consensusRules,
		State: state2,
		Validators: validators,
		VotingPower: power,
		Threshold: consensus.QuorumThreshold{Numerator: 1, Denominator: 1},
		Proposer: consensus.RoundRobinProposer{},
	})
	if err != nil {
		t.Fatal(err)
	}
	proposal2, err := consensus.NewBlockProposal(ctx2, candidate2)
	if err != nil {
		t.Fatal(err)
	}
	keyPair, err := crypto.NewEd25519KeyPair(bytes.Repeat([]byte{0x58}, 32))
	if err != nil {
		t.Fatal(err)
	}
	signer, err := crypto.NewEd25519Signer(keyPair.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	proposalMsg2 := consensus.Message{
		ProtocolVersion: state2.ProtocolVersion,
		ChainID: state2.ChainID,
		Epoch: state2.Epoch,
		Height: state2.Height,
		Round: state2.Round,
		Sender: append([]byte(nil), candidate2.Header.Proposer...),
		Type: consensus.MessageTypeProposal,
		Payload: proposal2.MessagePayload(),
	}
	proposalMsg2, err = proposalMsg2.Sign(signer)
	if err != nil {
		t.Fatal(err)
	}
	if err := runtime2.AcceptProposal(proposalMsg2); err != nil {
		t.Fatal(err)
	}
	vote2 := consensus.Message{
		ProtocolVersion: state2.ProtocolVersion,
		ChainID: state2.ChainID,
		Epoch: state2.Epoch,
		Height: state2.Height,
		Round: state2.Round,
		Sender: append([]byte(nil), candidate2.Header.Proposer...),
		Type: consensus.MessageTypeVote,
		Payload: proposal2.MessagePayload(),
	}
	vote2, err = vote2.Sign(signer)
	if err != nil {
		t.Fatal(err)
	}
	if err := runtime2.AddVote(vote2); err != nil {
		t.Fatal(err)
	}
	certificate2, err := runtime2.FinalizeProposal()
	if err != nil {
		t.Fatal(err)
	}
	if err := n.CommitFinalizedBlock(ctx2, candidate2, certificate2, validators, power, validatorResolver, senderResolver); err != nil {
		t.Fatal(err)
	}
	canonicalHeight2Hash := n.HeadHash

	// Build a different, otherwise valid height-1 candidate against the same
	// genesis context. It must not be mistaken for an exact canonical replay.
	alternateCtx := ctx1
	alternateCandidate, err := consensus.BuildBlockCandidate(consensus.BlockCandidateInput{
		Context: alternateCtx,
		Timestamp: candidate1.Header.Timestamp + 1,
		Transactions: []any{},
		Rules: rules,
	}, n.State)
	if err != nil {
		t.Fatal(err)
	}
	alternateProposal, err := consensus.NewBlockProposal(alternateCtx, alternateCandidate)
	if err != nil {
		t.Fatal(err)
	}
	alternateRuntime, err := consensus.NewValidatorRuntime(consensus.RuntimeConfig{
		Rules: consensusRules,
		State: ctx1.State,
		Validators: validators,
		VotingPower: power,
		Threshold: consensus.QuorumThreshold{Numerator: 1, Denominator: 1},
		Proposer: consensus.RoundRobinProposer{},
	})
	if err != nil {
		t.Fatal(err)
	}
	alternateProposalMsg := consensus.Message{
		ProtocolVersion: ctx1.State.ProtocolVersion,
		ChainID: ctx1.State.ChainID,
		Epoch: ctx1.State.Epoch,
		Height: ctx1.State.Height,
		Round: ctx1.State.Round,
		Sender: append([]byte(nil), alternateCandidate.Header.Proposer...),
		Type: consensus.MessageTypeProposal,
		Payload: alternateProposal.MessagePayload(),
	}
	alternateProposalMsg, err = alternateProposalMsg.Sign(signer)
	if err != nil {
		t.Fatal(err)
	}
	if err := alternateRuntime.AcceptProposal(alternateProposalMsg); err != nil {
		t.Fatal(err)
	}
	alternateVote := consensus.Message{
		ProtocolVersion: ctx1.State.ProtocolVersion,
		ChainID: ctx1.State.ChainID,
		Epoch: ctx1.State.Epoch,
		Height: ctx1.State.Height,
		Round: ctx1.State.Round,
		Sender: append([]byte(nil), alternateCandidate.Header.Proposer...),
		Type: consensus.MessageTypeVote,
		Payload: alternateProposal.MessagePayload(),
	}
	alternateVote, err = alternateVote.Sign(signer)
	if err != nil {
		t.Fatal(err)
	}
	if err := alternateRuntime.AddVote(alternateVote); err != nil {
		t.Fatal(err)
	}
	alternateCertificate, err := alternateRuntime.FinalizeProposal()
	if err != nil {
		t.Fatal(err)
	}

	alternateHash, err := block.Hash(alternateCandidate)
	if err != nil {
		t.Fatal(err)
	}
	if alternateHash == canonicalHeight1Hash {
		t.Fatal("alternate candidate unexpectedly matched canonical height-1 hash")
	}
	if err := n.CommitFinalizedBlock(ctx1, alternateCandidate, alternateCertificate, validators, power, validatorResolver, senderResolver); !errors.Is(err, node.ErrConsensusContextMismatch) {
		t.Fatalf("different lower-height candidate error = %v, want %v", err, node.ErrConsensusContextMismatch)
	}
	if n.Head.Header.Height != 2 || n.HeadHash != canonicalHeight2Hash {
		t.Fatal("canonical head changed after different lower-height candidate rejection")
	}
}

func TestConsensusRuntimeNegativeCrossHeightFutureCandidateStalePreviousHash(t *testing.T) {
	n, candidate1, certificate1, validators, power, ctx1, validatorResolver, senderResolver := finalizedHandoffFixture(t)
	if err := n.CommitFinalizedBlock(ctx1, candidate1, certificate1, validators, power, validatorResolver, senderResolver); err != nil {
		t.Fatal(err)
	}
	canonicalHeight1Hash := n.HeadHash

	state2, err := consensus.NewRoundState(devnet.ProtocolVersion, devnet.ChainID, 0, n.Head.Header.Height)
	if err != nil {
		t.Fatal(err)
	}
	ctx2 := consensus.BlockProductionContext{
		State:        state2,
		PreviousHash: canonicalHeight1Hash,
		Proposer:     append([]byte(nil), candidate1.Header.Proposer...),
	}
	rules, err := n.Config.BlockRules(nil)
	if err != nil {
		t.Fatal(err)
	}
	candidate2, err := consensus.BuildBlockCandidate(consensus.BlockCandidateInput{
		Context: ctx2,
		Timestamp: n.Head.Header.Timestamp + 1,
		Transactions: []any{},
		Rules: rules,
	}, n.State)
	if err != nil {
		t.Fatal(err)
	}
	messageRules := consensus.ValidationRules{
		ProtocolVersion: state2.ProtocolVersion,
		ChainID: state2.ChainID,
		RequireSender: true,
		RequireSignature: true,
	}
	runtime, err := consensus.NewValidatorRuntime(consensus.RuntimeConfig{
		Rules: messageRules,
		State: state2,
		Validators: validators,
		VotingPower: power,
		Threshold: consensus.QuorumThreshold{Numerator: 1, Denominator: 1},
		Proposer: consensus.RoundRobinProposer{},
	})
	if err != nil {
		t.Fatal(err)
	}
	proposal, err := consensus.NewBlockProposal(ctx2, candidate2)
	if err != nil {
		t.Fatal(err)
	}
	keyPair, err := crypto.NewEd25519KeyPair(bytes.Repeat([]byte{0x57}, 32))
	if err != nil {
		t.Fatal(err)
	}
	signer, err := crypto.NewEd25519Signer(keyPair.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	proposalMsg := consensus.Message{
		ProtocolVersion: state2.ProtocolVersion,
		ChainID: state2.ChainID,
		Epoch: state2.Epoch,
		Height: state2.Height,
		Round: state2.Round,
		Sender: append([]byte(nil), candidate2.Header.Proposer...),
		Type: consensus.MessageTypeProposal,
		Payload: proposal.MessagePayload(),
	}
	proposalMsg, err = proposalMsg.Sign(signer)
	if err != nil {
		t.Fatal(err)
	}
	if err := runtime.AcceptProposal(proposalMsg); err != nil {
		t.Fatal(err)
	}
	vote := consensus.Message{
		ProtocolVersion: state2.ProtocolVersion,
		ChainID: state2.ChainID,
		Epoch: state2.Epoch,
		Height: state2.Height,
		Round: state2.Round,
		Sender: append([]byte(nil), candidate2.Header.Proposer...),
		Type: consensus.MessageTypeVote,
		Payload: proposal.MessagePayload(),
	}
	vote, err = vote.Sign(signer)
	if err != nil {
		t.Fatal(err)
	}
	if err := runtime.AddVote(vote); err != nil {
		t.Fatal(err)
	}
	certificate2, err := runtime.FinalizeProposal()
	if err != nil {
		t.Fatal(err)
	}
	if err := n.CommitFinalizedBlock(ctx2, candidate2, certificate2, validators, power, validatorResolver, senderResolver); err != nil {
		t.Fatal(err)
	}
	canonicalHeight2Hash := n.HeadHash

	state3, err := consensus.NewRoundState(devnet.ProtocolVersion, devnet.ChainID, 0, n.Head.Header.Height)
	if err != nil {
		t.Fatal(err)
	}
	staleFutureCtx := consensus.BlockProductionContext{
		State:        state3,
		PreviousHash: canonicalHeight1Hash,
		Proposer:     append([]byte(nil), candidate1.Header.Proposer...),
	}
	candidate3, err := consensus.BuildBlockCandidate(consensus.BlockCandidateInput{
		Context: staleFutureCtx,
		Timestamp: n.Head.Header.Timestamp + 1,
		Transactions: []any{},
		Rules: rules,
	}, n.State)
	if err != nil {
		t.Fatal(err)
	}
	proposal3, err := consensus.NewBlockProposal(staleFutureCtx, candidate3)
	if err != nil {
		t.Fatal(err)
	}
	runtime3, err := consensus.NewValidatorRuntime(consensus.RuntimeConfig{
		Rules: messageRules,
		State: state3,
		Validators: validators,
		VotingPower: power,
		Threshold: consensus.QuorumThreshold{Numerator: 1, Denominator: 1},
		Proposer: consensus.RoundRobinProposer{},
	})
	if err != nil {
		t.Fatal(err)
	}
	proposalMsg3 := consensus.Message{
		ProtocolVersion: state3.ProtocolVersion,
		ChainID: state3.ChainID,
		Epoch: state3.Epoch,
		Height: state3.Height,
		Round: state3.Round,
		Sender: append([]byte(nil), candidate3.Header.Proposer...),
		Type: consensus.MessageTypeProposal,
		Payload: proposal3.MessagePayload(),
	}
	proposalMsg3, err = proposalMsg3.Sign(signer)
	if err != nil {
		t.Fatal(err)
	}
	if err := runtime3.AcceptProposal(proposalMsg3); err != nil {
		t.Fatal(err)
	}
	vote3 := consensus.Message{
		ProtocolVersion: state3.ProtocolVersion,
		ChainID: state3.ChainID,
		Epoch: state3.Epoch,
		Height: state3.Height,
		Round: state3.Round,
		Sender: append([]byte(nil), candidate3.Header.Proposer...),
		Type: consensus.MessageTypeVote,
		Payload: proposal3.MessagePayload(),
	}
	vote3, err = vote3.Sign(signer)
	if err != nil {
		t.Fatal(err)
	}
	if err := runtime3.AddVote(vote3); err != nil {
		t.Fatal(err)
	}
	certificate3, err := runtime3.FinalizeProposal()
	if err != nil {
		t.Fatal(err)
	}
	if err := n.CommitFinalizedBlock(staleFutureCtx, candidate3, certificate3, validators, power, validatorResolver, senderResolver); !errors.Is(err, node.ErrConsensusContextMismatch) {
		t.Fatalf("future candidate with stale previous hash error = %v, want %v", err, node.ErrConsensusContextMismatch)
	}
	if n.Head.Header.Height != 2 || n.HeadHash != canonicalHeight2Hash {
		t.Fatal("canonical head changed after future candidate stale-context rejection")
	}
}

func TestConsensusRuntimeNegativeCrossHeightStaleContext(t *testing.T) {
	n, candidate1, certificate1, validators, power, ctx1, validatorResolver, senderResolver := finalizedHandoffFixture(t)
	if err := n.CommitFinalizedBlock(ctx1, candidate1, certificate1, validators, power, validatorResolver, senderResolver); err != nil {
		t.Fatal(err)
	}
	canonicalHeight1Hash := n.HeadHash

	state2, err := consensus.NewRoundState(devnet.ProtocolVersion, devnet.ChainID, 0, n.Head.Header.Height)
	if err != nil {
		t.Fatal(err)
	}
	ctx2 := consensus.BlockProductionContext{
		State:        state2,
		PreviousHash: canonicalHeight1Hash,
		Proposer:     append([]byte(nil), candidate1.Header.Proposer...),
	}
	rules, err := n.Config.BlockRules(nil)
	if err != nil {
		t.Fatal(err)
	}
	candidate2, err := consensus.BuildBlockCandidate(consensus.BlockCandidateInput{
		Context: ctx2,
		Timestamp: n.Head.Header.Timestamp + 1,
		Transactions: []any{},
		Rules: rules,
	}, n.State)
	if err != nil {
		t.Fatal(err)
	}

	messageRules := consensus.ValidationRules{
		ProtocolVersion: state2.ProtocolVersion,
		ChainID:         state2.ChainID,
		RequireSender:   true,
		RequireSignature: true,
	}
	runtime, err := consensus.NewValidatorRuntime(consensus.RuntimeConfig{
		Rules: messageRules,
		State: state2,
		Validators: validators,
		VotingPower: power,
		Threshold: consensus.QuorumThreshold{Numerator: 1, Denominator: 1},
		Proposer: consensus.RoundRobinProposer{},
	})
	if err != nil {
		t.Fatal(err)
	}
	proposal, err := consensus.NewBlockProposal(ctx2, candidate2)
	if err != nil {
		t.Fatal(err)
	}
	keyPair, err := crypto.NewEd25519KeyPair(bytes.Repeat([]byte{0x56}, 32))
	if err != nil {
		t.Fatal(err)
	}
	signer, err := crypto.NewEd25519Signer(keyPair.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	proposalMsg := consensus.Message{
		ProtocolVersion: state2.ProtocolVersion,
		ChainID: state2.ChainID,
		Epoch: state2.Epoch,
		Height: state2.Height,
		Round: state2.Round,
		Sender: append([]byte(nil), candidate2.Header.Proposer...),
		Type: consensus.MessageTypeProposal,
		Payload: proposal.MessagePayload(),
	}
	proposalMsg, err = proposalMsg.Sign(signer)
	if err != nil {
		t.Fatal(err)
	}
	if err := runtime.AcceptProposal(proposalMsg); err != nil {
		t.Fatal(err)
	}
	vote := consensus.Message{
		ProtocolVersion: state2.ProtocolVersion,
		ChainID: state2.ChainID,
		Epoch: state2.Epoch,
		Height: state2.Height,
		Round: state2.Round,
		Sender: append([]byte(nil), candidate2.Header.Proposer...),
		Type: consensus.MessageTypeVote,
		Payload: proposal.MessagePayload(),
	}
	vote, err = vote.Sign(signer)
	if err != nil {
		t.Fatal(err)
	}
	if err := runtime.AddVote(vote); err != nil {
		t.Fatal(err)
	}
	certificate2, err := runtime.FinalizeProposal()
	if err != nil {
		t.Fatal(err)
	}

	staleContext := ctx2
	staleContext.State = ctx1.State
	staleContext.PreviousHash = ctx1.PreviousHash
	if err := n.CommitFinalizedBlock(staleContext, candidate2, certificate2, validators, power, validatorResolver, senderResolver); !errors.Is(err, node.ErrConsensusContextMismatch) {
		t.Fatalf("cross-height stale context error = %v, want %v", err, node.ErrConsensusContextMismatch)
	}
	if n.Head.Header.Height != 1 || n.HeadHash != canonicalHeight1Hash {
		t.Fatal("canonical head changed after cross-height stale-context rejection")
	}
}

func TestConsensusRuntimeNegativeMismatchedProposalPayload(t *testing.T) {
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
	runtime, err := consensus.NewValidatorRuntime(consensus.RuntimeConfig{
		Rules: consensus.ValidationRules{ProtocolVersion: state.ProtocolVersion, ChainID: state.ChainID, RequireSender: true},
		State: state, Validators: validators, VotingPower: power,
		Threshold: consensus.QuorumThreshold{Numerator: 1, Denominator: 1},
		Proposer: consensus.RoundRobinProposer{},
	})
	if err != nil {
		t.Fatal(err)
	}

	store := storage.NewMemoryStore()
	n, err := node.NewDevnet(store)
	if err != nil {
		t.Fatal(err)
	}
	ctx := consensus.BlockProductionContext{
		State: state, PreviousHash: n.HeadHash, Proposer: append([]byte(nil), validatorID...),
	}
	rules, err := n.Config.BlockRules(nil)
	if err != nil {
		t.Fatal(err)
	}
	candidate, err := consensus.BuildBlockCandidate(consensus.BlockCandidateInput{
		Context: ctx, Timestamp: n.Head.Header.Timestamp + 1, Transactions: []any{},
		Rules: rules,
	}, n.State)
	if err != nil {
		t.Fatal(err)
	}
	proposal, err := consensus.NewBlockProposal(ctx, candidate)
	if err != nil {
		t.Fatal(err)
	}
	proposal.Candidate.Header.Timestamp++
	if err := runtime.AcceptBlockProposal(proposal); err == nil {
		t.Fatal("mismatched proposal payload was accepted")
	}
	if runtime.State().Phase != consensus.PhaseProposal {
		t.Fatalf("runtime phase = %v, want proposal after rejected payload", runtime.State().Phase)
	}
}

func TestConsensusRuntimeNegativeStaleFinalizedContext(t *testing.T) {
	n, candidate, certificate, validators, power, ctx, validatorResolver, senderResolver := finalizedHandoffFixture(t)
	stale := ctx
	stale.PreviousHash = types.Hash{}
	if err := n.CommitFinalizedBlock(stale, candidate, certificate, validators, power, validatorResolver, senderResolver); !errors.Is(err, node.ErrConsensusContextMismatch) {
		t.Fatalf("stale context error = %v, want %v", err, node.ErrConsensusContextMismatch)
	}
	if n.Head.Header.Height != 0 {
		t.Fatalf("node head height = %d, want 0 after rejected stale context", n.Head.Header.Height)
	}
}

func TestConsensusRuntimeNegativeInvalidFinalityEvidence(t *testing.T) {
	n, candidate, certificate, validators, power, ctx, validatorResolver, senderResolver := finalizedHandoffFixture(t)
	certificate.Payload = []byte("tampered-finality-payload")
	if err := n.CommitFinalizedBlock(ctx, candidate, certificate, validators, power, validatorResolver, senderResolver); err == nil {
		t.Fatal("tampered finality evidence was accepted")
	}
	if n.Head.Header.Height != 0 {
		t.Fatalf("node head height = %d, want 0 after rejected finality evidence", n.Head.Header.Height)
	}
}

func TestConsensusRuntimeNegativeReplayedFinalizedBlock(t *testing.T) {
	n, candidate, certificate, validators, power, ctx, validatorResolver, senderResolver := finalizedHandoffFixture(t)
	if err := n.CommitFinalizedBlock(ctx, candidate, certificate, validators, power, validatorResolver, senderResolver); err != nil {
		t.Fatal(err)
	}
	if err := n.CommitFinalizedBlock(ctx, candidate, certificate, validators, power, validatorResolver, senderResolver); !errors.Is(err, node.ErrFinalizedBlockAlreadyCommitted) {
		t.Fatalf("replayed finalized block error = %v, want %v", err, node.ErrFinalizedBlockAlreadyCommitted)
	}
	if n.Head.Header.Height != 1 {
		t.Fatalf("node head height = %d, want 1 after replay rejection", n.Head.Header.Height)
	}
}

func finalizedHandoffFixture(t *testing.T) (*node.Node, block.Block, consensus.FinalityCertificate, consensus.ValidatorSet, consensus.VotingPowerSet, consensus.BlockProductionContext, node.ValidatorAuthorityResolver, node.TransactionAuthorityResolver) {
	t.Helper()
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
	ctx := consensus.BlockProductionContext{State: state, PreviousHash: n.HeadHash, Proposer: append([]byte(nil), validatorID...)}
	rules, err := n.Config.BlockRules(nil)
	if err != nil {
		t.Fatal(err)
	}
	candidate, err := consensus.BuildBlockCandidate(consensus.BlockCandidateInput{
		Context: ctx, Timestamp: n.Head.Header.Timestamp + 1, Transactions: []any{},
		Rules: rules,
	}, n.State)
	if err != nil {
		t.Fatal(err)
	}
	keyPair, err := crypto.NewEd25519KeyPair(bytes.Repeat([]byte{0x35}, 32))
	if err != nil {
		t.Fatal(err)
	}
	signer, err := crypto.NewEd25519Signer(keyPair.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	consensusRules := consensus.ValidationRules{
		ProtocolVersion: state.ProtocolVersion, ChainID: state.ChainID,
		RequireSender: true, RequireSignature: true,
	}
	runtime, err := consensus.NewValidatorRuntime(consensus.RuntimeConfig{
		Rules: consensusRules, State: state, Validators: validators, VotingPower: power,
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
	proposalMsg := consensus.Message{
		ProtocolVersion: state.ProtocolVersion, ChainID: state.ChainID,
		Epoch: state.Epoch, Height: state.Height, Round: state.Round,
		Sender: append([]byte(nil), validatorID...), Type: consensus.MessageTypeProposal,
		Payload: proposal.MessagePayload(),
	}
	proposalMsg, err = proposalMsg.Sign(signer)
	if err != nil {
		t.Fatal(err)
	}
	if err := runtime.AcceptProposal(proposalMsg); err != nil {
		t.Fatal(err)
	}
	vote := consensus.Message{
		ProtocolVersion: state.ProtocolVersion, ChainID: state.ChainID,
		Epoch: state.Epoch, Height: state.Height, Round: state.Round,
		Sender: append([]byte(nil), validatorID...), Type: consensus.MessageTypeVote,
		Payload: proposal.MessagePayload(),
	}
	vote, err = vote.Sign(signer)
	if err != nil {
		t.Fatal(err)
	}
	if err := runtime.AddVote(vote); err != nil {
		t.Fatal(err)
	}
	certificate, err := runtime.FinalizeProposal()
	if err != nil {
		t.Fatal(err)
	}
	return n, candidate, certificate, validators, power, ctx, runtimeValidatorAuthorityResolver{}, runtimeSenderAuthorityResolver{}
}
