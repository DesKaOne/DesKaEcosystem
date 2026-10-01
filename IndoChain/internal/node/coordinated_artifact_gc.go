package node

import (
	"bytes"
	"errors"
	"sort"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/consensus"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/storage"
)

var (
	ErrCoordinatedGCContextMismatch = errors.New("coordinated artifact GC context mismatch")
	ErrCoordinatedGCEvidenceMismatch = errors.New("coordinated artifact GC evidence mismatch")
)

// ApplyCoordinatedArtifactGC validates a quorum-backed cleanup authorization
// against this node's local canonical context and artifact identities, then
// delegates to the existing retention/lifecycle boundaries. It never mutates
// canonical state or consensus runtime.
func (n *Node) ApplyCoordinatedArtifactGC(
	decision consensus.ArtifactGCDecision,
	validators consensus.ValidatorSet,
	votingPower consensus.VotingPowerSet,
	validatorResolver ValidatorAuthorityResolver,
	candidateStore storage.CandidateStore,
	evidenceStore consensus.EvidenceStore,
	evidence []consensus.Message,
) (FinalityArtifactPruneResult, error) {
	if n == nil || n.Store == nil || n.State == nil {
		return FinalityArtifactPruneResult{}, ErrNilStore
	}
	if validatorResolver == nil {
		return FinalityArtifactPruneResult{}, ErrInvalidValidatorAuthority
	}
	if err := decision.Validate(validators, votingPower, validatorResolver); err != nil {
		return FinalityArtifactPruneResult{}, err
	}
	plan := decision.Plan
	if plan.ProtocolVersion != n.Config.ProtocolVersion ||
		plan.ChainID != n.Config.ChainID ||
		plan.CurrentEpoch > ^uint64(0) {
		return FinalityArtifactPruneResult{}, ErrCoordinatedGCContextMismatch
	}
	if n.Head.Header.Height != plan.CanonicalHeight {
		return FinalityArtifactPruneResult{}, ErrCoordinatedGCContextMismatch
	}

	candidateKey := storage.CandidateKey{
		Height: plan.CandidateHeight,
		Hash:   plan.CandidateHash,
	}
	keys := make([]string, 0, len(evidence))
	seen := make(map[string]struct{}, len(evidence))
	for _, msg := range evidence {
		key, err := consensus.ConsensusEvidenceKey(msg)
		if err != nil {
			return FinalityArtifactPruneResult{}, err
		}
		if _, ok := seen[key]; ok {
			return FinalityArtifactPruneResult{}, ErrCoordinatedGCEvidenceMismatch
		}
		seen[key] = struct{}{}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	if len(keys) != len(plan.EvidenceKeys) {
		return FinalityArtifactPruneResult{}, ErrCoordinatedGCEvidenceMismatch
	}
	for i := range keys {
		if keys[i] != plan.EvidenceKeys[i] {
			return FinalityArtifactPruneResult{}, ErrCoordinatedGCEvidenceMismatch
		}
	}

	return n.PruneHistoricalFinalityArtifacts(
		plan.Policy,
		plan.CurrentEpoch,
		candidateStore,
		evidenceStore,
		candidateKey,
		evidence,
	)
}

var _ = bytes.Equal
var _ types.Height
