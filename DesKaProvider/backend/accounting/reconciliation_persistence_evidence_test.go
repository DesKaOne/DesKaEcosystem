package accounting

import (
	"testing"
	"time"

	"github.com/DesKaOne/DesKaEcosystem/DesKaProvider/routing"
)

func TestClassifyTransactionPersistenceEvidence(t *testing.T) {
	now := time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name     string
		evidence routing.PersistenceOutcomeEvidence
		want     ReconciliationPersistenceResolution
	}{
		{"applied observed", routing.PersistenceOutcomeEvidence{Outcome: routing.PersistenceOutcomeApplied, Observed: true, Source: "postgres.provider_transactions", ObservedAt: now, Version: 7}, ReconciliationPersistenceConfirmedApplied},
		{"not applied observed", routing.PersistenceOutcomeEvidence{Outcome: routing.PersistenceOutcomeNotApplied, Observed: true, Source: "postgres.provider_transactions", ObservedAt: now}, ReconciliationPersistenceConfirmedNotApplied},
		{"unknown unreadable", routing.PersistenceOutcomeEvidence{Outcome: routing.PersistenceOutcomeUnknown, Observed: false, Source: "postgres.provider_transactions", ObservedAt: now}, ReconciliationPersistenceNeedsReviewUnknown},
		{"conflict observed", routing.PersistenceOutcomeEvidence{Outcome: routing.PersistenceOutcomeConflict, Observed: true, Source: "postgres.provider_transactions", ObservedAt: now}, ReconciliationPersistenceNeedsReviewConflict},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClassifyTransactionPersistenceEvidence(tt.evidence)
			if got.Resolution != tt.want {
				t.Fatalf("resolution = %q, want %q", got.Resolution, tt.want)
			}
			if !got.ObservedAt.Equal(now) || got.Source != tt.evidence.Source || got.Version != tt.evidence.Version {
				t.Fatalf("evidence provenance was not preserved: %+v", got)
			}
		})
	}
}

func TestClassifySettlementPersistenceEvidence(t *testing.T) {
	now := time.Date(2026, 10, 4, 12, 1, 0, 0, time.UTC)
	tests := []struct {
		name     string
		evidence SettlementPersistenceEvidence
		want     ReconciliationPersistenceResolution
	}{
		{"applied both observed", SettlementPersistenceEvidence{Outcome: SettlementPersistenceApplied, LedgerObserved: true, AuditObserved: true, Source: "postgres.ledger_transactions+settlement_audit", ObservedAt: now}, ReconciliationPersistenceConfirmedApplied},
		{"not applied both absent", SettlementPersistenceEvidence{Outcome: SettlementPersistenceNotApplied, LedgerObserved: true, AuditObserved: true, Source: "postgres.ledger_transactions+settlement_audit", ObservedAt: now}, ReconciliationPersistenceConfirmedNotApplied},
		{"partial conflict", SettlementPersistenceEvidence{Outcome: SettlementPersistenceConflict, LedgerObserved: true, AuditObserved: false, Source: "postgres.ledger_transactions+settlement_audit", ObservedAt: now}, ReconciliationPersistenceNeedsReviewConflict},
		{"unknown unreadable", SettlementPersistenceEvidence{Outcome: SettlementPersistenceUnknown, LedgerObserved: false, AuditObserved: false, Source: "postgres.ledger_transactions+settlement_audit", ObservedAt: now}, ReconciliationPersistenceNeedsReviewUnknown},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClassifySettlementPersistenceEvidence(tt.evidence)
			if got.Resolution != tt.want {
				t.Fatalf("resolution = %q, want %q", got.Resolution, tt.want)
			}
			if got.LedgerObserved != tt.evidence.LedgerObserved || got.AuditObserved != tt.evidence.AuditObserved {
				t.Fatalf("settlement observation flags were not preserved: %+v", got)
			}
		})
	}
}

func TestReconciliationReportWithPersistenceEvidenceIsNonMutating(t *testing.T) {
	start := time.Date(2026, 10, 4, 12, 2, 0, 0, time.UTC)
	end := start.Add(250 * time.Millisecond)
	report := ReconciliationReport{
		Items: []TransactionReconciliation{{ReferenceID: "ref-1", Status: ReconciliationCorrelated}},
		Snapshot: ReconciliationSnapshotMetadata{
			CaptureStartedAt: start, CaptureCompletedAt: end, SnapshotConsistency: ReconciliationSnapshotConsistencyCapturedVerified,
		},
	}
	evidence := ReconciliationPersistenceEvidence{
		Resolution: ReconciliationPersistenceNeedsReviewUnknown,
		Outcome: "unknown",
		Observed: false,
		Source: "postgres.provider_transactions",
		ObservedAt: end,
	}
	enriched := report.WithPersistenceEvidence("ref-1", evidence)
	if report.Items[0].PersistenceEvidence != nil {
		t.Fatal("original report was mutated")
	}
	if enriched.Items[0].PersistenceEvidence == nil {
		t.Fatal("persistence evidence was not surfaced")
	}
	got := enriched.Items[0].PersistenceEvidence
	if !got.CaptureStartedAt.Equal(start) || !got.CaptureCompletedAt.Equal(end) || got.SnapshotConsistency != ReconciliationSnapshotConsistencyCapturedVerified {
		t.Fatalf("capture-window metadata not attached: %+v", got)
	}
	if got.Resolution != ReconciliationPersistenceNeedsReviewUnknown {
		t.Fatalf("resolution = %q, want unknown review", got.Resolution)
	}
}
