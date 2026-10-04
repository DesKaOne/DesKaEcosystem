package accounting

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestEncodeReconciliationOperatorReportV1IsReadOnlyAndVersioned(t *testing.T) {
	observedAt := time.Date(2026, 10, 4, 13, 0, 0, 0, time.UTC)
	report := ReconciliationReport{
		Snapshot: ReconciliationSnapshotMetadata{
			CaptureStartedAt: startTimeForOperatorDeliveryTest(),
			CaptureCompletedAt: observedAt,
			CapturedAt: observedAt,
			ProviderTransactionCount: 1,
			LedgerTransactionCount: 1,
			SettlementAuditCount: 1,
			ProviderReader: "snapshot-capture",
			LedgerReader: "snapshot-capture",
			SettlementAuditReader: "snapshot-capture",
			SnapshotConsistency: ReconciliationSnapshotConsistencyCapturedVerified,
			SnapshotFingerprint: "delivery-fingerprint-v1",
			SnapshotWindowMillis: 4,
		},
		Items: []TransactionReconciliation{
			{
				ReferenceID: "ref-unknown",
				ProviderStatus: ProviderStatusSuccess,
				Status: ReconciliationLedgerMissing,
				PersistenceEvidence: &ReconciliationPersistenceEvidence{
					Resolution: ReconciliationPersistenceNeedsReviewUnknown,
					Outcome: "unknown",
					Observed: false,
					Source: "postgres.provider_transactions",
					ObservedAt: observedAt,
					Version: 21,
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
		},
	}

	var first bytes.Buffer
	if err := EncodeReconciliationOperatorReportV1(&first, report); err != nil {
		t.Fatalf("encode first report: %v", err)
	}
	var second bytes.Buffer
	if err := EncodeReconciliationOperatorReportV1(&second, report); err != nil {
		t.Fatalf("encode second report: %v", err)
	}

	if first.String() != second.String() {
		t.Fatalf("encoding is not deterministic:\nfirst=%s\nsecond=%s", first.String(), second.String())
	}

	var payload ReconciliationOperatorReportV1
	if err := json.Unmarshal(first.Bytes(), &payload); err != nil {
		t.Fatalf("decode encoded report: %v", err)
	}
	if payload.SchemaVersion != ReconciliationOperatorReportSchemaVersion {
		t.Fatalf("schema version = %q", payload.SchemaVersion)
	}
	if payload.Snapshot.SnapshotFingerprint != "delivery-fingerprint-v1" {
		t.Fatalf("snapshot fingerprint was dropped: %q", payload.Snapshot.SnapshotFingerprint)
	}
	if len(payload.Items) != 2 {
		t.Fatalf("items length = %d", len(payload.Items))
	}
	if payload.Items[0].ResolutionClass != ReconciliationOperatorUnresolved ||
		payload.Items[0].EvidenceSource != "postgres.provider_transactions" ||
		payload.Items[0].PersistenceVersion != 21 {
		t.Fatalf("unknown evidence was not preserved: %+v", payload.Items[0])
	}
	if payload.Items[1].ResolutionClass != ReconciliationOperatorReview ||
		!payload.Items[1].LedgerObserved || payload.Items[1].AuditObserved {
		t.Fatalf("conflict evidence was not preserved: %+v", payload.Items[1])
	}

	if report.Items[0].PersistenceEvidence.Resolution != ReconciliationPersistenceNeedsReviewUnknown {
		t.Fatal("delivery adapter mutated source report")
	}
	if !strings.Contains(first.String(), "\"schema_version\":\"v1\"") ||
		strings.Contains(first.String(), "retry") ||
		strings.Contains(first.String(), "resubmit") ||
		strings.Contains(first.String(), "repair") {
		t.Fatalf("delivery output violated read-only contract: %s", first.String())
	}
}

func TestEncodeReconciliationOperatorReportV1RejectsNilWriter(t *testing.T) {
	if err := EncodeReconciliationOperatorReportV1(nil, ReconciliationReport{}); err == nil {
		t.Fatal("expected nil writer error")
	}
}

func startTimeForOperatorDeliveryTest() time.Time {
	return time.Date(2026, 10, 4, 12, 59, 0, 0, time.UTC)
}
