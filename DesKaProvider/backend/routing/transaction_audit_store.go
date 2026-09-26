package routing

import (
	"errors"
	"sync"
)

// MemoryTransactionAuditStore is an append-only in-memory audit store for
// deterministic tests and local development.
type MemoryTransactionAuditStore struct {
	mu     sync.RWMutex
	events []TransactionAuditEvent
}

func NewMemoryTransactionAuditStore() *MemoryTransactionAuditStore {
	return &MemoryTransactionAuditStore{}
}

var _ TransactionAuditStore = (*MemoryTransactionAuditStore)(nil)
var _ ContextReadTransactionAuditStore = (*MemoryTransactionAuditStore)(nil)

func (s *MemoryTransactionAuditStore) Append(event TransactionAuditEvent) error {
	if event.ReferenceID == "" {
		return errors.New("audit reference ID is required")
	}
	if event.Action == "" {
		return errors.New("audit action is required")
	}
	if event.CreatedAt.IsZero() {
		return errors.New("audit created_at is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, event)
	return nil
}

func (s *MemoryTransactionAuditStore) AllContextE(ctx context.Context, referenceID string) ([]TransactionAuditEvent, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return s.All(referenceID), nil
}

func (s *MemoryTransactionAuditStore) All(referenceID string) []TransactionAuditEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []TransactionAuditEvent
	for _, event := range s.events {
		if event.ReferenceID == referenceID {
			result = append(result, event)
		}
	}
	return result
}

func TestMemoryTransactionAuditStoreAllContextEPropagatesCancellation(t *testing.T) {
	store := NewMemoryTransactionAuditStore()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	events, err := store.AllContextE(ctx, "ref-1")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation, got events=%#v err=%v", events, err)
	}
	if events != nil {
		t.Fatalf("canceled audit read must not expose history, got %#v", events)
	}
}

