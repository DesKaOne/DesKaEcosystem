package routing

import (
	"context"
	"time"

	"github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider/operational"
)

func administrativeFreshness(router *Router, name string, generatedAt time.Time) ProviderAdministrativeFreshness {
	result := ProviderAdministrativeFreshness{}

	if router.ProviderState != nil {
		state, ok := router.ProviderState.Get(name)
		if ok {
			result.CapabilityDrifted = providerCapabilityDrifted(router, name, state)
		}
	}

	if router.OperationalInput != nil {
		input, err := router.OperationalInput.ReadOperationalInput(
			context.Background(),
			name,
			router.OperationalMaxAge,
			generatedAt,
		)
		if err == nil {
			result.OperationalPresent = true
			result.OperationalLastCheckedAt = input.Snapshot.LastCheckedAt
			if router.OperationalMaxAge > 0 {
				freshness, freshnessErr := input.Snapshot.EvaluateFreshness(generatedAt, router.OperationalMaxAge)
				result.OperationalFresh = freshnessErr == nil && freshness == operational.FreshnessFresh
			}
		}
	}

	if router.Catalog != nil {
		snapshot, ok := router.Catalog.Get(name)
		if ok {
			result.CatalogPresent = true
			result.CatalogSyncedAt = snapshot.SyncedAt
			if router.CatalogMaxAge > 0 {
				result.CatalogFresh = isFresh(snapshot.SyncedAt, generatedAt, router.CatalogMaxAge)
			}
		}
	}

	return result
}

func providerCapabilityDrifted(router *Router, name string, state operational.ProviderState) bool {
	descriptor, err := router.Registry.Capabilities(name)
	if err != nil {
		return false
	}
	return operational.DetectCapabilityDrift(state, descriptor).Drifted()
}
