package transaction

import (
	"bytes"
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/crypto"
)

func testRules() ValidationRules {
	return ValidationRules{
		ProtocolVersion:  1,
		ChainID:          1001,
		RequireSender:    true,
		RequireRecipient: true,
		RequireSignature: true,
		MinGasLimit:      21000,
		MaxDataSize:      1024,
	}
}

func testTx(signer *crypto.Ed25519Signer) Transaction {
	tx := Transaction{
		Version:   1,
		ChainID:   1001,
		Nonce:     1,
		Sender:    types.Address{1, 2, 3},
		Recipient: types.Address{4, 5, 6},
		Value:     100,
		GasLimit:  21000,
		Data:      []byte("hello"),
	}
	sig, _ := Sign(tx, signer)
	tx.Signature = sig
	return tx
}

func TestValidateAndVerify(t *testing.T) {
	signer, err := crypto.NewEd25519Signer(bytes.Repeat([]byte{0x42}, 64))
	if err != nil {
		t.Fatal(err)
	}

	tx := testTx(signer)
	if err := ValidateAndVerify(tx, testRules(), signer.PublicKey()); err != nil {
		t.Fatal(err)
	}

	tx.ChainID = 1002
	if err := Validate(tx, testRules()); err != ErrInvalidChainID {
		t.Fatalf("expected chain id error, got %v", err)
	}
}

func TestValidateRejectsMissingFields(t *testing.T) {
	rules := testRules()
	tx := Transaction{Version: 1, ChainID: 1001, GasLimit: 21000}

	if err := Validate(tx, rules); err != ErrInvalidSender {
		t.Fatalf("expected sender error, got %v", err)
	}
}

func TestValidateRejectsBadSignature(t *testing.T) {
	signer, err := crypto.NewEd25519Signer(bytes.Repeat([]byte{0x42}, 64))
	if err != nil {
		t.Fatal(err)
	}
	tx := testTx(signer)
	tx.Signature[0] ^= 0xff

	if err := ValidateAndVerify(tx, testRules(), signer.PublicKey()); err != ErrInvalidSignature {
		t.Fatalf("expected signature error, got %v", err)
	}
}
