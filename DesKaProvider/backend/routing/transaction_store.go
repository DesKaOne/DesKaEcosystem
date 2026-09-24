package routing

import (
	"errors"
	"sync"
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
