package catalog

import (
	"context"
	"testing"
	"time"

	provider "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider"
	mock "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider/Mock"
)

func TestSyncServiceStoresProviderCatalogSnapshot(t *testing.T) {
	registry := provider.NewRegistry()
	if err := registry.Register("mock", mock.New(mock.Config{Products: []provider.Product{{Code:"xld10", Name:"XL 10K"}}})); err != nil { t.Fatal(err) }
	store := NewMemoryStore()
	svc, err := NewSyncService(registry, store)
	if err != nil { t.Fatal(err) }
	now := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
	svc.Now = func() time.Time { return now }
	if _, err := svc.SyncProvider(context.Background(), "mock"); err != nil { t.Fatal(err) }
	got, ok := store.Get("mock")
	if !ok { t.Fatal("expected catalog snapshot") }
	if got.SyncedAt != now || len(got.Products) != 1 || got.Products[0].Code != "xld10" { t.Fatalf("unexpected snapshot: %#v", got) }
}

func TestMemoryStoreCopiesProducts(t *testing.T) {
	store := NewMemoryStore()
	products := []provider.Product{{Code:"xld10", Name:"XL 10K"}}
	if err := store.Put(Snapshot{ProviderName:"mock", Products:products, SyncedAt:time.Now()}); err != nil { t.Fatal(err) }
	products[0].Name = "mutated"
	got, _ := store.Get("mock")
	if got.Products[0].Name != "XL 10K" { t.Fatalf("store leaked mutable product data: %#v", got.Products) }
}
