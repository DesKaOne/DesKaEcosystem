package routing

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	provider "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider"
)

var (
	ErrInvalidPurchaseRequest      = errors.New("invalid provider purchase request")
	ErrReferenceConflict           = errors.New("provider reference ID already used with different request")
	ErrWebhookTransactionNotFound  = errors.New("provider webhook reference ID not found")
	ErrWebhookReferenceConflict    = errors.New("provider webhook conflicts with stored transaction")
	ErrInvalidWebhookEvent         = errors.New("invalid provider webhook event")
)

type PurchaseRequest struct {
	ProductCode string
	CustomerNo  string
	ReferenceID string
	Amount      int64
	Testing     bool
}

type PurchaseExecution struct {
	ProviderName string
	Result       provider.PurchaseResult
}

type Service struct {
	Router       *Router
	Store        TransactionStore
	mu           sync.Mutex
	transactions map[string]*purchaseCall
}

type purchaseCall struct {
	request PurchaseRequest
	done    chan struct{}
	result  PurchaseExecution
	err     error
}

func NewService(router *Router) (*Service, error) {
	return NewServiceWithStore(router, NewMemoryTransactionStore())
}

func NewServiceWithStore(router *Router, store TransactionStore) (*Service, error) {
	if router == nil {
		return nil, errors.New("provider router is required")
	}
	if store == nil {
		return nil, errors.New("transaction store is required")
	}

	service := &Service{
		Router:       router,
		Store:        store,
		transactions: make(map[string]*purchaseCall),
	}
	for _, state := range store.All() {
		if state.Request.ReferenceID == "" || state.Execution.ProviderName == "" {
			return nil, errors.New("invalid persisted transaction state")
		}
		call := &purchaseCall{
			request: state.Request,
			done:    make(chan struct{}),
			result:  state.Execution,
		}
		close(call.done)
		service.transactions[state.Request.ReferenceID] = call
	}
	return service, nil
}

func (s *Service) Purchase(ctx context.Context, req PurchaseRequest) (PurchaseExecution, error) {
	if err := validatePurchaseRequest(req); err != nil {
		return PurchaseExecution{}, err
	}
	if err := ctx.Err(); err != nil {
		return PurchaseExecution{}, err
	}

	call, owner := s.startPurchase(req)
	if call == nil {
		return PurchaseExecution{}, ErrReferenceConflict
	}
	if !owner {
		select {
		case <-call.done:
			return call.result, call.err
		case <-ctx.Done():
			return PurchaseExecution{}, ctx.Err()
		}
	}

	providerName, err := s.selectProvider(ctx, req)
	if err != nil {
		s.finishPurchase(call, PurchaseExecution{}, err)
		return PurchaseExecution{}, err
	}

	// Persist the selected provider and a pending state before the external
	// submission. A crash after this point must recover to reconciliation,
	// not silently create a second provider submission.
	pending := PurchaseExecution{
		ProviderName: providerName,
		Result: provider.PurchaseResult{
			ReferenceID: req.ReferenceID,
			CustomerNo:  req.CustomerNo,
			ProductCode: req.ProductCode,
			Status:      provider.StatusPending,
		},
	}
	if err := s.Store.Put(TransactionState{Request: req, Execution: pending}); err != nil {
		err = fmt.Errorf("persist pending transaction state: %w", err)
		s.finishPurchase(call, pending, err)
		return pending, err
	}
	s.mu.Lock()
	call.result = pending
	s.mu.Unlock()

	result, err := s.executePurchase(ctx, providerName, req)
	if err == nil {
		if storeErr := s.Store.Put(TransactionState{Request: req, Execution: result}); storeErr != nil {
			err = fmt.Errorf("persist transaction result: %w", storeErr)
		}
	}
	if err != nil {
		// Keep the durable pending state. The caller receives the provider error,
		// while a restart can recover this reference and reconcile it safely.
		s.finishPurchase(call, pending, err)
		return pending, err
	}
	s.finishPurchase(call, result, nil)
	return result, nil
}

func (s *Service) HandleWebhook(ctx context.Context, event provider.WebhookEvent) (PurchaseExecution, error) {
	if err := validateWebhookEvent(event); err != nil {
		return PurchaseExecution{}, err
	}
	if err := ctx.Err(); err != nil {
		return PurchaseExecution{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	call, ok := s.transactions[event.ReferenceID]
	if !ok {
		return PurchaseExecution{}, ErrWebhookTransactionNotFound
	}
	if call.request.ProductCode != event.ProductCode || call.request.CustomerNo != event.CustomerNo {
		return PurchaseExecution{}, ErrWebhookReferenceConflict
	}

	select {
	case <-call.done:
	default:
		return PurchaseExecution{}, ErrWebhookReferenceConflict
	}

	incoming := provider.PurchaseResult{
		ReferenceID:  event.ReferenceID,
		CustomerNo:   event.CustomerNo,
		ProductCode:  event.ProductCode,
		Status:       event.Status,
		ProviderCode: event.ProviderCode,
		Message:      event.Message,
		SerialNumber: event.SerialNumber,
		Price:        event.Price,
	}

	current := call.result.Result
	if current.Status == provider.StatusSuccess || current.Status == provider.StatusFailed {
		if samePurchaseResult(current, incoming) {
			return call.result, nil
		}
		return PurchaseExecution{}, ErrWebhookReferenceConflict
	}
	if current.Status != provider.StatusPending {
		return PurchaseExecution{}, ErrWebhookReferenceConflict
	}
	if incoming.Status == provider.StatusPending {
		if err := s.persistLocked(call.request, PurchaseExecution{ProviderName: call.result.ProviderName, Result: incoming}); err != nil {
			return PurchaseExecution{}, err
		}
		call.result.Result = incoming
		return call.result, nil
	}
	if incoming.Status != provider.StatusSuccess && incoming.Status != provider.StatusFailed {
		return PurchaseExecution{}, ErrInvalidWebhookEvent
	}

	next := PurchaseExecution{ProviderName: call.result.ProviderName, Result: incoming}
	if err := s.persistLocked(call.request, next); err != nil {
		return PurchaseExecution{}, err
	}
	call.result = next
	return call.result, nil
}

func (s *Service) Reconcile(ctx context.Context, referenceID string) (PurchaseExecution, error) {
	if strings.TrimSpace(referenceID) == "" {
		return PurchaseExecution{}, fmt.Errorf("%w: reference ID is required", ErrInvalidPurchaseRequest)
	}
	if err := ctx.Err(); err != nil {
		return PurchaseExecution{}, err
	}

	s.mu.Lock()
	call, ok := s.transactions[referenceID]
	if !ok {
		s.mu.Unlock()
		return PurchaseExecution{}, ErrWebhookTransactionNotFound
	}
	request := call.request
	providerName := call.result.ProviderName
	if providerName == "" {
		s.mu.Unlock()
		return PurchaseExecution{}, ErrWebhookReferenceConflict
	}
	select {
	case <-call.done:
	default:
		s.mu.Unlock()
		return PurchaseExecution{}, ErrWebhookReferenceConflict
	}
	s.mu.Unlock()

	p, err := s.Router.Registry.Get(providerName)
	if err != nil {
		return PurchaseExecution{}, fmt.Errorf("get provider for reconciliation: %w", err)
	}
	status, err := p.GetStatus(ctx, provider.StatusRequest{
		ProductCode: request.ProductCode,
		CustomerNo:  request.CustomerNo,
		ReferenceID: referenceID,
	})
	if err != nil {
		return PurchaseExecution{}, fmt.Errorf("get status from provider %q: %w", providerName, err)
	}
	if status.ReferenceID != referenceID ||
		status.ProductCode != request.ProductCode ||
		status.CustomerNo != request.CustomerNo {
		return PurchaseExecution{}, ErrWebhookReferenceConflict
	}
	if status.Status != provider.StatusPending &&
		status.Status != provider.StatusSuccess &&
		status.Status != provider.StatusFailed {
		return PurchaseExecution{}, fmt.Errorf("%w: unsupported reconciliation status", ErrInvalidWebhookEvent)
	}

	incoming := provider.PurchaseResult{
		ReferenceID:  status.ReferenceID,
		CustomerNo:   status.CustomerNo,
		ProductCode:  status.ProductCode,
		Status:       status.Status,
		ProviderCode: status.ProviderCode,
		Message:      status.Message,
		SerialNumber: status.SerialNumber,
		Price:        status.Price,
	}

	s.mu.Lock()
	defer s.mu.Unlock()
current := call.result.Result
	if current.Status == provider.StatusSuccess || current.Status == provider.StatusFailed {
		if samePurchaseResult(current, incoming) {
			return call.result, nil
		}
		return PurchaseExecution{}, ErrWebhookReferenceConflict
	}
	if current.Status != provider.StatusPending {
		return PurchaseExecution{}, ErrWebhookReferenceConflict
	}
	next := PurchaseExecution{ProviderName: call.result.ProviderName, Result: incoming}
	expected := TransactionState{Request: call.request, Execution: call.result}
	if err := s.persistTransition(referenceID, expected, TransactionState{Request: call.request, Execution: next}); err != nil {
		if errors.Is(err, ErrTransactionStateConflict) {
			latest, ok := s.Store.Get(referenceID)
			if ok && latest.Request == call.request &&
				latest.Execution.ProviderName == call.result.ProviderName &&
				samePurchaseResult(latest.Execution.Result, incoming) {
				call.result = latest.Execution
				return call.result, nil
			}
			return PurchaseExecution{}, ErrWebhookReferenceConflict
		}
		return PurchaseExecution{}, err
	}
	call.result = next
	return call.result, nil
}

func (s *Service) persistLocked(request PurchaseRequest, execution PurchaseExecution) error {
	if err := s.Store.Put(TransactionState{Request: request, Execution: execution}); err != nil {
		return fmt.Errorf("persist transaction state: %w", err)
	}
	return nil
}

func (s *Service) persistTransition(referenceID string, previous, next TransactionState) error {
	if store, ok := s.Store.(AtomicTransactionStore); ok {
		if err := store.PutIfCurrent(referenceID, previous, next); err != nil {
			return fmt.Errorf("persist atomic transaction transition: %w", err)
		}
		return nil
	}
	if err := s.Store.Put(next); err != nil {
		return fmt.Errorf("persist transaction transition: %w", err)
	}
	return nil
}

func (s *Service) startPurchase(req PurchaseRequest) (*purchaseCall, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, ok := s.transactions[req.ReferenceID]; ok {
		if existing.request != req {
			return nil, false
		}
		return existing, false
	}
	call := &purchaseCall{request: req, done: make(chan struct{})}
	s.transactions[req.ReferenceID] = call
	return call, true
}

func (s *Service) selectProvider(ctx context.Context, req PurchaseRequest) (string, error) {
	name, err := s.Router.Select(ctx, Request{ProductCode: req.ProductCode, Amount: req.Amount})
	if err != nil {
		return "", err
	}
	if _, err := s.Router.Registry.Get(name); err != nil {
		return "", fmt.Errorf("get selected provider: %w", err)
	}
	return name, nil
}

func (s *Service) executePurchase(ctx context.Context, providerName string, req PurchaseRequest) (PurchaseExecution, error) {
	p, err := s.Router.Registry.Get(providerName)
	if err != nil {
		return PurchaseExecution{}, fmt.Errorf("get selected provider: %w", err)
	}
	result, err := p.Purchase(ctx, provider.PurchaseRequest{
		ProductCode: req.ProductCode,
		CustomerNo:  req.CustomerNo,
		ReferenceID: req.ReferenceID,
		Testing:     req.Testing,
	})
	if err != nil {
		return PurchaseExecution{}, fmt.Errorf("purchase with provider %q: %w", providerName, err)
	}
	return PurchaseExecution{ProviderName: providerName, Result: result}, nil
}

func (s *Service) finishPurchase(call *purchaseCall, result PurchaseExecution, err error) {
	s.mu.Lock()
	call.result, call.err = result, err
	s.mu.Unlock()
	close(call.done)
}

func validatePurchaseRequest(req PurchaseRequest) error {
	if strings.TrimSpace(req.ProductCode) == "" ||
		strings.TrimSpace(req.CustomerNo) == "" ||
		strings.TrimSpace(req.ReferenceID) == "" ||
		req.Amount <= 0 {
		return fmt.Errorf("%w: product code, customer number, reference ID, and positive amount are required", ErrInvalidPurchaseRequest)
	}
	return nil
}

func validateWebhookEvent(event provider.WebhookEvent) error {
	if strings.TrimSpace(event.ReferenceID) == "" ||
		strings.TrimSpace(event.ProductCode) == "" ||
		strings.TrimSpace(event.CustomerNo) == "" {
		return fmt.Errorf("%w: reference ID, product code, and customer number are required", ErrInvalidWebhookEvent)
	}
	if event.Status != provider.StatusPending &&
		event.Status != provider.StatusSuccess &&
		event.Status != provider.StatusFailed {
		return fmt.Errorf("%w: unsupported transaction status", ErrInvalidWebhookEvent)
	}
	return nil
}

func samePurchaseResult(a, b provider.PurchaseResult) bool {
	return a.ReferenceID == b.ReferenceID &&
		a.CustomerNo == b.CustomerNo &&
		a.ProductCode == b.ProductCode &&
		a.Status == b.Status &&
		a.ProviderCode == b.ProviderCode &&
		a.Message == b.Message &&
		a.SerialNumber == b.SerialNumber &&
		a.Price == b.Price
}
