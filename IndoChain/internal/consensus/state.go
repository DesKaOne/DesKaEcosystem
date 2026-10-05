package consensus

import (
	"errors"
	"fmt"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
)

type Phase uint8

const (
	PhaseProposal Phase = iota + 1
	PhasePrevote
	PhasePrecommit
	PhaseFinalized
)

var (
	ErrInvalidConsensusState = errors.New("invalid consensus state")
	ErrInvalidConsensusPhase = errors.New("invalid consensus phase")
	ErrStateContextMismatch = errors.New("consensus state context mismatch")
	ErrHeightRegression = errors.New("consensus height regression")
	ErrRoundRegression = errors.New("consensus round regression")
	ErrPhaseRegression = errors.New("consensus phase regression")
)

type RoundState struct {
	ProtocolVersion types.ProtocolVersion
	ChainID         types.ChainID
	Epoch           uint64
	Height          types.Height
	Round           uint64
	Phase           Phase
}

func NewRoundState(protocolVersion types.ProtocolVersion, chainID types.ChainID, epoch uint64, height types.Height) (RoundState, error) {
	if protocolVersion == 0 || chainID == 0 {
		return RoundState{}, ErrInvalidConsensusState
	}
	return RoundState{
		ProtocolVersion: protocolVersion,
		ChainID: chainID,
		Epoch: epoch,
		Height: height,
		Round: 0,
		Phase: PhaseProposal,
	}, nil
}

func (s RoundState) Validate() error {
	if s.ProtocolVersion == 0 || s.ChainID == 0 {
		return ErrInvalidConsensusState
	}
	switch s.Phase {
	case PhaseProposal, PhasePrevote, PhasePrecommit, PhaseFinalized:
	default:
		return ErrInvalidConsensusPhase
	}
	return nil
}

func (s RoundState) SameContext(other RoundState) bool {
	return s.ProtocolVersion == other.ProtocolVersion &&
		s.ChainID == other.ChainID &&
		s.Epoch == other.Epoch &&
		s.Height == other.Height
}

func (s RoundState) AdvancePhase(next Phase) (RoundState, error) {
	if err := s.Validate(); err != nil {
		return RoundState{}, err
	}
	if !validPhase(next) {
		return RoundState{}, ErrInvalidConsensusPhase
	}
	if next < s.Phase {
		return RoundState{}, fmt.Errorf("%w: %d -> %d", ErrPhaseRegression, s.Phase, next)
	}
	s.Phase = next
	return s, nil
}

func (s RoundState) AdvanceRound(next uint64) (RoundState, error) {
	if err := s.Validate(); err != nil {
		return RoundState{}, err
	}
	if next < s.Round {
		return RoundState{}, fmt.Errorf("%w: %d -> %d", ErrRoundRegression, s.Round, next)
	}
	if next == s.Round {
		return s, nil
	}
	s.Round = next
	s.Phase = PhaseProposal
	return s, nil
}

func (s RoundState) AdvanceHeight(nextHeight types.Height) (RoundState, error) {
	if err := s.Validate(); err != nil {
		return RoundState{}, err
	}
	if nextHeight < s.Height {
		return RoundState{}, fmt.Errorf("%w: %d -> %d", ErrHeightRegression, s.Height, nextHeight)
	}
	if nextHeight == s.Height {
		return s, nil
	}
	s.Height = nextHeight
	s.Round = 0
	s.Phase = PhaseProposal
	return s, nil
}

func validPhase(p Phase) bool {
	return p >= PhaseProposal && p <= PhaseFinalized
}
