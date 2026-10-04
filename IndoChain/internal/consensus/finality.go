package consensus

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
)

var (
	ErrInvalidFinalityCertificate = errors.New("invalid consensus finality certificate")
	ErrFinalityQuorumNotReached   = errors.New("finality quorum not reached")
)

// FinalityCertificate is a development boundary that proves a quorum of unique
// validator votes for one opaque payload in one exact consensus context.
//
// It does not define a production BFT algorithm, locking rule, timeout,
// validator-set transition, or canonical certificate encoding.
type FinalityCertificate struct {
	ProtocolVersion types.ProtocolVersion
	ChainID         types.ChainID
	Epoch           uint64
	Height          types.Height
	Round           uint64
	Payload         []byte
	Threshold       QuorumThreshold
	Votes           []Message
}

// NewFinalityCertificate builds a certificate from already collected votes.
// The certificate is accepted only when those votes reach the supplied quorum.
func NewFinalityCertificate(
	state RoundState,
	validators ValidatorSet,
	votingPower VotingPowerSet,
	threshold QuorumThreshold,
	payload []byte,
	votes []Message,
) (FinalityCertificate, error) {
	if err := state.Validate(); err != nil {
		return FinalityCertificate{}, err
	}
	if err := threshold.Validate(); err != nil {
		return FinalityCertificate{}, err
	}
	if len(payload) == 0 {
		return FinalityCertificate{}, ErrInvalidFinalityCertificate
	}

	aggregator, err := NewVoteAggregator(
		ValidationRules{
			ProtocolVersion: state.ProtocolVersion,
			ChainID:         state.ChainID,
			RequireSender:   true,
		},
		state,
		validators,
		votingPower,
	)
	if err != nil {
		return FinalityCertificate{}, err
	}
	for _, vote := range votes {
		if err := aggregator.AddVote(vote); err != nil {
			return FinalityCertificate{}, fmt.Errorf("add finality vote: %w", err)
		}
	}

	quorum, err := aggregator.QuorumForPayload(payload, threshold)
	if err != nil {
		return FinalityCertificate{}, err
	}
	if !quorum {
		return FinalityCertificate{}, ErrFinalityQuorumNotReached
	}

	return FinalityCertificate{
		ProtocolVersion: state.ProtocolVersion,
		ChainID:         state.ChainID,
		Epoch:           state.Epoch,
		Height:          state.Height,
		Round:           state.Round,
		Payload:         append([]byte(nil), payload...),
		Threshold:       threshold,
		Votes:           cloneVotes(aggregator.VotesForPayload(payload)),
	}, nil
}

// ValidateFinalityCertificate checks the certificate against the supplied
// consensus context and validator voting-power set. Validation does not
// advance or mutate the caller's round state.
func ValidateFinalityCertificate(
	certificate FinalityCertificate,
	state RoundState,
	validators ValidatorSet,
	votingPower VotingPowerSet,
) error {
	if err := certificate.validateContext(state); err != nil {
		return err
	}
	if err := certificate.Threshold.Validate(); err != nil {
		return err
	}
	if len(certificate.Payload) == 0 || len(certificate.Votes) == 0 {
		return ErrInvalidFinalityCertificate
	}

	aggregator, err := NewVoteAggregator(
		ValidationRules{
			ProtocolVersion: state.ProtocolVersion,
			ChainID:         state.ChainID,
			RequireSender:   true,
		},
		state,
		validators,
		votingPower,
	)
	if err != nil {
		return err
	}
	for _, vote := range certificate.Votes {
		if err := aggregator.AddVote(vote); err != nil {
			return fmt.Errorf("validate finality vote: %w", err)
		}
	}

	quorum, err := aggregator.QuorumForPayload(certificate.Payload, certificate.Threshold)
	if err != nil {
		return err
	}
	if !quorum {
		return ErrFinalityQuorumNotReached
	}

	return nil
}

func (c FinalityCertificate) validateContext(state RoundState) error {
	if err := state.Validate(); err != nil {
		return err
	}
	if c.ProtocolVersion != state.ProtocolVersion ||
		c.ChainID != state.ChainID ||
		c.Epoch != state.Epoch ||
		c.Height != state.Height ||
		c.Round != state.Round {
		return ErrStateContextMismatch
	}
	return nil
}

func cloneVotes(votes []Message) []Message {
	cloned := make([]Message, len(votes))
	for i, vote := range votes {
		cloned[i] = vote
		cloned[i].Sender = append([]byte(nil), vote.Sender...)
		cloned[i].Payload = append([]byte(nil), vote.Payload...)
		cloned[i].Signature = append([]byte(nil), vote.Signature...)
	}
	return cloned
}

func finalityPayloadEqual(a, b []byte) bool {
	return bytes.Equal(a, b)
}
