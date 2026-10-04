package routing

import (
	"context"
	"errors"
	"time"

	provider "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider"
)

type ProviderCapabilityRouteExplanation struct {
	Capability provider.Capability
	RouteEligible bool
	Reasons []ReadinessReason
}

type ProviderAdministrativeFreshness struct {
	OperationalPresent bool
	OperationalLastCheckedAt time.Time
	OperationalFresh bool
	CatalogPresent bool
	CatalogSyncedAt time.Time
	CatalogFresh bool
	CapabilityDrifted bool
}

type ProviderRouteExplanationSnapshot struct {
	ProviderName string
	Capabilities []ProviderCapabilityRouteExplanation
	Freshness ProviderAdministrativeFreshness
}

type AdministrativeRouteExplanationSnapshot struct {
	GeneratedAt time.Time
	Providers []ProviderRouteExplanationSnapshot
}

func ExplainAllProviderRoutes(ctx context.Context, router *Router) (AdministrativeRouteExplanationSnapshot, error) {
	if router == nil || router.Registry == nil {
		return AdministrativeRouteExplanationSnapshot{}, errors.New("provider router is required")
	}
	if ctx == nil {
		return AdministrativeRouteExplanationSnapshot{}, errors.New("context is required")
	}
	generatedAt := router.NowTime()
	names := router.Registry.Names()
	capabilities := provider.AllCapabilities()
	result := AdministrativeRouteExplanationSnapshot{
		GeneratedAt: generatedAt,
		Providers: make([]ProviderRouteExplanationSnapshot, 0, len(names)),
	}
	for _, name := range names {
		snapshot := ProviderRouteExplanationSnapshot{
			ProviderName: name,
			Capabilities: make([]ProviderCapabilityRouteExplanation, 0, len(capabilities)),
			Freshness: administrativeFreshness(router, name, generatedAt),
		}
		for _, capability := range capabilities {
			explanation, err := ExplainProviderRoute(ctx, router, name, capability, "", 0)
			if err != nil {
				return AdministrativeRouteExplanationSnapshot{}, err
			}
			snapshot.Capabilities = append(snapshot.Capabilities, ProviderCapabilityRouteExplanation{
				Capability: capability,
				RouteEligible: explanation.RouteEligible,
				Reasons: append([]ReadinessReason(nil), explanation.Reasons...),
			})
		}
		result.Providers = append(result.Providers, snapshot)
	}
	return result, nil
}
