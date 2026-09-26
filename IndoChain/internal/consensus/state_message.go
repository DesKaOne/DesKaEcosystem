package consensus

import "fmt"

var ErrConsensusMessageContextMismatch = fmt.Errorf("consensus message context mismatch")

// ValidateMessageAgainstState verifies that a consensus message belongs to
// the exact protocol, chain, epoch, height, and round represented by state.
//
// This is a development boundary between the message and round-state layers.
// It does not validate message type semantics, proposer authority, validator
// membership, voting power, quorum, or finality.
func ValidateMessageAgainstState(state RoundState, msg Message) error {
	if err := state.Validate(); err != nil {
		return err
	}
	if msg.ProtocolVersion != state.ProtocolVersion ||
		msg.ChainID != state.ChainID ||
		msg.Epoch != state.Epoch ||
		msg.Height != state.Height ||
		msg.Round != state.Round {
		return fmt.Errorf("%w: state=%d/%d/%d/%d/%d message=%d/%d/%d/%d/%d",
			ErrConsensusMessageContextMismatch,
			state.ProtocolVersion, state.ChainID, state.Epoch, state.Height, state.Round,
			msg.ProtocolVersion, msg.ChainID, msg.Epoch, msg.Height, msg.Round,
		)
	}
	return nil
}
