package accounting

import (
	"fmt"
	"time"
)

const ReconciliationOperatorReportSchemaVersion = "v1"

type ReconciliationOperatorSnapshotV1 struct {
	CaptureStartedAt         time.Time `json:"capture_started_at"`
	CaptureCompletedAt       time.Time `json:"capture_completed_at"`
	CapturedAt               time.Time `json:"captured_at"`
	ProviderTransactionCount int       `json:"provider_transaction_count"`
	LedgerTransactionCount   int       `json:"ledger_transaction_count"`
	SettlementAuditCount     int       `json:"settlement_audit_count"`
	ProviderReader           string    `json:"provider_reader"`
	LedgerReader             string    `json:"ledger_reader"`
	SettlementAuditReader    string    `json:"settlement_audit_reader"`
	SnapshotConsistency      string    `json:"snapshot_consistency"`
	SnapshotFingerprint      string    `json:"snapshot_fingerprint"`
	SnapshotWindowMillis     int64     `json:"snapshot_window_millis"`
}

type ReconciliationOperatorItemV1 struct {
	ReferenceID          string                             `json:"reference_id"`
	ReconciliationStatus ReconciliationStatus               `json:"reconciliation_status"`
	ProviderStatus       string                             `json:"provider_status"`
	Resolution           ReconciliationPersistenceResolution `json:"resolution"`
	ResolutionClass      ReconciliationOperatorResolution  `json:"resolution_class"`
	PersistenceOutcome   string                             `json:"persistence_outcome"`
	EvidenceObserved     bool                               `json:"evidence_observed"`
	EvidenceScope        ReconciliationPersistenceEvidenceScope `json:"evidence_scope"`
	SnapshotFingerprint  string `json:"snapshot_fingerprint"`
	EvidenceSource       string                             `json:"evidence_source"`
	EvidenceObservedAt   time.Time                          `json:"evidence_observed_at"`
	PersistenceVersion   int64                              `json:"persistence_version"`
	LedgerObserved       bool                               `json:"ledger_observed"`
	AuditObserved        bool                               `json:"audit_observed"`
	LedgerTransactionID  string                             `json:"ledger_transaction_id"`
	AuditEventID         string                             `json:"audit_event_id"`
}

type ReconciliationOperatorSummaryV1 struct {
	TotalItems           int `json:"total_items"`
	ConfirmedItems       int `json:"confirmed_items"`
	ReviewItems          int `json:"review_items"`
	UnresolvedItems      int `json:"unresolved_items"`
	WithoutEvidenceItems int `json:"without_evidence_items"`
}

type ReconciliationOperatorReportV1 struct {
	SchemaVersion string                            `json:"schema_version"`
	Snapshot      ReconciliationOperatorSnapshotV1 `json:"snapshot"`
	Items         []ReconciliationOperatorItemV1   `json:"items"`
	Summary       ReconciliationOperatorSummaryV1  `json:"summary"`
}

// Validate checks the V1 delivery contract without resolving evidence or
// changing financial state. It rejects internally inconsistent projections
// before they cross a delivery boundary.
func (r ReconciliationOperatorReportV1) Validate() error {
	if r.SchemaVersion != ReconciliationOperatorReportSchemaVersion {
		return fmt.Errorf("unsupported reconciliation operator report schema version %q", r.SchemaVersion)
	}
	switch r.Snapshot.SnapshotConsistency {
	case ReconciliationSnapshotConsistencyCaptured, ReconciliationSnapshotConsistencyCapturedVerified, ReconciliationSnapshotConsistencyLegacyMixed:
	default:
		return fmt.Errorf("unsupported reconciliation snapshot consistency %q", r.Snapshot.SnapshotConsistency)
	}
	if r.Snapshot.SnapshotConsistency == ReconciliationSnapshotConsistencyCapturedVerified &&
		r.Snapshot.SnapshotFingerprint == "" {
		return fmt.Errorf("verified reconciliation snapshot requires a snapshot fingerprint")
	}
	if r.Summary.TotalItems != len(r.Items) {
		return fmt.Errorf("reconciliation operator summary total_items=%d does not match items=%d", r.Summary.TotalItems, len(r.Items))
	}
	confirmed, review, unresolved, withoutEvidence := 0, 0, 0, 0
	for _, item := range r.Items {
		switch item.ResolutionClass {
		case ReconciliationOperatorConfirmed:
			confirmed++
		case ReconciliationOperatorReview:
			review++
		case ReconciliationOperatorUnresolved:
			unresolved++
		default:
			return fmt.Errorf("unsupported reconciliation operator resolution class %q", item.ResolutionClass)
		}

		if item.EvidenceScope == ReconciliationPersistenceEvidenceScopeSnapshotBound {
			if item.SnapshotFingerprint == "" {
				return fmt.Errorf("snapshot-bound evidence for %q requires a snapshot fingerprint", item.ReferenceID)
			}
			if item.SnapshotFingerprint != r.Snapshot.SnapshotFingerprint {
				return fmt.Errorf("snapshot-bound evidence for %q does not match the report snapshot fingerprint", item.ReferenceID)
			}
		} else if item.SnapshotFingerprint != "" {
			return fmt.Errorf("non-snapshot-bound evidence for %q cannot carry a snapshot fingerprint", item.ReferenceID)
		}

		if item.ResolutionClass == ReconciliationOperatorConfirmed {
			if r.Snapshot.SnapshotConsistency != ReconciliationSnapshotConsistencyCapturedVerified {
				return fmt.Errorf("confirmed item %q requires a verified reconciliation snapshot", item.ReferenceID)
			}
			if item.EvidenceScope != ReconciliationPersistenceEvidenceScopeSnapshotBound {
				return fmt.Errorf("confirmed item %q requires snapshot-bound evidence", item.ReferenceID)
			}
			if item.SnapshotFingerprint == "" || item.SnapshotFingerprint != r.Snapshot.SnapshotFingerprint {
				return fmt.Errorf("confirmed item %q requires matching snapshot fingerprint", item.ReferenceID)
			}
			switch item.Resolution {
			case ReconciliationPersistenceConfirmedApplied:
				if item.ReconciliationStatus != ReconciliationCorrelated || item.PersistenceOutcome != "applied" {
					return fmt.Errorf("confirmed applied item %q has inconsistent reconciliation status or persistence outcome", item.ReferenceID)
				}
			case ReconciliationPersistenceConfirmedNotApplied:
				if item.PersistenceOutcome != "not_applied" {
					return fmt.Errorf("confirmed not-applied item %q has inconsistent persistence outcome", item.ReferenceID)
				}
			default:
				return fmt.Errorf("confirmed item %q has unsupported persistence resolution %q", item.ReferenceID, item.Resolution)
			}
		}

		if item.EvidenceSource == "" && item.EvidenceObservedAt.IsZero() &&
			item.PersistenceOutcome == "" && item.PersistenceVersion == 0 &&
			!item.EvidenceObserved && !item.LedgerObserved && !item.AuditObserved {
			withoutEvidence++
		}
	}
	if r.Summary.ConfirmedItems != confirmed ||
		r.Summary.ReviewItems != review ||
		r.Summary.UnresolvedItems != unresolved ||
		r.Summary.WithoutEvidenceItems != withoutEvidence {
		return fmt.Errorf("reconciliation operator summary counts do not match item classifications")
	}
	return nil
}

// V1 returns the stable, read-only delivery contract for an operator report.
// The contract carries snapshot provenance and persistence evidence explicitly.
// It contains no financial command, retry, resubmission, or repair instruction.
func (r ReconciliationOperatorReport) V1() ReconciliationOperatorReportV1 {
	snapshot := ReconciliationOperatorSnapshotV1{
		CaptureStartedAt: r.Snapshot.CaptureStartedAt,
		CaptureCompletedAt: r.Snapshot.CaptureCompletedAt,
		CapturedAt: r.Snapshot.CapturedAt,
		ProviderTransactionCount: r.Snapshot.ProviderTransactionCount,
		LedgerTransactionCount: r.Snapshot.LedgerTransactionCount,
		SettlementAuditCount: r.Snapshot.SettlementAuditCount,
		ProviderReader: r.Snapshot.ProviderReader,
		LedgerReader: r.Snapshot.LedgerReader,
		SettlementAuditReader: r.Snapshot.SettlementAuditReader,
		SnapshotConsistency: r.Snapshot.SnapshotConsistency,
		SnapshotFingerprint: r.Snapshot.SnapshotFingerprint,
		SnapshotWindowMillis: r.Snapshot.SnapshotWindowMillis,
	}
	items := make([]ReconciliationOperatorItemV1, len(r.Items))
	for i, item := range r.Items {
		evidenceScope := item.EvidenceScope
		if evidenceScope == "" {
			evidenceScope = ReconciliationPersistenceEvidenceScopeUnspecified
		}
		items[i] = ReconciliationOperatorItemV1{
			ReferenceID: item.ReferenceID,
			ReconciliationStatus: item.ReconciliationStatus,
			ProviderStatus: item.ProviderStatus,
			Resolution: item.Resolution,
			ResolutionClass: item.ResolutionClass,
			PersistenceOutcome: item.PersistenceOutcome,
			EvidenceObserved: item.EvidenceObserved,
			EvidenceScope: evidenceScope,
			SnapshotFingerprint: item.SnapshotFingerprint,
			EvidenceSource: item.EvidenceSource,
			EvidenceObservedAt: item.EvidenceObservedAt,
			PersistenceVersion: item.PersistenceVersion,
			LedgerObserved: item.LedgerObserved,
			AuditObserved: item.AuditObserved,
			LedgerTransactionID: item.LedgerTransactionID,
			AuditEventID: item.AuditEventID,
		}
	}
	return ReconciliationOperatorReportV1{
		SchemaVersion: ReconciliationOperatorReportSchemaVersion,
		Snapshot: snapshot,
		Items: items,
		Summary: ReconciliationOperatorSummaryV1{
			TotalItems: r.Summary.TotalItems,
			ConfirmedItems: r.Summary.ConfirmedItems,
			ReviewItems: r.Summary.ReviewItems,
			UnresolvedItems: r.Summary.UnresolvedItems,
			WithoutEvidenceItems: r.Summary.WithoutEvidenceItems,
		},
	}
}
