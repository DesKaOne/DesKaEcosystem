package routing

import (
	"context"
	"errors"
	"testing"

	provider "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider"
	Mock "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider/Mock"
	"github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider/operational"
)

func TestServicePurchaseRoutesAndExecutesOnce(t *testing.T) {
	registry := provider.NewRegistry()
	mock := Mock.New(Mock.Config{
		Products:       []provider.Product{{Code: "pln20", Name: "PLN 20"}},
		ProviderCode:   "00",
		Message:        "success",
		PurchaseStatus: provider.StatusSuccess,
		Price:          20000,
	})
	if err := registry.Register("mock", mock); err != nil {
		t.Fatal(err)
	}

	store := operational.NewMemoryStore()
	if err := store.Put(operational.Snapshot{
		ProviderName: "mock",
		Balance:      100000,
		Health:       operational.HealthHealthy,
	}); err != nil {
		t.Fatal(err)
	}

	router, err := New(registry, store, map[string]int{"mock": 1})
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewService(router)
	if err != nil {
		t.Fatal(err)
	}

	execution, err := service.Purchase(context.Background(), PurchaseRequest{
		ProductCode: "pln20",
		CustomerNo:  "08123456789",
		ReferenceID: "ref-001",
		Amount:      20000,
	})
	if err != nil {
		t.Fatal(err)
	}
	if execution.ProviderName != "mock" {
		t.Fatalf("expected mock provider, got %q", execution.ProviderName)
	}
	if execution.Result.ReferenceID != "ref-001" {
		t.Fatalf("unexpected reference ID: %q", execution.Result.ReferenceID)
	}
	if execution.Result.Status != provider.StatusSuccess {
		t.Fatalf("unexpected status: %q", execution.Result.Status)
	}

	status, err := mock.GetStatus(context.Background(), provider.StatusRequest{
		ProductCode: "pln20",
		CustomerNo:  "08123456789",
		ReferenceID: "ref-001",
	})
	if err != nil {
		t.Fatal(err)
	}
	if status.Status != provider.StatusSuccess {
		t.Fatalf("expected persisted purchase status, got %q", status.Status)
	}
}

func TestServicePurchaseDoesNotFallbackAfterProviderError(t *testing.T) {
	registry := provider.NewRegistry()
	first := Mock.New(Mock.Config{
		Products:       []provider.Product{{Code: "pln20", Name: "PLN 20"}},
		PurchaseStatus: provider.StatusFailed,
		ProviderCode:   "99",
		Message:        "failed",
	})
	second := Mock.New(Mock.Config{
		Products:       []provider.Product{{Code: "pln20", Name: "PLN 20"}},
		PurchaseStatus: provider.StatusSuccess,
		ProviderCode:   "00",
		Message:        "success",
	})
	if err := registry.Register("first", first); err != nil {
		t.Fatal(err)
	}
	if err := registry.Register("second", second); err != nil {
		t.Fatal(err)
	}

	store := operational.NewMemoryStore()
	for _, name := range []string{"first", "second"} {
		if err := store.Put(operational.Snapshot{
			ProviderName: name,
			Balance:      100000,
			Health:       operational.HealthHealthy,
		}); err != nil {
			t.Fatal(err)
		}
	}

	router, err := New(registry, store, map[string]int{"first": 1, "second": 2})
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewService(router)
	if err != nil {
		t.Fatal(err)
	}

	execution, err := service.Purchase(context.Background(), PurchaseRequest{
		ProductCode: "pln20",
		CustomerNo:  "08123456789",
		ReferenceID: "ref-002",
		Amount:      20000,
	})
	if err != nil {
		t.Fatal(err)
	}
	if execution.ProviderName != "first" {
		t.Fatalf("expected first provider selection, got %q", execution.ProviderName)
	}
	if execution.Result.Status != provider.StatusFailed {
		t.Fatalf("expected provider failure to be returned, got %q", execution.Result.Status)
	}

	status, err := second.GetStatus(context.Background(), provider.StatusRequest{
		ProductCode: "pln20",
		CustomerNo:  "08123456789",
		ReferenceID: "ref-002",
	})
	if err != nil {
		t.Fatal(err)
	}
	if status.Status != provider.StatusPending {
		t.Fatalf("expected no fallback purchase, got status %q", status.Status)
	}
}

func TestServicePurchaseValidatesRequest(t *testing.T) {
	service, err := NewService(&Router{})
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.Purchase(context.Background(), PurchaseRequest{ProductCode: "pln20"})
	if !errors.Is(err, ErrInvalidPurchaseRequest) {
		t.Fatalf("expected invalid request error, got %v", err)
	}
}
