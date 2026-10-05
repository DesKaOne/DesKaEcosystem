package node

import (
	"errors"
	"fmt"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/block"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/storage"
)

type RecoveryArtifactStatus uint8

const (
	RecoveryArtifactPending RecoveryArtifactStatus = iota + 1
	RecoveryArtifactCommitted
	RecoveryArtifactStale
	RecoveryArtifactContextMismatch
)

func (s RecoveryArtifactStatus) String() string {
	switch s {
	case RecoveryArtifactPending:
		return "pending"
	case RecoveryArtifactCommitted:
		return "committed"
	case RecoveryArtifactStale:
		return "stale"
	case RecoveryArtifactContextMismatch:
		return "context-mismatched"
	default:
		return "unknown"
	}
}

var (
	ErrNilRecoveryArtifactStore = errors.New("nil recovery artifact store")
	ErrInvalidRecoveryArtifact = errors.New("invalid recovery artifact")
)

type RecoveryArtifactReport struct {
	Key     storage.CandidateKey
	Status  RecoveryArtifactStatus
	Deleted bool
}

// ClassifyCandidateRecoveryArtifact deterministically classifies one candidate
// against the node's current canonical head. It never mutates canonical state
// or the candidate store.
func (n *Node) ClassifyCandidateRecoveryArtifact(store storage.CandidateStore, key storage.CandidateKey) (RecoveryArtifactReport, error) {
	if n == nil || n.Store == nil || n.State == nil {
		return RecoveryArtifactReport{}, ErrNilStore
	}
	if store == nil {
		return RecoveryArtifactReport{}, ErrNilRecoveryArtifactStore
	}
	candidate, err := store.GetCandidate(key)
	if err != nil {
		return RecoveryArtifactReport{}, err
	}
	candidateHash, err := block.Hash(candidate)
	if err != nil {
		return RecoveryArtifactReport{}, fmt.Errorf("%w: hash candidate: %v", ErrInvalidRecoveryArtifact, err)
	}
	if candidate.Header.Height != key.Height || candidateHash != key.Hash {
		return RecoveryArtifactReport{}, ErrInvalidRecoveryArtifact
	}

	if canonical, canonicalHash, err := n.Store.GetBlock(key.Height); err == nil {
		if canonicalHash == key.Hash {
			return RecoveryArtifactReport{Key: key, Status: RecoveryArtifactCommitted}, nil
		}
		canonicalComputed, hashErr := block.Hash(canonical)
		if hashErr == nil && canonicalComputed == key.Hash {
			return RecoveryArtifactReport{Key: key, Status: RecoveryArtifactCommitted}, nil
		}
		return RecoveryArtifactReport{Key: key, Status: RecoveryArtifactStale}, nil
	} else if !errors.Is(err, storage.ErrBlockNotFound) {
		return RecoveryArtifactReport{}, err
	}

	switch {
	case key.Height == n.Head.Header.Height+1 && candidate.Header.PreviousHash == n.HeadHash:
		return RecoveryArtifactReport{Key: key, Status: RecoveryArtifactPending}, nil
	case key.Height <= n.Head.Header.Height:
		return RecoveryArtifactReport{Key: key, Status: RecoveryArtifactStale}, nil
	default:
		return RecoveryArtifactReport{Key: key, Status: RecoveryArtifactContextMismatch}, nil
	}
}

// ReconcileCandidateRecoveryArtifact applies the node-owned cleanup policy:
// committed and stale artifacts are deleted; pending and context-mismatched
// artifacts are retained for recovery/diagnostics. Cleanup is idempotent.
func (n *Node) ReconcileCandidateRecoveryArtifact(store storage.CandidateStore, key storage.CandidateKey) (RecoveryArtifactReport, error) {
	report, err := n.ClassifyCandidateRecoveryArtifact(store, key)
	if err != nil {
		return RecoveryArtifactReport{}, err
	}
	switch report.Status {
	case RecoveryArtifactCommitted, RecoveryArtifactStale:
		if err := store.DeleteCandidate(key); err != nil {
			return report, err
		}
		report.Deleted = true
	}
	return report, nil
}

// CandidateRecoveryArtifactKey constructs the exact durable identity for a
// candidate without mutating the candidate store.
func CandidateRecoveryArtifactKey(candidate block.Block) (storage.CandidateKey, error) {
	hash, err := block.Hash(candidate)
	if err != nil {
		return storage.CandidateKey{}, err
	}
	return storage.CandidateKey{Height: candidate.Header.Height, Hash: hash}, nil
}

