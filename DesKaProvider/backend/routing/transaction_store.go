package routing

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	provider "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider"
	payment "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/internal/Payment"
)

type TransactionKind string
const (
	TransactionKindPPOB TransactionKind = "ppob"
	TransactionKindPayment TransactionKind = "payment"
)
type TransactionState struct {
	Kind TransactionKind
	Request PurchaseRequest
	Execution PurchaseExecution
	Payment *payment.Transaction
	Version int64
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
// ContextReadTransactionStore is an optional read-error-aware extension.
// It preserves ContextTransactionStore compatibility while allowing request-scoped
// callers to distinguish cancellation, database failures, and not-found results.
type ContextReadTransactionStore interface {
	ContextTransactionStore
	GetContextE(ctx context.Context, referenceID string) (TransactionState, bool, error)
	AllContextE(ctx context.Context) ([]TransactionState, error)
}

// CreateIfAbsentTransactionStore is the submission-authorization boundary.
// A successful create returns created=true; an existing reference returns its
// durable state with created=false. Implementations must make the decision
// atomically so concurrent service instances cannot both authorize submission.
type CreateIfAbsentTransactionStore interface {
	CreateIfAbsentContext(ctx context.Context, state TransactionState) (existing TransactionState, created bool, err error)
}

// AtomicTransactionStore provides a compare-and-transition boundary for stores
// that can enforce transaction identity and state transitions atomically.
// Database-backed implementations must map this operation to a single
// transaction/conditional update across processes.
type AtomicTransactionStore interface {
	TransactionStore
	PutIfCurrent(referenceID string, previous, next TransactionState) error
}


// TransactionAuditEvent is an append-only operational record for transaction
// lifecycle and reconciliation observations. It is deliberately separate from
// TransactionState so audit failures cannot mutate financial state.
type TransactionAuditEvent struct {
	ReferenceID string
	Action      string
	Previous    string
	Next        string
	ProviderName string
	Message     string
	CreatedAt   time.Time
}

// TransactionAuditStore persists transaction audit events without allowing
// callers to update or delete previously appended records.
type TransactionAuditStore interface {
	Append(event TransactionAuditEvent) error
	All(referenceID string) []TransactionAuditEvent
}

// ContextReadTransactionAuditStore is an optional error-aware read boundary.
// It preserves TransactionAuditStore compatibility while allowing callers to
// distinguish cancellation, deadlines, database failures, and successful reads.
type ContextReadTransactionAuditStore interface {
	TransactionAuditStore
	AllContextE(ctx context.Context, referenceID string) ([]TransactionAuditEvent, error)
}

var (
	ErrTransactionStateConflict = errors.New("transaction state changed concurrently")
	// ErrTransactionPersistenceAmbiguous means a durable write was attempted but
	// the persistence layer could not establish whether the mutation committed.
	// Callers must reconcile/read durable state rather than retrying the external
	// provider operation.
	ErrTransactionPersistenceAmbiguous = errors.New("transaction persistence outcome is ambiguous")
)

func wrapTransactionPersistenceAmbiguous(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%w: %w", ErrTransactionPersistenceAmbiguous, err)
}

func normalizeTransactionKind(kind TransactionKind) TransactionKind {
	if kind == "" { return TransactionKindPPOB }
	return kind
}
func transactionReferenceID(state TransactionState) string {
	if normalizeTransactionKind(state.Kind) == TransactionKindPayment && state.Payment != nil { return state.Payment.ReferenceID }
	return state.Request.ReferenceID
}
func sameTransactionIdentity(a,b TransactionState) bool {
	if normalizeTransactionKind(a.Kind) != normalizeTransactionKind(b.Kind) || a.Execution.ProviderName != b.Execution.ProviderName { return false }
	switch normalizeTransactionKind(a.Kind) {
	case TransactionKindPPOB: return a.Request == b.Request
	case TransactionKindPayment:
		if a.Payment == nil || b.Payment == nil { return false }
		return a.Payment.ReferenceID == b.Payment.ReferenceID && a.Payment.Amount == b.Payment.Amount && a.Payment.Currency == b.Payment.Currency && a.Payment.CustomerID == b.Payment.CustomerID && a.Payment.Description == b.Payment.Description
	default: return false
	}
}
func validateTransactionTransition(previous,next TransactionState) error {
	if !sameTransactionIdentity(previous,next) { return ErrReferenceConflict }
	switch normalizeTransactionKind(previous.Kind) {
	case TransactionKindPPOB:
		if previous.Execution.Result.Status == provider.StatusSuccess || previous.Execution.Result.Status == provider.StatusFailed {
			if !samePurchaseResult(previous.Execution.Result,next.Execution.Result) { return ErrReferenceConflict }; return nil
		}
		if previous.Execution.Result.Status != provider.StatusPending { return ErrReferenceConflict }
		if next.Execution.Result.Status != provider.StatusPending && next.Execution.Result.Status != provider.StatusSuccess && next.Execution.Result.Status != provider.StatusFailed { return ErrReferenceConflict }
		return nil
	case TransactionKindPayment: return validatePaymentTransactionTransition(previous,next)
	default: return ErrReferenceConflict
	}
}

type MemoryTransactionStore struct {
	mu           sync.RWMutex
	transactions map[string]TransactionState
}

func NewMemoryTransactionStore() *MemoryTransactionStore {
	return &MemoryTransactionStore{transactions: make(map[string]TransactionState)}
}

var _ ContextTransactionStore = (*MemoryTransactionStore)(nil)
var _ ContextReadTransactionStore = (*MemoryTransactionStore)(nil)
var _ CreateIfAbsentTransactionStore = (*MemoryTransactionStore)(nil)

func (s *MemoryTransactionStore) GetContextE(ctx context.Context, referenceID string) (TransactionState, bool, error) {
	if err := ctx.Err(); err != nil { return TransactionState{}, false, err }
	state, ok := s.Get(referenceID)
	return state, ok, nil
}

func (s *MemoryTransactionStore) AllContextE(ctx context.Context) ([]TransactionState, error) {
	if err := ctx.Err(); err != nil { return nil, err }
	return s.All(), nil
}

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

func (s *MemoryTransactionStore) CreateIfAbsentContext(ctx context.Context, state TransactionState) (TransactionState, bool, error) {
	if err := ctx.Err(); err != nil {
		return TransactionState{}, false, err
	}
	if err := validateTransactionState(state); err != nil { return TransactionState{}, false, err }
	s.mu.Lock()
	defer s.mu.Unlock()
	if current, ok := s.transactions[transactionReferenceID(state)]; ok {
		if !sameTransactionIdentity(current,state) { return TransactionState{}, false, ErrReferenceConflict }
		return current, false, nil
	}
	s.transactions[transactionReferenceID(state)] = state
	return state, true, nil
}

func (s *MemoryTransactionStore) Get(referenceID string) (TransactionState, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	state, ok := s.transactions[referenceID]
	return state, ok
}

func (s *MemoryTransactionStore) Put(state TransactionState) error {
	if err := validateTransactionState(state); err != nil { return err }
	s.mu.Lock()
	defer s.mu.Unlock()
	if previous, ok := s.transactions[transactionReferenceID(state)]; ok {
		if err := validateTransactionTransition(previous, state); err != nil {
			return err
		}
	}
	s.transactions[transactionReferenceID(state)] = state
	return nil
}

func (s *MemoryTransactionStore) PutIfCurrent(referenceID string, previous, next TransactionState) error {
	if referenceID == "" {
		return ErrReferenceConflict
	}
	if transactionReferenceID(previous) != referenceID || transactionReferenceID(next) != referenceID {
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
