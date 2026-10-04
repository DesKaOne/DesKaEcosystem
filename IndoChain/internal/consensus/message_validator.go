package consensus

import "fmt"

var ErrConsensusMessageUnauthorized = fmt.Errorf("consensus message sender is not an active validator")

// ValidateMessageSender verifies that a message sender belongs to the supplied
// validator membership. It does not assign voting power or determine quorum.
func ValidateMessageSender(msg Message, validators ValidatorSet) error {
	if err := validators.Validate(); err != nil {
		return err
	}
	if len(msg.Sender) == 0 {
		return ErrMissingSender
	}
	if !validators.Contains(msg.Sender) {
		return fmt.Errorf("%w: sender not found", ErrConsensusMessageUnauthorized)
	}
	return nil
}
