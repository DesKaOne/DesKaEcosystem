package consensus

import (
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
