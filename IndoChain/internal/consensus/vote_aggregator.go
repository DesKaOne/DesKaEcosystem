package consensus

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
)

var (
	ErrInvalidVote = errors.New("invalid consensus vote")
	ErrDuplicateVote = errors.New("duplicate consensus vote")
	ErrVoteSenderNotInVotingPower = errors.New("vote sender has no voting power")
)

// VoteAggregator is a development boundary for collecting unique votes within
// one exact consensus context. It does not define production locking,
// timeout, finality, or vote semantics.
type VoteAggregator struct {
	Rules       ValidationRules
	State       RoundState
	Validators  ValidatorSet
	VotingPower VotingPowerSet
	Votes       []Message
}

func NewVoteAggregator(rules ValidationRules, state RoundState, validators ValidatorSet, votingPower VotingPowerSet) (VoteAggregator, error) {
	if err := state.Validate(); err != nil {
		return VoteAggregator{}, err
	}
	if err := validators.Validate(); err != nil {
		return VoteAggregator{}, err
	}
	if err := votingPower.Validate(); err != nil {
		return VoteAggregator{}, err
	}
	if rules.ProtocolVersion == 0 || rules.ChainID == 0 {
		return VoteAggregator{}, ErrInvalidConsensusMessage
	}
	if rules.ProtocolVersion != state.ProtocolVersion || rules.ChainID != state.ChainID {
		return VoteAggregator{}, ErrStateContextMismatch
	}
	return VoteAggregator{
		Rules: rules, State: state, Validators: validators, VotingPower: votingPower,
	}, nil
}

// AddVote validates and records one unique vote. Signature verification is
// intentionally separate because validator-to-public-key authority is not yet
// part of the v0.1 consensus boundary.
func (a *VoteAggregator) AddVote(msg Message) error {
	if a == nil {
		return ErrInvalidVote
	}
	if msg.Type != MessageTypeVote && msg.Type != MessageTypePrevote && msg.Type != MessageTypePrecommit {
		return ErrInvalidVote
	}
	if err := ValidateConsensusMessage(msg, MessageValidationContext{
		Rules: a.Rules, State: a.State, Validators: a.Validators,
	}); err != nil {
		return err
	}
	if _, ok := a.VotingPower.PowerOf(msg.Sender); !ok {
		return ErrVoteSenderNotInVotingPower
	}
	for _, existing := range a.Votes {
		if bytes.Equal(existing.Sender, msg.Sender) {
			return fmt.Errorf("%w: sender %x", ErrDuplicateVote, msg.Sender)
		}
	}
	cloned := msg
	cloned.Sender = append([]byte(nil), msg.Sender...)
	cloned.Payload = append([]byte(nil), msg.Payload...)
	cloned.Signature = append([]byte(nil), msg.Signature...)
	a.Votes = append(a.Votes, cloned)
	return nil
}

func (a VoteAggregator) VotesForPayload(payload []byte) []Message {
	var result []Message
	for _, vote := range a.Votes {
		if bytes.Equal(vote.Payload, payload) {
			cloned := vote
			cloned.Sender = append([]byte(nil), vote.Sender...)
			cloned.Payload = append([]byte(nil), vote.Payload...)
			cloned.Signature = append([]byte(nil), vote.Signature...)
			result = append(result, cloned)
		}
	}
	return result
}

func (a VoteAggregator) VotingPowerForPayload(payload []byte) (uint64, error) {
	if err := a.VotingPower.Validate(); err != nil {
		return 0, err
	}
	var total uint64
	for _, vote := range a.Votes {
		if !bytes.Equal(vote.Payload, payload) {
			continue
		}
		power, ok := a.VotingPower.PowerOf(vote.Sender)
		if !ok {
			return 0, ErrVoteSenderNotInVotingPower
		}
		if ^uint64(0)-total < power {
			return 0, ErrInvalidVotingPowerSet
		}
		total += power
	}
	return total, nil
}

func (a VoteAggregator) QuorumForPayload(payload []byte, threshold QuorumThreshold) (bool, error) {
	votedPower, err := a.VotingPowerForPayload(payload)
	if err != nil {
		return false, err
	}
	totalPower, err := a.VotingPower.TotalPower()
	if err != nil {
		return false, err
	}
	return QuorumReached(votedPower, totalPower, threshold)
}

func (a VoteAggregator) Context() (types.ProtocolVersion, types.ChainID, uint64, types.Height, uint64) {
	return a.State.ProtocolVersion, a.State.ChainID, a.State.Epoch, a.State.Height, a.State.Round
}
