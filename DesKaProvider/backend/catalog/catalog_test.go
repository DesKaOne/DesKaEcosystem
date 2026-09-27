package catalog

import (
	"context"
	"errors"
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


func TestMemoryStoreRejectsOlderSnapshot(t *testing.T) {
	store := NewMemoryStore()
	newer := time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC)
	older := newer.Add(-time.Minute)
	if err := store.Put(Snapshot{ProviderName: "mock", SyncedAt: newer}); err != nil {
		t.Fatal(err)
	}
	if err := store.Put(Snapshot{ProviderName: "mock", SyncedAt: older}); err != ErrSnapshotOlder {
		t.Fatalf("expected older snapshot rejection, got %v", err)
	}
	got, ok := store.Get("mock")
	if !ok || !got.SyncedAt.Equal(newer) {
		t.Fatalf("older snapshot replaced current state: %#v", got)
	}
}


func TestSyncServiceRecordsFailureAndRecoveryStatus(t *testing.T) {
	registry := provider.NewRegistry()
	if err := registry.Register("mock", mock.New(mock.Config{Products: []provider.Product{{Code: "xld10", Name: "XL 10K"}}})); err != nil {
		t.Fatal(err)
	}
	store := NewMemoryStore()
	svc, err := NewSyncService(registry, store)
	if err != nil {
		t.Fatal(err)
	}
	first := time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC)
	second := first.Add(time.Minute)
	now := first
	svc.Now = func() time.Time { return now }

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := svc.SyncProvider(ctx, "mock"); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled sync, got %v", err)
	}
	status, ok := svc.Status("mock")
	if !ok {
		t.Fatal("expected provider sync status after failed attempt")
	}
	if !status.LastAttemptAt.Equal(first) || !status.LastSuccessAt.IsZero() || status.ConsecutiveFailures != 1 || status.LastError != context.Canceled.Error() {
		t.Fatalf("unexpected failure status: %#v", status)
	}

	now = second
	if _, err := svc.SyncProvider(context.Background(), "mock"); err != nil {
		t.Fatal(err)
	}
	status, ok = svc.Status("mock")
	if !ok {
		t.Fatal("expected provider sync status after recovery")
	}
	if !status.LastAttemptAt.Equal(second) || !status.LastSuccessAt.Equal(second) || status.ConsecutiveFailures != 0 || status.LastError != "" {
		t.Fatalf("unexpected recovered status: %#v", status)
	}
}


type cancellationAwareProvider struct {
	*mock.Provider
	calls int
}

func (p *cancellationAwareProvider) GetProducts(ctx context.Context, req provider.ProductRequest) ([]provider.Product, error) {
	p.calls++
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return p.Provider.GetProducts(ctx, req)
}

func TestSyncAllStopsAfterContextCancellation(t *testing.T) {
	first := &cancellationAwareProvider{Provider: mock.New(mock.Config{})}
	second := &cancellationAwareProvider{Provider: mock.New(mock.Config{})}
	registry := provider.NewRegistry()
	if err := registry.Register("first", first); err != nil { t.Fatal(err) }
	if err := registry.Register("second", second); err != nil { t.Fatal(err) }

	svc, err := NewSyncService(registry, NewMemoryStore())
	if err != nil { t.Fatal(err) }
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	errs := svc.SyncAll(ctx)
	if !errors.Is(errs["first"], context.Canceled) {
		t.Fatalf("expected first provider cancellation, got %v", errs["first"])
	}
	if first.calls != 1 {
		t.Fatalf("expected first provider to be attempted once, got %d", first.calls)
	}
	if second.calls != 0 {
		t.Fatalf("expected second provider not to be attempted after cancellation, got %d", second.calls)
	}
}
