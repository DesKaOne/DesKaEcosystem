package accounting

import (
    "testing"
    "time"
)

func TestSettlementPersistenceEvidenceDistinguishesPartialAndUnreadable(t *testing.T) {
    partial := SettlementPersistenceEvidence{
        Outcome: SettlementPersistenceConflict,
        LedgerObserved: true,
        AuditObserved: false,
        Source: "postgres.ledger_transactions+settlement_audit",
        ObservedAt: time.Date(2026, 10, 4, 10, 0, 0, 0, time.UTC),
    }
    unreadable := SettlementPersistenceEvidence{
        Outcome: SettlementPersistenceUnknown,
        LedgerObserved: false,
        AuditObserved: false,
        Source: "postgres.ledger_transactions+settlement_audit",
        ObservedAt: partial.ObservedAt,
    }
    if partial.Outcome != SettlementPersistenceConflict || !partial.LedgerObserved || partial.AuditObserved {
        t.Fatalf("partial durable evidence must remain conflict with source observations: %#v", partial)
    }
    if unreadable.Outcome != SettlementPersistenceUnknown || unreadable.LedgerObserved || unreadable.AuditObserved {
        t.Fatalf("unreadable durable evidence must remain unknown and unobserved: %#v", unreadable)
    }
}
