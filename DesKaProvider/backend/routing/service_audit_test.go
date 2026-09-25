package routing

import (
	"context"
	"errors"
	"testing"

	provider "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider"
	Mock "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider/Mock"
	"github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider/operational"
)

type failTransactionAuditStore struct {
	err error
}

func (s failTransactionAuditStore) Append(TransactionAuditEvent) error {
	return s.err
}

func (s failTransactionAuditStore) All(string) []TransactionAuditEvent {
	return nil
}

func newAuditTestService(t *testing.T, mock provider.PPOBProvider, audit TransactionAuditStore) *Service {
	t.Helper()
	registry := provider.NewRegistry()
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
	service, err := NewServiceWithStoreAndAudit(router, NewMemoryTransactionStore(), audit)
	if err != nil {
		t.Fatal(err)
	}
	return service
}

func TestServiceAuditRecordsPurchaseLifecycleAndTerminalConflict(t *testing.T) {
	mock := Mock.New(Mock.Config{
		Products:       []provider.Product{{Code: "pln20", Name: "PLN 20"}},
		ProviderCode:   "00",
		Message:        "success",
		PurchaseStatus: provider.StatusSuccess,
		Price:          20000,
	})
	audit := NewMemoryTransactionAuditStore()
	service := newAuditTestService(t, mock, audit)

	req := PurchaseRequest{
		ProductCode: "pln20",
		CustomerNo:  "08123456789",
		ReferenceID: "ref-audit-lifecycle",
		Amount:      20000,
	}
	if _, err := service.Purchase(context.Background(), req); err != nil {
		t.Fatal(err)
	}

	events := audit.All(req.ReferenceID)
	if len(events) != 2 {
		t.Fatalf("expected purchase pending/result audit events, got %d", len(events))
	}
	if events[0].Action != "PURCHASE_PENDING" || events[1].Action != "PURCHASE_RESULT" {
		t.Fatalf("unexpected purchase audit actions: %#v", events)
	}

	_, err := service.HandleWebhook(context.Background(), provider.WebhookEvent{
		ReferenceID:  req.ReferenceID,
		ProductCode:  req.ProductCode,
		CustomerNo:   req.CustomerNo,
		Status:       provider.StatusFailed,
		ProviderCode: "99",
		Message:      "conflicting observation",
	})
	if !errors.Is(err, ErrWebhookReferenceConflict) {
		t.Fatalf("expected terminal webhook conflict, got %v", err)
	}

	events = audit.All(req.ReferenceID)
	if len(events) != 3 || events[2].Action != "WEBHOOK_TERMINAL_CONFLICT" {
		t.Fatalf("expected terminal conflict audit event, got %#v", events)
	}
	if got := mock.PurchaseCount(req.ReferenceID); got != 1 {
		t.Fatalf("expected one provider submission, got %d", got)
	}
}

func TestServiceAuditFailureDoesNotResubmitOrRevertTerminalState(t *testing.T) {
	mock := Mock.New(Mock.Config{
		Products:       []provider.Product{{Code: "pln20", Name: "PLN 20"}},
		ProviderCode:   "00",
		Message:        "success",
		PurchaseStatus: provider.StatusSuccess,
		Price:          20000,
	})
	auditErr := errors.New("audit unavailable")
	service := newAuditTestService(t, mock, failTransactionAuditStore{err: auditErr})

	req := PurchaseRequest{
		ProductCode: "pln20",
		CustomerNo:  "08123456789",
		ReferenceID: "ref-audit-failure",
		Amount:      20000,
	}
	execution, err := service.Purchase(context.Background(), req)
	if !errors.Is(err, auditErr) {
		t.Fatalf("expected audit error after durable terminal state, got %v", err)
	}
	if execution.Result.Status != provider.StatusSuccess {
		t.Fatalf("expected terminal success to remain visible, got %q", execution.Result.Status)
	}
	if got := mock.PurchaseCount(req.ReferenceID); got != 1 {
		t.Fatalf("expected exactly one provider submission, got %d", got)
	}

	persisted, ok := service.Store.Get(req.ReferenceID)
	if !ok || persisted.Execution.Result.Status != provider.StatusSuccess {
		t.Fatalf("expected durable terminal success after audit failure, got %#v", persisted)
	}

	execution, err = service.Purchase(context.Background(), req)
	if !errors.Is(err, auditErr) {
		t.Fatalf("expected original audit error on idempotent duplicate, got %v", err)
	}
	if execution.Result.Status != provider.StatusSuccess {
		t.Fatalf("expected idempotent terminal result, got %q", execution.Result.Status)
	}
	if got := mock.PurchaseCount(req.ReferenceID); got != 1 {
		t.Fatalf("audit failure must not authorize a second provider submission, got %d", got)
	}
}
