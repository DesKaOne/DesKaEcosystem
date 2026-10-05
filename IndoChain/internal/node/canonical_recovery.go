package node

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/consensus"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/block"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/storage"
)

var (
	ErrFinalizedCommitNoValidEvidence = errors.New("no valid finality evidence")
)

type FinalizedCommitClassification uint8

const (
	FinalizedCommitNoValidEvidence FinalizedCommitClassification = iota
	FinalizedCommitEvidencePresentCanonicalMissing
	FinalizedCommitCanonicalMatched
	FinalizedCommitCanonicalContextMismatch
)

func (c FinalizedCommitClassification) String() string {
	switch c {
	case FinalizedCommitNoValidEvidence:
		return "NO_VALID_FINALITY_EVIDENCE"
	case FinalizedCommitEvidencePresentCanonicalMissing:
		return "FINALITY_EVIDENCE_PRESENT_CANONICAL_MISSING"
	case FinalizedCommitCanonicalMatched:
		return "FINALITY_EVIDENCE_PRESENT_CANONICAL_MATCHED"
	case FinalizedCommitCanonicalContextMismatch:
		return "CANONICAL_PRESENT_BUT_CONTEXT_MISMATCH"
	default:
		return "UNKNOWN_FINALIZED_COMMIT_CLASSIFICATION"
	}
}

// ClassifyFinalizedCommit validates the finality-to-canonical identity boundary
// and classifies the current canonical state without mutating storage.
// The candidate block is required because finality evidence identifies the
// finalized payload/hash but does not contain the complete block body.
func (n *Node) ClassifyFinalizedCommit(
	ctx consensus.BlockProductionContext,
	candidate block.Block,
	certificate consensus.FinalityCertificate,
	validators consensus.ValidatorSet,
	votingPower consensus.VotingPowerSet,
	validatorResolver ValidatorAuthorityResolver,
) (FinalizedCommitClassification, error) {
	if n == nil || n.Store == nil || n.State == nil {
		return FinalizedCommitNoValidEvidence, ErrNilStore
	}
	if validatorResolver == nil {
		return FinalizedCommitNoValidEvidence, errors.New("missing finalized-block validator resolver")
	}
	if len(certificate.Payload) == 0 || len(certificate.Votes) == 0 {
		return FinalizedCommitNoValidEvidence, ErrFinalizedCommitNoValidEvidence
	}

	candidateHash, err := block.Hash(candidate)
	if err != nil {
		return FinalizedCommitNoValidEvidence, fmt.Errorf("hash finalized candidate: %w", err)
	}
	if candidateHash == (types.Hash{}) || !bytes.Equal(certificate.Payload, candidateHash[:]) {
		return FinalizedCommitCanonicalContextMismatch, ErrConsensusContextMismatch
	}
	if err := validateCanonicalConsensusContext(n, ctx); err != nil {
		return FinalizedCommitCanonicalContextMismatch, err
	}
	if ctx.State.Height+1 != candidate.Header.Height ||
		candidate.Header.PreviousHash != n.HeadHash ||
		candidate.Header.ChainID != n.Config.ChainID ||
		candidate.Header.Version != n.Config.ProtocolVersion {
		return FinalizedCommitCanonicalContextMismatch, ErrConsensusContextMismatch
	}

	if _, err := consensus.ValidateFinalizedBlockWithAuthority(
		ctx, candidate, certificate, validators, votingPower, validatorResolver,
	); err != nil {
		return FinalizedCommitNoValidEvidence, fmt.Errorf("%w: %v", ErrFinalizedCommitNoValidEvidence, err)
	}

	canonicalBlock, canonicalHash, err := n.Store.GetBlock(candidate.Header.Height)
	if err == nil {
		if canonicalHash == candidateHash {
			return FinalizedCommitCanonicalMatched, nil
		}
		storedHash, hashErr := block.Hash(canonicalBlock)
		if hashErr == nil && storedHash == candidateHash {
			return FinalizedCommitCanonicalMatched, nil
		}
		return FinalizedCommitCanonicalContextMismatch, ErrConsensusContextMismatch
	}
	if !errors.Is(err, storage.ErrBlockNotFound) {
		return FinalizedCommitCanonicalContextMismatch, err
	}

	return FinalizedCommitEvidencePresentCanonicalMissing, nil
}
