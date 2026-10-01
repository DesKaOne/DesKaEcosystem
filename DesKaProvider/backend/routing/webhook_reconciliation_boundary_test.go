package routing

import (
    "context"
    "errors"
    "testing"

    provider "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider"
    Mock "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider/Mock"
    "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider/operational"
)

func TestServiceWebhookAndReconciliationRemainAllowedWhenProviderLifecycleDisabled(t *testing.T) {
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

    operationalStore := operational.NewMemoryStore()
    if err := operationalStore.Put(operational.Snapshot{
        ProviderName: "mock",
        Balance:      100000,
        Health:       operational.HealthHealthy,
    }); err != nil {
        t.Fatal(err)
    }

    stateStore := operational.NewProviderStateStore()
    state, err := operational.NewProviderState("mock")
    if err != nil {
        t.Fatal(err)
    }
    state.Lifecycle = operational.LifecycleDisabled
    state.Capabilities = []operational.Capability{operational.CapabilityPPOB}
    state.EnabledCapabilities = nil
    if err := stateStore.Put(state); err != nil {
        t.Fatal(err)
    }

    router, err := NewWithState(registry, operationalStore, map[string]int{"mock": 1}, stateStore)
    if err != nil {
        t.Fatal(err)
    }
    service, err := NewService(router)
    if err != nil {
        t.Fatal(err)
    }

    req := PurchaseRequest{
        ProductCode: "pln20",
        CustomerNo:  "08123456789",
        ReferenceID: "ref-disabled-observation",
        Amount:      20000,
    }
    pending := TransactionState{
        Request: req,
        Execution: PurchaseExecution{
            ProviderName: "mock",
            Result: provider.PurchaseResult{
                ReferenceID: req.ReferenceID,
                CustomerNo:  req.CustomerNo,
                ProductCode: req.ProductCode,
                Status:      provider.StatusPending,
                Price:       req.Amount,
            },
        },
    }
    if err := service.Store.Put(pending); err != nil {
        t.Fatal(err)
    }

    webhookResult, err := service.HandleWebhookFromProvider(context.Background(), "mock", provider.WebhookEvent{
        ReferenceID: req.ReferenceID,
        CustomerNo:  req.CustomerNo,
        ProductCode: req.ProductCode,
        Status:      provider.StatusSuccess,
        ProviderCode: "00",
        Message:     "success",
        Price:       req.Amount,
    })
    if err != nil {
        t.Fatalf("webhook observation must remain processable while lifecycle is disabled: %v", err)
    }
    if webhookResult.Result.Status != provider.StatusSuccess {
        t.Fatalf("expected webhook to persist terminal observation, got %#v", webhookResult)
    }

    if _, err := service.Purchase(context.Background(), PurchaseRequest{
        ProductCode: "pln20",
        CustomerNo:  "08123456789",
        ReferenceID: "ref-disabled-side-effect",
        Amount:      20000,
    }); err == nil || !errors.Is(err, ErrNoProviderAvailable) {
        t.Fatalf("disabled lifecycle must block new PPOB execution, got %v", err)
    }
    if got := mock.PurchaseCount("ref-disabled-side-effect"); got != 0 {
        t.Fatalf("disabled lifecycle must not authorize a new provider purchase, got %d calls", got)
    }
}

func TestServiceReconcileRemainsAllowedWhenProviderLifecycleDisabled(t *testing.T) {
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

    operationalStore := operational.NewMemoryStore()
    if err := operationalStore.Put(operational.Snapshot{
        ProviderName: "mock",
        Balance:      100000,
        Health:       operational.HealthHealthy,
    }); err != nil {
        t.Fatal(err)
    }

    stateStore := operational.NewProviderStateStore()
    state, err := operational.NewProviderState("mock")
    if err != nil {
        t.Fatal(err)
    }
    state.Lifecycle = operational.LifecycleEnabled
    state.Capabilities = []operational.Capability{operational.CapabilityPPOB}
    state.EnabledCapabilities = []operational.Capability{operational.CapabilityPPOB}
    if err := stateStore.Put(state); err != nil {
        t.Fatal(err)
    }

    router, err := NewWithState(registry, operationalStore, map[string]int{"mock": 1}, stateStore)
    if err != nil {
        t.Fatal(err)
    }
    service, err := NewService(router)
    if err != nil {
        t.Fatal(err)
    }

    req := PurchaseRequest{
        ProductCode: "pln20",
        CustomerNo:  "08123456789",
        ReferenceID: "ref-disabled-reconcile",
        Amount:      20000,
    }
    if _, err := service.Purchase(context.Background(), req); err != nil {
        t.Fatal(err)
    }
    if got := mock.PurchaseCount(req.ReferenceID); got != 1 {
        t.Fatalf("expected one initial provider purchase, got %d", got)
    }

    state.Lifecycle = operational.LifecycleDisabled
    state.EnabledCapabilities = nil
    if err := stateStore.Put(state); err != nil {
        t.Fatal(err)
    }

    reconciled, err := service.Reconcile(context.Background(), req.ReferenceID)
    if err != nil {
        t.Fatalf("reconciliation must remain observationally available while lifecycle is disabled: %v", err)
    }
    if reconciled.Result.Status != provider.StatusSuccess {
        t.Fatalf("expected reconciled success, got %#v", reconciled)
    }
    if got := mock.PurchaseCount(req.ReferenceID); got != 0 {
        t.Fatalf("reconciliation must not resubmit provider purchase, got %d calls", got)
    }
}