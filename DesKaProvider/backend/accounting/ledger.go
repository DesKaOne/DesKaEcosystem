package accounting

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

type Direction string

const (
	Debit  Direction = "DEBIT"
	Credit Direction = "CREDIT"
)

type Entry struct {
	LineID    int
	AccountID string
	Direction Direction
	Amount    int64
	Currency  string
	Memo      string
}

type LedgerTransaction struct {
	ID             string
	ReferenceID    string
	SourceType     string
	SourceID       string
	Currency       string
	Description    string
	CreatedAt      time.Time
	Entries        []Entry
}

var (
	ErrInvalidLedgerTransaction = errors.New("invalid ledger transaction")
	ErrLedgerConflict            = errors.New("ledger transaction identity conflict")
	ErrLedgerNotFound             = errors.New("ledger transaction not found")
)

func (t LedgerTransaction) Validate() error {
	if t.ID == "" || t.ReferenceID == "" || t.SourceType == "" || t.SourceID == "" || t.Currency == "" {
		return fmt.Errorf("%w: id, reference id, source type, source id, and currency are required", ErrInvalidLedgerTransaction)
	}
	if t.CreatedAt.IsZero() {
		return fmt.Errorf("%w: created at is required", ErrInvalidLedgerTransaction)
	}
	if len(t.Entries) < 2 {
		return fmt.Errorf("%w: at least two entries are required", ErrInvalidLedgerTransaction)
	}
	var debit, credit int64
	for i, e := range t.Entries {
		if e.LineID != i+1 {
			return fmt.Errorf("%w: line ids must be contiguous from 1", ErrInvalidLedgerTransaction)
		}
		if e.AccountID == "" || e.Amount <= 0 || e.Currency != t.Currency {
			return fmt.Errorf("%w: invalid entry %d", ErrInvalidLedgerTransaction, i+1)
		}
		switch e.Direction {
		case Debit:
			debit += e.Amount
		case Credit:
			credit += e.Amount
		default:
			return fmt.Errorf("%w: invalid direction on entry %d", ErrInvalidLedgerTransaction, i+1)
		}
	}
	if debit != credit {
		return fmt.Errorf("%w: debits=%d credits=%d", ErrInvalidLedgerTransaction, debit, credit)
	}
	return nil
}

func sameLedgerTransaction(a, b LedgerTransaction) bool {
	if a.ID != b.ID || a.ReferenceID != b.ReferenceID || a.SourceType != b.SourceType ||
		a.SourceID != b.SourceID || a.Currency != b.Currency ||
		a.Description != b.Description || !a.CreatedAt.Truncate(time.Microsecond).Equal(b.CreatedAt.Truncate(time.Microsecond)) ||
		len(a.Entries) != len(b.Entries) {
		return false
	}
	for i := range a.Entries {
		if a.Entries[i] != b.Entries[i] {
			return false
		}
	}
	return true
}

type Store interface {
	Append(LedgerTransaction) error
	Get(id string) (LedgerTransaction, bool)
	All() []LedgerTransaction
}

type MemoryStore struct {
	mu           sync.RWMutex
	transactions map[string]LedgerTransaction
	audits       map[string]SettlementAudit
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{transactions: make(map[string]LedgerTransaction), audits: make(map[string]SettlementAudit)}
}

func cloneTransaction(t LedgerTransaction) LedgerTransaction {
	t.Entries = append([]Entry(nil), t.Entries...)
	return t
}

func (s *MemoryStore) AppendContext(_ context.Context, t LedgerTransaction) error {
	return s.Append(t)
}

func (s *MemoryStore) GetContext(_ context.Context, id string) (LedgerTransaction, bool, error) {
	t, ok := s.Get(id)
	return t, ok, nil
}

func sameSettlementAudit(a, b SettlementAudit) bool {
	return a.EventID == b.EventID && a.TransactionID == b.TransactionID &&
		a.ReferenceID == b.ReferenceID && a.SourceType == b.SourceType &&
		a.SourceID == b.SourceID && a.Status == b.Status &&
		a.CreatedAt.Truncate(time.Microsecond).Equal(b.CreatedAt.Truncate(time.Microsecond))
}

func (s *MemoryStore) AppendSettlement(_ context.Context, t LedgerTransaction, audit SettlementAudit) error {
	if err := t.Validate(); err != nil {
		return err
	}
	if err := audit.Validate(); err != nil {
		return err
	}
	if audit.TransactionID != t.ID || audit.ReferenceID != t.ReferenceID ||
		audit.SourceType != t.SourceType || audit.SourceID != t.SourceID {
		return ErrLedgerConflict
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if current, ok := s.transactions[t.ID]; ok {
		currentAudit, auditOK := s.audits[t.ID]
		if sameLedgerTransaction(current, t) && auditOK && sameSettlementAudit(currentAudit, audit) {
			return nil
		}
		return ErrLedgerConflict
	}
	if existing, ok := s.audits[audit.TransactionID]; ok && !sameSettlementAudit(existing, audit) {
		return ErrLedgerConflict
	}
	s.transactions[t.ID] = cloneTransaction(t)
	s.audits[t.ID] = audit
	return nil
}

func (s *MemoryStore) GetSettlementAudit(_ context.Context, transactionID string) (SettlementAudit, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	a, ok := s.audits[transactionID]
	return a, ok, nil
}

func (s *MemoryStore) Append(t LedgerTransaction) error {
	if err := t.Validate(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if current, ok := s.transactions[t.ID]; ok {
		if sameLedgerTransaction(current, t) {
			return nil
		}
		return ErrLedgerConflict
	}
	s.transactions[t.ID] = cloneTransaction(t)
	return nil
}

func (s *MemoryStore) Get(id string) (LedgerTransaction, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.transactions[id]
	if !ok {
		return LedgerTransaction{}, false
	}
	return cloneTransaction(t), true
}

func (s *MemoryStore) All() []LedgerTransaction {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]LedgerTransaction, 0, len(s.transactions))
	for _, t := range s.transactions {
		out = append(out, cloneTransaction(t))
	}
	return out
}
