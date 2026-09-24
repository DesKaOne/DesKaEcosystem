package consensus

import (
	"bytes"
	"errors"
	"fmt"
)

var (
	ErrInvalidConsensusRuntime   = errors.New("invalid consensus runtime")
	ErrUnexpectedProposer        = errors.New("unexpected consensus proposer")
	ErrInvalidRuntimePhase       = errors.New("invalid consensus runtime phase")
	ErrConflictingLockedProposal = errors.New("conflicting locked proposal")
	ErrRoundChangeFinalized      = errors.New("cannot change round after finalization")
)

type RuntimeConfig struct {
	Rules       ValidationRules
	State       RoundState
	Validators  ValidatorSet
	VotingPower VotingPowerSet
	Threshold   QuorumThreshold
	Proposer    ProposerSelector
}

type ValidatorRuntime struct {
	rules       ValidationRules
	state       RoundState
	validators  ValidatorSet
	votingPower VotingPowerSet
	threshold   QuorumThreshold
	proposer    ProposerSelector
	votes          *VoteAggregator
	proposal       []byte
	lockedProposal []byte
	certificate    *FinalityCertificate
}

func NewValidatorRuntime(config RuntimeConfig) (*ValidatorRuntime, error) {
	if err := config.State.Validate(); err != nil {
		return nil, err
	}
	if err := config.Validators.Validate(); err != nil {
		return nil, err
	}
	if err := config.VotingPower.Validate(); err != nil {
		return nil, err
	}
	if err := config.Threshold.Validate(); err != nil {
		return nil, err
	}
	if config.Rules.ProtocolVersion != config.State.ProtocolVersion ||
		config.Rules.ChainID != config.State.ChainID {
		return nil, ErrInvalidConsensusRuntime
	}
	if config.Proposer == nil {
		return nil, ErrInvalidConsensusRuntime
	}

	aggregator, err := NewVoteAggregator(
		config.Rules,
		config.State,
		config.Validators,
		config.VotingPower,
	)
	if err != nil {
		return nil, err
	}

	return &ValidatorRuntime{
		rules:       config.Rules,
		state:       config.State,
		validators:  cloneValidatorSet(config.Validators),
		votingPower: cloneVotingPowerSet(config.VotingPower),
		threshold:   config.Threshold,
		proposer:    config.Proposer,
		votes:       &aggregator,
	}, nil
}

func (r *ValidatorRuntime) State() RoundState { return r.state }

// AdvanceRound moves the runtime to a strictly newer round after a timeout
// or round-change event. The current proposal and round-local votes are reset,
// while the locked proposal is preserved for the next round.
func (r *ValidatorRuntime) AdvanceRound(next uint64) error {
	if r == nil {
		return ErrInvalidConsensusRuntime
	}
	if r.state.Phase == PhaseFinalized {
		return ErrRoundChangeFinalized
	}
	if next <= r.state.Round {
		return fmt.Errorf("%w: current=%d next=%d", ErrRoundRegression, r.state.Round, next)
	}

	nextState, err := r.state.AdvanceRound(next)
	if err != nil {
		return err
	}
	aggregator, err := NewVoteAggregator(
		r.rules,
		nextState,
		r.validators,
		r.votingPower,
	)
	if err != nil {
		return err
	}

	r.state = nextState
	r.votes = &aggregator
	r.proposal = nil
	r.certificate = nil
	return nil
}

func (r *ValidatorRuntime) ExpectedProposer() ([]byte, error) {
	if r == nil || r.proposer == nil {
		return nil, ErrInvalidConsensusRuntime
	}
	return r.proposer.Proposer(r.state, r.validators)
}

func (r *ValidatorRuntime) AcceptProposal(msg Message) error {
	if r == nil {
		return ErrInvalidConsensusRuntime
	}
	if r.state.Phase != PhaseProposal {
		return ErrInvalidRuntimePhase
	}
	if msg.Type != MessageTypeProposal {
		return fmt.Errorf("%w: expected proposal message", ErrInvalidConsensusRuntime)
	}
	if err := ValidateConsensusMessage(msg, MessageValidationContext{
		Rules: r.rules, State: r.state, Validators: r.validators,
	}); err != nil {
		return err
	}
	expected, err := r.ExpectedProposer()
	if err != nil {
		return err
	}
	if !bytes.Equal(expected, msg.Sender) {
		return fmt.Errorf("%w: expected %q got %q", ErrUnexpectedProposer, expected, msg.Sender)
	}
	if len(msg.Payload) == 0 {
		return ErrInvalidConsensusRuntime
	}
	if len(r.lockedProposal) > 0 && !bytes.Equal(r.lockedProposal, msg.Payload) {
		return fmt.Errorf("%w: locked=%q received=%q", ErrConflictingLockedProposal, r.lockedProposal, msg.Payload)
	}
	r.proposal = append([]byte(nil), msg.Payload...)
	r.state.Phase = PhasePrevote
	return nil
}

// AcceptBlockProposal converts a validated block proposal into the opaque
// consensus proposal consumed by the runtime. The block candidate itself is
// not executed or committed by this method.
func (r *ValidatorRuntime) AcceptBlockProposal(proposal BlockProposal) error {
	if r == nil {
		return ErrInvalidConsensusRuntime
	}
	if r.state.Phase != PhaseProposal {
		return ErrInvalidRuntimePhase
	}
	expected, err := r.ExpectedProposer()
	if err != nil {
		return err
	}
	if proposal.Candidate.Header.Version != r.state.ProtocolVersion ||
		proposal.Candidate.Header.ChainID != r.state.ChainID ||
		proposal.Candidate.Header.Height != r.state.Height+1 {
		return ErrInvalidConsensusRuntime
	}
	if !bytes.Equal(proposal.Candidate.Header.Proposer, expected) {
		return fmt.Errorf("%w: expected %q got %q", ErrUnexpectedProposer, expected, proposal.Candidate.Header.Proposer)
	}
	expectedPayload, err := ValidateProducedBlock(BlockProductionContext{
		State:        r.state,
		PreviousHash: proposal.Candidate.Header.PreviousHash,
		Proposer:     proposal.Candidate.Header.Proposer,
	}, proposal.Candidate)
	if err != nil {
		return err
	}
	payload := proposal.MessagePayload()
	if !proposal.SamePayload(expectedPayload[:]) || !bytes.Equal(payload, expectedPayload[:]) {
		return ErrInvalidConsensusRuntime
	}
	return r.AcceptProposal(Message{
		ProtocolVersion: r.state.ProtocolVersion,
		ChainID:         r.state.ChainID,
		Epoch:           r.state.Epoch,
		Height:          r.state.Height,
		Round:           r.state.Round,
		Sender:          append([]byte(nil), proposal.Candidate.Header.Proposer...),
		Type:            MessageTypeProposal,
		Payload:         payload,
	})
}

func (r *ValidatorRuntime) AddVote(msg Message) error {
	if r == nil {
		return ErrInvalidConsensusRuntime
	}
	if r.state.Phase != PhasePrevote && r.state.Phase != PhasePrecommit {
		return ErrInvalidRuntimePhase
	}
	if len(r.proposal) == 0 {
		return ErrInvalidConsensusRuntime
	}
	if err := ValidateConsensusMessage(msg, MessageValidationContext{
		Rules: r.rules, State: r.state, Validators: r.validators,
	}); err != nil {
		return err
	}
	if _, ok := r.votingPower.PowerOf(msg.Sender); !ok {
		return ErrVoteSenderNotInVotingPower
	}
	if len(r.lockedProposal) > 0 && !bytes.Equal(r.lockedProposal, msg.Payload) {
		return fmt.Errorf("%w: locked=%q received=%q", ErrConflictingLockedProposal, r.lockedProposal, msg.Payload)
	}
	if err := r.votes.AddVote(msg); err != nil {
		return err
	}
	if r.state.Phase == PhasePrevote {
		reached, err := r.votes.QuorumForPayload(r.proposal, r.threshold)
		if err != nil {
			return err
		}
		if reached {
			r.lockedProposal = append([]byte(nil), r.proposal...)
			r.state.Phase = PhasePrecommit
		}
	}
	return nil
}

func (r *ValidatorRuntime) FinalizeProposal() (FinalityCertificate, error) {
	if r == nil {
		return FinalityCertificate{}, ErrInvalidConsensusRuntime
	}
	if r.state.Phase != PhasePrecommit {
		return FinalityCertificate{}, ErrInvalidRuntimePhase
	}
	if len(r.lockedProposal) == 0 || !bytes.Equal(r.lockedProposal, r.proposal) {
		return FinalityCertificate{}, ErrConflictingLockedProposal
	}
	certificate, err := NewFinalityCertificate(
		r.state,
		r.validators,
		r.votingPower,
		r.threshold,
		r.proposal,
		r.votes.VotesForPayload(r.proposal),
	)
	if err != nil {
		return FinalityCertificate{}, err
	}
	r.certificate = &certificate
	r.state.Phase = PhaseFinalized
	return certificate, nil
}

// FinalizedCertificate returns the certificate produced by this runtime after
// the proposal reached the Finalized phase. The returned certificate is cloned
// so callers cannot mutate runtime-owned consensus evidence.
func (r *ValidatorRuntime) FinalizedCertificate() (FinalityCertificate, error) {
	if r == nil {
		return FinalityCertificate{}, ErrInvalidConsensusRuntime
	}
	if r.state.Phase != PhaseFinalized || r.certificate == nil {
		return FinalityCertificate{}, ErrInvalidRuntimePhase
	}
	certificate := *r.certificate
	certificate.Payload = append([]byte(nil), r.certificate.Payload...)
	certificate.Votes = cloneVotes(r.certificate.Votes)
	return certificate, nil
}

func cloneValidatorSet(set ValidatorSet) ValidatorSet {
	cloned := ValidatorSet{Validators: make([][]byte, len(set.Validators))}
	for i, id := range set.Validators {
		cloned.Validators[i] = append([]byte(nil), id...)
	}
	return cloned
}

func cloneVotingPowerSet(set VotingPowerSet) VotingPowerSet {
	cloned := VotingPowerSet{Validators: make([]ValidatorVotingPower, len(set.Validators))}
	for i, entry := range set.Validators {
		cloned.Validators[i] = ValidatorVotingPower{
			ValidatorID: append([]byte(nil), entry.ValidatorID...),
			Power:       entry.Power,
		}
	}
	return cloned
}
