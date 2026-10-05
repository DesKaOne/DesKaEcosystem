package consensus

import (
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
)

func TestArtifactRetentionPolicyCandidateWindow(t *testing.T) {
	policy := ArtifactRetentionPolicy{KeepRecentHeights: 3}
	if prunable, err := policy.CandidatePrunable(10, 7); err != nil || !prunable {
		t.Fatalf("expected height 7 to be prunable at canonical 10 with window 3, got prunable=%v err=%v", prunable, err)
	}
	if prunable, err := policy.CandidatePrunable(10, 8); err != nil || prunable {
		t.Fatalf("expected height 8 to remain retained, got prunable=%v err=%v", prunable, err)
	}
	if prunable, err := policy.CandidatePrunable(2, 1); err != nil || prunable {
		t.Fatalf("expected no pruning before window can be established, got prunable=%v err=%v", prunable, err)
	}
}

func TestArtifactRetentionPolicyEvidenceRequiresHeightAndEpoch(t *testing.T) {
	policy := ArtifactRetentionPolicy{KeepRecentHeights: 3, KeepRecentEpochs: 2}
	if prunable, err := policy.EvidencePrunable(10, 7, 5, 2); err != nil || !prunable {
		t.Fatalf("expected old height and epoch to be prunable, got prunable=%v err=%v", prunable, err)
	}
	if prunable, err := policy.EvidencePrunable(10, 7, 5, 4); err != ErrArtifactStillRetained || prunable {
		t.Fatalf("expected recent epoch to retain evidence, got prunable=%v err=%v", prunable, err)
	}
	if prunable, err := policy.EvidencePrunable(10, 9, 5, 2); err != ErrArtifactStillRetained || prunable {
		t.Fatalf("expected recent height to retain evidence, got prunable=%v err=%v", prunable, err)
	}
}

func TestArtifactRetentionPolicyRejectsFutureContext(t *testing.T) {
	policy := ArtifactRetentionPolicy{KeepRecentHeights: 2}
	if _, err := policy.CandidatePrunable(types.Height(5), types.Height(6)); err != ErrRetentionContextMismatch {
		t.Fatalf("expected candidate context mismatch, got %v", err)
	}
	if _, err := policy.EvidencePrunable(5, 4, 2, 3); err != ErrRetentionContextMismatch {
		t.Fatalf("expected evidence epoch context mismatch, got %v", err)
	}
}

func TestArtifactRetentionPolicyRejectsUnconfiguredPolicy(t *testing.T) {
	if _, err := (ArtifactRetentionPolicy{}).CandidatePrunable(10, 1); err != ErrInvalidRetentionPolicy {
		t.Fatalf("expected invalid policy, got %v", err)
	}
}
