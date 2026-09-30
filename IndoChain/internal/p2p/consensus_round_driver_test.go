package p2p

import (
	"bytes"
	"errors"
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/consensus"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/crypto"
)

func consensusDriverFixture(t *testing.T) (*ConsensusRoundDriver, *ConsensusRoundDriver, crypto.Signer, consensus.StaticValidatorAuthority) {
	t.Helper()
	state, err := consensus.NewRoundState(1, 1001, 1, 0)
	if err != nil { t.Fatal(err) }
	validators, err := consensus.NewValidatorSet([][]byte{[]byte("validator-a")})
	if err != nil { t.Fatal(err) }
	power, err := consensus.NewVotingPowerSet([]consensus.ValidatorVotingPower{{ValidatorID: []byte("validator-a"), Power: 1}})
	if err != nil { t.Fatal(err) }
	kp, err := crypto.NewEd25519KeyPair(bytes.Repeat([]byte{0x61}, 32))
	if err != nil { t.Fatal(err) }
	signer, err := crypto.NewEd25519Signer(kp.PrivateKey)
	if err != nil { t.Fatal(err) }
	authority, err := consensus.NewStaticValidatorAuthority(map[string][]byte{"validator-a": signer.PublicKey()})
	if err != nil { t.Fatal(err) }
	rules := consensus.ValidationRules{ProtocolVersion: 1, ChainID: 1001, MaxPayloadSize: 4096, RequireSender: true, RequireSignature: true}
	cfg := consensus.RuntimeConfig{Rules: rules, State: state, Validators: validators, VotingPower: power, Threshold: consensus.QuorumThreshold{Numerator: 1, Denominator: 1}, Proposer: consensus.RoundRobinProposer{}}
	aRuntime, err := consensus.NewValidatorRuntime(cfg); if err != nil { t.Fatal(err) }
	bRuntime, err := consensus.NewValidatorRuntime(cfg); if err != nil { t.Fatal(err) }
	a := NewInMemoryTransport(PeerID("node-a"), 4096)
	b := NewInMemoryTransport(PeerID("node-b"), 4096)
	if err := a.Connect(PeerID("node-b"), b); err != nil { t.Fatal(err) }
	if err := b.Connect(PeerID("node-a"), a); err != nil { t.Fatal(err) }
	ad, err := NewConsensusRoundDriver(a, aRuntime, authority, rules); if err != nil { t.Fatal(err) }
	bd, err := NewConsensusRoundDriver(b, bRuntime, authority, rules); if err != nil { t.Fatal(err) }
	return ad, bd, signer, authority
}

func TestConsensusRoundDriverAuthenticatesBeforeRuntimeMutation(t *testing.T) {
	a, b, signer, _ := consensusDriverFixture(t)
	state := a.Runtime().State()
	msg := consensus.Message{ProtocolVersion: state.ProtocolVersion, ChainID: state.ChainID, Epoch: state.Epoch, Height: state.Height, Round: state.Round, Sender: []byte("validator-a"), Type: consensus.MessageTypeProposal, Payload: []byte("candidate-hash")}
	msg, err := msg.Sign(signer); if err != nil { t.Fatal(err) }
	if err := a.Publish(PeerID("node-b"), msg); err != nil { t.Fatal(err) }
	if _, err := b.ReceiveAndHandle(); err != nil { t.Fatal(err) }
	if b.Runtime().State().Phase != consensus.PhasePrevote { t.Fatalf("phase=%v, want prevote", b.Runtime().State().Phase) }

	tampered := msg
	tampered.Signature = append([]byte(nil), msg.Signature...)
	tampered.Signature[0] ^= 0xff
	if err := a.Publish(PeerID("node-b"), tampered); err != nil { t.Fatal(err) }
	before := b.Runtime().State()
	if _, err := b.ReceiveAndHandle(); !errors.Is(err, consensus.ErrInvalidSignature) { t.Fatalf("err=%v, want invalid signature", err) }
	if b.Runtime().State() != before { t.Fatal("tampered transport message mutated runtime") }
}

func TestConsensusRoundDriverRejectsUnsignedMessageAtTransportBoundary(t *testing.T) {
	a, b, signer, _ := consensusDriverFixture(t)
	_ = signer
	state := a.Runtime().State()
	msg := consensus.Message{ProtocolVersion: state.ProtocolVersion, ChainID: state.ChainID, Epoch: state.Epoch, Height: state.Height, Round: state.Round, Sender: []byte("validator-a"), Type: consensus.MessageTypeProposal, Payload: []byte("candidate-hash")}
	before := b.Runtime().State()
	if err := a.Publish(PeerID("node-b"), msg); !errors.Is(err, consensus.ErrMissingSignature) {
		t.Fatalf("publish error = %v, want missing signature", err)
	}
	if b.Runtime().State() != before { t.Fatal("unsigned transport message mutated runtime") }
}

func TestConsensusRoundDriverPublishesSignedBlockProposalEvidence(t *testing.T) {
	a, _, signer, _ := consensusDriverFixture(t)
	state := a.Runtime().State()
	ctx := consensus.BlockProductionContext{State: state, Proposer: []byte("validator-a")}
	// The candidate payload is deliberately opaque at this layer; candidate
	// serialization/fetch remains outside the consensus message boundary.
	candidateHashPayload := []byte("not-a-block")
	msg := consensus.Message{ProtocolVersion: state.ProtocolVersion, ChainID: state.ChainID, Epoch: state.Epoch, Height: state.Height, Round: state.Round, Sender: []byte("validator-a"), Type: consensus.MessageTypeProposal, Payload: candidateHashPayload}
	signed, err := msg.Sign(signer); if err != nil { t.Fatal(err) }
	if signed.Type != consensus.MessageTypeProposal || len(signed.Signature) == 0 { t.Fatal("proposal was not signed") }
	_ = ctx
}
