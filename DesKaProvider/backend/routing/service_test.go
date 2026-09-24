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

	_, err = second.GetStatus(context.Background(), provider.StatusRequest{
		ProductCode: "pln20",
		CustomerNo:  "08123456789",
		ReferenceID: "ref-002",
	})
	if !errors.Is(err, Mock.ErrTransactionNotFound) {
		t.Fatalf("expected no fallback purchase, got %v", err)
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

func TestServicePurchaseIsIdempotentByReferenceID(t *testing.T) {
	registry := provider.NewRegistry()
	mock := Mock.New(Mock.Config{Products: []provider.Product{{Code: "pln20", Name: "PLN 20"}}, ProviderCode: "00", PurchaseStatus: provider.StatusSuccess, Price: 20000})
	if err := registry.Register("mock", mock); err != nil { t.Fatal(err) }
	store := operational.NewMemoryStore()
	if err := store.Put(operational.Snapshot{ProviderName: "mock", Balance: 100000, Health: operational.HealthHealthy}); err != nil { t.Fatal(err) }
	router, err := New(registry, store, map[string]int{"mock": 1})
	if err != nil { t.Fatal(err) }
	service, err := NewService(router)
	if err != nil { t.Fatal(err) }

	req := PurchaseRequest{ProductCode: "pln20", CustomerNo: "08123456789", ReferenceID: "ref-idempotent", Amount: 20000}
	first, err := service.Purchase(context.Background(), req)
	if err != nil { t.Fatal(err) }
	second, err := service.Purchase(context.Background(), req)
	if err != nil { t.Fatal(err) }
	if first != second { t.Fatalf("expected repeated request to return identical execution: %#v != %#v", first, second) }

	status, err := mock.GetStatus(context.Background(), provider.StatusRequest{ProductCode: req.ProductCode, CustomerNo: req.CustomerNo, ReferenceID: req.ReferenceID})
	if err != nil { t.Fatal(err) }
	if status.ReferenceID != req.ReferenceID || status.Status != provider.StatusSuccess { t.Fatalf("unexpected stored status: %#v", status) }
}

func TestServicePurchaseRejectsReferenceConflict(t *testing.T) {
	registry := provider.NewRegistry()
	mock := Mock.New(Mock.Config{Products: []provider.Product{{Code: "pln20", Name: "PLN 20"}}})
	if err := registry.Register("mock", mock); err != nil { t.Fatal(err) }
	store := operational.NewMemoryStore()
	if err := store.Put(operational.Snapshot{ProviderName: "mock", Balance: 100000, Health: operational.HealthHealthy}); err != nil { t.Fatal(err) }
	router, err := New(registry, store, map[string]int{"mock": 1})
	if err != nil { t.Fatal(err) }
	service, err := NewService(router)
	if err != nil { t.Fatal(err) }

	_, err = service.Purchase(context.Background(), PurchaseRequest{ProductCode: "pln20", CustomerNo: "08123456789", ReferenceID: "ref-conflict", Amount: 20000})
	if err != nil { t.Fatal(err) }
	_, err = service.Purchase(context.Background(), PurchaseRequest{ProductCode: "pln20", CustomerNo: "08987654321", ReferenceID: "ref-conflict", Amount: 20000})
	if !errors.Is(err, ErrReferenceConflict) { t.Fatalf("expected reference conflict, got %v", err) }
}

func TestServicePurchaseConcurrentDuplicatesSubmitOnce(t *testing.T) {
	registry := provider.NewRegistry()
	mock := Mock.New(Mock.Config{Products: []provider.Product{{Code: "pln20", Name: "PLN 20"}}, ProviderCode: "00", PurchaseStatus: provider.StatusSuccess, Price: 20000})
	if err := registry.Register("mock", mock); err != nil { t.Fatal(err) }
	store := operational.NewMemoryStore()
	if err := store.Put(operational.Snapshot{ProviderName: "mock", Balance: 100000, Health: operational.HealthHealthy}); err != nil { t.Fatal(err) }
	router, err := New(registry, store, map[string]int{"mock": 1})
	if err != nil { t.Fatal(err) }
	service, err := NewService(router)
	if err != nil { t.Fatal(err) }

	req := PurchaseRequest{ProductCode: "pln20", CustomerNo: "08123456789", ReferenceID: "ref-concurrent", Amount: 20000}
	const callers = 16
	results := make(chan PurchaseExecution, callers)
	errs := make(chan error, callers)
	start := make(chan struct{})
	for i := 0; i < callers; i++ {
		go func() {
			<-start
			result, err := service.Purchase(context.Background(), req)
			results <- result
			errs <- err
		}()
	}
	close(start)
	for i := 0; i < callers; i++ {
		if err := <-errs; err != nil { t.Fatal(err) }
	}
	var first PurchaseExecution
	for i := 0; i < callers; i++ {
		result := <-results
		if i == 0 { first = result; continue }
		if result != first { t.Fatalf("duplicate execution mismatch: %#v != %#v", result, first) }
	}
	if got := mock.PurchaseCount(req.ReferenceID); got != 1 {
		t.Fatalf("expected exactly one provider purchase submission, got %d", got)
	}
}


func TestServiceHandleWebhookCorrelatesPendingTransaction(t *testing.T) {
	registry := provider.NewRegistry()
	mock := Mock.New(Mock.Config{
		Products: []provider.Product{{Code: "pln20", Name: "PLN 20"}},
		ProviderCode: "00", Message: "pending", PurchaseStatus: provider.StatusPending, Price: 20000,
	})
	if err := registry.Register("mock", mock); err != nil { t.Fatal(err) }
	store := operational.NewMemoryStore()
	if err := store.Put(operational.Snapshot{ProviderName: "mock", Balance: 100000, Health: operational.HealthHealthy}); err != nil { t.Fatal(err) }
	router, err := New(registry, store, map[string]int{"mock": 1})
	if err != nil { t.Fatal(err) }
	service, err := NewService(router)
	if err != nil { t.Fatal(err) }

	req := PurchaseRequest{ProductCode: "pln20", CustomerNo: "08123456789", ReferenceID: "ref-webhook", Amount: 20000}
	execution, err := service.Purchase(context.Background(), req)
	if err != nil { t.Fatal(err) }
	if execution.Result.Status != provider.StatusPending { t.Fatalf("expected pending purchase, got %q", execution.Result.Status) }

	event := provider.WebhookEvent{
		ReferenceID: req.ReferenceID, CustomerNo: req.CustomerNo, ProductCode: req.ProductCode,
		Status: provider.StatusSuccess, ProviderCode: "00", Message: "success", SerialNumber: "SN-1", Price: 20000,
	}
	updated, err := service.HandleWebhook(context.Background(), event)
	if err != nil { t.Fatal(err) }
	if updated.ProviderName != "mock" || updated.Result.Status != provider.StatusSuccess || updated.Result.SerialNumber != "SN-1" {
		t.Fatalf("unexpected webhook result: %#v", updated)
	}

	duplicate, err := service.HandleWebhook(context.Background(), event)
	if err != nil { t.Fatal(err) }
	if duplicate != updated { t.Fatalf("expected idempotent webhook result: %#v != %#v", duplicate, updated) }
}

func TestServiceHandleWebhookRejectsUnknownAndConflictingReferences(t *testing.T) {
	registry := provider.NewRegistry()
	mock := Mock.New(Mock.Config{Products: []provider.Product{{Code: "pln20", Name: "PLN 20"}}, ProviderCode: "00", PurchaseStatus: provider.StatusPending})
	if err := registry.Register("mock", mock); err != nil { t.Fatal(err) }
	store := operational.NewMemoryStore()
	if err := store.Put(operational.Snapshot{ProviderName: "mock", Balance: 100000, Health: operational.HealthHealthy}); err != nil { t.Fatal(err) }
	router, err := New(registry, store, map[string]int{"mock": 1})
	if err != nil { t.Fatal(err) }
	service, err := NewService(router)
	if err != nil { t.Fatal(err) }

	_, err = service.HandleWebhook(context.Background(), provider.WebhookEvent{
		ReferenceID: "unknown", CustomerNo: "08123456789", ProductCode: "pln20", Status: provider.StatusSuccess,
	})
	if !errors.Is(err, ErrWebhookTransactionNotFound) { t.Fatalf("expected unknown reference error, got %v", err) }

	req := PurchaseRequest{ProductCode: "pln20", CustomerNo: "08123456789", ReferenceID: "ref-webhook-conflict", Amount: 20000}
	if _, err := service.Purchase(context.Background(), req); err != nil { t.Fatal(err) }
	_, err = service.HandleWebhook(context.Background(), provider.WebhookEvent{
		ReferenceID: req.ReferenceID, CustomerNo: "08987654321", ProductCode: req.ProductCode, Status: provider.StatusSuccess,
	})
	if !errors.Is(err, ErrWebhookReferenceConflict) { t.Fatalf("expected webhook reference conflict, got %v", err) }
}
