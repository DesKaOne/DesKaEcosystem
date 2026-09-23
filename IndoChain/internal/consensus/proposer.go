package consensus

import (
	"errors"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
)

var (
	ErrNoValidators = errors.New("no validators available")
	ErrInvalidProposerRound = errors.New("invalid proposer round")
)

// ProposerSelector is the consensus boundary used to determine the expected
// proposer for a round. Production proposer policy remains an open protocol
// decision.
type ProposerSelector interface {
	Proposer(state RoundState, validators ValidatorSet) ([]byte, error)
}

// RoundRobinProposer is a deterministic development selector. It walks the
// canonical byte-sorted ValidatorSet by round and does not model stake,
// voting power, proposer priority, randomness, or slashing.
type RoundRobinProposer struct{}

func (RoundRobinProposer) Proposer(state RoundState, validators ValidatorSet) ([]byte, error) {
	if err := state.Validate(); err != nil {
		return nil, err
	}
	if err := validators.Validate(); err != nil {
		return nil, err
	}
	if len(validators.Validators) == 0 {
		return nil, ErrNoValidators
	}
	if state.Round > uint64(^uint(0)) {
		return nil, ErrInvalidProposerRound
	}

	index := state.Round % uint64(len(validators.Validators))
	return append([]byte(nil), validators.Validators[index]...), nil
}

func expectedProposerIndex(round uint64, validatorCount int) (int, error) {
	if validatorCount <= 0 {
		return 0, ErrNoValidators
	}
	if types.Height(round) < 0 {
		return 0, ErrInvalidProposerRound
	}
	return int(round % uint64(validatorCount)), nil
}
