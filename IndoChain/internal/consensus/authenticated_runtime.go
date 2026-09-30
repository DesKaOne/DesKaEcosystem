package consensus

import "errors"

var ErrAuthenticatedConsensusAuthorityMissing = errors.New("authenticated consensus authority resolver missing")

// AcceptAuthenticatedProposal verifies the proposal signature against the
// validator authority before handing it to the runtime state machine.
// The legacy AcceptProposal path remains available as a development boundary.
func (r *ValidatorRuntime) AcceptAuthenticatedProposal(
	msg Message,
	resolver TimeoutAuthorityResolver,
) error {
	if r == nil {
		return ErrInvalidConsensusRuntime
	}
	if resolver == nil {
		return ErrAuthenticatedConsensusAuthorityMissing
	}
	if msg.Type != MessageTypeProposal {
		return ErrInvalidConsensusMessage
	}
	if err := ValidateConsensusMessage(msg, MessageValidationContext{
		Rules: ValidationRules{
			ProtocolVersion: r.rules.ProtocolVersion,
			ChainID: r.rules.ChainID,
			MaxPayloadSize: r.rules.MaxPayloadSize,
			RequireSender: true,
			RequireSignature: true,
		},
		State: r.state,
		Validators: r.validators,
	}); err != nil {
		return err
	}
	if err := verifyValidatorMessageSignature(msg, resolver); err != nil {
		return err
	}
	return r.AcceptProposal(msg)
}

// AddAuthenticatedVote verifies a prevote/precommit signature before the vote
// reaches the round-local aggregation state. MessageTypeVote is normalized by
// the existing AddVote compatibility path.
func (r *ValidatorRuntime) AddAuthenticatedVote(
	msg Message,
	resolver TimeoutAuthorityResolver,
) error {
	if r == nil {
		return ErrInvalidConsensusRuntime
	}
	if resolver == nil {
		return ErrAuthenticatedConsensusAuthorityMissing
	}
	if msg.Type != MessageTypeVote &&
		msg.Type != MessageTypePrevote &&
		msg.Type != MessageTypePrecommit {
		return ErrInvalidRuntimeVoteType
	}
	if err := ValidateConsensusMessage(msg, MessageValidationContext{
		Rules: ValidationRules{
			ProtocolVersion: r.rules.ProtocolVersion,
			ChainID: r.rules.ChainID,
			MaxPayloadSize: r.rules.MaxPayloadSize,
			RequireSender: true,
			RequireSignature: true,
		},
		State: r.state,
		Validators: r.validators,
	}); err != nil {
		return err
	}
	if err := verifyValidatorMessageSignature(msg, resolver); err != nil {
		return err
	}
	return r.AddVote(msg)
}
