package p2p

import (
	"bytes"
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/consensus"
)

func TestConsensusRoundDriverPublishesAndAcceptsFinalityEvidence(t *testing.T) {
	a, b, signer, authority := consensusDriverFixture(t)
	state := a.Runtime().State()

	proposal := consensus.Message{
		ProtocolVersion: state.ProtocolVersion,
		ChainID: state.ChainID,
		Epoch: state.Epoch,
		Height: state.Height,
		Round: state.Round,
		Sender: []byte("validator-a"),
		Type: consensus.MessageTypeProposal,
		Payload: []byte("block-8"),
	}
	proposal, err := proposal.Sign(signer)
	if err != nil { t.Fatal(err) }
	if err := a.Publish(PeerID("node-b"), proposal); err != nil { t.Fatal(err) }
	if err := a.Runtime().AcceptAuthenticatedProposal(proposal, authority); err != nil { t.Fatal(err) }
	if _, err := b.ReceiveAndHandle(); err != nil { t.Fatal(err) }

	prevote, err := consensus.BuildSignedVoteMessage(
		state, []byte("validator-a"), consensus.MessageTypePrevote, proposal.Payload, signer,
	)
	if err != nil { t.Fatal(err) }
	if err := a.Publish(PeerID("node-b"), prevote); err != nil { t.Fatal(err) }
	precommit, err := a.PublishAuthenticatedPrevoteAndMaybePrecommit(
		PeerID("node-b"), prevote, []byte("validator-a"), signer,
	)
	if err != nil { t.Fatal(err) }
	if precommit.Type != consensus.MessageTypePrecommit { t.Fatalf("type=%d, want precommit", precommit.Type) }

	// Deliver prevote then precommit to B so both runtimes have the same lock/finality context.
	if _, err := b.ReceiveAndHandle(); err != nil { t.Fatal(err) }
	if _, err := b.ReceiveAndHandle(); err != nil { t.Fatal(err) }

	before := a.Runtime().State()
	msg, certificate, err := a.PublishFinalityEvidence(PeerID("node-b"), []byte("validator-a"), signer)
	if err != nil { t.Fatal(err) }
	if !bytes.Equal(certificate.Payload, proposal.Payload) { t.Fatal("unexpected certificate payload") }
	if a.Runtime().State() != before { t.Fatal("publishing evidence mutated local phase") }

	_, received, err := ReceiveConsensus(b.transport, b.rules)
	if err != nil { t.Fatal(err) }
	if received.Type != consensus.MessageTypeFinalityEvidence { t.Fatalf("type=%d, want finality evidence", received.Type) }

	accepted, err := b.AcceptFinalityEvidence(received)
	if err != nil { t.Fatal(err) }
	if !bytes.Equal(accepted.Payload, certificate.Payload) { t.Fatal("accepted evidence payload mismatch") }
	if b.Runtime().State().Phase != consensus.PhasePrecommit { t.Fatalf("phase=%v, want precommit", b.Runtime().State().Phase) }

	// Keep authority referenced to ensure the fixture remains an authenticated path.
	_ = authority
	_ = msg
}
