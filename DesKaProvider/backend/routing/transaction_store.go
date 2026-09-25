package routing

import (
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

func (s *MemoryTransactionStore) All() []TransactionState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]TransactionState, 0, len(s.transactions))
	for _, state := range s.transactions {
		result = append(result, state)
	}
	return result
}
