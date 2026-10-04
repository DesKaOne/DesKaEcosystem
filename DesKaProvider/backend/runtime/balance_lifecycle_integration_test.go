package runtime

import (
    "context"
    "errors"
    "testing"
    "time"

    provider "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider"
    "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider/operational"
)

func runtimeBalanceStub() provider.PPOBProvider {
    return runtimeTestProvider{balance: 4400000}
}

type runtimeTestProvider struct{ balance int64 }

func (p runtimeTestProvider) GetProducts(context.Context, provider.ProductRequest) ([]provider.Product, error) {
    return nil, provider.ErrUnsupportedOperation
}
func (p runtimeTestProvider) Inquiry(context.Context, provider.InquiryRequest) (provider.InquiryResult, error) {
    return provider.InquiryResult{}, provider.ErrUnsupportedOperation
}
func (p runtimeTestProvider) Purchase(context.Context, provider.PurchaseRequest) (provider.PurchaseResult, error) {
    return provider.PurchaseResult{}, provider.ErrUnsupportedOperation
}
func (p runtimeTestProvider) GetStatus(context.Context, provider.StatusRequest) (provider.PurchaseStatus, error) {
    return provider.PurchaseStatus{}, provider.ErrUnsupportedOperation
}
func (p runtimeTestProvider) HandleWebhook(context.Context, provider.WebhookRequest) (provider.WebhookEvent, error) {
    return provider.WebhookEvent{}, provider.ErrUnsupportedOperation
}
func (p runtimeTestProvider) GetBalance(context.Context) (int64, error) {
    return p.balance, nil
}

func TestServiceRunOwnsAndStopsBalanceWorker(t *testing.T) {
    registry := provider.NewRegistry()
    if err := registry.Register("mock", runtimeBalanceStub()); err != nil {
        t.Fatal(err)
    }
    store := operational.NewMemoryStore()
    syncService, err := operational.NewSyncService(registry, store, "IDR", 3)
    if err != nil {
        t.Fatal(err)
    }
    service, err := New(syncService, time.Hour)
    if err != nil {
        t.Fatal(err)
    }

    ctx, cancel := context.WithCancel(context.Background())
    done := make(chan error, 1)
    go func() { done <- service.Run(ctx) }()

    deadline := time.After(time.Second)
    for {
        if snapshot, ok := store.Get("mock"); ok {
            if snapshot.Balance != 4400000 || snapshot.Health != operational.HealthHealthy {
                t.Fatalf("unexpected runtime balance snapshot: %#v", snapshot)
            }
            break
        }
        select {
        case <-deadline:
            t.Fatal("runtime service did not start the owned balance worker")
        default:
            time.Sleep(time.Millisecond)
        }
    }

    if err := service.Run(context.Background()); !errors.Is(err, operational.ErrSyncWorkerRunning) {
        t.Fatalf("expected duplicate runtime Run to reject active balance worker, got %v", err)
    }

    cancel()
    select {
    case err := <-done:
        if !errors.Is(err, context.Canceled) {
            t.Fatalf("expected runtime Run to return context cancellation, got %v", err)
        }
    case <-time.After(time.Second):
        t.Fatal("runtime service did not stop after context cancellation")
    }

    if service.balanceLifecycle.Running() {
        t.Fatal("runtime-owned balance worker remained active after Run returned")
    }
}
