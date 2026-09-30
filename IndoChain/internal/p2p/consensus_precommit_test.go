package p2p

import (
	"bytes"
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/consensus"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/crypto"
)

func TestPublishAuthenticatedPrevoteAndMaybePrecommitEmitsLocalPrecommitAfterQuorum(t *testing.T) {
	driver, _, signer, _ := consensusDriverFixture(t)
	state := driver.Runtime().State()

	proposal := consensus.Message{
		ProtocolVersion: state.ProtocolVersion,
		ChainID: state.ChainID,
		Epoch: state.Epoch,
		Height: state.Height,
		Round: state.Round,
		Sender: []byte("validator-a"),
		Type: consensus.MessageTypeProposal,
		Payload: []byte("proposal-hash"),
	}
	proposal, err := proposal.Sign(signer)
	if err != nil {
		t.Fatal(err)
	}
	if err := driver.Runtime().AcceptAuthenticatedProposal(proposal, driver.RoundDriver().Authority()); err != nil {
		t.Fatal(err)
	}

	prevote, err := consensus.BuildSignedVoteMessage(
		state,
		[]byte("validator-a"),
		consensus.MessageTypePrevote,
		proposal.Payload,
		signer,
	)
	if err != nil {
		t.Fatal(err)
	}

	precommit, err := driver.PublishAuthenticatedPrevoteAndMaybePrecommit(
		PeerID("node-b"),
		prevote,
		[]byte("validator-a"),
		signer,
	)
	if err != nil {
		t.Fatal(err)
	}
	if precommit.Type != consensus.MessageTypePrecommit {
		t.Fatalf("precommit type=%d, want %d", precommit.Type, consensus.MessageTypePrecommit)
	}
	if !bytes.Equal(precommit.Sender, []byte("validator-a")) {
		t.Fatalf("precommit sender=%q, want validator-a", precommit.Sender)
	}
	if !bytes.Equal(precommit.Payload, proposal.Payload) {
		t.Fatalf("precommit payload=%q, want proposal payload", precommit.Payload)
	}
	if driver.Runtime().State().Phase != consensus.PhasePrecommit {
		t.Fatalf("phase=%v, want precommit", driver.Runtime().State().Phase)
	}
	if got := len(driver.Runtime().PrecommitVotes()); got != 1 {
		t.Fatalf("precommit votes=%d, want 1", got)
	}
}
