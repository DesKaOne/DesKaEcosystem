package routing

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	provider "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider"
	"github.com/DesKaOne/DesKaEcosystem/DesKaProvider/catalog"
	"github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider/operational"
)

var (
	ErrNoProviderAvailable = errors.New("no provider available")
	ErrInvalidRouteRequest = errors.New("invalid provider route request")
	ErrCatalogStale = errors.New("provider catalog is stale")
)

const defaultCatalogMaxAge = 30 * time.Minute

type Request struct {
	ProductCode string
	Amount      int64
}

type Router struct {
	Registry       *provider.Registry
	Store          operational.Store
	Priorities     map[string]int
	Catalog        catalog.Store
	CatalogMaxAge  time.Duration
	ProviderState  *operational.ProviderStateStore
}

type candidate struct {
	name     string
	priority int
}

func New(registry *provider.Registry, store operational.Store, priorities map[string]int) (*Router, error) {
	return newRouter(registry, store, priorities, nil, 0, nil)
}

func NewWithState(registry *provider.Registry, store operational.Store, priorities map[string]int, stateStore *operational.ProviderStateStore) (*Router, error) {
	return newRouter(registry, store, priorities, nil, 0, stateStore)
}

func NewWithCatalogAndState(registry *provider.Registry, store operational.Store, priorities map[string]int, catalogStore catalog.Store, stateStore *operational.ProviderStateStore) (*Router, error) {
	return newRouter(registry, store, priorities, catalogStore, defaultCatalogMaxAge, stateStore)
}

func NewWithCatalog(registry *provider.Registry, store operational.Store, priorities map[string]int, catalogStore catalog.Store) (*Router, error) {
	return newRouter(registry, store, priorities, catalogStore, defaultCatalogMaxAge, nil)
}

func NewWithCatalogMaxAge(registry *provider.Registry, store operational.Store, priorities map[string]int, catalogStore catalog.Store, maxAge time.Duration) (*Router, error) {
	if maxAge <= 0 {
		return nil, errors.New("catalog max age must be greater than zero")
	}
	return newRouter(registry, store, priorities, catalogStore, maxAge, nil)
}

func newRouter(registry *provider.Registry, store operational.Store, priorities map[string]int, catalogStore catalog.Store, catalogMaxAge time.Duration, stateStore *operational.ProviderStateStore) (*Router, error) {
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
	return &Router{Registry: registry, Store: store, Priorities: copied, Catalog: catalogStore, CatalogMaxAge: catalogMaxAge, ProviderState: stateStore}, nil
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
		if r.ProviderState != nil {
			state, ok := r.ProviderState.Get(name)
			if !ok || !state.Enabled() || !state.Supports(operational.CapabilityPPOB) {
				continue
			}
		}
		snapshot, ok := r.Store.Get(name)
		if !ok || snapshot.Health != operational.HealthHealthy || snapshot.Balance < req.Amount {
			continue
		}

		if r.Catalog != nil {
			snapshot, ok := r.Catalog.Get(name)
			if !ok {
				continue
			}
			if r.CatalogMaxAge > 0 && time.Since(snapshot.SyncedAt) > r.CatalogMaxAge {
				continue
			}
			if !hasProduct(snapshot.Products, req.ProductCode) {
				continue
			}
		} else {
			p, err := r.Registry.Get(name)
			if err != nil {
				continue
			}
			products, err := p.GetProducts(ctx, provider.ProductRequest{})
			if err != nil || !hasProduct(products, req.ProductCode) {
				continue
			}
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
