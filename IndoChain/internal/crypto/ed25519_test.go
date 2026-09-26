package crypto

import (
	"bytes"
	"crypto/ed25519"
	"testing"
)

func TestEd25519SignVerify(t *testing.T) {
	seed := bytes.Repeat([]byte{0x42}, ed25519.SeedSize)
	signer, err := NewEd25519Signer(ed25519.NewKeyFromSeed(seed))
	if err != nil {
		t.Fatal(err)
	}
	message := []byte("IndoChain development crypto test")
	signature, err := signer.Sign(message)
	if err != nil {
		t.Fatal(err)
	}
	if !VerifyEd25519(message, signature, signer.PublicKey()) {
		t.Fatal("signature verification failed")
	}
	if VerifyEd25519([]byte("tampered"), signature, signer.PublicKey()) {
		t.Fatal("tampered message unexpectedly verified")
	}
}

func TestDomainSeparatedMessage(t *testing.T) {
	message := []byte("payload")
	a := DomainSeparatedMessage("INDOCHAIN-TX", 1, message)
	b := DomainSeparatedMessage("INDOCHAIN-BLOCK", 1, message)
	if bytes.Equal(a, b) {
		t.Fatal("different domains produced identical signing bytes")
	}
}
