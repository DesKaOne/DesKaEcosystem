package accounting

import (
	"testing"
	"time"
)

func TestReconciliationReportOperatorReportIsDeterministicAndReadOnly(t *testing.T) {
	observedAt := time.Date(2026, 10, 4, 12, 30, 0, 0, time.UTC)
	report := ReconciliationReport{
		Snapshot: ReconciliationSnapshotMetadata{
			CaptureStartedAt: startTimeForOperatorTest(),
			CaptureCompletedAt: observedAt,
			SnapshotConsistency: ReconciliationSnapshotConsistencyCapturedVerified,
		},
		Items: []TransactionReconciliation{
			{
				ReferenceID: "ref-applied",
				ProviderStatus: ProviderStatusSuccess,
				Status: ReconciliationCorrelated,
				LedgerTransactionID: "ledger-1",
				SettlementAuditEventID: "audit-1",
				PersistenceEvidence: &ReconciliationPersistenceEvidence{
					Resolution: ReconciliationPersistenceConfirmedApplied,
					Outcome: "applied",
					Observed: true,
					Source: "postgres.provider_transactions",
					ObservedAt: observedAt,
					Version: 9,
				},
			},
			{
				ReferenceID: "ref-conflict",
				ProviderStatus: ProviderStatusSuccess,
				Status: ReconciliationCorrelationConflict,
				PersistenceEvidence: &ReconciliationPersistenceEvidence{
					Resolution: ReconciliationPersistenceNeedsReviewConflict,
					Outcome: "conflict",
					Observed: true,
					LedgerObserved: true,
					AuditObserved: false,
					Source: "postgres.ledger_transactions+settlement_audit",
					ObservedAt: observedAt,
				},
			},
			{
				ReferenceID: "ref-none",
				ProviderStatus: ProviderStatusSuccess,
				Status: ReconciliationLedgerMissing,
			},
		},
	}
	view := report.OperatorReport()
	if view.Summary.TotalItems != 3 || view.Summary.ConfirmedItems != 1 || view.Summary.ReviewItems != 1 ||
		view.Summary.UnresolvedItems != 0 || view.Summary.WithoutEvidenceItems != 1 {
		t.Fatalf("unexpected summary: %+v", view.Summary)
	}
	if view.Items[0].ResolutionClass != ReconciliationOperatorConfirmed {
		t.Fatalf("applied resolution class = %q", view.Items[0].ResolutionClass)
	}
	if view.Items[1].ResolutionClass != ReconciliationOperatorReview {
		t.Fatalf("conflict resolution class = %q", view.Items[1].ResolutionClass)
	}
	if view.Items[2].Resolution != ReconciliationPersistenceNeedsReviewUnknown ||
		view.Items[2].ResolutionClass != ReconciliationOperatorUnresolved {
		t.Fatalf("missing evidence was not kept unresolved: %+v", view.Items[2])
	}
	if !view.Items[0].EvidenceObservedAt.Equal(observedAt) || view.Items[0].PersistenceVersion != 9 {
		t.Fatalf("provenance was not preserved: %+v", view.Items[0])
	}
	if report.Items[0].PersistenceEvidence.Resolution != ReconciliationPersistenceConfirmedApplied {
		t.Fatal("operator projection mutated source report")
	}
}

func startTimeForOperatorTest() time.Time {
	return time.Date(2026, 10, 4, 12, 29, 0, 0, time.UTC)
}
