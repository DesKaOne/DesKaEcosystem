package consensus

import (
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/crypto"
)

var (
	ErrInvalidTimeoutMessage      = errors.New("invalid consensus timeout message")
	ErrTimeoutAuthorityMissing    = errors.New("timeout authority resolver missing")
	ErrTimeoutTargetRoundMismatch = errors.New("timeout target round mismatch")
)

const timeoutPayloadSize = 8

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
	if err := state.Validate(); err != nil {
		return Message{}, err
	}
	if len(validatorID) == 0 {
		return Message{}, ErrMissingSender
	}
	if nextRound <= state.Round {
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
		Payload:         encodeTimeoutTargetRound(nextRound),
	}
	signed, err := msg.Sign(signer)
	if err != nil {
		return Message{}, err
	}
	return signed, nil
}

// TimeoutTargetRound decodes the canonical timeout payload.
func TimeoutTargetRound(msg Message) (uint64, error) {
	if msg.Type != MessageTypeTimeout || len(msg.Payload) != timeoutPayloadSize {
		return 0, ErrInvalidTimeoutMessage
	}
	return binary.BigEndian.Uint64(msg.Payload), nil
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
	if len(messages) == 0 {
		return TimeoutCertificate{}, ErrInvalidTimeoutMessage
	}

	var nextRound uint64
	senders := make([][]byte, 0, len(messages))
	for i, msg := range messages {
		target, err := ValidateTimeoutMessage(msg, state, validators, rules, resolver)
		if err != nil {
			return TimeoutCertificate{}, fmt.Errorf("timeout message %d: %w", i, err)
		}
		if i == 0 {
			nextRound = target
		} else if target != nextRound {
			return TimeoutCertificate{}, ErrTimeoutTargetRoundMismatch
		}
		senders = append(senders, append([]byte(nil), msg.Sender...))
	}

	return NewTimeoutCertificate(
		state,
		validators,
		votingPower,
		threshold,
		nextRound,
		senders,
	)
}

func encodeTimeoutTargetRound(nextRound uint64) []byte {
	var payload [timeoutPayloadSize]byte
	binary.BigEndian.PutUint64(payload[:], nextRound)
	return payload[:]
}
