package accounting

import (
	"encoding/json"
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

func TestReconciliationOperatorReportV1PreservesDeliveryContract(t *testing.T) {
	observedAt := time.Date(2026, 10, 4, 12, 30, 0, 0, time.UTC)
	report := ReconciliationReport{
		Snapshot: ReconciliationSnapshotMetadata{
			CaptureStartedAt: startTimeForOperatorTest(),
			CaptureCompletedAt: observedAt,
			CapturedAt: observedAt,
			ProviderTransactionCount: 3,
			LedgerTransactionCount: 2,
			SettlementAuditCount: 2,
			ProviderReader: "snapshot-capture",
			LedgerReader: "snapshot-capture",
			SettlementAuditReader: "snapshot-capture",
			SnapshotConsistency: ReconciliationSnapshotConsistencyCapturedVerified,
			SnapshotFingerprint: "fingerprint-v1",
			SnapshotWindowMillis: 17,
		},
		Items: []TransactionReconciliation{
			{
				ReferenceID: "ref-unknown",
				Status: ReconciliationLedgerMissing,
				PersistenceEvidence: &ReconciliationPersistenceEvidence{
					Resolution: ReconciliationPersistenceNeedsReviewUnknown,
					Outcome: "unknown",
					Observed: false,
					Source: "postgres.provider_transactions",
					ObservedAt: observedAt,
					Version: 12,
				},
			},
			{
				ReferenceID: "ref-conflict",
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
		},
	}
	report = report.WithPersistenceEvidence("ref-unknown", ReconciliationPersistenceEvidence{
		Resolution: ReconciliationPersistenceNeedsReviewUnknown,
		Outcome: "unknown",
		Observed: false,
		Source: "postgres.provider_transactions",
		ObservedAt: observedAt,
		Version: 12,
	})
	view := report.OperatorReport()
	delivery := view.V1()

	if delivery.SchemaVersion != ReconciliationOperatorReportSchemaVersion {
		t.Fatalf("schema version = %q", delivery.SchemaVersion)
	}
	if delivery.Snapshot.SnapshotFingerprint != "fingerprint-v1" ||
		delivery.Snapshot.SnapshotConsistency != ReconciliationSnapshotConsistencyCapturedVerified ||
		delivery.Snapshot.ProviderReader != "snapshot-capture" ||
		delivery.Snapshot.SnapshotWindowMillis != 17 {
		t.Fatalf("snapshot provenance was dropped: %+v", delivery.Snapshot)
	}
	if len(delivery.Items) != 2 {
		t.Fatalf("items length = %d", len(delivery.Items))
	}
	if delivery.Items[0].EvidenceSource != "postgres.provider_transactions" ||
		delivery.Items[0].PersistenceVersion != 12 ||
		delivery.Items[0].ResolutionClass != ReconciliationOperatorUnresolved {
		t.Fatalf("unknown evidence was not preserved: %+v", delivery.Items[0])
	}
	if delivery.Items[1].ResolutionClass != ReconciliationOperatorReview ||
		delivery.Items[1].LedgerObserved != true ||
		delivery.Items[1].AuditObserved != false {
		t.Fatalf("conflict evidence was not preserved: %+v", delivery.Items[1])
	}

	payload, err := json.Marshal(delivery)
	if err != nil {
		t.Fatalf("marshal delivery contract: %v", err)
	}
	jsonText := string(payload)
	for _, required := range []string{
		"\"schema_version\":\"v1\"",
		"\"snapshot\"",
		"\"snapshot_fingerprint\":\"fingerprint-v1\"",
		"\"items\"",
		"\"resolution\":\"needs_review_unknown\"",
		"\"resolution_class\":\"unresolved\"",
		"\"resolution\":\"needs_review_conflict\"",
		"\"resolution_class\":\"review\"",
		"\"evidence_source\":\"postgres.provider_transactions\"",
		"\"persistence_version\":12",
	} {
		if !contains(jsonText, required) {
			t.Fatalf("delivery JSON dropped required contract field %q: %s", required, jsonText)
		}
	}
}

func contains(value, required string) bool {
	return len(value) >= len(required) && stringIndex(value, required) >= 0
}

func stringIndex(value, required string) int {
	for i := 0; i+len(required) <= len(value); i++ {
		if value[i:i+len(required)] == required {
			return i
		}
	}
	return -1
}

func startTimeForOperatorTest() time.Time {
	return time.Date(2026, 10, 4, 12, 29, 0, 0, time.UTC)
}
