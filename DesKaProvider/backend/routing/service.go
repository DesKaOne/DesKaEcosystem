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
	ErrInvalidPurchaseRequest = errors.New("invalid provider purchase request")
	ErrReferenceConflict = errors.New("provider reference ID already used with different request")
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
	Router *Router
	mu sync.Mutex
	transactions map[string]*purchaseCall
}

type purchaseCall struct {
	request PurchaseRequest
	done chan struct{}
	result PurchaseExecution
	err error
}

func NewService(router *Router) (*Service, error) {
	if router == nil {
		return nil, errors.New("provider router is required")
	}
	return &Service{Router: router, transactions: make(map[string]*purchaseCall)}, nil
}

func (s *Service) Purchase(ctx context.Context, req PurchaseRequest) (PurchaseExecution, error) {
	if err := validatePurchaseRequest(req); err != nil { return PurchaseExecution{}, err }
	if err := ctx.Err(); err != nil { return PurchaseExecution{}, err }

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

	call.result, call.err = s.executePurchase(ctx, req)
	close(call.done)
	return call.result, call.err
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

func (s *Service) executePurchase(ctx context.Context, req PurchaseRequest) (PurchaseExecution, error) {
	name, err := s.Router.Select(ctx, Request{ProductCode: req.ProductCode, Amount: req.Amount})
	if err != nil { return PurchaseExecution{}, err }
	p, err := s.Router.Registry.Get(name)
	if err != nil { return PurchaseExecution{}, fmt.Errorf("get selected provider: %w", err) }
	result, err := p.Purchase(ctx, provider.PurchaseRequest{
		ProductCode: req.ProductCode, CustomerNo: req.CustomerNo, ReferenceID: req.ReferenceID, Testing: req.Testing,
	})
	if err != nil { return PurchaseExecution{}, fmt.Errorf("purchase with provider %q: %w", name, err) }
	return PurchaseExecution{ProviderName: name, Result: result}, nil
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
