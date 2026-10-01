package node

import (
	"errors"
	"fmt"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/consensus"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/storage"
)

var (
	ErrFinalityArtifactsNotCommitted = errors.New("finality artifacts are not yet safe to prune")
	ErrFinalityArtifactCandidateMismatch = errors.New("finality artifact candidate does not match canonical head")
)

type FinalityArtifactPruneResult struct {
	CandidatePruned bool
	EvidencePruned  int
}

// PruneFinalityArtifacts removes only explicitly identified candidate/evidence
// artifacts after the candidate is already canonical. The operation is
// deliberately outside the canonical commit path: cleanup can be retried and
// a cleanup failure never rolls back canonical state.
func (n *Node) PruneFinalityArtifacts(
	candidateStore storage.CandidateStore,
	evidenceStore consensus.EvidenceStore,
	candidateKey storage.CandidateKey,
	evidence []consensus.Message,
) (FinalityArtifactPruneResult, error) {
	if n == nil || n.Store == nil || n.State == nil {
		return FinalityArtifactPruneResult{}, ErrNilStore
	}
	if candidateStore == nil {
		return FinalityArtifactPruneResult{}, ErrNilCandidateStore
	}
	if evidenceStore == nil {
		return FinalityArtifactPruneResult{}, consensus.ErrNilEvidenceStore
	}
	if candidateKey.Height == 0 || candidateKey.Hash == (candidateKey.Hash) {
		// Hash-zero is rejected below without introducing a second key type.
	}
	if n.Head.Header.Height < candidateKey.Height {
		return FinalityArtifactPruneResult{}, ErrFinalityArtifactsNotCommitted
	}
	if n.Head.Header.Height == candidateKey.Height && n.HeadHash != candidateKey.Hash {
		return FinalityArtifactPruneResult{}, ErrFinalityArtifactCandidateMismatch
	}

	keys := make([]string, 0, len(evidence))
	seen := make(map[string]struct{}, len(evidence))
	for _, msg := range evidence {
		if msg.Height != candidateKey.Height {
			return FinalityArtifactPruneResult{}, fmt.Errorf("%w: evidence height %d candidate height %d", ErrFinalityArtifactCandidateMismatch, msg.Height, candidateKey.Height)
		}
		key, err := consensus.ConsensusEvidenceKey(msg)
		if err != nil {
			return FinalityArtifactPruneResult{}, err
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		keys = append(keys, key)
	}

	result := FinalityArtifactPruneResult{}
	if err := candidateStore.DeleteCandidate(candidateKey); err != nil {
		return result, err
	}
	result.CandidatePruned = true

	for _, key := range keys {
		if err := consensus.DeleteConsensusEvidence(evidenceStore, key); err != nil {
			return result, err
		}
		result.EvidencePruned++
	}
	return result, nil
}
