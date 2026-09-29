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
	ErrOperationalSnapshotStale = errors.New("provider operational snapshot is stale")
	ErrProviderCapabilityDrift = errors.New("provider capability metadata drift detected")
)

const (
	defaultCatalogMaxAge = 30 * time.Minute
	defaultOperationalSnapshotMaxAge = 2 * time.Minute
)

type Request struct {
	ProductCode string
	Amount      int64
}

type Router struct {
	Now            func() time.Time
	Registry       *provider.Registry
	OperationalInput OperationalInputReader
	Priorities     map[string]int
	Catalog        catalog.Store
	CatalogMaxAge  time.Duration
	OperationalMaxAge time.Duration
	ProviderState  *operational.ProviderStateStore
}

type candidate struct {
	name     string
	priority int
}

func New(registry *provider.Registry, store operational.Store, priorities map[string]int) (*Router, error) {
	return newRouter(registry, store, priorities, nil, 0, 0, nil)
}

func NewWithState(registry *provider.Registry, store operational.Store, priorities map[string]int, stateStore *operational.ProviderStateStore) (*Router, error) {
	return newRouter(registry, store, priorities, nil, 0, 0, stateStore)
}

func NewWithCatalogAndState(registry *provider.Registry, store operational.Store, priorities map[string]int, catalogStore catalog.Store, stateStore *operational.ProviderStateStore) (*Router, error) {
	return newRouter(registry, store, priorities, catalogStore, defaultCatalogMaxAge, 0, stateStore)
}
func NewWithCatalogAndStateAndOperationalMaxAge(registry *provider.Registry, store operational.Store, priorities map[string]int, catalogStore catalog.Store, stateStore *operational.ProviderStateStore, maxAge time.Duration) (*Router, error) {
	if maxAge <= 0 { return nil, errors.New("operational snapshot max age must be greater than zero") }
	return newRouter(registry, store, priorities, catalogStore, defaultCatalogMaxAge, maxAge, stateStore)
}

func NewWithCatalog(registry *provider.Registry, store operational.Store, priorities map[string]int, catalogStore catalog.Store) (*Router, error) {
	return newRouter(registry, store, priorities, catalogStore, defaultCatalogMaxAge, 0, nil)
}

func NewWithCatalogMaxAge(registry *provider.Registry, store operational.Store, priorities map[string]int, catalogStore catalog.Store, maxAge time.Duration) (*Router, error) {
	if maxAge <= 0 {
		return nil, errors.New("catalog max age must be greater than zero")
	}
	return newRouter(registry, store, priorities, catalogStore, maxAge, 0, nil)
}

func newRouter(registry *provider.Registry, store operational.Store, priorities map[string]int, catalogStore catalog.Store, catalogMaxAge time.Duration, operationalMaxAge time.Duration, stateStore *operational.ProviderStateStore) (*Router, error) {
	if registry == nil {
		return nil, errors.New("provider registry is required")
	}
	if store == nil {
		return nil, errors.New("operational store is required")
	}
	operationalInput, err := NewStoreOperationalInputReader(store)
	if err != nil {
		return nil, err
	}
	copied := make(map[string]int, len(priorities))
	for name, priority := range priorities {
		copied[normalize(name)] = priority
	}
	return &Router{Registry: registry, OperationalInput: operationalInput, Priorities: copied, Catalog: catalogStore, CatalogMaxAge: catalogMaxAge, OperationalMaxAge: operationalMaxAge, ProviderState: stateStore, Now: time.Now}, nil
}

func (r *Router) Select(ctx context.Context, req Request) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	now := r.NowTime()
	if req.ProductCode == "" || req.Amount <= 0 {
		return "", fmt.Errorf("%w: product code and positive amount are required", ErrInvalidRouteRequest)
	}

	candidates := make([]candidate, 0)
	var staleCatalog, staleOperational, capabilityDrift bool
	for _, name := range r.Registry.Names() {
		if r.ProviderState != nil {
			state, ok := r.ProviderState.Get(name)
			if !ok || !state.Enabled() || !state.Supports(operational.CapabilityPPOB) {
				continue
			}
			// Operational enablement is not sufficient by itself. The registry
			// capability descriptor must also explicitly mark PPOB as implemented
			// and enabled before this provider becomes route-eligible.
			descriptor, err := r.Registry.Capabilities(name)
			if err != nil {
				continue
			}
			// Registry.Register predates the capability matrix. An empty
			// descriptor is retained as a compatibility mode for callers that
			// have not migrated their registry entry yet. Once capability
			// metadata exists, eligibility is strict and must explicitly require
			// an implemented and enabled PPOB capability.
			if len(descriptor.Capabilities) > 0 {
				drift := operational.DetectCapabilityDrift(state, descriptor)
				if drift.Drifted() {
					capabilityDrift = true
					continue
				}
			}
			if len(descriptor.Capabilities) > 0 && !descriptor.Supports(provider.CapabilityPPOB) {
				continue
			}
		}
		if r.OperationalInput == nil {
			return "", errors.New("operational input reader is required")
		}
		operationalInput, err := r.OperationalInput.ReadOperationalInput(ctx, name, r.OperationalMaxAge, now)
		if err != nil {
			if errors.Is(err, operational.ErrOperationalSnapshotNotFound) {
				continue
			}
			continue
		}
		snapshot := operationalInput.Snapshot
		if snapshot.Health != operational.HealthHealthy || snapshot.Balance < req.Amount {
			continue
		}
		if r.OperationalMaxAge > 0 && !isFresh(snapshot.LastCheckedAt, now, r.OperationalMaxAge) {
			staleOperational = true
			continue
		}

		var catalogSnapshot *catalog.Snapshot
		if r.Catalog != nil {
			value, ok := r.Catalog.Get(name)
			if !ok {
				continue
			}
			catalogSnapshot = &value
			if r.CatalogMaxAge > 0 && !isFresh(value.SyncedAt, now, r.CatalogMaxAge) {
				staleCatalog = true
				continue
			}
			if !hasProduct(value.Products, req.ProductCode) {
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
		if err := validateRoutingCandidateInput(routingCandidateInput{
			ProviderName: name,
			Operational:  operationalInput,
			Catalog:      catalogSnapshot,
			Priority:     priority,
		}, now, r.OperationalMaxAge, r.CatalogMaxAge); err != nil {
			continue
		}
		candidates = append(candidates, candidate{name: name, priority: priority})
	}

	if len(candidates) == 0 {
		errs := []error{ErrNoProviderAvailable}
		if staleOperational {
			errs = append(errs, ErrOperationalSnapshotStale)
		}
		if staleCatalog {
			errs = append(errs, ErrCatalogStale)
		}
		if capabilityDrift {
			errs = append(errs, ErrProviderCapabilityDrift)
		}
		return "", errors.Join(errs...)
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].priority != candidates[j].priority {
			return candidates[i].priority < candidates[j].priority
		}
		return candidates[i].name < candidates[j].name
	})
	return candidates[0].name, nil
}

func isFresh(timestamp, now time.Time, maxAge time.Duration) bool {
	if timestamp.IsZero() || maxAge <= 0 {
		return false
	}
	age := now.Sub(timestamp)
	return age >= 0 && age <= maxAge
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
