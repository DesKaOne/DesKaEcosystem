package consensus

import (
	"bytes"
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/crypto"
)

func TestBuildSignedVoteMessageBindsPhaseAndContext(t *testing.T) {
	state, err := NewRoundState(1, 1001, 2, 7)
	if err != nil {
		t.Fatal(err)
	}
	kp, err := crypto.NewEd25519KeyPair(bytes.Repeat([]byte{0x71}, 32))
	if err != nil {
		t.Fatal(err)
	}
	signer, err := crypto.NewEd25519Signer(kp.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}

	msg, err := BuildSignedVoteMessage(
		state,
		[]byte("validator-a"),
		MessageTypePrecommit,
		[]byte("proposal-hash"),
		signer,
	)
	if err != nil {
		t.Fatal(err)
	}
	if msg.Type != MessageTypePrecommit || msg.Height != 7 || msg.Round != 2 {
		t.Fatalf("unexpected vote context: type=%d height=%d round=%d", msg.Type, msg.Height, msg.Round)
	}
	if err := VerifyMessageSignature(msg, signer.PublicKey()); err != nil {
		t.Fatalf("VerifyMessageSignature() error = %v", err)
	}
}

func TestBuildSignedVoteMessageRejectsProposalVoteType(t *testing.T) {
	state, err := NewRoundState(1, 1001, 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	kp, err := crypto.NewEd25519KeyPair(bytes.Repeat([]byte{0x72}, 32))
	if err != nil {
		t.Fatal(err)
	}
	signer, err := crypto.NewEd25519Signer(kp.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := BuildSignedVoteMessage(state, []byte("validator-a"), MessageTypeProposal, []byte("p"), signer); err == nil {
		t.Fatal("BuildSignedVoteMessage() error = nil, want invalid vote type")
	}
}
