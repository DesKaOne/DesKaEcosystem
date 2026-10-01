package node

import (
	"errors"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/consensus"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/storage"
)

var ErrHistoricalArtifactsRetained = errors.New("historical finality artifacts remain inside retention window")

// PruneHistoricalFinalityArtifacts applies a deterministic height/epoch
// retention policy to one explicitly identified finality artifact set.
// Policy evaluation is completed before any deletion, so an ineligible
// artifact cannot cause partial cleanup.
func (n *Node) PruneHistoricalFinalityArtifacts(
	policy consensus.ArtifactRetentionPolicy,
	currentEpoch uint64,
	candidateStore storage.CandidateStore,
	evidenceStore consensus.EvidenceStore,
	candidateKey storage.CandidateKey,
	evidence []consensus.Message,
) (FinalityArtifactPruneResult, error) {
	if n == nil || n.Store == nil || n.State == nil {
		return FinalityArtifactPruneResult{}, ErrNilStore
	}
	if err := policy.Validate(); err != nil {
		return FinalityArtifactPruneResult{}, err
	}

	prunable, err := policy.CandidatePrunable(n.Head.Header.Height, candidateKey.Height)
	if err != nil {
		return FinalityArtifactPruneResult{}, err
	}
	if !prunable {
		return FinalityArtifactPruneResult{}, ErrHistoricalArtifactsRetained
	}

	for _, msg := range evidence {
		prunable, err := policy.EvidencePrunable(
			n.Head.Header.Height,
			msg.Height,
			currentEpoch,
			msg.Epoch,
		)
		if err != nil {
			return FinalityArtifactPruneResult{}, err
		}
		if !prunable {
			return FinalityArtifactPruneResult{}, ErrHistoricalArtifactsRetained
		}
	}

	return n.PruneFinalityArtifacts(
		candidateStore,
		evidenceStore,
		candidateKey,
		evidence,
	)
}
