package routing

import (
	"context"
	"errors"

	provider "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider"
)

type ProviderCapabilityRouteExplanation struct {
	Capability provider.Capability
	RouteEligible bool
	Reasons []ReadinessReason
}

type ProviderRouteExplanationSnapshot struct {
	ProviderName string
	Capabilities []ProviderCapabilityRouteExplanation
}

type AdministrativeRouteExplanationSnapshot struct {
	Providers []ProviderRouteExplanationSnapshot
}

func ExplainAllProviderRoutes(ctx context.Context, router *Router) (AdministrativeRouteExplanationSnapshot, error) {
	if router == nil || router.Registry == nil {
		return AdministrativeRouteExplanationSnapshot{}, errors.New("provider router is required")
	}
	if ctx == nil {
		return AdministrativeRouteExplanationSnapshot{}, errors.New("context is required")
	}
	names := router.Registry.Names()
	capabilities := provider.AllCapabilities()
	result := AdministrativeRouteExplanationSnapshot{
		Providers: make([]ProviderRouteExplanationSnapshot, 0, len(names)),
	}
	for _, name := range names {
		snapshot := ProviderRouteExplanationSnapshot{
			ProviderName: name,
			Capabilities: make([]ProviderCapabilityRouteExplanation, 0, len(capabilities)),
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
