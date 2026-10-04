package consensus

import "fmt"

// MessageValidationContext combines the deterministic consensus boundaries
// already implemented for message structure, round-state context, and
// validator membership. It intentionally does not assign voting power,
// proposer eligibility, quorum, or finality.
type MessageValidationContext struct {
	Rules      ValidationRules
	State      RoundState
	Validators ValidatorSet
}

// ValidateConsensusMessage applies the currently available consensus-message
// validation boundaries in dependency order.
func ValidateConsensusMessage(msg Message, ctx MessageValidationContext) error {
	if err := ValidateMessage(msg, ctx.Rules); err != nil {
		return err
	}
	if err := ValidateMessageAgainstState(ctx.State, msg); err != nil {
		return err
	}
	if err := ValidateMessageSender(msg, ctx.Validators); err != nil {
		return fmt.Errorf("consensus message sender validation: %w", err)
	}
	return nil
}
