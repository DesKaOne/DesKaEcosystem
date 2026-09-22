package consensus

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/crypto"
)

type MessageType uint8

const (
	MessageTypeProposal MessageType = iota + 1
	MessageTypeVote
	MessageTypeFinalityEvidence
	MessageTypeValidatorSetUpdate
)

var (
	ErrInvalidConsensusMessage = errors.New("invalid consensus message")
	ErrWrongChainID = errors.New("wrong consensus chain id")
	ErrWrongProtocolVersion = errors.New("wrong consensus protocol version")
	ErrMissingSender = errors.New("missing consensus sender")
	ErrMissingSignature = errors.New("missing consensus signature")
	ErrInvalidMessageType = errors.New("invalid consensus message type")
	ErrMessageTooLarge = errors.New("consensus message too large")
	ErrInvalidSignature = errors.New("invalid consensus signature")
)

const signingDomain = "INDOCHAIN-CONSENSUS"

type Message struct {
	ProtocolVersion types.ProtocolVersion
	ChainID types.ChainID
	Epoch uint64
	Height types.Height
	Round uint64
	Sender []byte
	Type MessageType
	Payload []byte
	Signature []byte
}

type ValidationRules struct {
	ProtocolVersion types.ProtocolVersion
	ChainID types.ChainID
	MaxPayloadSize uint32
	RequireSender bool
	RequireSignature bool
}

func (m Message) SigningBytes() []byte {
	var b bytes.Buffer
	putU16(&b, uint16(m.ProtocolVersion))
	putU64(&b, uint64(m.ChainID))
	putU64(&b, m.Epoch)
	putU64(&b, uint64(m.Height))
	putU64(&b, m.Round)
	putBytes(&b, m.Sender)
	b.WriteByte(byte(m.Type))
	putBytes(&b, m.Payload)
	return crypto.DomainSeparatedMessage(signingDomain, uint16(m.ProtocolVersion), b.Bytes())
}

func (m Message) Sign(signer crypto.Signer) (Message, error) {
	if signer == nil { return Message{}, ErrMissingSignature }
	sig, err := signer.Sign(m.SigningBytes())
	if err != nil { return Message{}, err }
	m.Signature = append([]byte(nil), sig...)
	return m, nil
}

func ValidateMessage(m Message, rules ValidationRules) error {
	if rules.ProtocolVersion == 0 || rules.ChainID == 0 { return ErrInvalidConsensusMessage }
	if m.ProtocolVersion != rules.ProtocolVersion { return ErrWrongProtocolVersion }
	if m.ChainID != rules.ChainID { return ErrWrongChainID }
	switch m.Type {
	case MessageTypeProposal, MessageTypeVote, MessageTypeFinalityEvidence, MessageTypeValidatorSetUpdate:
	default: return ErrInvalidMessageType
	}
	if rules.RequireSender && len(m.Sender) == 0 { return ErrMissingSender }
	if rules.RequireSignature && len(m.Signature) == 0 { return ErrMissingSignature }
	if rules.MaxPayloadSize > 0 && uint32(len(m.Payload)) > rules.MaxPayloadSize { return ErrMessageTooLarge }
	return nil
}

func VerifyMessageSignature(m Message, publicKey []byte) error {
	if len(publicKey) == 0 || len(m.Signature) == 0 { return ErrInvalidSignature }
	if !crypto.VerifyEd25519(m.SigningBytes(), m.Signature, publicKey) { return ErrInvalidSignature }
	return nil
}

func putU16(b *bytes.Buffer, v uint16) {
	var x [2]byte
	binary.BigEndian.PutUint16(x[:], v)
	b.Write(x[:])
}

func putU64(b *bytes.Buffer, v uint64) {
	var x [8]byte
	binary.BigEndian.PutUint64(x[:], v)
	b.Write(x[:])
}

func putBytes(b *bytes.Buffer, v []byte) {
	if uint64(len(v)) > uint64(^uint32(0)) { panic(fmt.Sprintf("consensus field too large: %d", len(v))) }
	var x [4]byte
	binary.BigEndian.PutUint32(x[:], uint32(len(v)))
	b.Write(x[:])
	b.Write(v)
}
