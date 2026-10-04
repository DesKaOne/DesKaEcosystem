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
			SnapshotFingerprint: "snap-test",
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
					ObservationScope: ReconciliationPersistenceEvidenceScopeSnapshotBound,
					SnapshotFingerprint: "snap-test",
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



func TestReconciliationPersistenceEvidenceBindToSnapshotRequiresFingerprint(t *testing.T) {
	snapshot := ReconciliationSnapshotMetadata{SnapshotFingerprint: "snap-123", SnapshotConsistency: ReconciliationSnapshotConsistencyCapturedVerified}
	evidence := ReconciliationPersistenceEvidence{Resolution: ReconciliationPersistenceConfirmedApplied, Outcome: "applied", Observed: true, ObservationScope: ReconciliationPersistenceEvidenceScopeIndependent}
	bound := evidence.BindToSnapshot(snapshot)
	if bound.ObservationScope != ReconciliationPersistenceEvidenceScopeSnapshotBound || bound.SnapshotFingerprint != "snap-123" {
		t.Fatalf("bound evidence = %+v; want snapshot-bound with fingerprint", bound)
	}
	withoutFingerprint := evidence.BindToSnapshot(ReconciliationSnapshotMetadata{SnapshotConsistency: ReconciliationSnapshotConsistencyCapturedVerified})
	if withoutFingerprint.ObservationScope != ReconciliationPersistenceEvidenceScopeUnspecified || withoutFingerprint.SnapshotFingerprint != "" {
		t.Fatalf("binding without fingerprint = %+v; want unspecified", withoutFingerprint)
	}
}

func TestReconciliationOperatorReportRejectsMismatchedSnapshotFingerprint(t *testing.T) {
	report := ReconciliationReport{
		Snapshot: ReconciliationSnapshotMetadata{SnapshotConsistency: ReconciliationSnapshotConsistencyCapturedVerified, SnapshotFingerprint: "snap-current"},
		Items: []TransactionReconciliation{{
			ReferenceID: "ref-1", Status: ReconciliationCorrelated,
			PersistenceEvidence: &ReconciliationPersistenceEvidence{
				Resolution: ReconciliationPersistenceConfirmedApplied, Outcome: "applied", Observed: true,
				ObservationScope: ReconciliationPersistenceEvidenceScopeSnapshotBound, SnapshotFingerprint: "snap-other",
			},
		}},
	}
	view := report.OperatorReport()
	if view.Summary.ConfirmedItems != 0 || view.Summary.ReviewItems != 1 {
		t.Fatalf("mismatched fingerprint was confirmed: %+v", view.Summary)
	}
}

func TestReconciliationOperatorReportDoesNotTreatIndependentEvidenceAsSnapshotBound(t *testing.T) {
	observedAt := time.Date(2026, 10, 4, 14, 0, 0, 0, time.UTC)
	report := ReconciliationReport{
		Snapshot: ReconciliationSnapshotMetadata{
			CaptureStartedAt: time.Date(2026, 10, 4, 13, 59, 0, 0, time.UTC),
			CaptureCompletedAt: time.Date(2026, 10, 4, 13, 59, 30, 0, time.UTC),
			SnapshotConsistency: ReconciliationSnapshotConsistencyCapturedVerified,
		},
		Items: []TransactionReconciliation{
			{
				ReferenceID: "ref-independent-applied",
				Status: ReconciliationCorrelated,
				PersistenceEvidence: &ReconciliationPersistenceEvidence{
					Resolution: ReconciliationPersistenceConfirmedApplied,
					Outcome: "applied",
					Observed: true,
					Source: "postgres.provider_transactions",
					ObservedAt: observedAt,
					ObservationScope: ReconciliationPersistenceEvidenceScopeIndependent,
				},
			},
			{
				ReferenceID: "ref-independent-not-applied",
				Status: ReconciliationCorrelated,
				PersistenceEvidence: &ReconciliationPersistenceEvidence{
					Resolution: ReconciliationPersistenceConfirmedNotApplied,
					Outcome: "not_applied",
					Observed: true,
					Source: "postgres.provider_transactions",
					ObservedAt: observedAt,
					ObservationScope: ReconciliationPersistenceEvidenceScopeIndependent,
				},
			},
		},
	}
	view := report.OperatorReport()
	if view.Summary.ConfirmedItems != 0 || view.Summary.ReviewItems != 2 {
		t.Fatalf("independent evidence was promoted: %+v", view.Summary)
	}
	for _, item := range view.Items {
		if item.ResolutionClass != ReconciliationOperatorReview {
			t.Fatalf("independent evidence resolution = %q; want review", item.ResolutionClass)
		}
		if item.EvidenceScope != ReconciliationPersistenceEvidenceScopeIndependent {
			t.Fatalf("evidence scope = %q; want independent", item.EvidenceScope)
		}
	}
}

func TestReconciliationPersistenceEvidenceMetadataDoesNotImplySnapshotBinding(t *testing.T) {
	evidence := ReconciliationPersistenceEvidence{
		Resolution: ReconciliationPersistenceConfirmedApplied,
		Outcome: "applied",
		Observed: true,
		ObservationScope: ReconciliationPersistenceEvidenceScopeIndependent,
	}
	snapshot := ReconciliationSnapshotMetadata{
		CaptureStartedAt: startTimeForOperatorTest(),
		CaptureCompletedAt: time.Date(2026, 10, 4, 12, 31, 0, 0, time.UTC),
		SnapshotConsistency: ReconciliationSnapshotConsistencyCapturedVerified,
	}
	boundMetadata := evidence.WithSnapshotMetadata(snapshot)
	if boundMetadata.ObservationScope != ReconciliationPersistenceEvidenceScopeIndependent {
		t.Fatalf("snapshot metadata changed evidence scope to %q", boundMetadata.ObservationScope)
	}
}

func TestReconciliationOperatorReportDoesNotConfirmAppliedEvidenceWithoutCorrelation(t *testing.T) {
	observedAt := time.Date(2026, 10, 4, 12, 45, 0, 0, time.UTC)
	for _, status := range []ReconciliationStatus{
		ReconciliationLedgerMissing,
		ReconciliationCorrelationConflict,
		ReconciliationDuplicateReference,
		ReconciliationDuplicateAuditIdentity,
	} {
		report := ReconciliationReport{
			Snapshot: ReconciliationSnapshotMetadata{
				CaptureStartedAt: startTimeForOperatorTest(),
				CaptureCompletedAt: observedAt,
				SnapshotConsistency: ReconciliationSnapshotConsistencyCapturedVerified,
			},
			Items: []TransactionReconciliation{{
				ReferenceID: "ref-applied-but-unresolved",
				Status: status,
				PersistenceEvidence: &ReconciliationPersistenceEvidence{
					Resolution: ReconciliationPersistenceConfirmedApplied,
					Outcome: "applied",
					Observed: true,
					Source: "postgres.provider_transactions",
					ObservedAt: observedAt,
					Version: 11,
				},
			}},
		}
		view := report.OperatorReport()
		if view.Items[0].ResolutionClass != ReconciliationOperatorReview {
			t.Fatalf("status %q promoted applied evidence to %q; want review", status, view.Items[0].ResolutionClass)
		}
		if view.Summary.ConfirmedItems != 0 || view.Summary.ReviewItems != 1 {
			t.Fatalf("status %q summary = %+v; want zero confirmed and one review", status, view.Summary)
		}
	}
}

func TestReconciliationOperatorReportDoesNotConfirmUnverifiedSnapshot(t *testing.T) {
	observedAt := time.Date(2026, 10, 4, 13, 5, 0, 0, time.UTC)
	for _, consistency := range []string{
		ReconciliationSnapshotConsistencyCaptured,
		ReconciliationSnapshotConsistencyLegacyMixed,
	} {
		report := ReconciliationReport{
			Snapshot: ReconciliationSnapshotMetadata{
				CaptureStartedAt: startTimeForOperatorTest(),
				CaptureCompletedAt: observedAt,
				SnapshotConsistency: consistency,
			},
			Items: []TransactionReconciliation{{
				ReferenceID: "ref-mixed-snapshot",
				Status: ReconciliationCorrelated,
				PersistenceEvidence: &ReconciliationPersistenceEvidence{
					Resolution: ReconciliationPersistenceConfirmedApplied,
					Outcome: "applied",
					Observed: true,
					Source: "postgres.provider_transactions",
					ObservedAt: observedAt,
					Version: 13,
				},
			}},
		}
		view := report.OperatorReport()
		if view.Items[0].ResolutionClass != ReconciliationOperatorReview {
			t.Fatalf("snapshot consistency %q promoted applied evidence to %q; want review", consistency, view.Items[0].ResolutionClass)
		}
		if view.Summary.ConfirmedItems != 0 || view.Summary.ReviewItems != 1 {
			t.Fatalf("snapshot consistency %q summary = %+v; want zero confirmed and one review", consistency, view.Summary)
		}
	}
}

func TestReconciliationOperatorReportDoesNotConfirmNotAppliedFromUnverifiedSnapshot(t *testing.T) {
	observedAt := time.Date(2026, 10, 4, 13, 10, 0, 0, time.UTC)
	report := ReconciliationReport{
		Snapshot: ReconciliationSnapshotMetadata{
			CaptureStartedAt: startTimeForOperatorTest(),
			CaptureCompletedAt: observedAt,
			SnapshotConsistency: ReconciliationSnapshotConsistencyLegacyMixed,
		},
		Items: []TransactionReconciliation{{
			ReferenceID: "ref-not-applied-mixed",
			Status: ReconciliationCorrelated,
			PersistenceEvidence: &ReconciliationPersistenceEvidence{
				Resolution: ReconciliationPersistenceConfirmedNotApplied,
				Outcome: "not_applied",
				Observed: true,
				Source: "postgres.provider_transactions",
				ObservedAt: observedAt,
				Version: 14,
			},
		}},
	}
	view := report.OperatorReport()
	if view.Items[0].ResolutionClass != ReconciliationOperatorReview {
		t.Fatalf("unverified not-applied evidence promoted to %q; want review", view.Items[0].ResolutionClass)
	}
	if view.Summary.ConfirmedItems != 0 || view.Summary.ReviewItems != 1 {
		t.Fatalf("unverified not-applied summary = %+v; want zero confirmed and one review", view.Summary)
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
		"\"evidence_scope\":\"unspecified\"",
		"\"persistence_version\":12",
	} {
		if !contains(jsonText, required) {
			t.Fatalf("delivery JSON dropped required contract field %q: %s", required, jsonText)
		}
	}
}

func TestReconciliationOperatorReportV1ValidateRejectsInconsistentSummary(t *testing.T) {
	report := ReconciliationOperatorReportV1{
		SchemaVersion: ReconciliationOperatorReportSchemaVersion,
		Items: []ReconciliationOperatorItemV1{{
			ReferenceID: "ref-1",
			ResolutionClass: ReconciliationOperatorConfirmed,
		}},
		Summary: ReconciliationOperatorSummaryV1{
			TotalItems: 0,
			ConfirmedItems: 0,
		},
	}
	if err := report.Validate(); err == nil {
		t.Fatal("inconsistent V1 summary must be rejected")
	}
}

func TestReconciliationOperatorReportV1ValidateRejectsUnknownResolutionClass(t *testing.T) {
	report := ReconciliationOperatorReportV1{
		SchemaVersion: ReconciliationOperatorReportSchemaVersion,
		Items: []ReconciliationOperatorItemV1{{
			ReferenceID: "ref-invalid",
			ResolutionClass: ReconciliationOperatorResolution("invalid"),
		}},
		Summary: ReconciliationOperatorSummaryV1{
			TotalItems: 1,
		},
	}
	if err := report.Validate(); err == nil {
		t.Fatal("unsupported resolution class must be rejected")
	}
}

func TestReconciliationOperatorReportV1ValidateAcceptsProjectedReport(t *testing.T) {
	observedAt := time.Date(2026, 10, 4, 12, 30, 0, 0, time.UTC)
	report := ReconciliationReport{
		Snapshot: ReconciliationSnapshotMetadata{
			CaptureStartedAt: startTimeForOperatorTest(),
			CaptureCompletedAt: observedAt,
			SnapshotConsistency: ReconciliationSnapshotConsistencyCapturedVerified,
		},
		Items: []TransactionReconciliation{{
			ReferenceID: "ref-valid",
			Status: ReconciliationCorrelated,
			PersistenceEvidence: &ReconciliationPersistenceEvidence{
				Resolution: ReconciliationPersistenceConfirmedApplied,
				Outcome: "applied",
				Observed: true,
				Source: "postgres.provider_transactions",
				ObservedAt: observedAt,
				Version: 15,
				ObservationScope: ReconciliationPersistenceEvidenceScopeSnapshotBound,
			},
		}},
	}
	delivery := report.OperatorReport().V1()
	if err := delivery.Validate(); err != nil {
		t.Fatalf("projected V1 report must validate: %v", err)
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
