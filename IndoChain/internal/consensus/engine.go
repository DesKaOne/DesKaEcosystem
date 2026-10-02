package consensus

import (
	"errors"
	"time"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/crypto"
)

var (
	ErrNilConsensusEngine = errors.New("nil consensus engine")
	ErrInvalidConsensusEngine = errors.New("invalid consensus engine")
	ErrStaleConsensusTimeout = errors.New("stale consensus timeout")
	ErrConsensusTimeoutFinalized = errors.New("consensus timeout after finalization")
)

// TimeoutPolicy defines deterministic phase-local timeout durations. The
// scheduler/clock remains outside consensus; the engine owns only the token
// that identifies the currently armed timeout.
type TimeoutPolicy struct {
	Proposal  time.Duration
	Prevote   time.Duration
	Precommit time.Duration
}

func (p TimeoutPolicy) Validate() error {
	if p.Proposal <= 0 || p.Prevote <= 0 || p.Precommit <= 0 {
		return ErrInvalidConsensusEngine
	}
	return nil
}

func (p TimeoutPolicy) ForPhase(phase Phase) (time.Duration, error) {
	switch phase {
	case PhaseProposal:
		return p.Proposal, nil
	case PhasePrevote:
		return p.Prevote, nil
	case PhasePrecommit:
		return p.Precommit, nil
	case PhaseFinalized:
		return 0, ErrConsensusTimeoutFinalized
	default:
		return 0, ErrInvalidConsensusPhase
	}
}

// TimeoutToken is an opaque identity for one armed local timeout. Height,
// round, phase and generation are all checked before the timeout can mutate
// the consensus runtime, making cancelled/stale timer delivery fail closed.
type TimeoutToken struct {
	Height     uint64
	Round      uint64
	Phase      Phase
	Generation uint64
}

// ConsensusEngine is the production orchestration layer above ValidatorRuntime.
// It owns local timeout lifecycle and event routing, but does not own clocks,
// P2P transport, canonical storage, or application policy.
type ConsensusEngine struct {
	runtime    *ValidatorRuntime
	driver     *RoundDriver
	validator  []byte
	signer     crypto.Signer
	policy     TimeoutPolicy
	generation uint64
	armed      bool
}

func NewConsensusEngine(
	runtime *ValidatorRuntime,
	authority TimeoutAuthorityResolver,
	validatorID []byte,
	signer crypto.Signer,
	policy TimeoutPolicy,
) (*ConsensusEngine, error) {
	if runtime == nil {
		return nil, ErrNilConsensusEngine
	}
	if authority == nil || len(validatorID) == 0 || signer == nil {
		return nil, ErrInvalidConsensusEngine
	}
	if err := policy.Validate(); err != nil {
		return nil, err
	}
	if !runtime.Validators().Contains(validatorID) {
		return nil, ErrValidatorNotFound
	}
	if _, ok := runtime.votingPower.PowerOf(validatorID); !ok {
		return nil, ErrVoteSenderNotInVotingPower
	}
	driver, err := NewRoundDriver(runtime, authority)
	if err != nil {
		return nil, err
	}
	return &ConsensusEngine{
		runtime:   runtime,
		driver:    driver,
		validator: append([]byte(nil), validatorID...),
		signer:    signer,
		policy:    policy,
	}, nil
}

func (e *ConsensusEngine) Runtime() *ValidatorRuntime {
	if e == nil {
		return nil
	}
	return e.runtime
}

func (e *ConsensusEngine) Driver() *RoundDriver {
	if e == nil {
		return nil
	}
	return e.driver
}

// ValidatorID returns the local validator identity used for authenticated
// consensus events. Callers receive a defensive copy.
func (e *ConsensusEngine) ValidatorID() []byte {
	if e == nil {
		return nil
	}
	return append([]byte(nil), e.validator...)
}

// ExpectedProposer returns the proposer selected by the validator runtime for
// the current height/round. Proposal handoff callers must use this authority
// instead of deriving proposer identity from local policy.
func (e *ConsensusEngine) ExpectedProposer() ([]byte, error) {
	if e == nil || e.runtime == nil {
		return nil, ErrNilConsensusEngine
	}
	return e.runtime.ExpectedProposer()
}

// ArmTimeout invalidates every previous timeout token and returns a new token
// for the runtime's exact height/round/phase.
func (e *ConsensusEngine) ArmTimeout() (TimeoutToken, time.Duration, error) {
	if e == nil || e.runtime == nil {
		return TimeoutToken{}, 0, ErrNilConsensusEngine
	}
	state := e.runtime.State()
	if err := state.Validate(); err != nil {
		return TimeoutToken{}, 0, err
	}
	duration, err := e.policy.ForPhase(state.Phase)
	if err != nil {
		return TimeoutToken{}, 0, err
	}
	e.generation++
	e.armed = true
	return TimeoutToken{
		Height: uint64(state.Height), Round: state.Round,
		Phase: state.Phase, Generation: e.generation,
	}, duration, nil
}

// CancelTimeout invalidates the currently armed timeout without changing
// consensus state.
func (e *ConsensusEngine) CancelTimeout() {
	if e == nil {
		return
	}
	e.generation++
	e.armed = false
}

func (e *ConsensusEngine) HandleMessage(msg Message) error {
	if e == nil || e.driver == nil {
		return ErrNilConsensusEngine
	}
	before := e.runtime.State()
	if err := e.driver.HandleMessage(msg); err != nil {
		return err
	}
	after := e.runtime.State()
	if before != after {
		_, _, _ = e.ArmTimeout()
	}
	return nil
}

// HandleTimeout consumes exactly one currently armed timeout token and returns
// the authenticated timeout evidence that the caller should disseminate.
// Quorum-backed round advancement remains separate: receiving the other
// validators' timeout evidence is required before the runtime changes round.
func (e *ConsensusEngine) HandleTimeout(token TimeoutToken) (Message, error) {
	if e == nil || e.runtime == nil || e.driver == nil {
		return Message{}, ErrNilConsensusEngine
	}
	state := e.runtime.State()
	if state.Phase == PhaseFinalized {
		return Message{}, ErrConsensusTimeoutFinalized
	}
	if !e.armed || token.Generation != e.generation ||
		token.Height != uint64(state.Height) ||
		token.Round != state.Round || token.Phase != state.Phase {
		return Message{}, ErrStaleConsensusTimeout
	}
	e.armed = false

	var (
		msg Message
		err error
	)
	if e.runtime.lockedProof != nil {
		msg, err = NewTimeoutMessageWithLockProof(
			state, e.validator, state.Round+1, *e.runtime.lockedProof, e.signer,
		)
	} else {
		msg, err = NewTimeoutMessageWithLockRound(
			state, e.validator, state.Round+1, e.runtime.lockedRound,
			e.runtime.lockedProposal, e.signer,
		)
	}
	if err != nil {
		return Message{}, err
	}
	if err := e.driver.HandleMessage(msg); err != nil {
		return Message{}, err
	}
	return msg, nil
}

// TryAdvanceRound consumes the currently collected timeout evidence. On
// success it arms the timeout for the new proposal phase. If quorum is not
// reached, evidence remains queued in the driver for later retry.
func (e *ConsensusEngine) TryAdvanceRound() (TimeoutCertificate, error) {
	if e == nil || e.driver == nil {
		return TimeoutCertificate{}, ErrNilConsensusEngine
	}
	certificate, err := e.driver.AdvanceRoundFromTimeoutEvidence()
	if err != nil {
		return TimeoutCertificate{}, err
	}
	if _, _, err := e.ArmTimeout(); err != nil {
		return TimeoutCertificate{}, err
	}
	return certificate, nil
}

func (e *ConsensusEngine) ArmedTimeout() bool {
	return e != nil && e.armed
}
