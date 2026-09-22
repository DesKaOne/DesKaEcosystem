package payment

import (
	"context"
	"errors"
	"sync"
)

var ErrDuplicatePayment = errors.New("duplicate payment")

type MemoryPaymentStore struct {
	mu       sync.RWMutex
	payments map[string]Payment
	byKey    map[string]string
}

func NewMemoryPaymentStore() *MemoryPaymentStore {
	return &MemoryPaymentStore{
		payments: make(map[string]Payment),
		byKey:    make(map[string]string),
	}
}

func (s *MemoryPaymentStore) Create(ctx context.Context, payment Payment) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.payments[payment.ID]; exists {
		return ErrDuplicatePayment
	}

	if existingID, exists := s.byKey[payment.IdempotencyKey]; exists && existingID != payment.ID {
		return ErrDuplicatePayment
	}

	s.payments[payment.ID] = payment
	s.byKey[payment.IdempotencyKey] = payment.ID
	return nil
}

func (s *MemoryPaymentStore) Get(ctx context.Context, id string) (Payment, error) {
	if err := ctx.Err(); err != nil {
		return Payment{}, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	payment, ok := s.payments[id]
	if !ok {
		return Payment{}, ErrPaymentNotFound
	}
	return payment, nil
}

func (s *MemoryPaymentStore) Save(ctx context.Context, payment Payment) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.payments[payment.ID]; !exists {
		return ErrPaymentNotFound
	}

	s.payments[payment.ID] = payment
	s.byKey[payment.IdempotencyKey] = payment.ID
	return nil
}
