package routing

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	provider "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider"
	Mock "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider/Mock"
	"github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider/operational"
)



type failPutTransactionStore struct {
	base      TransactionStore
	failAfter int
	puts      int
}

func (s *failPutTransactionStore) Get(referenceID string) (TransactionState, bool) {
	return s.base.Get(referenceID)
}

func (s *failPutTransactionStore) Put(state TransactionState) error {
	s.puts++
	if s.failAfter > 0 && s.puts >= s.failAfter {
		return errors.New("injected transaction store failure")
	}
	return s.base.Put(state)
}

func (s *failPutTransactionStore) All() []TransactionState {
	return s.base.All()
}

func TestServicePurchasePersistsPendingBeforeSubmission(t *testing.T) {
	registry := provider.NewRegistry()
	mock := Mock.New(Mock.Config{Products: []provider.Product{{Code: "pln20", Name: "PLN 20"}}, ProviderCode: "00", PurchaseStatus: provider.StatusSuccess, Price: 20000})
	if err := registry.Register("mock", mock); err != nil {
		t.Fatal(err)
	}
	store := operational.NewMemoryStore()
	if err := store.Put(operational.Snapshot{ProviderName: "mock", Balance: 100000, Health: operational.HealthHealthy}); err != nil {
		t.Fatal(err)
	}
	router, err := New(registry, store, map[string]int{"mock": 1})
	if err != nil {
		t.Fatal(err)
	}
	transactionStore := NewMemoryTransactionStore()
	failingStore := &failPutTransactionStore{base: transactionStore, failAfter: 1}
	service, err := NewServiceWithStore(router, failingStore)
	if err != nil {
		t.Fatal(err)
	}

	req := PurchaseRequest{ProductCode: "pln20", CustomerNo: "08123456789", ReferenceID: "ref-persist-before-submit", Amount: 20000}
	_, err = service.Purchase(context.Background(), req)
	if err == nil {
		t.Fatal("expected persistence failure before provider submission")
	}
	if got := mock.PurchaseCount(req.ReferenceID); got != 0 {
		t.Fatalf("expected provider submission to be blocked by pending-state persistence failure, got %d", got)
	}
	if _, ok := transactionStore.Get(req.ReferenceID); ok {
		t.Fatal("expected failed pending persistence not to create durable state")
	}
}

func TestServicePurchaseKeepsPendingStateWhenResultPersistenceFails(t *testing.T) {
	registry := provider.NewRegistry()
	mock := Mock.New(Mock.Config{Products: []provider.Product{{Code: "pln20", Name: "PLN 20"}}, ProviderCode: "00", PurchaseStatus: provider.StatusSuccess, Price: 20000})
	if err := registry.Register("mock", mock); err != nil {
		t.Fatal(err)
	}
	store := operational.NewMemoryStore()
	if err := store.Put(operational.Snapshot{ProviderName: "mock", Balance: 100000, Health: operational.HealthHealthy}); err != nil {
		t.Fatal(err)
	}
	router, err := New(registry, store, map[string]int{"mock": 1})
	if err != nil {
		t.Fatal(err)
	}
	transactionStore := NewMemoryTransactionStore()
	failingStore := &failPutTransactionStore{base: transactionStore, failAfter: 2}
	service, err := NewServiceWithStore(router, failingStore)
	if err != nil {
		t.Fatal(err)
	}

	req := PurchaseRequest{ProductCode: "pln20", CustomerNo: "08123456789", ReferenceID: "ref-result-persist-failure", Amount: 20000}
	execution, err := service.Purchase(context.Background(), req)
	if err == nil {
		t.Fatal("expected result persistence failure")
	}
	if execution.Result.Status != provider.StatusPending {
		t.Fatalf("expected caller to retain pending state, got %q", execution.Result.Status)
	}
	if got := mock.PurchaseCount(req.ReferenceID); got != 1 {
		t.Fatalf("expected exactly one provider submission, got %d", got)
	}
	persisted, ok := transactionStore.Get(req.ReferenceID)
	if !ok {
		t.Fatal("expected durable pending state to remain")
	}
	if persisted.Execution.Result.Status != provider.StatusPending {
		t.Fatalf("expected durable pending state after result persistence failure, got %q", persisted.Execution.Result.Status)
	}
}

type mismatchedStatusProvider struct {
	provider.PPOBProvider
}

func (p mismatchedStatusProvider) GetStatus(ctx context.Context, req provider.StatusRequest) (provider.PurchaseStatus, error) {
	status, err := p.PPOBProvider.GetStatus(ctx, req)
	if err != nil {
		return provider.PurchaseStatus{}, err
	}
	status.CustomerNo = "08999999999"
	return status, nil
}

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


func TestServiceReconcileUsesProviderStatusWithoutResubmitting(t *testing.T) {
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

	req := PurchaseRequest{ProductCode: "pln20", CustomerNo: "08123456789", ReferenceID: "ref-reconcile", Amount: 20000}
	execution, err := service.Purchase(context.Background(), req)
	if err != nil { t.Fatal(err) }
	if execution.Result.Status != provider.StatusPending { t.Fatalf("expected pending purchase, got %q", execution.Result.Status) }

	reconciled, err := service.Reconcile(context.Background(), req.ReferenceID)
	if err != nil { t.Fatal(err) }
	if reconciled != execution { t.Fatalf("expected reconciliation to preserve provider status, got %#v vs %#v", reconciled, execution) }
	if got := mock.PurchaseCount(req.ReferenceID); got != 1 { t.Fatalf("expected reconciliation not to resubmit purchase, got %d submissions", got) }
}

func TestServiceReconcileRejectsIdentityMismatch(t *testing.T) {
	registry := provider.NewRegistry()
	mock := Mock.New(Mock.Config{
		Products:       []provider.Product{{Code: "pln20", Name: "PLN 20"}},
		ProviderCode:   "00",
		Message:        "pending",
		PurchaseStatus: provider.StatusPending,
	})
	if err := registry.Register("mock", mismatchedStatusProvider{PPOBProvider: mock}); err != nil {
		t.Fatal(err)
	}
	store := operational.NewMemoryStore()
	if err := store.Put(operational.Snapshot{ProviderName: "mock", Balance: 100000, Health: operational.HealthHealthy}); err != nil {
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

	req := PurchaseRequest{ProductCode: "pln20", CustomerNo: "08123456789", ReferenceID: "ref-reconcile-mismatch", Amount: 20000}
	if _, err := service.Purchase(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	_, err = service.Reconcile(context.Background(), req.ReferenceID)
	if !errors.Is(err, ErrWebhookReferenceConflict) {
		t.Fatalf("expected identity mismatch error, got %v", err)
	}
}

func TestServiceRestartRecoversDurableTransactionState(t *testing.T) {
	storePath := filepath.Join(t.TempDir(), "transactions", "state.json")
	firstRegistry := provider.NewRegistry()
	firstMock := Mock.New(Mock.Config{
		Products:       []provider.Product{{Code: "pln20", Name: "PLN 20"}},
		ProviderCode:   "00",
		Message:        "pending",
		PurchaseStatus: provider.StatusPending,
		Price:          20000,
	})
	if err := firstRegistry.Register("mock", firstMock); err != nil {
		t.Fatal(err)
	}
	operationalStore := operational.NewMemoryStore()
	if err := operationalStore.Put(operational.Snapshot{ProviderName: "mock", Balance: 100000, Health: operational.HealthHealthy}); err != nil {
		t.Fatal(err)
	}
	router, err := New(firstRegistry, operationalStore, map[string]int{"mock": 1})
	if err != nil {
		t.Fatal(err)
	}
	transactionStore, err := NewJSONFileTransactionStore(storePath)
	if err != nil {
		t.Fatal(err)
	}
	firstService, err := NewServiceWithStore(router, transactionStore)
	if err != nil {
		t.Fatal(err)
	}

	req := PurchaseRequest{ProductCode: "pln20", CustomerNo: "08123456789", ReferenceID: "ref-restart", Amount: 20000}
	first, err := firstService.Purchase(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if first.Result.Status != provider.StatusPending {
		t.Fatalf("expected pending purchase, got %q", first.Result.Status)
	}
	if got := firstMock.PurchaseCount(req.ReferenceID); got != 1 {
		t.Fatalf("expected one initial provider submission, got %d", got)
	}

	secondRegistry := provider.NewRegistry()
	secondMock := Mock.New(Mock.Config{
		Products:       []provider.Product{{Code: "pln20", Name: "PLN 20"}},
		ProviderCode:   "00",
		Message:        "pending",
		PurchaseStatus: provider.StatusPending,
		Price:          20000,
	})
	if err := secondRegistry.Register("mock", secondMock); err != nil {
		t.Fatal(err)
	}
	secondOperationalStore := operational.NewMemoryStore()
	if err := secondOperationalStore.Put(operational.Snapshot{ProviderName: "mock", Balance: 100000, Health: operational.HealthHealthy}); err != nil {
		t.Fatal(err)
	}
	secondRouter, err := New(secondRegistry, secondOperationalStore, map[string]int{"mock": 1})
	if err != nil {
		t.Fatal(err)
	}
	recoveredStore, err := NewJSONFileTransactionStore(storePath)
	if err != nil {
		t.Fatal(err)
	}
	recoveredService, err := NewServiceWithStore(secondRouter, recoveredStore)
	if err != nil {
		t.Fatal(err)
	}

	recovered, err := recoveredService.Purchase(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if recovered != first {
		t.Fatalf("expected recovered transaction state: %#v != %#v", recovered, first)
	}
	if got := secondMock.PurchaseCount(req.ReferenceID); got != 0 {
		t.Fatalf("expected restart-safe idempotency without resubmission, got %d submissions", got)
	}

}


func TestTransactionStoreRejectsTerminalOverwrite(t *testing.T) {
	store := NewMemoryTransactionStore()
	req := PurchaseRequest{ProductCode: "pln20", CustomerNo: "08123456789", ReferenceID: "ref-terminal", Amount: 20000}
	success := TransactionState{Request: req, Execution: PurchaseExecution{ProviderName: "mock", Result: provider.PurchaseResult{
		ReferenceID: req.ReferenceID, CustomerNo: req.CustomerNo, ProductCode: req.ProductCode, Status: provider.StatusSuccess, ProviderCode: "00",
	}}}
	if err := store.Put(success); err != nil { t.Fatal(err) }

	mutated := success
	mutated.Execution.Result.Message = "mutated"
	if err := store.Put(mutated); !errors.Is(err, ErrReferenceConflict) {
		t.Fatalf("expected terminal overwrite rejection, got %v", err)
	}
}

func TestTransactionStoreRejectsRequestMutation(t *testing.T) {
	store := NewMemoryTransactionStore()
	req := PurchaseRequest{ProductCode: "pln20", CustomerNo: "08123456789", ReferenceID: "ref-request-mutation", Amount: 20000}
	pending := TransactionState{Request: req, Execution: PurchaseExecution{ProviderName: "mock", Result: provider.PurchaseResult{
		ReferenceID: req.ReferenceID, CustomerNo: req.CustomerNo, ProductCode: req.ProductCode, Status: provider.StatusPending,
	}}}
	if err := store.Put(pending); err != nil { t.Fatal(err) }

	mutated := pending
	mutated.Request.Amount = 21000
	if err := store.Put(mutated); !errors.Is(err, ErrReferenceConflict) {
		t.Fatalf("expected request mutation rejection, got %v", err)
	}
}
