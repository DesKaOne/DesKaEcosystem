package state

import (
	"errors"
	"sync"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
)

var (
	ErrAccountNotFound     = errors.New("account not found")
	ErrInsufficientBalance = errors.New("insufficient balance")
	ErrInvalidAmount       = errors.New("invalid transfer amount")
	ErrNonceMismatch       = errors.New("nonce mismatch")
)

type Account struct {
	Balance uint64
	Nonce   types.Nonce
}

// State is an in-memory development representation of canonical account state.
// Persistence and state commitment are intentionally left to later protocol work.
type State struct {
	mu       sync.RWMutex
	accounts map[string]Account
}

func New() *State {
	return &State{accounts: make(map[string]Account)}
}

func (s *State) Get(address types.Address) (Account, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	account, ok := s.accounts[string(address)]
	return account, ok
}

func (s *State) Set(address types.Address, account Account) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.accounts[string(address)] = account
}

func (s *State) Delete(address types.Address) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.accounts, string(address))
}

func (s *State) Transfer(sender, recipient types.Address, amount uint64, expectedNonce types.Nonce) error {
	if amount == 0 {
		return ErrInvalidAmount
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	from, ok := s.accounts[string(sender)]
	if !ok {
		return ErrAccountNotFound
	}
	if from.Nonce != expectedNonce {
		return ErrNonceMismatch
	}
	if from.Balance < amount {
		return ErrInsufficientBalance
	}

	to := s.accounts[string(recipient)]
	from.Balance -= amount
	from.Nonce++
	to.Balance += amount

	s.accounts[string(sender)] = from
	s.accounts[string(recipient)] = to
	return nil
}

// Snapshot returns a deep copy suitable for deterministic execution experiments.
func (s *State) Snapshot() *State {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := New()
	for address, account := range s.accounts {
		out.accounts[address] = account
	}
	return out
}

// Replace replaces the current state with a snapshot.
func (s *State) Replace(snapshot *State) {
	if snapshot == nil {
		return
	}

	snapshot.mu.RLock()
	defer snapshot.mu.RUnlock()

	s.mu.Lock()
	defer s.mu.Unlock()

	s.accounts = make(map[string]Account, len(snapshot.accounts))
	for address, account := range snapshot.accounts {
		s.accounts[address] = account
	}
}
