package routing

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	provider "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider"
	mock "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider/Mock"
	"github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider/operational"
	"github.com/DesKaOne/DesKaEcosystem/DesKaProvider/catalog"
)

func TestAdministrativeSnapshotsRemainConsistentWithRoutingAcrossTransitions(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 9, 30, 2, 0, 0, 0, time.UTC)
	registry := provider.NewRegistry()
	descriptor := provider.CapabilityDescriptor{
		Capabilities: map[provider.Capability]provider.CapabilityStatus{
			provider.CapabilityPPOB: {
				AdapterImplemented: true,
				Configured:         true,
				Tested:             true,
				Enabled:            true,
			},
		},
	}
	for _, name := range []string{"alpha", "beta"} {
		if err := registry.RegisterWithCapabilities(name, mock.New(mock.Config{
			Products: []provider.Product{{Code: "xld10", Name: "Test"}},
		}), descriptor); err != nil {
			t.Fatal(err)
		}
	}

	states := operational.NewProviderStateStore()
	for _, item := range []struct {
		name      string
		lifecycle operational.Lifecycle
	}{
		{name: "alpha", lifecycle: operational.LifecycleDisabled},
		{name: "beta", lifecycle: operational.LifecycleEnabled},
	} {
		d, err := registry.Capabilities(item.name)
		if err != nil {
			t.Fatal(err)
		}
		if err := states.Put(operational.ProviderState{
			ProviderName:          item.name,
			Lifecycle:             item.lifecycle,
			Capabilities:          []operational.Capability{operational.CapabilityPPOB},
			CapabilityFingerprint: operational.CapabilityMetadataFingerprint(d),
		}); err != nil {
			t.Fatal(err)
		}
	}

	operationalStore := operational.NewMemoryStore()
	catalogStore := catalog.NewMemoryStore()
	for _, name := range []string{"alpha", "beta"} {
		if err := operationalStore.Put(operational.Snapshot{
			ProviderName:  name,
			Balance:       100000,
			Currency:      "IDR",
			Health:        operational.HealthHealthy,
			LastCheckedAt: now,
			LastSuccessAt:  now,
		}); err != nil {
			t.Fatal(err)
		}
		if err := catalogStore.Put(catalog.Snapshot{
			ProviderName: name,
			Products:     []provider.Product{{Code: "xld10", Name: "Test"}},
			SyncedAt:     now,
		}); err != nil {
			t.Fatal(err)
		}
	}

	router, err := NewWithCatalogAndStateAndOperationalMaxAge(
		registry, operationalStore, nil, catalogStore, states, time.Minute,
	)
	if err != nil {
		t.Fatal(err)
	}
	router.CatalogMaxAge = time.Minute
	router.Now = func() time.Time { return now }

	assertSnapshotAndRoute := func(t *testing.T, wantProvider string, wantRouteError error) AdministrativeRouteExplanationSnapshot {
		t.Helper()

		snapshotBefore, err := ExplainAllProviderRoutes(ctx, router)
		if err != nil {
			t.Fatal(err)
		}
		selected, routeErr := router.Select(ctx, Request{ProductCode: "xld10", Amount: 100})
		if wantRouteError != nil {
			if routeErr == nil {
				t.Fatalf("expected routing error %v, selected=%q", wantRouteError, selected)
			}
			if !errors.Is(routeErr, wantRouteError) {
				t.Fatalf("expected routing error to satisfy %v, got %v", wantRouteError, routeErr)
			}
		} else {
			if routeErr != nil {
				t.Fatalf("unexpected routing error: %v", routeErr)
			}
			if selected != wantProvider {
				t.Fatalf("unexpected selected provider: got %q want %q", selected, wantProvider)
			}
		}

		if wantRouteError == nil {
			for _, providerSnapshot := range snapshotBefore.Providers {
				if providerSnapshot.ProviderName != selected {
					continue
				}
				for _, capability := range providerSnapshot.Capabilities {
					if capability.Capability == provider.CapabilityPPOB && !capability.RouteEligible {
						t.Fatalf("selected provider %q was not route-eligible in administrative snapshot: %#v", selected, capability)
					}
				}
			}
		}

		snapshotAfter, err := ExplainAllProviderRoutes(ctx, router)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(snapshotBefore, snapshotAfter) {
			t.Fatalf("interleaved administrative re-read changed snapshot without source transition: before=%#v after=%#v", snapshotBefore, snapshotAfter)
		}

		return snapshotAfter
	}

	initial := assertSnapshotAndRoute(t, "beta", nil)
	if providerCapabilityEligible(initial, "alpha", provider.CapabilityPPOB) {
		t.Fatal("disabled alpha unexpectedly appeared route-eligible")
	}
	if !providerCapabilityEligible(initial, "beta", provider.CapabilityPPOB) {
		t.Fatal("enabled beta did not appear route-eligible")
	}

	admin, err := operational.NewProviderAdminService(states)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Enable("alpha"); err != nil {
		t.Fatal(err)
	}
	enabled := assertSnapshotAndRoute(t, "alpha", nil)
	if !providerCapabilityEligible(enabled, "alpha", provider.CapabilityPPOB) {
		t.Fatal("explicitly enabled alpha did not become route-eligible")
	}

	alphaState, ok := states.Get("alpha")
	if !ok {
		t.Fatal("expected alpha provider state")
	}
	alphaState.CapabilityFingerprint = "drifted"
	if err := states.Put(alphaState); err != nil {
		t.Fatal(err)
	}

	drifted := assertSnapshotAndRoute(t, "beta", nil)
	if providerCapabilityEligible(drifted, "alpha", provider.CapabilityPPOB) {
		t.Fatal("drifted alpha remained route-eligible in administrative snapshot")
	}
	if !snapshotHasReason(drifted, "alpha", provider.CapabilityPPOB, ReasonCapabilityDrift) {
		t.Fatal("drifted alpha explanation did not expose capability_drift")
	}

	if _, err := admin.Disable("beta"); err != nil {
		t.Fatal(err)
	}
	blocked := assertSnapshotAndRoute(t, "", ErrNoProviderAvailable)
	if providerCapabilityEligible(blocked, "alpha", provider.CapabilityPPOB) {
		t.Fatal("drifted alpha unexpectedly became eligible after beta disable")
	}
	if providerCapabilityEligible(blocked, "beta", provider.CapabilityPPOB) {
		t.Fatal("disabled beta unexpectedly became eligible")
	}
	if !snapshotHasReason(blocked, "alpha", provider.CapabilityPPOB, ReasonCapabilityDrift) {
		t.Fatal("aggregate blocked snapshot lost alpha capability drift evidence")
	}
	if !snapshotHasReason(blocked, "beta", provider.CapabilityPPOB, ReasonLifecycleDisabled) {
		t.Fatal("aggregate blocked snapshot lost beta lifecycle-disabled evidence")
	}

	beforeRepeat, err := ExplainAllProviderRoutes(ctx, router)
	if err != nil {
		t.Fatal(err)
	}
	_, routeErr := router.Select(ctx, Request{ProductCode: "xld10", Amount: 100})
	if routeErr == nil {
		t.Fatal("expected terminal routing to remain blocked")
	}
	afterRepeat, err := ExplainAllProviderRoutes(ctx, router)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(beforeRepeat, afterRepeat) {
		t.Fatalf("repeated terminal explanation changed around routing call: before=%#v after=%#v", beforeRepeat, afterRepeat)
	}
	if !errors.Is(routeErr, ErrNoProviderAvailable) || !errors.Is(routeErr, ErrProviderCapabilityDrift) {
		t.Fatalf("unexpected terminal aggregate routing error: %v", routeErr)
	}
}

func providerCapabilityEligible(snapshot AdministrativeRouteExplanationSnapshot, name string, capability provider.Capability) bool {
	for _, providerSnapshot := range snapshot.Providers {
		if providerSnapshot.ProviderName != name {
			continue
		}
		for _, item := range providerSnapshot.Capabilities {
			if item.Capability == capability {
				return item.RouteEligible
			}
		}
	}
	return false
}

func snapshotHasReason(snapshot AdministrativeRouteExplanationSnapshot, name string, capability provider.Capability, code ReadinessReasonCode) bool {
	for _, providerSnapshot := range snapshot.Providers {
		if providerSnapshot.ProviderName != name {
			continue
		}
		for _, item := range providerSnapshot.Capabilities {
			if item.Capability != capability {
				continue
			}
			for _, reason := range item.Reasons {
				if reason.Code == code {
					return true
				}
			}
		}
	}
	return false
}
