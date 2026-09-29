package routing

import (
	"context"
	"reflect"
	"testing"
	"time"

	provider "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider"
	mock "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider/Mock"
	"github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider/operational"
	"github.com/DesKaOne/DesKaEcosystem/DesKaProvider/catalog"
)

func TestAdministrativeDiagnosticSnapshotsRemainIdempotentAcrossMixedTransitions(t *testing.T) {
	now := time.Date(2026, 9, 30, 1, 30, 0, 0, time.UTC)
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
	d, err := registry.Capabilities("alpha")
	if err != nil {
		t.Fatal(err)
	}
	fingerprint := operational.CapabilityMetadataFingerprint(d)
	if err := states.Put(operational.ProviderState{
		ProviderName:          "alpha",
		Lifecycle:             operational.LifecycleDisabled,
		Capabilities:          []operational.Capability{operational.CapabilityPPOB},
		CapabilityFingerprint: fingerprint,
	}); err != nil {
		t.Fatal(err)
	}
	d, err = registry.Capabilities("beta")
	if err != nil {
		t.Fatal(err)
	}
	if err := states.Put(operational.ProviderState{
		ProviderName:          "beta",
		Lifecycle:             operational.LifecycleEnabled,
		Capabilities:          []operational.Capability{operational.CapabilityPPOB},
		CapabilityFingerprint: operational.CapabilityMetadataFingerprint(d),
	}); err != nil {
		t.Fatal(err)
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

	first, err := ExplainAllProviderRoutes(context.Background(), router)
	if err != nil {
		t.Fatal(err)
	}
	repeated, err := ExplainAllProviderRoutes(context.Background(), router)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, repeated) {
		t.Fatalf("repeated administrative snapshots are not idempotent: first=%#v repeated=%#v", first, repeated)
	}
	if !first.GeneratedAt.Equal(now) {
		t.Fatalf("unexpected deterministic generation time: got %v want %v", first.GeneratedAt, now)
	}
	if len(first.Providers) != 2 {
		t.Fatalf("expected two providers, got %d", len(first.Providers))
	}

	admin, err := operational.NewProviderAdminService(states)
	if err != nil {
		t.Fatal(err)
	}
	beforeAlpha := first.Providers[0]
	beforeBeta := first.Providers[1]

	if _, err := admin.Enable("alpha"); err != nil {
		t.Fatal(err)
	}
	afterEnable, err := ExplainAllProviderRoutes(context.Background(), router)
	if err != nil {
		t.Fatal(err)
	}
	afterEnableRepeat, err := ExplainAllProviderRoutes(context.Background(), router)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(afterEnable, afterEnableRepeat) {
		t.Fatalf("post-enable administrative snapshots are not idempotent: first=%#v repeated=%#v", afterEnable, afterEnableRepeat)
	}
	if !afterEnable.GeneratedAt.Equal(first.GeneratedAt) {
		t.Fatalf("generation metadata changed without router clock/state-time change: before=%v after=%v", first.GeneratedAt, afterEnable.GeneratedAt)
	}
	if reflect.DeepEqual(beforeAlpha, afterEnable.Providers[0]) {
		t.Fatalf("explicit lifecycle transition did not change alpha explanation: before=%#v after=%#v", beforeAlpha, afterEnable.Providers[0])
	}
	if !reflect.DeepEqual(beforeBeta, afterEnable.Providers[1]) {
		t.Fatalf("alpha transition unexpectedly changed beta explanation: before=%#v after=%#v", beforeBeta, afterEnable.Providers[1])
	}

	betaState, ok := states.Get("beta")
	if !ok {
		t.Fatal("expected beta provider state")
	}
	betaState.CapabilityFingerprint = "drifted"
	if err := states.Put(betaState); err != nil {
		t.Fatal(err)
	}
	afterDrift, err := ExplainAllProviderRoutes(context.Background(), router)
	if err != nil {
		t.Fatal(err)
	}
	afterDriftRepeat, err := ExplainAllProviderRoutes(context.Background(), router)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(afterDrift, afterDriftRepeat) {
		t.Fatalf("post-drift administrative snapshots are not idempotent: first=%#v repeated=%#v", afterDrift, afterDriftRepeat)
	}
	if !afterDrift.GeneratedAt.Equal(now) {
		t.Fatalf("drift observation changed generation time: got %v want %v", afterDrift.GeneratedAt, now)
	}
	if !reflect.DeepEqual(afterDrift.Providers[0], afterEnable.Providers[0]) {
		t.Fatalf("beta drift unexpectedly changed alpha explanation: alphaBefore=%#v alphaAfter=%#v", afterEnable.Providers[0], afterDrift.Providers[0])
	}
	if reflect.DeepEqual(afterDrift.Providers[1], afterEnable.Providers[1]) {
		t.Fatalf("beta drift did not change beta explanation: before=%#v after=%#v", afterEnable.Providers[1], afterDrift.Providers[1])
	}

	betaDiagnostic, err := admin.Diagnose("beta", registry)
	if err != nil {
		t.Fatal(err)
	}
	if !betaDiagnostic.Drifted {
		t.Fatalf("expected beta diagnostic drift after persisted fingerprint change: %#v", betaDiagnostic)
	}
	diagnosticsA, err := admin.DiagnoseAll(registry)
	if err != nil {
		t.Fatal(err)
	}
	diagnosticsB, err := admin.DiagnoseAll(registry)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(diagnosticsA, diagnosticsB) {
		t.Fatalf("repeated provider diagnostics are not idempotent: first=%#v repeated=%#v", diagnosticsA, diagnosticsB)
	}

	if _, err := admin.ReconcileCapabilityState("beta", registry); err != nil {
		t.Fatal(err)
	}
	afterReconcile, err := ExplainAllProviderRoutes(context.Background(), router)
	if err != nil {
		t.Fatal(err)
	}
	afterReconcileRepeat, err := ExplainAllProviderRoutes(context.Background(), router)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(afterReconcile, afterReconcileRepeat) {
		t.Fatalf("post-reconcile administrative snapshots are not idempotent: first=%#v repeated=%#v", afterReconcile, afterReconcileRepeat)
	}
	if !afterReconcile.GeneratedAt.Equal(now) {
		t.Fatalf("reconcile changed generation time: got %v want %v", afterReconcile.GeneratedAt, now)
	}
	if !reflect.DeepEqual(afterReconcile.Providers[0], afterDrift.Providers[0]) {
		t.Fatalf("beta reconciliation unexpectedly changed alpha explanation: before=%#v after=%#v", afterDrift.Providers[0], afterReconcile.Providers[0])
	}
	if reflect.DeepEqual(afterReconcile.Providers[1], afterDrift.Providers[1]) {
		t.Fatalf("beta reconciliation did not change beta explanation: before=%#v after=%#v", afterDrift.Providers[1], afterReconcile.Providers[1])
	}

	finalDiagnosticsA, err := admin.DiagnoseAll(registry)
	if err != nil {
		t.Fatal(err)
	}
	finalDiagnosticsB, err := admin.DiagnoseAll(registry)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(finalDiagnosticsA, finalDiagnosticsB) {
		t.Fatalf("final repeated provider diagnostics are not idempotent: first=%#v repeated=%#v", finalDiagnosticsA, finalDiagnosticsB)
	}

	alphaState, ok := states.Get("alpha")
	if !ok || !alphaState.Enabled() {
		t.Fatalf("alpha explicit enable was not retained: %#v", alphaState)
	}
	betaState, ok = states.Get("beta")
	if !ok || betaState.Enabled() {
		t.Fatalf("beta reconciliation must remain disabled until explicit re-enable: %#v", betaState)
	}
}
