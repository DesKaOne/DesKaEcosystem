package accounting

import "testing"

func TestReconciliationOperatorReportV1ValidateRejectsForgedConfirmedApplied(t *testing.T) {
	report := ReconciliationOperatorReportV1{
		SchemaVersion: ReconciliationOperatorReportSchemaVersion,
		Snapshot: ReconciliationOperatorSnapshotV1{
			SnapshotConsistency: ReconciliationSnapshotConsistencyCapturedVerified,
			SnapshotFingerprint: "snapshot-1",
		},
		Items: []ReconciliationOperatorItemV1{{
			ReferenceID:          "ref-1",
			ReconciliationStatus: ReconciliationCorrelated,
			Resolution:           ReconciliationPersistenceConfirmedApplied,
			ResolutionClass:      ReconciliationOperatorConfirmed,
			PersistenceOutcome:   "applied",
			EvidenceObserved:     true,
			EvidenceScope:        ReconciliationPersistenceEvidenceScopeIndependent,
		}},
		Summary: ReconciliationOperatorSummaryV1{
			TotalItems:     1,
			ConfirmedItems: 1,
		},
	}

	if err := report.Validate(); err == nil {
		t.Fatal("expected forged confirmed-applied report to be rejected")
	}
}

func TestReconciliationOperatorReportV1ValidateRejectsMismatchedSnapshotBinding(t *testing.T) {
	report := ReconciliationOperatorReportV1{
		SchemaVersion: ReconciliationOperatorReportSchemaVersion,
		Snapshot: ReconciliationOperatorSnapshotV1{
			SnapshotConsistency: ReconciliationSnapshotConsistencyCapturedVerified,
			SnapshotFingerprint: "snapshot-1",
		},
		Items: []ReconciliationOperatorItemV1{{
			ReferenceID:          "ref-1",
			ReconciliationStatus: ReconciliationCorrelated,
			Resolution:           ReconciliationPersistenceConfirmedApplied,
			ResolutionClass:      ReconciliationOperatorConfirmed,
			PersistenceOutcome:   "applied",
			EvidenceObserved:     true,
			EvidenceScope:        ReconciliationPersistenceEvidenceScopeSnapshotBound,
			SnapshotFingerprint:  "snapshot-2",
		}},
		Summary: ReconciliationOperatorSummaryV1{
			TotalItems:     1,
			ConfirmedItems: 1,
		},
	}

	if err := report.Validate(); err == nil {
		t.Fatal("expected mismatched snapshot-bound report to be rejected")
	}
}

func TestReconciliationOperatorReportV1ValidateAcceptsConfirmedSnapshotBoundApplied(t *testing.T) {
	report := ReconciliationOperatorReportV1{
		SchemaVersion: ReconciliationOperatorReportSchemaVersion,
		Snapshot: ReconciliationOperatorSnapshotV1{
			SnapshotConsistency: ReconciliationSnapshotConsistencyCapturedVerified,
			SnapshotFingerprint: "snapshot-1",
		},
		Items: []ReconciliationOperatorItemV1{{
			ReferenceID:          "ref-1",
			ReconciliationStatus: ReconciliationCorrelated,
			Resolution:           ReconciliationPersistenceConfirmedApplied,
			ResolutionClass:      ReconciliationOperatorConfirmed,
			PersistenceOutcome:   "applied",
			EvidenceObserved:     true,
			EvidenceScope:        ReconciliationPersistenceEvidenceScopeSnapshotBound,
			SnapshotFingerprint:  "snapshot-1",
		}},
		Summary: ReconciliationOperatorSummaryV1{
			TotalItems:     1,
			ConfirmedItems: 1,
		},
	}

	if err := report.Validate(); err != nil {
		t.Fatalf("expected valid confirmed snapshot-bound report, got %v", err)
	}
}
