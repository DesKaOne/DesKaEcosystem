package accounting

import (
	"time"

	"github.com/DesKaOne/DesKaEcosystem/DesKaProvider/routing"
)

type ReconciliationPersistenceResolution string

type ReconciliationPersistenceEvidenceScope string

const (
	ReconciliationPersistenceEvidenceScopeUnspecified ReconciliationPersistenceEvidenceScope = "unspecified"
	ReconciliationPersistenceEvidenceScopeSnapshotBound ReconciliationPersistenceEvidenceScope = "snapshot_bound"
	ReconciliationPersistenceEvidenceScopeIndependent ReconciliationPersistenceEvidenceScope = "independent"
)

const (
	ReconciliationPersistenceConfirmedApplied    ReconciliationPersistenceResolution = "confirmed_applied"
	ReconciliationPersistenceConfirmedNotApplied ReconciliationPersistenceResolution = "confirmed_not_applied"
	ReconciliationPersistenceNeedsReviewUnknown  ReconciliationPersistenceResolution = "needs_review_unknown"
	ReconciliationPersistenceNeedsReviewConflict ReconciliationPersistenceResolution = "needs_review_conflict"
)

type ReconciliationPersistenceEvidence struct {
	Resolution          ReconciliationPersistenceResolution
	Outcome             string
	Observed            bool
	LedgerObserved      bool
	AuditObserved       bool
	Source              string
	ObservedAt          time.Time
	Version             int64
	CaptureStartedAt    time.Time
	CaptureCompletedAt  time.Time
	SnapshotConsistency string
	ObservationScope    ReconciliationPersistenceEvidenceScope
}

func ClassifyTransactionPersistenceEvidence(evidence routing.PersistenceOutcomeEvidence) ReconciliationPersistenceEvidence {
	return ReconciliationPersistenceEvidence{
		Resolution: classifyReconciliationPersistenceOutcome(string(evidence.Outcome), evidence.Observed),
		Outcome:    string(evidence.Outcome),
		Observed:   evidence.Observed,
		Source:     evidence.Source,
		ObservedAt:      evidence.ObservedAt,
		Version:         evidence.Version,
		ObservationScope: ReconciliationPersistenceEvidenceScopeIndependent,
	}
}

func ClassifySettlementPersistenceEvidence(evidence SettlementPersistenceEvidence) ReconciliationPersistenceEvidence {
	observed := evidence.LedgerObserved && evidence.AuditObserved
	return ReconciliationPersistenceEvidence{
		Resolution:     classifyReconciliationPersistenceOutcome(string(evidence.Outcome), observed),
		Outcome:        string(evidence.Outcome),
		Observed:       observed,
		LedgerObserved: evidence.LedgerObserved,
		AuditObserved:  evidence.AuditObserved,
		Source:         evidence.Source,
		ObservedAt:      evidence.ObservedAt,
		ObservationScope: ReconciliationPersistenceEvidenceScopeIndependent,
	}
}

func classifyReconciliationPersistenceOutcome(outcome string, observed bool) ReconciliationPersistenceResolution {
	switch outcome {
	case "applied":
		if observed {
			return ReconciliationPersistenceConfirmedApplied
		}
		return ReconciliationPersistenceNeedsReviewUnknown
	case "not_applied":
		if observed {
			return ReconciliationPersistenceConfirmedNotApplied
		}
		return ReconciliationPersistenceNeedsReviewUnknown
	case "conflict":
		return ReconciliationPersistenceNeedsReviewConflict
	default:
		return ReconciliationPersistenceNeedsReviewUnknown
	}
}

func (r ReconciliationPersistenceEvidence) WithSnapshotMetadata(snapshot ReconciliationSnapshotMetadata) ReconciliationPersistenceEvidence {
	// Snapshot metadata describes the reconciliation report that carries this
	// evidence. It must not imply that the evidence observation itself was
	// captured inside that snapshot. Binding is an explicit producer claim.
	r.CaptureStartedAt = snapshot.CaptureStartedAt
	r.CaptureCompletedAt = snapshot.CaptureCompletedAt
	r.SnapshotConsistency = snapshot.SnapshotConsistency
	return r
}
