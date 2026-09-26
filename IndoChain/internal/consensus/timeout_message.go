package consensus

import (
	"bytes"
	"errors"
	"encoding/binary"
	"fmt"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/crypto"
)

var (
	ErrInvalidTimeoutMessage      = errors.New("invalid consensus timeout message")
	ErrTimeoutAuthorityMissing    = errors.New("timeout authority resolver missing")
	ErrTimeoutTargetRoundMismatch = errors.New("timeout target round mismatch")
)

const timeoutPayloadPrefixSize = 20

// TimeoutAuthorityResolver resolves the public key used to authenticate a
// validator's timeout message. The resolver is intentionally external to the
// validator-set model in v0.1.
type TimeoutAuthorityResolver interface {
	PublicKeyForValidator(validatorID []byte) ([]byte, error)
}

// NewTimeoutMessage creates a signed timeout message for the current round.
// nextRound must be strictly newer than the supplied round state.
func NewTimeoutMessage(
	state RoundState,
	validatorID []byte,
	nextRound uint64,
	signer crypto.Signer,
) (Message, error) {
	return NewTimeoutMessageWithLockRound(state, validatorID, nextRound, state.Round, nil, signer)
}

// NewTimeoutMessageWithLock creates a signed timeout message carrying the
// sender's locked proposal, if any. The lock is part of the signed evidence.
func NewTimeoutMessageWithLock(
	state RoundState,
	validatorID []byte,
	nextRound uint64,
	lockedProposal []byte,
	signer crypto.Signer,
) (Message, error) {
	return NewTimeoutMessageWithLockRound(state, validatorID, nextRound, state.Round, lockedProposal, signer)
}

// NewTimeoutMessageWithLockRound creates signed timeout evidence with an explicit lock round.
func NewTimeoutMessageWithLockRound(
	state RoundState,
	validatorID []byte,
	nextRound uint64,
	lockedRound uint64,
	lockedProposal []byte,
	signer crypto.Signer,
) (Message, error) {
	if err := state.Validate(); err != nil {
		return Message{}, err
	}
	if len(validatorID) == 0 {
		return Message{}, ErrMissingSender
	}
	if nextRound <= state.Round {
		return Message{}, ErrInvalidTimeoutRound
	}
	if len(lockedProposal) > 0 && lockedRound > state.Round {
		return Message{}, ErrInvalidTimeoutRound
	}
	if signer == nil {
		return Message{}, ErrMissingSignature
	}

	msg := Message{
		ProtocolVersion: state.ProtocolVersion,
		ChainID:         state.ChainID,
		Epoch:           state.Epoch,
		Height:          state.Height,
		Round:           state.Round,
		Sender:          append([]byte(nil), validatorID...),
		Type:            MessageTypeTimeout,
		Payload:         encodeTimeoutEvidenceWithRound(nextRound, lockedRound, lockedProposal),
	}
	signed, err := msg.Sign(signer)
	if err != nil {
		return Message{}, err
	}
	return signed, nil
}

// NewTimeoutMessageWithLockProof creates a signed timeout message carrying
// a complete, verifiable proof-of-lock. The proof is bound into the signed
// payload, so tampering with any proof field invalidates the timeout signature.
func NewTimeoutMessageWithLockProof(
	state RoundState,
	validatorID []byte,
	nextRound uint64,
	proof LockProof,
	signer crypto.Signer,
) (Message, error) {
	if err := state.Validate(); err != nil { return Message{}, err }
	if len(validatorID) == 0 { return Message{}, ErrMissingSender }
	if nextRound <= state.Round { return Message{}, ErrInvalidTimeoutRound }
	if signer == nil { return Message{}, ErrMissingSignature }
	encoded, err := EncodeLockProof(proof)
	if err != nil { return Message{}, err }
	if proof.LockedRound > state.Round { return Message{}, ErrInvalidTimeoutRound }
	msg := Message{
		ProtocolVersion: state.ProtocolVersion, ChainID: state.ChainID, Epoch: state.Epoch,
		Height: state.Height, Round: state.Round, Sender: append([]byte(nil), validatorID...),
		Type: MessageTypeTimeout,
		Payload: encodeTimeoutEvidenceWithRoundAndProof(nextRound, proof.LockedRound, proof.Proposal, encoded),
	}
	return msg.Sign(signer)
}

// TimeoutTargetRound decodes the target round from the canonical timeout payload.
func TimeoutTargetRound(msg Message) (uint64, error) {
	nextRound, _, _, _, err := decodeTimeoutEvidence(msg)
	return nextRound, err
}

// TimeoutLockedProposal returns a defensive copy of the lock context carried
// by a signed timeout message. An empty result means the sender carried no lock.
func TimeoutLockedProposal(msg Message) ([]byte, error) {
	_, _, lockedProposal, _, err := decodeTimeoutEvidence(msg)
	return append([]byte(nil), lockedProposal...), err
}

// TimeoutLockedRound returns the round in which the carried lock was formed.
func TimeoutLockedRound(msg Message) (uint64, error) {
	_, lockedRound, _, _, err := decodeTimeoutEvidence(msg)
	return lockedRound, err
}

// TimeoutLockProof returns a defensive decoded proof-of-lock, if present.
func TimeoutLockProof(msg Message) (*LockProof, error) {
	_, _, _, encoded, err := decodeTimeoutEvidence(msg)
	if err != nil { return nil, err }
	if len(encoded) == 0 { return nil, nil }
	proof, err := DecodeLockProof(encoded)
	if err != nil { return nil, err }
	return cloneLockProofPtr(&proof), nil
}

// ValidateTimeoutMessage validates structure, exact consensus context, sender
// membership, target-round semantics, and signature authority.
func ValidateTimeoutMessage(
	msg Message,
	state RoundState,
	validators ValidatorSet,
	rules ValidationRules,
	resolver TimeoutAuthorityResolver,
) (uint64, error) {
	rules.RequireSender = true
	rules.RequireSignature = true
	if err := ValidateConsensusMessage(msg, MessageValidationContext{
		Rules:      rules,
		State:      state,
		Validators: validators,
	}); err != nil {
		return 0, err
	}
	if msg.Type != MessageTypeTimeout {
		return 0, ErrInvalidTimeoutMessage
	}
	nextRound, err := TimeoutTargetRound(msg)
	if err != nil {
		return 0, err
	}
	if nextRound <= state.Round {
		return 0, ErrInvalidTimeoutRound
	}
	if resolver == nil {
		return 0, ErrTimeoutAuthorityMissing
	}
	sender := append([]byte(nil), msg.Sender...)
	publicKey, err := resolver.PublicKeyForValidator(sender)
	if err != nil {
		return 0, err
	}
	if len(publicKey) == 0 {
		return 0, ErrInvalidSignature
	}
	if err := VerifyMessageSignature(msg, publicKey); err != nil {
		return 0, err
	}
	return nextRound, nil
}

// NewTimeoutCertificateFromMessages authenticates timeout messages and turns
// their unique validator identities into the existing deterministic timeout
// evidence boundary. No runtime state is advanced by this function.
func NewTimeoutCertificateFromMessages(
	state RoundState,
	validators ValidatorSet,
	votingPower VotingPowerSet,
	threshold QuorumThreshold,
	messages []Message,
	rules ValidationRules,
	resolver TimeoutAuthorityResolver,
) (TimeoutCertificate, error) {
	if len(messages) == 0 { return TimeoutCertificate{}, ErrInvalidTimeoutMessage }
	var nextRound uint64
	var lockedProposal []byte
	var lockedRound uint64
	var lockProof *LockProof
	senders := make([][]byte, 0, len(messages))
	for i, msg := range messages {
		target, err := ValidateTimeoutMessage(msg, state, validators, rules, resolver)
		if err != nil { return TimeoutCertificate{}, fmt.Errorf("timeout message %d: %w", i, err) }
		messageLockRound, err := TimeoutLockedRound(msg)
		if err != nil { return TimeoutCertificate{}, fmt.Errorf("timeout message %d: %w", i, err) }
		messageLock, err := TimeoutLockedProposal(msg)
		if err != nil { return TimeoutCertificate{}, fmt.Errorf("timeout message %d: %w", i, err) }
		messageProof, err := TimeoutLockProof(msg)
		if err != nil { return TimeoutCertificate{}, fmt.Errorf("timeout message %d: %w", i, err) }
		if i == 0 {
			nextRound = target
			lockedProposal = append([]byte(nil), messageLock...)
			lockedRound = messageLockRound
			lockProof = messageProof
		} else {
			if target != nextRound { return TimeoutCertificate{}, ErrTimeoutTargetRoundMismatch }
			if !bytes.Equal(messageLock, lockedProposal) || messageLockRound != lockedRound {
				return TimeoutCertificate{}, ErrConflictingTimeoutLock
			}
			if (messageProof == nil) != (lockProof == nil) {
				return TimeoutCertificate{}, ErrConflictingTimeoutLock
			}
			if messageProof != nil {
				encodedA, _ := EncodeLockProof(*lockProof)
				encodedB, _ := EncodeLockProof(*messageProof)
				if !bytes.Equal(encodedA, encodedB) { return TimeoutCertificate{}, ErrConflictingTimeoutLock }
			}
		}
		senders = append(senders, append([]byte(nil), msg.Sender...))
	}
	if len(lockedProposal) > 0 {
		if lockProof == nil { return TimeoutCertificate{}, ErrInvalidLockProof }
		if err := ValidateLockProofWithAuthority(*lockProof, state, validators, votingPower, resolver); err != nil { return TimeoutCertificate{}, err }
	}
	return NewTimeoutCertificateWithLockProof(state, validators, votingPower, threshold, nextRound, senders, lockProof)
}

func encodeTimeoutEvidence(nextRound uint64, lockedProposal []byte) []byte {
	return encodeTimeoutEvidenceWithRoundAndProof(nextRound, 0, lockedProposal, nil)
}

func encodeTimeoutEvidenceWithRound(nextRound uint64, lockedRound uint64, lockedProposal []byte) []byte {
	return encodeTimeoutEvidenceWithRoundAndProof(nextRound, lockedRound, lockedProposal, nil)
}

func encodeTimeoutEvidenceWithRoundAndProof(nextRound uint64, lockedRound uint64, lockedProposal []byte, proof []byte) []byte {
	if uint64(len(lockedProposal)) > uint64(^uint32(0)) || uint64(len(proof)) > uint64(^uint32(0)) { return nil }
	payload := make([]byte, timeoutPayloadPrefixSize+len(lockedProposal)+4+len(proof))
	binary.BigEndian.PutUint64(payload[:8], nextRound)
	binary.BigEndian.PutUint64(payload[8:16], lockedRound)
	binary.BigEndian.PutUint32(payload[16:20], uint32(len(lockedProposal)))
	copy(payload[20:], lockedProposal)
	offset := timeoutPayloadPrefixSize + len(lockedProposal)
	binary.BigEndian.PutUint32(payload[offset:offset+4], uint32(len(proof)))
	copy(payload[offset+4:], proof)
	return payload
}

func decodeTimeoutEvidence(msg Message) (uint64, uint64, []byte, []byte, error) {
	if msg.Type != MessageTypeTimeout || len(msg.Payload) < timeoutPayloadPrefixSize+4 { return 0, 0, nil, nil, ErrInvalidTimeoutMessage }
	nextRound := binary.BigEndian.Uint64(msg.Payload[:8])
	lockedRound := binary.BigEndian.Uint64(msg.Payload[8:16])
	lockLen := binary.BigEndian.Uint32(msg.Payload[16:20])
	offset := timeoutPayloadPrefixSize
	if uint64(offset)+uint64(lockLen)+4 > uint64(len(msg.Payload)) { return 0, 0, nil, nil, ErrInvalidTimeoutMessage }
	lockedProposal := append([]byte(nil), msg.Payload[offset:offset+int(lockLen)]...)
	offset += int(lockLen)
	proofLen := binary.BigEndian.Uint32(msg.Payload[offset:offset+4])
	offset += 4
	if uint64(offset)+uint64(proofLen) != uint64(len(msg.Payload)) { return 0, 0, nil, nil, ErrInvalidTimeoutMessage }
	proof := append([]byte(nil), msg.Payload[offset:]...)
	return nextRound, lockedRound, lockedProposal, proof, nil
}
