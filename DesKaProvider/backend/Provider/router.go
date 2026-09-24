package provider

import (
	"context"
	"errors"
	"fmt"
	"sort"
)

var (
	ErrNoProviderAvailable = errors.New("no provider available")
	ErrInvalidRouteRequest  = errors.New("invalid provider route request")
)

type RouteRequest struct {
	ProductCode string
	Amount      int64
}

type Router struct {
	Registry   *Registry
	Store      operationalSnapshotStore
	Priorities map[string]int
}

type operationalSnapshotStore interface {
	Get(string) (OperationalSnapshot, bool)
}

type OperationalSnapshot struct {
	ProviderName        string
	Balance             int64
	Health              string
	ConsecutiveFailures int
}

type routeCandidate struct {
	name     string
	priority int
}

func NewRouter(registry *Registry, store operationalSnapshotStore, priorities map[string]int) (*Router, error) {
	if registry == nil {
		return nil, errors.New("provider registry is required")
	}
	if store == nil {
		return nil, errors.New("operational store is required")
	}
	copied := make(map[string]int, len(priorities))
	for name, priority := range priorities {
		copied[normalizeName(name)] = priority
	}
	return &Router{Registry: registry, Store: store, Priorities: copied}, nil
}

func (r *Router) Select(ctx context.Context, req RouteRequest) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if req.ProductCode == "" || req.Amount <= 0 {
		return "", fmt.Errorf("%w: product code and positive amount are required", ErrInvalidRouteRequest)
	}

	candidates := make([]routeCandidate, 0)
	for _, name := range r.Registry.Names() {
		snapshot, ok := r.Store.Get(name)
		if !ok || snapshot.Health != "healthy" || snapshot.Balance < req.Amount {
			continue
		}

		p, err := r.Registry.Get(name)
		if err != nil {
			continue
		}
		products, err := p.GetProducts(ctx, ProductRequest{})
		if err != nil {
			continue
		}
		if !hasProduct(products, req.ProductCode) {
			continue
		}

		priority, ok := r.Priorities[name]
		if !ok {
			priority = 0
		}
		candidates = append(candidates, routeCandidate{name: name, priority: priority})
	}

	if len(candidates) == 0 {
		return "", ErrNoProviderAvailable
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].priority != candidates[j].priority {
			return candidates[i].priority < candidates[j].priority
		}
		return candidates[i].name < candidates[j].name
	})
	return candidates[0].name, nil
}

func hasProduct(products []Product, code string) bool {
	for _, product := range products {
		if product.Code == code {
			return true
		}
	}
	return false
}
