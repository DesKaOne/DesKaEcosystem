package routing

import (
	"context"
	"errors"
	"testing"
	"time"

	provider "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider"
	mock "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider/Mock"
	"github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider/operational"
	"github.com/DesKaOne/DesKaEcosystem/DesKaProvider/catalog"
)

func TestAdministrativeTransitionRoutingParity(t *testing.T) {
	now := time.Date(2026, 9, 29, 16, 0, 0, 0, time.UTC)

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
	if err := registry.RegisterWithCapabilities("mock", mock.New(mock.Config{
		Products: []provider.Product{{Code: "xld10", Name: "Test"}},
	}), descriptor); err != nil {
		t.Fatal(err)
	}

	states := operational.NewProviderStateStore()
	d, err := registry.Capabilities("mock")
	if err != nil {
		t.Fatal(err)
	}
	if err := states.Put(operational.ProviderState{
		ProviderName:          "mock",
		Lifecycle:             operational.LifecycleDisabled,
		Capabilities:          []operational.Capability{operational.CapabilityPPOB},
		CapabilityFingerprint: operational.CapabilityMetadataFingerprint(d),
	}); err != nil {
		t.Fatal(err)
	}

	operationalStore := operational.NewMemoryStore()
	putOperational := func(lastChecked time.Time, health operational.Health, balance int64) {
		t.Helper()
		if err := operationalStore.Put(operational.Snapshot{
			ProviderName:  "mock",
			Balance:       balance,
			Currency:      "IDR",
			Health:        health,
			LastCheckedAt: lastChecked,
			LastSuccessAt: lastChecked,
		}); err != nil {
			t.Fatal(err)
		}
	}
	putOperational(now, operational.HealthHealthy, 100000)

	catalogStore := catalog.NewMemoryStore()
	putCatalog := func(syncedAt time.Time) {
		t.Helper()
		if err := catalogStore.Put(catalog.Snapshot{
			ProviderName: "mock",
			Products:     []provider.Product{{Code: "xld10", Name: "Test"}},
			SyncedAt:    syncedAt,
		}); err != nil {
			t.Fatal(err)
		}
	}
	putCatalog(now)

	router, err := NewWithCatalogAndStateAndOperationalMaxAge(
		registry, operationalStore, nil, catalogStore, states, time.Minute,
	)
	if err != nil {
		t.Fatal(err)
	}
	router.CatalogMaxAge = time.Minute
	router.Now = func() time.Time { return now }

	admin, err := operational.NewProviderAdminService(states)
	if err != nil {
		t.Fatal(err)
	}

	assertParity := func(name string, wantEligible bool, wantErr error) {
		t.Helper()
		explanation, err := ExplainProviderRoute(
			context.Background(), router, name, provider.CapabilityPPOB, "xld10", 100,
		)
		if err != nil {
			t.Fatal(err)
		}
		if explanation.RouteEligible != wantEligible {
			t.Fatalf("%s: explanation RouteEligible=%v want %v reasons=%#v",
				name, explanation.RouteEligible, wantEligible, explanation.Reasons)
		}

		selected, selectErr := router.Select(context.Background(), Request{
			ProductCode: "xld10",
			Amount:      100,
		})
		if wantEligible {
			if selectErr != nil || selected != "mock" {
				t.Fatalf("%s: eligible explanation must match Router.Select success: selected=%q err=%v explanation=%#v",
					name, selected, selectErr, explanation)
			}
			return
		}
		if selectErr == nil {
			t.Fatalf("%s: blocked explanation must match Router.Select rejection: selected=%q explanation=%#v",
				name, selected, explanation)
		}
		if wantErr != nil && !errors.Is(selectErr, wantErr) {
			t.Fatalf("%s: Router.Select error=%v does not expose expected sentinel %v; explanation=%#v",
				name, selectErr, wantErr, explanation)
		}
	}

	// Disabled -> blocked.
	assertParity("disabled", false, ErrNoProviderAvailable)

	// Explicit enable -> eligible. This is the only lifecycle mutation that
	// authorizes the provider to cross the lifecycle gate.
	if _, err := admin.Enable("mock"); err != nil {
		t.Fatal(err)
	}
	assertParity("enabled", true, nil)

	// Capability drift -> blocked. Administrative explanation and Router.Select
	// must observe the same transition without diagnostics mutating state.
	state, ok := states.Get("mock")
	if !ok {
		t.Fatal("expected persisted provider state")
	}
	state.CapabilityFingerprint = "drifted"
	if err := states.Put(state); err != nil {
		t.Fatal(err)
	}
	driftExplanation, err := ExplainProviderRoute(
		context.Background(), router, "mock", provider.CapabilityPPOB, "xld10", 100,
	)
	if err != nil {
		t.Fatal(err)
	}
	if found, blocking := reason(driftExplanation, ReasonCapabilityDrift); !found || !blocking {
		t.Fatalf("expected blocking capability-drift reason: %#v", driftExplanation)
	}
	assertParity("drifted", false, ErrProviderCapabilityDrift)

	// Reconcile clears drift but intentionally disables lifecycle. The provider
	// must remain blocked until the next explicit enable.
	reconciled, err := admin.ReconcileCapabilityState("mock", registry)
	if err != nil {
		t.Fatal(err)
	}
	if reconciled.Drifted {
		t.Fatalf("reconciliation must clear capability drift: %#v", reconciled)
	}
	if reconciled.State.Enabled() {
		t.Fatalf("reconciliation must not re-enable lifecycle: %#v", reconciled)
	}
	assertParity("reconciled-disabled", false, ErrNoProviderAvailable)

	if _, err := admin.Enable("mock"); err != nil {
		t.Fatal(err)
	}
	assertParity("reconciled-enabled", true, nil)

	// Fresh operational state -> stale operational snapshot.
	putOperational(now.Add(-2*time.Minute), operational.HealthHealthy, 100000)
	staleOperational, err := ExplainProviderRoute(
		context.Background(), router, "mock", provider.CapabilityPPOB, "xld10", 100,
	)
	if err != nil {
		t.Fatal(err)
	}
	if found, blocking := reason(staleOperational, ReasonOperationalSnapshotStale); !found || !blocking {
		t.Fatalf("expected blocking operational-stale reason: %#v", staleOperational)
	}
	assertParity("operational-stale", false, ErrOperationalSnapshotStale)

	putOperational(now, operational.HealthHealthy, 100000)
	assertParity("operational-recovered", true, nil)

	// Fresh operational state with stale catalog -> catalog-blocked.
	putCatalog(now.Add(-2 * time.Minute))
	staleCatalog, err := ExplainProviderRoute(
		context.Background(), router, "mock", provider.CapabilityPPOB, "xld10", 100,
	)
	if err != nil {
		t.Fatal(err)
	}
	if found, blocking := reason(staleCatalog, ReasonCatalogStale); !found || !blocking {
		t.Fatalf("expected blocking catalog-stale reason: %#v", staleCatalog)
	}
	assertParity("catalog-stale", false, ErrCatalogStale)

	putCatalog(now)
	assertParity("catalog-recovered", true, nil)

	// Repeat the terminal healthy state to prove the transition sequence does
	// not accumulate stale diagnostic reasons or alter routing inputs.
	first, err := ExplainProviderRoute(
		context.Background(), router, "mock", provider.CapabilityPPOB, "xld10", 100,
	)
	if err != nil {
		t.Fatal(err)
	}
	second, err := ExplainProviderRoute(
		context.Background(), router, "mock", provider.CapabilityPPOB, "xld10", 100,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !equalRouteExplanations(first, second) {
		t.Fatalf("terminal explanation became non-deterministic after transitions: first=%#v second=%#v", first, second)
	}
}

func equalRouteExplanations(a, b ProviderRouteExplanation) bool {
	if a.ProviderName != b.ProviderName || a.Capability != b.Capability || a.RouteEligible != b.RouteEligible {
		return false
	}
	if len(a.Reasons) != len(b.Reasons) {
		return false
	}
	for i := range a.Reasons {
		if a.Reasons[i] != b.Reasons[i] {
			return false
		}
	}
	return true
}
