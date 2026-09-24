package routing

import (
	"context"
	"errors"
	"fmt"
	"strings"

	provider "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider"
)

var ErrInvalidPurchaseRequest = errors.New("invalid provider purchase request")

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
}

func NewService(router *Router) (*Service, error) {
	if router == nil {
		return nil, errors.New("provider router is required")
	}
	return &Service{Router: router}, nil
}

func (s *Service) Purchase(ctx context.Context, req PurchaseRequest) (PurchaseExecution, error) {
	if err := validatePurchaseRequest(req); err != nil {
		return PurchaseExecution{}, err
	}
	if err := ctx.Err(); err != nil {
		return PurchaseExecution{}, err
	}

	name, err := s.Router.Select(ctx, Request{
		ProductCode: req.ProductCode,
		Amount:      req.Amount,
	})
	if err != nil {
		return PurchaseExecution{}, err
	}

	p, err := s.Router.Registry.Get(name)
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
		return PurchaseExecution{}, fmt.Errorf("purchase with provider %q: %w", name, err)
	}

	return PurchaseExecution{
		ProviderName: name,
		Result:       result,
	}, nil
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
