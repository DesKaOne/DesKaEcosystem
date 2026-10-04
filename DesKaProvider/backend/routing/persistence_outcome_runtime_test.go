package routing

import (
    "context"
    "errors"
    "testing"

    provider "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider"
)

type ambiguousTransactionStore struct {
    putErr  error
    outcome PersistenceOutcome
    resolveErr error
    resolveCalls int
}

func (s *ambiguousTransactionStore) Get(string) (TransactionState, bool) { return TransactionState{}, false }
func (s *ambiguousTransactionStore) Put(TransactionState) error { return s.putErr }
func (s *ambiguousTransactionStore) All() []TransactionState { return nil }
func (s *ambiguousTransactionStore) ResolvePersistenceOutcomeContext(context.Context, string, TransactionState) (PersistenceOutcome, TransactionState, error) {
    s.resolveCalls++
    return s.outcome, TransactionState{}, s.resolveErr
}

func TestPersistTransitionResolvesAppliedAmbiguityWithoutRetry(t *testing.T) {
    previous := postgresPendingState()
    next := previous
    next.Execution.Result.Status = provider.StatusSuccess
    store := &ambiguousTransactionStore{
        putErr:  ErrTransactionPersistenceAmbiguous,
        outcome: PersistenceOutcomeApplied,
    }
    service := &Service{Store: store}

    if err := service.persistTransition(context.Background(), previous.Request.ReferenceID, previous, next); err != nil {
        t.Fatalf("applied ambiguous persistence should resolve as success: %v", err)
    }
    if store.resolveCalls != 1 {
        t.Fatalf("expected exactly one read-only resolution, got %d", store.resolveCalls)
    }
}

func TestPersistTransitionKeepsAmbiguityUnresolved(t *testing.T) {
    previous := postgresPendingState()
    next := previous
    next.Execution.Result.Status = provider.StatusSuccess
    store := &ambiguousTransactionStore{
        putErr:  ErrTransactionPersistenceAmbiguous,
        outcome: PersistenceOutcomeNotApplied,
    }
    service := &Service{Store: store}

    err := service.persistTransition(context.Background(), previous.Request.ReferenceID, previous, next)
    if !errors.Is(err, ErrTransactionPersistenceAmbiguous) {
        t.Fatalf("not-applied ambiguity must remain unresolved, got %v", err)
    }
    if store.resolveCalls != 1 {
        t.Fatalf("expected exactly one read-only resolution, got %d", store.resolveCalls)
    }
}
