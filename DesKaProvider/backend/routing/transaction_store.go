package routing

import (
	"context"
	"errors"
	"sync"

	provider "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider"
)

type TransactionState struct {
	Request   PurchaseRequest
	Execution PurchaseExecution
}

type TransactionStore interface {
	Get(referenceID string) (TransactionState, bool)
	Put(state TransactionState) error
	All() []TransactionState
}

// ContextTransactionStore is the context-aware persistence boundary.
// TransactionStore remains available for existing callers; new service paths
// prefer these methods so cancellation and deadlines can reach the database.
type ContextTransactionStore interface {
	TransactionStore
	GetContext(ctx context.Context, referenceID string) (TransactionState, bool)
	PutContext(ctx context.Context, state TransactionState) error
	AllContext(ctx context.Context) []TransactionState
	PutIfCurrentContext(ctx context.Context, referenceID string, previous, next TransactionState) error
}

// AtomicTransactionStore provides a compare-and-transition boundary for stores
// that can enforce transaction identity and state transitions atomically.
// Database-backed implementations must map this operation to a single
// transaction/conditional update across processes.
type AtomicTransactionStore interface {
	TransactionStore
	PutIfCurrent(referenceID string, previous, next TransactionState) error
}

var ErrTransactionStateConflict = errors.New("transaction state changed concurrently")

func validateTransactionTransition(previous, next TransactionState) error {
	if previous.Request != next.Request {
		return ErrReferenceConflict
	}
	if previous.Execution.ProviderName != next.Execution.ProviderName {
		return ErrReferenceConflict
	}
	if previous.Execution.Result.Status == provider.StatusSuccess || previous.Execution.Result.Status == provider.StatusFailed {
		if !samePurchaseResult(previous.Execution.Result, next.Execution.Result) {
			return ErrReferenceConflict
		}
		return nil
	}
	if previous.Execution.Result.Status != provider.StatusPending {
		return ErrReferenceConflict
	}
	if next.Execution.Result.Status != provider.StatusPending &&
		next.Execution.Result.Status != provider.StatusSuccess &&
		next.Execution.Result.Status != provider.StatusFailed {
		return ErrReferenceConflict
	}
	return nil
}

type MemoryTransactionStore struct {
	mu           sync.RWMutex
	transactions map[string]TransactionState
}

func NewMemoryTransactionStore() *MemoryTransactionStore {
	return &MemoryTransactionStore{transactions: make(map[string]TransactionState)}
}

var _ ContextTransactionStore = (*MemoryTransactionStore)(nil)

func (s *MemoryTransactionStore) GetContext(ctx context.Context, referenceID string) (TransactionState, bool) {
	if err := ctx.Err(); err != nil {
		return TransactionState{}, false
	}
	return s.Get(referenceID)
}

func (s *MemoryTransactionStore) PutContext(ctx context.Context, state TransactionState) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return s.Put(state)
}

func (s *MemoryTransactionStore) AllContext(ctx context.Context) []TransactionState {
	if err := ctx.Err(); err != nil {
		return nil
	}
	return s.All()
}

func (s *MemoryTransactionStore) PutIfCurrentContext(ctx context.Context, referenceID string, previous, next TransactionState) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return s.PutIfCurrent(referenceID, previous, next)
}

func (s *MemoryTransactionStore) Get(referenceID string) (TransactionState, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	state, ok := s.transactions[referenceID]
	return state, ok
}

func (s *MemoryTransactionStore) Put(state TransactionState) error {
	if state.Request.ReferenceID == "" {
		return errors.New("transaction reference ID is required")
	}
	if state.Execution.ProviderName == "" {
		return errors.New("transaction provider name is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if previous, ok := s.transactions[state.Request.ReferenceID]; ok {
		if err := validateTransactionTransition(previous, state); err != nil {
			return err
		}
	}
	s.transactions[state.Request.ReferenceID] = state
	return nil
}

func (s *MemoryTransactionStore) PutIfCurrent(referenceID string, previous, next TransactionState) error {
	if referenceID == "" || next.Request.ReferenceID != referenceID || previous.Request.ReferenceID != referenceID {
		return ErrReferenceConflict
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	current, ok := s.transactions[referenceID]
	if !ok || current != previous {
		return ErrTransactionStateConflict
	}
	if err := validateTransactionTransition(previous, next); err != nil {
		return err
	}
	s.transactions[referenceID] = next
	return nil
}

func (s *MemoryTransactionStore) All() []TransactionState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]TransactionState, 0, len(s.transactions))
	for _, state := range s.transactions {
		result = append(result, state)
	}
	return result
}
