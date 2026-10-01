package consensus

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
)

var (
	ErrInvalidCanonicalCommit         = errors.New("invalid canonical commit")
	ErrCanonicalCommitContextMismatch = errors.New("canonical commit context mismatch")
	ErrNoCanonicalCommitPublication   = errors.New("no canonical commit publication")
)

// CanonicalCommit is the deterministic publication produced by the node only
// after a block and its resulting state have been durably committed.
type CanonicalCommit struct {
	Height    types.Height
	BlockHash types.Hash
	StateRoot types.Hash
}

func NewCanonicalCommit(height types.Height, blockHash, stateRoot types.Hash) (CanonicalCommit, error) {
	if height == 0 || blockHash == (types.Hash{}) || stateRoot == (types.Hash{}) {
		return CanonicalCommit{}, ErrInvalidCanonicalCommit
	}
	return CanonicalCommit{Height: height, BlockHash: blockHash, StateRoot: stateRoot}, nil
}

func (r *ValidatorRuntime) ValidateCanonicalCommit(commit CanonicalCommit) error {
	if r == nil {
		return ErrInvalidConsensusRuntime
	}
	if err := r.state.Validate(); err != nil {
		return err
	}
	if commit.Height == 0 || commit.BlockHash == (types.Hash{}) || commit.StateRoot == (types.Hash{}) {
		return ErrInvalidCanonicalCommit
	}
	if r.state.Phase != PhaseFinalized {
		return ErrInvalidRuntimePhase
	}
	expectedHeight := r.state.Height + 1
	if commit.Height != expectedHeight {
		return fmt.Errorf("%w: expected height %d got %d", ErrCanonicalCommitContextMismatch, expectedHeight, commit.Height)
	}
	if len(r.proposal) == 0 || !bytes.Equal(r.proposal, commit.BlockHash[:]) {
		return fmt.Errorf("%w: committed block hash does not match finalized proposal", ErrCanonicalCommitContextMismatch)
	}
	return nil
}

func (r *ValidatorRuntime) PublishCanonicalCommit(commit CanonicalCommit) error {
	if err := r.ValidateCanonicalCommit(commit); err != nil {
		return err
	}
	nextState, err := r.state.AdvanceHeight(commit.Height)
	if err != nil {
		return err
	}
	prevotes, err := NewVoteAggregator(r.rules, nextState, r.validators, r.votingPower)
	if err != nil {
		return err
	}
	precommits, err := NewVoteAggregator(r.rules, nextState, r.validators, r.votingPower)
	if err != nil {
		return err
	}
	r.state = nextState
	r.prevotes = &prevotes
	r.precommits = &precommits
	r.proposal = nil
	r.lockedProposal = nil
	r.lockedRound = 0
	r.lockedProof = nil
	r.certificate = nil
	r.lastCanonicalCommit = &CanonicalCommit{Height: commit.Height, BlockHash: commit.BlockHash, StateRoot: commit.StateRoot}
	return nil
}

