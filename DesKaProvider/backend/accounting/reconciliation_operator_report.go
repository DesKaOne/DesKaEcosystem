package accounting

import "time"

type ReconciliationOperatorResolution string

const (
	ReconciliationOperatorConfirmed ReconciliationOperatorResolution = "confirmed"
	ReconciliationOperatorReview     ReconciliationOperatorResolution = "review"
	ReconciliationOperatorUnresolved ReconciliationOperatorResolution = "unresolved"
)

type ReconciliationOperatorItem struct {
	ReferenceID          string
	ReconciliationStatus ReconciliationStatus
	ProviderStatus       string
	Resolution           ReconciliationPersistenceResolution
	ResolutionClass      ReconciliationOperatorResolution
	PersistenceOutcome   string
	EvidenceObserved     bool
	EvidenceScope        ReconciliationPersistenceEvidenceScope
	EvidenceSource       string
	EvidenceObservedAt   time.Time
	PersistenceVersion   int64
	LedgerObserved       bool
	AuditObserved        bool
	LedgerTransactionID  string
	AuditEventID         string
}

type ReconciliationOperatorSummary struct {
	TotalItems           int
	ConfirmedItems       int
	ReviewItems          int
	UnresolvedItems      int
	WithoutEvidenceItems int
}

type ReconciliationOperatorReport struct {
	Snapshot ReconciliationSnapshotMetadata
	Items    []ReconciliationOperatorItem
	Summary  ReconciliationOperatorSummary
}

// OperatorReport returns a deterministic, read-only projection of a
// reconciliation report. It never resolves evidence, performs persistence
// writes, or changes the financial state.
func (r ReconciliationReport) OperatorReport() ReconciliationOperatorReport {
	items := make([]ReconciliationOperatorItem, 0, len(r.Items))
	summary := ReconciliationOperatorSummary{TotalItems: len(r.Items)}
	for _, item := range r.Items {
		operatorItem := ReconciliationOperatorItem{
			ReferenceID:          item.ReferenceID,
			ReconciliationStatus: item.Status,
			ProviderStatus:       item.ProviderStatus,
			LedgerTransactionID:  item.LedgerTransactionID,
			AuditEventID:         item.SettlementAuditEventID,
			Resolution:           ReconciliationPersistenceNeedsReviewUnknown,
			ResolutionClass:      ReconciliationOperatorUnresolved,
		}
		if item.PersistenceEvidence == nil {
			summary.WithoutEvidenceItems++
		} else {
			evidence := item.PersistenceEvidence
			operatorItem.Resolution = evidence.Resolution
			operatorItem.PersistenceOutcome = evidence.Outcome
			operatorItem.EvidenceObserved = evidence.Observed
			operatorItem.EvidenceScope = evidence.ObservationScope
			operatorItem.EvidenceSource = evidence.Source
			operatorItem.EvidenceObservedAt = evidence.ObservedAt
			operatorItem.PersistenceVersion = evidence.Version
			operatorItem.LedgerObserved = evidence.LedgerObserved
			operatorItem.AuditObserved = evidence.AuditObserved
			switch evidence.Resolution {
			case ReconciliationPersistenceConfirmedApplied:
				// Durable "applied" evidence is only operator-confirmable when
				// the reconciliation established a valid correlation AND the
				// snapshot was verified across all participating stores.
				// Mixed or unverified observations are never authoritative.
				if item.Status == ReconciliationCorrelated &&
					r.Snapshot.SnapshotConsistency == ReconciliationSnapshotConsistencyCapturedVerified &&
					evidence.ObservationScope == ReconciliationPersistenceEvidenceScopeSnapshotBound {
					operatorItem.ResolutionClass = ReconciliationOperatorConfirmed
					summary.ConfirmedItems++
				} else {
					operatorItem.ResolutionClass = ReconciliationOperatorReview
					summary.ReviewItems++
				}
			case ReconciliationPersistenceConfirmedNotApplied:
				if r.Snapshot.SnapshotConsistency == ReconciliationSnapshotConsistencyCapturedVerified &&
					evidence.ObservationScope == ReconciliationPersistenceEvidenceScopeSnapshotBound {
					operatorItem.ResolutionClass = ReconciliationOperatorConfirmed
					summary.ConfirmedItems++
				} else {
					operatorItem.ResolutionClass = ReconciliationOperatorReview
					summary.ReviewItems++
				}
			case ReconciliationPersistenceNeedsReviewConflict:
				operatorItem.ResolutionClass = ReconciliationOperatorReview
				summary.ReviewItems++
			default:
				operatorItem.ResolutionClass = ReconciliationOperatorUnresolved
				summary.UnresolvedItems++
			}
		}
		items = append(items, operatorItem)
	}
	return ReconciliationOperatorReport{Snapshot: r.Snapshot, Items: items, Summary: summary}
}
