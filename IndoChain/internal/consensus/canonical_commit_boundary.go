package consensus

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/block"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/state"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
)

var (
	ErrNilCanonicalCommitter       = errors.New("nil canonical committer")
	ErrInvalidCanonicalCommit      = errors.New("invalid canonical commit candidate")
	ErrCanonicalCommitFailed       = errors.New("canonical commit failed")
	ErrCanonicalCommitHashMismatch = errors.New("canonical commit hash mismatch")
)

// CanonicalCommitCandidate is the fully validated candidate handed from
// consensus/execution coordination to the canonical storage boundary.
//
// The candidate is deliberately immutable by convention: implementations must
// not publish it as canonical until CommitBlockState succeeds.
type CanonicalCommitCandidate struct {
	Block block.Block
	Hash  types.Hash
	State *state.State
}

// CanonicalCommitter is the narrow consensus-to-storage handoff contract.
// storage.ChainStore implementations satisfy this interface structurally.
// Consensus does not own persistence, filesystem I/O, or durable atomicity.
type CanonicalCommitter interface {
	CommitBlockState(block.Block, types.Hash, *state.State) error
}

// ValidateCanonicalCommitCandidate verifies the handoff payload before the
// canonical store is called. It does not mutate the candidate or canonical
// state.
func ValidateCanonicalCommitCandidate(candidate CanonicalCommitCandidate) error {
	if candidate.State == nil {
		return ErrInvalidCanonicalCommit
	}
	if candidate.Hash == (types.Hash{}) {
		return ErrInvalidCanonicalCommit
	}
	computed, err := block.Hash(candidate.Block)
	if err != nil {
		return fmt.Errorf("%w: hash block: %v", ErrInvalidCanonicalCommit, err)
	}
	if computed != candidate.Hash {
		return ErrCanonicalCommitHashMismatch
	}
	if candidate.Block.Header.StateRoot != (types.Hash{}) &&
		candidate.Block.Header.StateRoot != candidate.State.Root() {
		return fmt.Errorf("%w: state root mismatch", ErrInvalidCanonicalCommit)
	}
	return nil
}

// CommitCanonicalCandidate performs only the canonical handoff. Consensus
// runtime/recovery publication must happen after this function returns nil.
// A commit error is returned without publishing any consensus runtime state.
func CommitCanonicalCandidate(committer CanonicalCommitter, candidate CanonicalCommitCandidate) error {
	if committer == nil {
		return ErrNilCanonicalCommitter
	}
	if err := ValidateCanonicalCommitCandidate(candidate); err != nil {
		return err
	}
	if err := committer.CommitBlockState(candidate.Block, candidate.Hash, candidate.State); err != nil {
		return fmt.Errorf("%w: %w", ErrCanonicalCommitFailed, err)
	}
	return nil
}


var ErrCanonicalCommitPublicationContextMismatch = errors.New("canonical commit publication context mismatch")

// CanonicalCommitPublication carries the exact canonical outputs observed by
// consensus after durable node commit. It does not grant consensus ownership
// of canonical storage.
type CanonicalCommitPublication struct {
	Height    types.Height
	BlockHash types.Hash
	StateRoot types.Hash
}

func ValidateCanonicalCommitPublication(runtime *ValidatorRuntime, publication CanonicalCommitPublication) error {
	if runtime == nil {
		return ErrInvalidConsensusRuntime
	}
	if err := runtime.state.Validate(); err != nil {
		return err
	}
	if publication.Height == 0 || publication.BlockHash == (types.Hash{}) || publication.StateRoot == (types.Hash{}) {
		return ErrInvalidCanonicalCommit
	}
	if runtime.state.Phase != PhaseFinalized {
		return ErrInvalidRuntimePhase
	}
	expectedHeight := runtime.state.Height + 1
	if publication.Height != expectedHeight {
		return fmt.Errorf("%w: expected height %d got %d", ErrCanonicalCommitPublicationContextMismatch, expectedHeight, publication.Height)
	}
	if len(runtime.proposal) == 0 || !bytes.Equal(runtime.proposal, publication.BlockHash[:]) {
		return fmt.Errorf("%w: finalized proposal hash mismatch", ErrCanonicalCommitPublicationContextMismatch)
	}
	return nil
}

// PublishCanonicalCommit consumes a publication emitted only after canonical
// storage commit and deterministically opens the next proposal height.
func (r *ValidatorRuntime) PublishCanonicalCommit(publication CanonicalCommitPublication) error {
	if err := ValidateCanonicalCommitPublication(r, publication); err != nil {
		return err
	}
	nextState, err := r.state.AdvanceHeight(publication.Height)
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
	return nil
}
