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
	ErrInvalidRuntimeVoteType   = errors.New("invalid runtime vote type")
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
	prevotes       *VoteAggregator
	precommits     *VoteAggregator
	proposal       []byte
	lockedProposal []byte
	lockedRound    uint64
	lockedProof    *LockProof
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

	prevotes, err := NewVoteAggregator(
		config.Rules,
		config.State,
		config.Validators,
		config.VotingPower,
	)
	if err != nil {
		return nil, err
	}
	precommits, err := NewVoteAggregator(
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
		prevotes:    &prevotes,
		precommits:  &precommits,
	}, nil
}

func (r *ValidatorRuntime) State() RoundState { return r.state }

func (r *ValidatorRuntime) Proposal() []byte {
	if r == nil { return nil }
	return append([]byte(nil), r.proposal...)
}

func (r *ValidatorRuntime) Validators() ValidatorSet {
	if r == nil { return ValidatorSet{} }
	return cloneValidatorSet(r.validators)
}

func (r *ValidatorRuntime) PrecommitVotes() []Message {
	if r == nil || r.precommits == nil { return nil }
	return cloneVotes(r.precommits.Votes)
}


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
	prevotes, err := NewVoteAggregator(
		r.rules,
		nextState,
		r.validators,
		r.votingPower,
	)
	if err != nil {
		return err
	}
	precommits, err := NewVoteAggregator(
		r.rules,
		nextState,
		r.validators,
		r.votingPower,
	)
	if err != nil {
		return err
	}

	r.state = nextState
	r.prevotes = &prevotes
	r.precommits = &precommits
	r.proposal = nil
	r.certificate = nil
	return nil
}

// AdvanceRoundWithTimeoutEvidence authenticates signed timeout messages, builds
// deterministic timeout evidence, and only then advances the runtime to the
// certificate's target round. Failed evidence validation leaves the runtime
// unchanged.
func (r *ValidatorRuntime) AdvanceRoundWithTimeoutEvidence(
	messages []Message,
	resolver TimeoutAuthorityResolver,
) (TimeoutCertificate, error) {
	if r == nil {
		return TimeoutCertificate{}, ErrInvalidConsensusRuntime
	}
	certificate, err := NewTimeoutCertificateFromMessages(
		r.state,
		r.validators,
		r.votingPower,
		r.threshold,
		messages,
		r.rules,
		resolver,
	)
	if err != nil {
		return TimeoutCertificate{}, err
	}
	if err := ValidateTimeoutCertificate(
		certificate,
		r.state,
		r.validators,
		r.votingPower,
	); err != nil {
		return TimeoutCertificate{}, err
	}
	if len(r.lockedProposal) > 0 {
		if !bytes.Equal(r.lockedProposal, certificate.LockedProposal) {
			return TimeoutCertificate{}, ErrConflictingTimeoutLock
		}
		if len(certificate.LockedProposal) == 0 {
			return TimeoutCertificate{}, ErrConflictingTimeoutLock
		}
	}
	if err := r.AdvanceRound(certificate.NextRound); err != nil {
		return TimeoutCertificate{}, err
	}
	if len(certificate.LockedProposal) > 0 && (len(r.lockedProposal) == 0 || certificate.LockedRound > r.lockedRound) {
		r.lockedProposal = append([]byte(nil), certificate.LockedProposal...)
		r.lockedRound = certificate.LockedRound
		r.lockedProof = cloneLockProofPtr(certificate.LockProof)
	}
	return certificate, nil
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
	// Legacy MessageTypeVote remains accepted as a compatibility input and is
	// normalized into the explicit phase-specific evidence bucket.
	if msg.Type == MessageTypeVote {
		if r.state.Phase != PhasePrevote {
			return ErrInvalidRuntimeVoteType
		}
		msg.Type = MessageTypePrevote
	}
	if r.state.Phase == PhasePrevote && msg.Type != MessageTypePrevote {
		return ErrInvalidRuntimeVoteType
	}
	if r.state.Phase == PhasePrecommit && msg.Type != MessageTypePrecommit {
		return ErrInvalidRuntimeVoteType
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

	if r.state.Phase == PhasePrevote {
		if err := r.prevotes.AddVote(msg); err != nil {
			return err
		}
		reached, err := r.prevotes.QuorumForPayload(r.proposal, r.threshold)
		if err != nil {
			return err
		}
		if reached {
			r.lockedProposal = append([]byte(nil), r.proposal...)
			r.lockedRound = r.state.Round
			r.lockedProof = nil
			r.state.Phase = PhasePrecommit
		}
		return nil
	}

	if err := r.precommits.AddVote(msg); err != nil {
		return err
	}
	reached, err := r.precommits.QuorumForPayload(r.proposal, r.threshold)
	if err != nil { return err }
	if reached {
		certificate, err := NewPrecommitCertificate(
			r.state, r.validators, r.votingPower, r.threshold,
			r.proposal, r.precommits.VotesForPayload(r.proposal),
		)
		if err != nil { return err }
		proof, err := NewLockProof(r.state.Round, r.proposal, certificate)
		if err != nil { return err }
		r.lockedProof = &proof
	}
	return nil
}

func (r *ValidatorRuntime) FinalizeProposal(resolver TimeoutAuthorityResolver) (FinalityCertificate, error) {
	if r == nil {
		return FinalityCertificate{}, ErrInvalidConsensusRuntime
	}
	if r.state.Phase != PhasePrecommit {
		return FinalityCertificate{}, ErrInvalidRuntimePhase
	}
	if len(r.lockedProposal) == 0 || !bytes.Equal(r.lockedProposal, r.proposal) {
		return FinalityCertificate{}, ErrConflictingLockedProposal
	}

	precommitCertificate, err := NewPrecommitCertificate(
		r.state,
		r.validators,
		r.votingPower,
		r.threshold,
		r.proposal,
		r.precommits.VotesForPayload(r.proposal),
	)
	if err != nil {
		return FinalityCertificate{}, err
	}
	if err := ValidatePrecommitCertificateWithAuthority(
		precommitCertificate,
		r.state,
		r.validators,
		r.votingPower,
		resolver,
	); err != nil {
		return FinalityCertificate{}, err
	}
	proof, err := NewLockProof(r.state.Round, r.proposal, precommitCertificate)
	if err != nil { return FinalityCertificate{}, err }
	if err := ValidateLockProofWithAuthority(
		proof,
		r.state,
		r.validators,
		r.votingPower,
		resolver,
	); err != nil {
		return FinalityCertificate{}, err
	}

	certificate, err := NewFinalityCertificate(
		r.state,
		r.validators,
		r.votingPower,
		r.threshold,
		r.proposal,
		precommitCertificate.Votes,
	)
	if err != nil {
		return FinalityCertificate{}, err
	}
	if err := ValidateFinalityCertificate(
		certificate,
		r.state,
		r.validators,
		r.votingPower,
	); err != nil {
		return FinalityCertificate{}, err
	}
	if !sameConsensusEvidence(certificate.Votes, precommitCertificate.Votes) {
		return FinalityCertificate{}, ErrInvalidFinalityCertificate
	}

	// The finality certificate is derived directly from the authenticated
	// precommit certificate above. Requiring exact vote/sender/payload/signature
	// equality prevents a second, unauthenticated evidence set from being
	// substituted between authenticated precommit validation and finalization.
	// All authenticated evidence has passed. Only now mutate finalized state.
	r.lockedProof = &proof
	r.certificate = &certificate
	r.state.Phase = PhaseFinalized
	return certificate, nil
}

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

func sameConsensusEvidence(a, b []Message) bool {
	if len(a) != len(b) { return false }
	for i := range a {
		if !bytes.Equal(a[i].Sender, b[i].Sender) ||
			a[i].Type != b[i].Type ||
			!bytes.Equal(a[i].Payload, b[i].Payload) ||
			!bytes.Equal(a[i].Signature, b[i].Signature) {
			return false
		}
	}
	return true
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
