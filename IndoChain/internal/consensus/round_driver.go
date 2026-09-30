package consensus

import (
	"errors"
	"fmt"
)

var (
	ErrNilRoundDriver           = errors.New("nil consensus round driver")
	ErrUnsupportedDriverMessage = errors.New("unsupported consensus round-driver message")
	ErrTimeoutEvidencePending   = errors.New("consensus timeout evidence pending")
)

// RoundDriver is the deterministic orchestration boundary above ValidatorRuntime.
//
// It intentionally does not own a clock, peer discovery, retransmission, or
// network failure detector. Those operational concerns remain outside the
// consensus state machine. The driver owns only message routing and timeout
// evidence collection, and every state transition is delegated to the runtime.
type RoundDriver struct {
	runtime         *ValidatorRuntime
	authority       TimeoutAuthorityResolver
	timeoutMessages []Message
}

// NewRoundDriver constructs an authenticated event-driven consensus driver.
func NewRoundDriver(
	runtime *ValidatorRuntime,
	authority TimeoutAuthorityResolver,
) (*RoundDriver, error) {
	if runtime == nil {
		return nil, ErrNilRoundDriver
	}
	if authority == nil {
		return nil, ErrAuthenticatedConsensusAuthorityMissing
	}
	return &RoundDriver{
		runtime:   runtime,
		authority: authority,
	}, nil
}

func (d *RoundDriver) Runtime() *ValidatorRuntime {
	if d == nil {
		return nil
	}
	return d.runtime
}

// HandleMessage routes one authenticated consensus message through the runtime.
//
// Proposal, prevote, and precommit messages are applied immediately. Timeout
// messages are queued until AdvanceRoundFromTimeoutEvidence is called so the
// caller can define the evidence batch boundary without introducing a hidden
// scheduler into consensus.
func (d *RoundDriver) HandleMessage(msg Message) error {
	if d == nil || d.runtime == nil {
		return ErrNilRoundDriver
	}
	if err := ValidateConsensusMessage(msg, MessageValidationContext{
		Rules: ValidationRules{
			ProtocolVersion: d.runtime.rules.ProtocolVersion,
			ChainID: d.runtime.rules.ChainID,
			MaxPayloadSize: d.runtime.rules.MaxPayloadSize,
			RequireSender: true,
			RequireSignature: true,
		},
		State: d.runtime.state,
		Validators: d.runtime.validators,
	}); err != nil {
		return err
	}
	if err := verifyValidatorMessageSignature(msg, d.authority); err != nil {
		return err
	}

	switch msg.Type {
	case MessageTypeProposal:
		return d.runtime.AcceptProposal(msg)
	case MessageTypeVote, MessageTypePrevote, MessageTypePrecommit:
		return d.runtime.AddVote(msg)
	case MessageTypeTimeout:
		d.timeoutMessages = append(d.timeoutMessages, cloneMessage(msg))
		return nil
	default:
		return fmt.Errorf("%w: type=%d", ErrUnsupportedDriverMessage, msg.Type)
	}
}

// TimeoutEvidence returns the currently queued timeout messages defensively.
func (d *RoundDriver) TimeoutEvidence() []Message {
	if d == nil {
		return nil
	}
	return cloneVotes(d.timeoutMessages)
}

// AdvanceRoundFromTimeoutEvidence submits the queued timeout batch atomically
// to ValidatorRuntime. The queue is cleared only after a successful transition.
func (d *RoundDriver) AdvanceRoundFromTimeoutEvidence() (TimeoutCertificate, error) {
	if d == nil || d.runtime == nil {
		return TimeoutCertificate{}, ErrNilRoundDriver
	}
	if len(d.timeoutMessages) == 0 {
		return TimeoutCertificate{}, ErrTimeoutEvidencePending
	}
	certificate, err := d.runtime.AdvanceRoundWithTimeoutEvidence(
		cloneVotes(d.timeoutMessages),
		d.authority,
	)
	if err != nil {
		return TimeoutCertificate{}, err
	}
	d.timeoutMessages = nil
	return certificate, nil
}

// ClearTimeoutEvidence explicitly discards queued timeout evidence. This is a
// driver operation only; it never mutates consensus runtime state.
func (d *RoundDriver) ClearTimeoutEvidence() {
	if d == nil {
		return
	}
	d.timeoutMessages = nil
}

func cloneMessage(msg Message) Message {
	msg.Sender = append([]byte(nil), msg.Sender...)
	msg.Payload = append([]byte(nil), msg.Payload...)
	msg.Signature = append([]byte(nil), msg.Signature...)
	return msg
}
