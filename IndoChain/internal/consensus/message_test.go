package consensus

import (
	"bytes"
	"errors"
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/crypto"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
)

func testConsensusMessage() Message {
	return Message{ProtocolVersion: 1, ChainID: 1001, Epoch: 2, Height: 7, Round: 3, Sender: []byte{1, 2, 3}, Type: MessageTypeVote, Payload: []byte{9, 8, 7}}
}

func testConsensusRules() ValidationRules {
	return ValidationRules{ProtocolVersion: 1, ChainID: 1001, MaxPayloadSize: 16, RequireSender: true, RequireSignature: true}
}

func TestValidateMessage(t *testing.T) {
	m := testConsensusMessage()
	keyPair, err := crypto.NewEd25519KeyPair(bytes.Repeat([]byte{0x42}, 32))
	if err != nil { t.Fatalf("new key pair: %v", err) }
	signer, err := crypto.NewEd25519Signer(keyPair.PrivateKey)
	if err != nil { t.Fatalf("new signer: %v", err) }
	m, err = m.Sign(signer)
	if err != nil { t.Fatalf("sign: %v", err) }
	if err := ValidateMessage(m, testConsensusRules()); err != nil { t.Fatalf("validate: %v", err) }
	if err := VerifyMessageSignature(m, signer.PublicKey()); err != nil { t.Fatalf("verify: %v", err) }
}

func TestValidateMessageRejectsWrongContext(t *testing.T) {
	m := testConsensusMessage()
	m.ProtocolVersion = types.ProtocolVersion(2)
	if err := ValidateMessage(m, ValidationRules{ProtocolVersion: 1, ChainID: 1001}); !errors.Is(err, ErrWrongProtocolVersion) { t.Fatalf("expected protocol version error, got %v", err) }
	m = testConsensusMessage()
	m.ChainID = 1002
	if err := ValidateMessage(m, ValidationRules{ProtocolVersion: 1, ChainID: 1001}); !errors.Is(err, ErrWrongChainID) { t.Fatalf("expected chain id error, got %v", err) }
}

func TestValidateMessageRejectsMalformedFields(t *testing.T) {
	rules := testConsensusRules()
	m := testConsensusMessage()
	if err := ValidateMessage(m, rules); !errors.Is(err, ErrMissingSignature) { t.Fatalf("expected missing signature error, got %v", err) }
	m.Signature = []byte{1}
	m.Sender = nil
	if err := ValidateMessage(m, rules); !errors.Is(err, ErrMissingSender) { t.Fatalf("expected missing sender error, got %v", err) }
	m.Sender = []byte{1}
	m.Type = 99
	if err := ValidateMessage(m, rules); !errors.Is(err, ErrInvalidMessageType) { t.Fatalf("expected message type error, got %v", err) }
}

func TestValidateMessageRejectsOversizedPayload(t *testing.T) {
	m := testConsensusMessage()
	m.Payload = bytes.Repeat([]byte{1}, 17)
	if err := ValidateMessage(m, ValidationRules{ProtocolVersion: 1, ChainID: 1001, MaxPayloadSize: 16}); !errors.Is(err, ErrMessageTooLarge) { t.Fatalf("expected payload size error, got %v", err) }
}

func TestConsensusSigningBytesAreDeterministicAndContextBound(t *testing.T) {
	m := testConsensusMessage()
	a := m.SigningBytes()
	b := m.SigningBytes()
	if !bytes.Equal(a, b) { t.Fatal("signing bytes are not deterministic") }
	m.ChainID = 1002
	if bytes.Equal(a, m.SigningBytes()) { t.Fatal("signing bytes are not chain-bound") }
}
