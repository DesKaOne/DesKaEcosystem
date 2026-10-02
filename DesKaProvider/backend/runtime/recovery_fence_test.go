package runtime

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	provider "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider"
	mock "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider/Mock"
	"github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider/operational"
	"github.com/DesKaOne/DesKaEcosystem/DesKaProvider/catalog"
	"github.com/DesKaOne/DesKaEcosystem/DesKaProvider/routing"
)

type recoveryFenceBalanceProvider struct {
	*mock.Provider
	balance    int64
	failBalance bool
}

func (p *recoveryFenceBalanceProvider) GetBalance(ctx context.Context) (int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if p.failBalance {
		return 0, context.DeadlineExceeded
	}
	return p.balance, nil
}

func TestRecoveryFenceBlocksPersistedFreshSnapshotsUntilCurrentGenerationRefreshes(t *testing.T) {
	dir := t.TempDir()
	operationalPath := filepath.Join(dir, "operational.json")
	catalogPath := filepath.Join(dir, "catalog.json")
	now := time.Date(2026, 10, 3, 4, 0, 0, 0, time.UTC)

	providerImpl := &recoveryFenceBalanceProvider{
		Provider: mock.New(mock.Config{
			Products: []provider.Product{{Code: "xld10", Name: "XL 10K"}},
		}),
		balance: 200000,
	}
	registry := provider.NewRegistry()
	if err := registry.Register("mock", providerImpl); err != nil {
		t.Fatal(err)
	}

	persistedOperational, err := operational.NewJSONFileStore(operationalPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := persistedOperational.Put(operational.Snapshot{
		ProviderName:  "mock",
		Balance:       200000,
		Currency:      "IDR",
		Health:        operational.HealthHealthy,
		LastCheckedAt: now,
		LastSuccessAt: now,
	}); err != nil {
		t.Fatal(err)
	}

	persistedCatalog, err := catalog.NewJSONFileStore(catalogPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := persistedCatalog.Put(catalog.Snapshot{
		ProviderName: "mock",
		Products:     []provider.Product{{Code: "xld10", Name: "XL 10K"}},
		SyncedAt:     now,
	}); err != nil {
		t.Fatal(err)
	}

	// A new runtime generation starts with no readiness marks even though both
	// durable snapshots are younger than the routing freshness windows.
	operationalFence := newRecoveryFenceState()
	catalogFence := newRecoveryFenceState()
	operationalSyncStore := newOperationalSyncStore(persistedOperational, operationalFence)
	operationalRoutingStore := newOperationalRoutingStore(persistedOperational, operationalFence)
	catalogSyncStore := newCatalogSyncStore(persistedCatalog, catalogFence)
	catalogRoutingStore := newCatalogRoutingStore(persistedCatalog, catalogFence)

	operationalSync, err := operational.NewSyncService(registry, operationalSyncStore, "IDR", 3)
	if err != nil {
		t.Fatal(err)
	}
	operationalSync.Now = func() time.Time { return now }

	catalogSync, err := catalog.NewSyncService(registry, catalogSyncStore)
	if err != nil {
		t.Fatal(err)
	}
	catalogSync.Now = func() time.Time { return now }

	router, err := routing.NewWithCatalogMaxAge(
		registry,
		operationalRoutingStore,
		nil,
		catalogRoutingStore,
		time.Hour,
	)
	if err != nil {
		t.Fatal(err)
	}
	router.Now = func() time.Time { return now.Add(time.Minute) }

	if _, err := router.Select(context.Background(), routing.Request{ProductCode: "xld10", Amount: 10000}); err == nil {
		t.Fatal("persisted fresh snapshots must remain non-route-eligible before current-generation refresh")
	}

	// A failed operational refresh must not open the operational gate. Catalog
	// success alone is insufficient to authorize routing.
	providerImpl.failBalance = true
	if _, err := operationalSync.SyncProvider(context.Background(), "mock"); err == nil {
		t.Fatal("expected injected operational refresh failure")
	}
	if _, err := catalogSync.SyncProvider(context.Background(), "mock"); err != nil {
		t.Fatal(err)
	}
	if _, err := router.Select(context.Background(), routing.Request{ProductCode: "xld10", Amount: 10000}); err == nil {
		t.Fatal("catalog readiness must not compensate for an unrefreshed operational generation")
	}

	providerImpl.failBalance = false
	if _, err := operationalSync.SyncProvider(context.Background(), "mock"); err != nil {
		t.Fatal(err)
	}
	if _, err := catalogSync.SyncProvider(context.Background(), "mock"); err != nil {
		t.Fatal(err)
	}

	got, err := router.Select(context.Background(), routing.Request{ProductCode: "xld10", Amount: 10000})
	if err != nil {
		t.Fatal(err)
	}
	if got != "mock" {
		t.Fatalf("expected refreshed provider to become route-eligible, got %q", got)
	}
}

func TestRecoveryFenceIsFreshPerRuntimeGeneration(t *testing.T) {
	dir := t.TempDir()
	operationalPath := filepath.Join(dir, "operational.json")
	catalogPath := filepath.Join(dir, "catalog.json")
	now := time.Date(2026, 10, 3, 4, 0, 0, 0, time.UTC)

	providerImpl := &recoveryFenceBalanceProvider{
		Provider: mock.New(mock.Config{Products: []provider.Product{{Code: "xld10"}}}),
		balance: 100000,
	}
	registry := provider.NewRegistry()
	if err := registry.Register("mock", providerImpl); err != nil {
		t.Fatal(err)
	}

	operationalStore, err := operational.NewJSONFileStore(operationalPath)
	if err != nil {
		t.Fatal(err)
	}
	catalogStore, err := catalog.NewJSONFileStore(catalogPath)
	if err != nil {
		t.Fatal(err)
	}

	// Generation A refreshes both durable boundaries.
	fenceA := newRecoveryFenceState()
	opSyncA := newOperationalSyncStore(operationalStore, fenceA)
	catSyncA := newCatalogSyncStore(catalogStore, fenceA)
	opA, err := operational.NewSyncService(registry, opSyncA, "IDR", 3)
	if err != nil {
		t.Fatal(err)
	}
	opA.Now = func() time.Time { return now }
	catA, err := catalog.NewSyncService(registry, catSyncA)
	if err != nil {
		t.Fatal(err)
	}
	catA.Now = func() time.Time { return now }
	if _, err := opA.SyncProvider(context.Background(), "mock"); err != nil {
		t.Fatal(err)
	}
	if _, err := catA.SyncProvider(context.Background(), "mock"); err != nil {
		t.Fatal(err)
	}

	// Generation B gets a new fence. It must not inherit generation A's
	// in-memory readiness state even though durable timestamps remain fresh.
	fenceB := newRecoveryFenceState()
	if fenceB.isReady("mock") {
		t.Fatal("new runtime generation inherited stale readiness")
	}
	opB := newOperationalRoutingStore(operationalStore, fenceB)
	catB := newCatalogRoutingStore(catalogStore, fenceB)
	router, err := routing.NewWithCatalogMaxAge(registry, opB, nil, catB, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	router.Now = func() time.Time { return now.Add(time.Minute) }
	if _, err := router.Select(context.Background(), routing.Request{ProductCode: "xld10", Amount: 10000}); err == nil {
		t.Fatal("generation B must not route from generation A snapshots")
	}
}
