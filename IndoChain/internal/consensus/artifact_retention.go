package consensus

import (
	"errors"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
)

var (
	ErrInvalidRetentionPolicy = errors.New("invalid artifact retention policy")
	ErrRetentionContextMismatch = errors.New("retention context mismatch")
	ErrArtifactStillRetained = errors.New("artifact is still within retention window")
)

// ArtifactRetentionPolicy describes the minimum historical safety window for
// durable finality artifacts. It is a pure policy: it does not inspect stores,
// mutate consensus, or perform deletion.
type ArtifactRetentionPolicy struct {
	KeepRecentHeights uint64
	KeepRecentEpochs  uint64
}

// Validate checks policy parameters before they are used for pruning decisions.
func (p ArtifactRetentionPolicy) Validate() error {
	if p.KeepRecentHeights == 0 && p.KeepRecentEpochs == 0 {
		return ErrInvalidRetentionPolicy
	}
	return nil
}

// CandidatePrunable reports whether a candidate at height is outside the
// height-based recovery window. Candidates have no epoch in CandidateStore,
// so epoch retention is intentionally not inferred for them.
func (p ArtifactRetentionPolicy) CandidatePrunable(
	canonicalHeight types.Height,
	candidateHeight types.Height,
) (bool, error) {
	if err := p.Validate(); err != nil {
		return false, err
	}
	if candidateHeight > canonicalHeight {
		return false, ErrRetentionContextMismatch
	}
	if p.KeepRecentHeights == 0 {
		return false, nil
	}
	cutoff := uint64(canonicalHeight)
	if cutoff < p.KeepRecentHeights {
		return false, nil
	}
	return uint64(candidateHeight) <= cutoff-p.KeepRecentHeights, nil
}

// EvidencePrunable reports whether evidence is outside both configured
// recovery windows. Height is always checked; when epoch retention is enabled,
// evidence from an epoch still inside the retained epoch window is preserved.
func (p ArtifactRetentionPolicy) EvidencePrunable(
	canonicalHeight types.Height,
	evidenceHeight types.Height,
	currentEpoch uint64,
	evidenceEpoch uint64,
) (bool, error) {
	if err := p.Validate(); err != nil {
		return false, err
	}
	if evidenceHeight > canonicalHeight || evidenceEpoch > currentEpoch {
		return false, ErrRetentionContextMismatch
	}

	heightSafe := p.KeepRecentHeights == 0
	if p.KeepRecentHeights > 0 {
		cutoff := uint64(canonicalHeight)
		heightSafe = cutoff >= p.KeepRecentHeights &&
			uint64(evidenceHeight) <= cutoff-p.KeepRecentHeights
	}

	epochSafe := p.KeepRecentEpochs == 0
	if p.KeepRecentEpochs > 0 {
		epochCutoff := currentEpoch
		epochSafe = epochCutoff >= p.KeepRecentEpochs &&
			evidenceEpoch <= epochCutoff-p.KeepRecentEpochs
	}

	if !heightSafe || !epochSafe {
		return false, ErrArtifactStillRetained
	}
	return true, nil
}
