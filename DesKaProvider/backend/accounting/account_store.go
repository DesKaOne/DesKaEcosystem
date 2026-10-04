package accounting

import (
	"errors"
	"sync"
)

var ErrAccountConflict = errors.New("account identity conflict")

type AccountStore interface {
	CreateIfAbsent(Account) (Account, bool, error)
	Get(id string) (Account, bool)
}

type MemoryAccountStore struct {
	mu       sync.RWMutex
	accounts map[string]Account
}

func NewMemoryAccountStore() *MemoryAccountStore {
	return &MemoryAccountStore{accounts: make(map[string]Account)}
}

func (s *MemoryAccountStore) CreateIfAbsent(account Account) (Account, bool, error) {
	if err := account.Validate(); err != nil {
		return Account{}, false, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if current, ok := s.accounts[account.ID]; ok {
		if current == account {
			return current, false, nil
		}
		return Account{}, false, ErrAccountConflict
	}
	s.accounts[account.ID] = account
	return account, true, nil
}

func (s *MemoryAccountStore) Get(id string) (Account, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	account, ok := s.accounts[id]
	return account, ok
}
