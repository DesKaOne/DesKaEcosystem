package routing

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	provider "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider"
	"github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider/operational"
)

var (
	ErrNoProviderAvailable = errors.New("no provider available")
	ErrInvalidRouteRequest = errors.New("invalid provider route request")
)

type Request struct {
	ProductCode string
	Amount      int64
}

type Router struct {
	Registry   *provider.Registry
	Store      operational.Store
	Priorities map[string]int
}

type candidate struct {
	name     string
	priority int
}

func New(registry *provider.Registry, store operational.Store, priorities map[string]int) (*Router, error) {
	if registry == nil {
		return nil, errors.New("provider registry is required")
	}
	if store == nil {
		return nil, errors.New("operational store is required")
	}
	copied := make(map[string]int, len(priorities))
	for name, priority := range priorities {
		copied[normalize(name)] = priority
	}
	return &Router{Registry: registry, Store: store, Priorities: copied}, nil
}

func (r *Router) Select(ctx context.Context, req Request) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if req.ProductCode == "" || req.Amount <= 0 {
		return "", fmt.Errorf("%w: product code and positive amount are required", ErrInvalidRouteRequest)
	}

	candidates := make([]candidate, 0)
	for _, name := range r.Registry.Names() {
		snapshot, ok := r.Store.Get(name)
		if !ok || snapshot.Health != operational.HealthHealthy || snapshot.Balance < req.Amount {
			continue
		}

		p, err := r.Registry.Get(name)
		if err != nil {
			continue
		}
		products, err := p.GetProducts(ctx, provider.ProductRequest{})
		if err != nil || !hasProduct(products, req.ProductCode) {
			continue
		}

		priority, ok := r.Priorities[name]
		if !ok {
			priority = 0
		}
		candidates = append(candidates, candidate{name: name, priority: priority})
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

func hasProduct(products []provider.Product, code string) bool {
	for _, product := range products {
		if product.Code == code {
			return true
		}
	}
	return false
}

func normalize(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}
