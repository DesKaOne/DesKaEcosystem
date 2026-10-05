package accounting

import (
    "context"
    "errors"
    "testing"
)

type ambiguousSettlementStore struct {
    appendCalls int
    outcome SettlementPersistenceOutcome
    resolveCalls int
    resolveErr error
}

func (s *ambiguousSettlementStore) AppendSettlement(context.Context, LedgerTransaction, SettlementAudit) error {
    s.appendCalls++
    return ErrSettlementPersistenceAmbiguous
}

func (s *ambiguousSettlementStore) GetSettlementAudit(context.Context, string) (SettlementAudit, bool, error) {
    return SettlementAudit{}, false, nil
}

func (s *ambiguousSettlementStore) ResolveSettlementPersistenceOutcome(context.Context, LedgerTransaction, SettlementAudit) (SettlementPersistenceOutcome, error) {
    s.resolveCalls++
    return s.outcome, s.resolveErr
}

func TestSettlementPosterResolvesAppliedAmbiguityWithoutRetry(t *testing.T) {
    store := &ambiguousSettlementStore{outcome: SettlementPersistenceApplied}
    poster, err := NewSettlementPoster(store)
    if err != nil {
        t.Fatal(err)
    }

    if err := poster.Post(context.Background(), validSettlementPosting()); err != nil {
        t.Fatalf("applied ambiguous settlement should resolve as success: %v", err)
    }
    if store.appendCalls != 1 {
        t.Fatalf("expected exactly one settlement write attempt, got %d", store.appendCalls)
    }
    if store.resolveCalls != 1 {
        t.Fatalf("expected exactly one read-only resolution, got %d", store.resolveCalls)
    }
}

func TestSettlementPosterKeepsUnresolvedAmbiguity(t *testing.T) {
    store := &ambiguousSettlementStore{outcome: SettlementPersistenceNotApplied}
    poster, err := NewSettlementPoster(store)
    if err != nil {
        t.Fatal(err)
    }

    err = poster.Post(context.Background(), validSettlementPosting())
    if !errors.Is(err, ErrSettlementPersistenceAmbiguous) {
        t.Fatalf("not-applied ambiguity must remain unresolved, got %v", err)
    }
    if store.appendCalls != 1 || store.resolveCalls != 1 {
        t.Fatalf("expected one write and one read-only resolution, got writes=%d reads=%d", store.appendCalls, store.resolveCalls)
    }
}
