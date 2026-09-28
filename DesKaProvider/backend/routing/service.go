package routing

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	provider "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider"
	payment "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/internal/Payment"
)

var (
	ErrPaymentCapabilityDisabled = errors.New("payment capability is not enabled")
	ErrPaymentSubmissionClaimed = errors.New("payment submission already claimed")
	ErrPaymentSubmissionConflict = errors.New("payment submission state conflict")
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
	AuditStore   TransactionAuditStore
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
	return NewServiceWithStoreAndAudit(router, NewMemoryTransactionStore(), NewMemoryTransactionAuditStore())
}

// NewServiceWithStoreContext constructs a service using the caller's initialization
// context when the configured transaction store supports error-aware context reads.
// This prevents startup database failures from being mistaken for an empty store.
func NewServiceWithStoreContext(ctx context.Context, router *Router, store TransactionStore) (*Service, error) {
	return NewServiceWithStoreContextAndAudit(ctx, router, store, NewMemoryTransactionAuditStore())
}

func NewServiceWithStoreContextAndAudit(ctx context.Context, router *Router, store TransactionStore, auditStore TransactionAuditStore) (*Service, error) {
	if ctx == nil {
		return nil, errors.New("initialization context is required")
	}
	return newServiceWithStoreContext(ctx, router, store, auditStore)
}

func NewServiceWithStore(router *Router, store TransactionStore) (*Service, error) {
	return NewServiceWithStoreAndAudit(router, store, NewMemoryTransactionAuditStore())
}

func NewServiceWithStoreAndAudit(router *Router, store TransactionStore, auditStore TransactionAuditStore) (*Service, error) {
	return newServiceWithStoreContext(context.Background(), router, store, auditStore)
}

func newServiceWithStoreContext(ctx context.Context, router *Router, store TransactionStore, auditStore TransactionAuditStore) (*Service, error) {
	if router == nil {
		return nil, errors.New("provider router is required")
	}
	if store == nil {
		return nil, errors.New("transaction store is required")
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	service := &Service{
		Router:       router,
		Store:        store,
		AuditStore:   auditStore,
		transactions: make(map[string]*purchaseCall),
	}
	var states []TransactionState
	var err error
	if scoped, ok := store.(ContextReadTransactionStore); ok {
		states, err = scoped.AllContextE(ctx)
		if err != nil {
			return nil, fmt.Errorf("load persisted transaction state: %w", err)
		}
	} else if scoped, ok := store.(ContextTransactionStore); ok {
		states = scoped.AllContext(ctx)
		if err := ctx.Err(); err != nil {
			return nil, err
		}
	} else {
		states = store.All()
	}
	for _, state := range states {
		if normalizeTransactionKind(state.Kind) == TransactionKindPayment {
			if state.Payment == nil || state.Payment.ReferenceID == "" || state.Execution.ProviderName == "" {
				return nil, errors.New("invalid persisted payment transaction state")
			}
			continue
		}
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



// SubmitPayment authorizes exactly one external payment submission for a durable
// ReferenceID. The atomic create-if-absent operation is the authorization gate:
// only the caller that creates the pending durable state may invoke the provider.
// Existing pending state is deliberately returned without resubmission, including
// after restart. A provider error leaves the durable state pending because the
// external outcome may be ambiguous; callers must reconcile rather than retry.
func (s *Service) SubmitPayment(ctx context.Context, providerName string, req payment.PaymentRequest) (payment.PaymentResult, error) {
	if s == nil || s.Router == nil || s.Router.Registry == nil {
		return payment.PaymentResult{}, errors.New("payment service router is required")
	}
	if err := payment.ValidateRequest(req); err != nil {
		return payment.PaymentResult{}, err
	}
	if err := ctx.Err(); err != nil {
		return payment.PaymentResult{}, err
	}
	providerName = strings.TrimSpace(providerName)
	if providerName == "" {
		return payment.PaymentResult{}, errors.New("payment provider name is required")
	}
	status, ok := s.Router.Registry.Capabilities(providerName)
	if ok != nil {
		return payment.PaymentResult{}, fmt.Errorf("get payment capability %q: %w", providerName, ok)
	}
	capability, exists := status.Status(provider.CapabilityPayment)
	if !exists || !capability.AdapterImplemented || !capability.Enabled {
		return payment.PaymentResult{}, fmt.Errorf("%w: %s", ErrPaymentCapabilityDisabled, providerName)
	}
	p, err := s.Router.Registry.GetPaymentProvider(providerName)
	if err != nil {
		return payment.PaymentResult{}, fmt.Errorf("get payment provider %q: %w", providerName, err)
	}
	pending, err := NewPaymentTransactionState(req, providerName)
	if err != nil {
		return payment.PaymentResult{}, err
	}
	claimed, created, err := createTransactionIfAbsentContext(ctx, s.Store, pending)
	if err != nil {
		if errors.Is(err, ErrReferenceConflict) {
			return payment.PaymentResult{}, ErrPaymentSubmissionConflict
		}
		return payment.PaymentResult{}, fmt.Errorf("claim payment submission: %w", err)
	}
	if !created {
		if !sameTransactionIdentity(claimed, pending) {
			return payment.PaymentResult{}, ErrPaymentSubmissionConflict
		}
		return payment.PaymentResult{}, fmt.Errorf("%w: %s", ErrPaymentSubmissionClaimed, req.ReferenceID)
	}

	_ = s.appendAudit(TransactionAuditEvent{
		ReferenceID: req.ReferenceID,
		Action: "PAYMENT_SUBMISSION_CLAIMED",
		Previous: "",
		Next: string(payment.StatusPending),
		ProviderName: providerName,
		Message: "durable payment submission claim created",
	})

	result, err := p.CreatePayment(ctx, req)
	if err != nil {
		_ = s.appendAudit(TransactionAuditEvent{
			ReferenceID: req.ReferenceID,
			Action: "PAYMENT_SUBMISSION_PROVIDER_ERROR",
			Previous: string(payment.StatusPending),
			Next: string(payment.StatusPending),
			ProviderName: providerName,
			Message: err.Error(),
		})
		return payment.PaymentResult{}, fmt.Errorf("create payment with provider %q: %w", providerName, err)
	}
	if result.ReferenceID != req.ReferenceID || result.Amount != req.Amount || !strings.EqualFold(result.Currency, req.Currency) {
		_ = s.appendAudit(TransactionAuditEvent{
			ReferenceID: req.ReferenceID,
			Action: "PAYMENT_SUBMISSION_INVALID_RESULT",
			Previous: string(payment.StatusPending),
			Next: string(payment.StatusPending),
			ProviderName: providerName,
			Message: "provider result did not match durable payment identity",
		})
		return payment.PaymentResult{}, ErrPaymentSubmissionConflict
	}
	if err := payment.ValidateStatus(result.Status); err != nil {
		return payment.PaymentResult{}, err
	}
	next := claimed
	next.Payment = &payment.Transaction{
		ReferenceID: result.ReferenceID,
		ProviderReference: result.ProviderReference,
		Amount: result.Amount,
		Currency: result.Currency,
		CustomerID: req.CustomerID,
		Description: req.Description,
		Status: result.Status,
		Message: result.Message,
	}
	next.Execution.Result.ReferenceID = result.ReferenceID
	next.Execution.Result.Status = provider.TransactionStatus(result.Status)
	next.Execution.Result.Message = result.Message
	if err := s.persistTransition(ctx, req.ReferenceID, claimed, next); err != nil {
		_ = s.appendAudit(TransactionAuditEvent{
			ReferenceID: req.ReferenceID,
			Action: "PAYMENT_SUBMISSION_RESULT_PERSIST_FAILURE",
			Previous: string(payment.StatusPending),
			Next: string(payment.StatusPending),
			ProviderName: providerName,
			Message: err.Error(),
		})
		return result, fmt.Errorf("persist payment submission result: %w", err)
	}
	_ = s.appendAudit(TransactionAuditEvent{
		ReferenceID: req.ReferenceID,
		Action: "PAYMENT_SUBMITTED",
		Previous: string(payment.StatusPending),
		Next: string(result.Status),
		ProviderName: providerName,
		Message: result.Message,
	})
	return result, nil
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

	// Resolve durable state before routing or provider submission. A restart
	// or another service instance may already own this reference.
	existing, found, readErr := getTransactionContextE(ctx, s.Store, req.ReferenceID)
	if readErr != nil {
		err := fmt.Errorf("load existing transaction before submission: %w", readErr)
		s.finishPurchase(call, PurchaseExecution{}, err)
		return PurchaseExecution{}, err
	}
	if found {
		if existing.Request != req {
			s.finishPurchase(call, PurchaseExecution{}, ErrReferenceConflict)
			return PurchaseExecution{}, ErrReferenceConflict
		}
		s.finishPurchase(call, existing.Execution, nil)
		return existing.Execution, nil
	}

	providerName, err := s.selectProvider(ctx, req)
	if err != nil {
		s.finishPurchase(call, PurchaseExecution{}, err)
		return PurchaseExecution{}, err
	}

	// Atomically create the durable pending state. Only the service instance
	// that creates the row is authorized to submit to the external provider.
	pending := PurchaseExecution{
		ProviderName: providerName,
		Result: provider.PurchaseResult{
			ReferenceID: req.ReferenceID,
			CustomerNo:  req.CustomerNo,
			ProductCode: req.ProductCode,
			Status:      provider.StatusPending,
		},
	}
	claimed, created, err := createTransactionIfAbsentContext(ctx, s.Store, TransactionState{Request: req, Execution: pending})
	if err != nil {
		err = fmt.Errorf("create pending transaction state: %w", err)
		s.finishPurchase(call, PurchaseExecution{}, err)
		return PurchaseExecution{}, err
	}
	if !created {
		if claimed.Request != req {
			s.finishPurchase(call, PurchaseExecution{}, ErrReferenceConflict)
			return PurchaseExecution{}, ErrReferenceConflict
		}
		s.finishPurchase(call, claimed.Execution, nil)
		return claimed.Execution, nil
	}
	s.mu.Lock()
	call.result = pending
	s.mu.Unlock()

	// Audit is observational only. Its failure must never block the external
	// provider submission because the durable pending state is the safety gate.
	_ = s.appendAudit(TransactionAuditEvent{
		ReferenceID: req.ReferenceID,
		Action: "PURCHASE_PENDING",
		Previous: "",
		Next: string(provider.StatusPending),
		ProviderName: providerName,
		Message: "provider submission authorized by durable pending state",
	})

	result, err := s.executePurchase(ctx, providerName, req)
	if err != nil {
		// Keep the durable pending state. The caller receives the provider error,
		// while a restart can recover this reference and reconcile it safely.
		_ = s.appendAudit(TransactionAuditEvent{
			ReferenceID: req.ReferenceID,
			Action: "PURCHASE_PROVIDER_ERROR",
			Previous: string(provider.StatusPending),
			Next: string(provider.StatusPending),
			ProviderName: providerName,
			Message: err.Error(),
		})
		s.finishPurchase(call, pending, err)
		return pending, err
	}
	if storeErr := putTransactionContext(ctx, s.Store, TransactionState{Request: req, Execution: result}); storeErr != nil {
		err = fmt.Errorf("persist transaction result: %w", storeErr)
		_ = s.appendAudit(TransactionAuditEvent{
			ReferenceID: req.ReferenceID,
			Action: "PURCHASE_RESULT_PERSIST_FAILURE",
			Previous: string(provider.StatusPending),
			Next: string(provider.StatusPending),
			ProviderName: providerName,
			Message: err.Error(),
		})
		s.finishPurchase(call, pending, err)
		return pending, err
	}
	if auditErr := s.appendAudit(TransactionAuditEvent{
		ReferenceID: req.ReferenceID,
		Action: "PURCHASE_RESULT",
		Previous: string(provider.StatusPending),
		Next: string(result.Result.Status),
		ProviderName: providerName,
		Message: result.Result.Message,
	}); auditErr != nil {
		// The terminal transaction is already durable. Audit failure is reported
		// without reverting state or authorizing another provider submission.
		err = fmt.Errorf("audit transaction result: %w", auditErr)
		s.finishPurchase(call, result, err)
		return result, err
	}
	s.finishPurchase(call, result, nil)
	return result, nil
}

// HandleWebhook applies a normalized provider event when the caller has
// already authenticated and selected the provider adapter. It is retained as
// a compatibility path for trusted internal callers; new ingress code should
// use HandleWebhookFromProvider so the provider identity is verified against
// the durable transaction before state mutation.
func (s *Service) HandleWebhook(ctx context.Context, event provider.WebhookEvent) (PurchaseExecution, error) {
	return s.handleWebhook(ctx, "", event)
}

// HandleWebhookFromProvider applies a normalized webhook only when the event
// source provider matches the provider durably selected for the transaction.
// Provider identity is correlation data, not customer/account data, and must
// never be inferred from the event reference alone.
func (s *Service) HandleWebhookFromProvider(ctx context.Context, providerName string, event provider.WebhookEvent) (PurchaseExecution, error) {
	if strings.TrimSpace(providerName) == "" {
		return PurchaseExecution{}, fmt.Errorf("%w: provider name is required", ErrInvalidWebhookEvent)
	}
	return s.handleWebhook(ctx, providerName, event)
}

func (s *Service) handleWebhook(ctx context.Context, providerName string, event provider.WebhookEvent) (PurchaseExecution, error) {
	if err := validateWebhookEvent(event); err != nil { return PurchaseExecution{}, err }
	if err := ctx.Err(); err != nil { return PurchaseExecution{}, err }

	s.mu.Lock()
	if call, ok := s.transactions[event.ReferenceID]; ok {
		select {
		case <-call.done:
		default:
			s.mu.Unlock()
			return PurchaseExecution{}, ErrWebhookReferenceConflict
		}
	}
	s.mu.Unlock()

	latest, found, readErr := getTransactionContextE(ctx, s.Store, event.ReferenceID)
	if readErr != nil { return PurchaseExecution{}, fmt.Errorf("reload transaction for webhook: %w", readErr) }
	if !found { return PurchaseExecution{}, ErrWebhookTransactionNotFound }
	if latest.Request.ProductCode != event.ProductCode || latest.Request.CustomerNo != event.CustomerNo { return PurchaseExecution{}, ErrWebhookReferenceConflict }
	if providerName != "" && latest.Execution.ProviderName != providerName { return PurchaseExecution{}, ErrWebhookReferenceConflict }

	incoming := provider.PurchaseResult{ReferenceID:event.ReferenceID, CustomerNo:event.CustomerNo, ProductCode:event.ProductCode, Status:event.Status, ProviderCode:event.ProviderCode, Message:event.Message, SerialNumber:event.SerialNumber, Price:event.Price}
	latestResult := latest.Execution.Result
	if latestResult.Status == provider.StatusSuccess || latestResult.Status == provider.StatusFailed {
		if sameObservedProviderResult(latestResult, incoming) { s.syncLocalTransaction(latest); return latest.Execution, nil }
		_ = s.appendAudit(TransactionAuditEvent{ReferenceID:event.ReferenceID, Action:"WEBHOOK_TERMINAL_CONFLICT", Previous:string(latestResult.Status), Next:string(incoming.Status), ProviderName:latest.Execution.ProviderName, Message:incoming.Message})
		return PurchaseExecution{}, ErrWebhookReferenceConflict
	}
	if latestResult.Status != provider.StatusPending { return PurchaseExecution{}, ErrWebhookReferenceConflict }
	if incoming.Status != provider.StatusPending && incoming.Status != provider.StatusSuccess && incoming.Status != provider.StatusFailed { return PurchaseExecution{}, ErrInvalidWebhookEvent }

	previous := latest
	next := TransactionState{Request:latest.Request, Execution:PurchaseExecution{ProviderName:latest.Execution.ProviderName, Result:incoming}, Version:latest.Version}
	if err := s.persistTransition(ctx, event.ReferenceID, previous, next); err != nil {
		if !errors.Is(err, ErrTransactionStateConflict) { return PurchaseExecution{}, err }
		latestAfterConflict, ok, readErr := getTransactionContextE(ctx, s.Store, event.ReferenceID)
		if readErr != nil { return PurchaseExecution{}, fmt.Errorf("reload transaction after webhook conflict: %w", readErr) }
		if !ok || latestAfterConflict.Request != latest.Request || latestAfterConflict.Execution.ProviderName != latest.Execution.ProviderName { return PurchaseExecution{}, ErrWebhookReferenceConflict }
		latest = latestAfterConflict
		latestResult = latest.Execution.Result
		if latestResult.Status == provider.StatusSuccess || latestResult.Status == provider.StatusFailed {
			if sameObservedProviderResult(latestResult, incoming) { s.syncLocalTransaction(latest); return latest.Execution, nil }
			return PurchaseExecution{}, ErrWebhookReferenceConflict
		}
		if latestResult.Status != provider.StatusPending { return PurchaseExecution{}, ErrWebhookReferenceConflict }
		previous = latest
		next.Request, next.Execution.ProviderName, next.Version = latest.Request, latest.Execution.ProviderName, latest.Version
		if err := s.persistTransition(ctx, event.ReferenceID, previous, next); err != nil {
			if errors.Is(err, ErrTransactionStateConflict) { return PurchaseExecution{}, ErrTransactionStateConflict }
			return PurchaseExecution{}, err
		}
	}
	s.syncLocalTransaction(next)
	action := "WEBHOOK_PENDING"
	if incoming.Status != provider.StatusPending { action = "WEBHOOK_TERMINAL" }
	if auditErr := s.appendAudit(TransactionAuditEvent{ReferenceID:event.ReferenceID, Action:action, Previous:string(previous.Execution.Result.Status), Next:string(incoming.Status), ProviderName:next.Execution.ProviderName, Message:incoming.Message}); auditErr != nil {
		return next.Execution, fmt.Errorf("audit webhook event: %w", auditErr)
	}
	return next.Execution, nil
}

func (s *Service) syncLocalTransaction(state TransactionState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	call, ok := s.transactions[state.Request.ReferenceID]
	if !ok {
		call = &purchaseCall{request:state.Request, done:make(chan struct{})}
		close(call.done)
		s.transactions[state.Request.ReferenceID] = call
	}
	call.request, call.result, call.err = state.Request, state.Execution, nil
}

func (s *Service) Reconcile(ctx context.Context, referenceID string) (PurchaseExecution, error) {
	if strings.TrimSpace(referenceID) == "" {
		return PurchaseExecution{}, fmt.Errorf("%w: reference ID is required", ErrInvalidPurchaseRequest)
	}
	if err := ctx.Err(); err != nil {
		return PurchaseExecution{}, err
	}

	latest, ok, readErr := getTransactionContextE(ctx, s.Store, referenceID)
	if readErr != nil {
		return PurchaseExecution{}, fmt.Errorf("reload transaction for reconciliation: %w", readErr)
	}
	if !ok {
		return PurchaseExecution{}, ErrWebhookTransactionNotFound
	}
	request := latest.Request
	providerName := latest.Execution.ProviderName
	if providerName == "" {
		return PurchaseExecution{}, ErrWebhookReferenceConflict
	}
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

	latest, ok, readErr = getTransactionContextE(ctx, s.Store, referenceID)
	if readErr != nil {
		return PurchaseExecution{}, fmt.Errorf("reload transaction before reconciliation transition: %w", readErr)
	}
	if !ok || latest.Request != request || latest.Execution.ProviderName != providerName {
		return PurchaseExecution{}, ErrWebhookReferenceConflict
	}
	latestResult := latest.Execution.Result
	if latestResult.Status == provider.StatusSuccess || latestResult.Status == provider.StatusFailed {
		if samePurchaseResult(latestResult, incoming) {
			s.syncLocalTransaction(latest)
			return latest.Execution, nil
		}
		return PurchaseExecution{}, ErrWebhookReferenceConflict
	}
	if latestResult.Status != provider.StatusPending {
		return PurchaseExecution{}, ErrWebhookReferenceConflict
	}

	next := TransactionState{
		Request: request,
		Execution: PurchaseExecution{ProviderName: providerName, Result: incoming},
		Version: latest.Version,
	}
	if err := s.persistTransition(ctx, referenceID, latest, next); err != nil {
		if !errors.Is(err, ErrTransactionStateConflict) {
			return PurchaseExecution{}, err
		}
		latestAfterConflict, ok, readErr := getTransactionContextE(ctx, s.Store, referenceID)
		if readErr != nil {
			return PurchaseExecution{}, fmt.Errorf("reload transaction after reconciliation conflict: %w", readErr)
		}
		if !ok || latestAfterConflict.Request != request || latestAfterConflict.Execution.ProviderName != providerName {
			return PurchaseExecution{}, ErrWebhookReferenceConflict
		}
		latestResult = latestAfterConflict.Execution.Result
		if latestResult.Status == provider.StatusSuccess || latestResult.Status == provider.StatusFailed {
			if samePurchaseResult(latestResult, incoming) {
				s.syncLocalTransaction(latestAfterConflict)
				return latestAfterConflict.Execution, nil
			}
			return PurchaseExecution{}, ErrWebhookReferenceConflict
		}
		if latestResult.Status == provider.StatusPending {
			s.syncLocalTransaction(latestAfterConflict)
			return latestAfterConflict.Execution, nil
		}
		return PurchaseExecution{}, ErrWebhookReferenceConflict
	}

	s.syncLocalTransaction(next)
	if auditErr := s.appendAudit(TransactionAuditEvent{
		ReferenceID: referenceID,
		Action: "RECONCILIATION",
		Previous: string(latestResult.Status),
		Next: string(incoming.Status),
		ProviderName: providerName,
		Message: incoming.Message,
	}); auditErr != nil {
		return next.Execution, fmt.Errorf("audit reconciliation event: %w", auditErr)
	}
	return next.Execution, nil
}

func (s *Service) persistLocked(ctx context.Context, request PurchaseRequest, execution PurchaseExecution) error {
	if err := putTransactionContext(ctx, s.Store, TransactionState{Request: request, Execution: execution}); err != nil {
		return fmt.Errorf("persist transaction state: %w", err)
	}
	return nil
}

func (s *Service) persistTransition(ctx context.Context, referenceID string, previous, next TransactionState) error {
	if store, ok := s.Store.(ContextTransactionStore); ok {
		if err := store.PutIfCurrentContext(ctx, referenceID, previous, next); err != nil {
			return fmt.Errorf("persist atomic transaction transition: %w", err)
		}
		return nil
	}
	if err := putTransactionContext(ctx, s.Store, next); err != nil {
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

func sameWebhookResult(a, b provider.PurchaseResult) bool { return sameObservedProviderResult(a, b) }

func sameObservedProviderResult(a, b provider.PurchaseResult) bool {
	return a.ReferenceID == b.ReferenceID &&
		a.CustomerNo == b.CustomerNo &&
		a.ProductCode == b.ProductCode &&
		a.Status == b.Status &&
		a.ProviderCode == b.ProviderCode &&
		a.SerialNumber == b.SerialNumber &&
		a.Price == b.Price
}

func getTransactionContext(ctx context.Context, store TransactionStore, referenceID string) (TransactionState, bool) {
	state, ok, _ := getTransactionContextE(ctx, store, referenceID)
	return state, ok
}

func getTransactionContextE(ctx context.Context, store TransactionStore, referenceID string) (TransactionState, bool, error) {
	if scoped, ok := store.(ContextReadTransactionStore); ok {
		return scoped.GetContextE(ctx, referenceID)
	}
	if scoped, ok := store.(ContextTransactionStore); ok {
		state, found := scoped.GetContext(ctx, referenceID)
		if err := ctx.Err(); err != nil { return TransactionState{}, false, err }
		return state, found, nil
	}
	state, found := store.Get(referenceID)
	return state, found, nil
}

func putTransactionContext(ctx context.Context, store TransactionStore, state TransactionState) error {
	if scoped, ok := store.(ContextTransactionStore); ok {
		return scoped.PutContext(ctx, state)
	}
	return store.Put(state)
}

func createTransactionIfAbsentContext(ctx context.Context, store TransactionStore, state TransactionState) (TransactionState, bool, error) {
	scoped, ok := store.(CreateIfAbsentTransactionStore)
	if !ok {
		return TransactionState{}, false, errors.New("transaction store does not support atomic create-if-absent")
	}
	return scoped.CreateIfAbsentContext(ctx, state)
}


func (s *Service) appendAudit(event TransactionAuditEvent) error {
	if s.AuditStore == nil {
		return nil
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now().UTC()
	}
	return s.AuditStore.Append(event)
}
