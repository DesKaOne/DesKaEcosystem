package state

import (
	"errors"
	"math"
	"sync"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
)

var (
	ErrAccountNotFound     = errors.New("account not found")
	ErrInsufficientBalance = errors.New("insufficient balance")
	ErrInvalidAmount       = errors.New("invalid transfer amount")
	ErrNonceMismatch       = errors.New("nonce mismatch")
	ErrBalanceOverflow     = errors.New("balance overflow")
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

	senderKey := string(sender)
	recipientKey := string(recipient)

	from, ok := s.accounts[senderKey]
	if !ok {
		return ErrAccountNotFound
	}
	if from.Nonce != expectedNonce {
		return ErrNonceMismatch
	}
	if from.Balance < amount {
		return ErrInsufficientBalance
	}

	if senderKey == recipientKey {
		from.Nonce++
		s.accounts[senderKey] = from
		return nil
	}

	to := s.accounts[recipientKey]
	if amount > math.MaxUint64-to.Balance {
		return ErrBalanceOverflow
	}

	from.Balance -= amount
	from.Nonce++
	to.Balance += amount

	s.accounts[senderKey] = from
	s.accounts[recipientKey] = to
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
	if snapshot == nil || snapshot == s {
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
