package routing

import (
    "testing"
    "time"
)

func TestPersistenceOutcomeEvidenceDistinguishesAbsentFromUnreadable(t *testing.T) {
    absent := PersistenceOutcomeEvidence{
        Outcome: PersistenceOutcomeNotApplied,
        Observed: true,
        Source: "postgres.provider_transactions",
        ObservedAt: time.Date(2026, 10, 4, 10, 0, 0, 0, time.UTC),
    }
    unreadable := PersistenceOutcomeEvidence{
        Outcome: PersistenceOutcomeUnknown,
        Observed: false,
        Source: "postgres.provider_transactions",
        ObservedAt: absent.ObservedAt,
    }
    if !absent.Observed || absent.Outcome != PersistenceOutcomeNotApplied {
        t.Fatalf("absent durable state must be explicitly observed as not_applied: %#v", absent)
    }
    if unreadable.Observed || unreadable.Outcome != PersistenceOutcomeUnknown {
        t.Fatalf("unreadable durable state must remain unknown and unobserved: %#v", unreadable)
    }
}
