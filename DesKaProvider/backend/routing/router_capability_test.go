package routing

import (
    "context"
    "errors"
    "testing"

    provider "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider"
    mock "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider/Mock"
    "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider/operational"
)

func TestRouterRequiresExplicitRegistryCapabilityEligibility(t *testing.T) {
    cases := []struct {
        name string
        status provider.CapabilityStatus
        want string
    }{
        {
            name: "disabled",
            status: provider.CapabilityStatus{AdapterImplemented: true, Enabled: false},
        },
        {
            name: "not-implemented",
            status: provider.CapabilityStatus{AdapterImplemented: false, Enabled: false},
        },
        {
            name: "implemented-and-enabled",
            status: provider.CapabilityStatus{AdapterImplemented: true, Enabled: true},
            want: "mock",
        },
    }

    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {
            registry := provider.NewRegistry()
            implementation := mock.New(mock.Config{
                Products: []provider.Product{{Code: "xld10", Name: "Test"}},
            })
            if err := registry.RegisterWithCapabilities("mock", implementation, provider.CapabilityDescriptor{
                Capabilities: map[provider.Capability]provider.CapabilityStatus{
                    provider.CapabilityPPOB: tc.status,
                },
            }); err != nil {
                t.Fatal(err)
            }

            store := operational.NewMemoryStore()
            if err := store.Put(operational.Snapshot{
                ProviderName: "mock",
                Balance: 100000,
                Health: operational.HealthHealthy,
            }); err != nil {
                t.Fatal(err)
            }

            states := operational.NewProviderStateStore()
            if err := states.Put(operational.ProviderState{
                ProviderName: "mock",
                Lifecycle: operational.LifecycleEnabled,
                Capabilities: []operational.Capability{operational.CapabilityPPOB},
            }); err != nil {
                t.Fatal(err)
            }

            router, err := NewWithState(registry, store, nil, states)
            if err != nil {
                t.Fatal(err)
            }

            got, err := router.Select(context.Background(), Request{
                ProductCode: "xld10",
                Amount: 50000,
            })
            if tc.want == "" {
                if !errors.Is(err, ErrNoProviderAvailable) {
                    t.Fatalf("expected capability-ineligible provider to be rejected, got provider=%q err=%v", got, err)
                }
                return
            }
            if err != nil {
                t.Fatal(err)
            }
            if got != tc.want {
                t.Fatalf("expected %q, got %q", tc.want, got)
            }
        })
    }
}

func TestRouterCapabilityEligibilityDoesNotOverrideHealthOrBalanceGates(t *testing.T) {
    registry := provider.NewRegistry()
    if err := registry.RegisterWithCapabilities("mock", mock.New(mock.Config{
        Products: []provider.Product{{Code: "xld10", Name: "Test"}},
    }), provider.CapabilityDescriptor{
        Capabilities: map[provider.Capability]provider.CapabilityStatus{
            provider.CapabilityPPOB: {AdapterImplemented: true, Enabled: true},
        },
    }); err != nil {
        t.Fatal(err)
    }

    store := operational.NewMemoryStore()
    if err := store.Put(operational.Snapshot{
        ProviderName: "mock",
        Balance: 1000,
        Health: operational.HealthUnhealthy,
    }); err != nil {
        t.Fatal(err)
    }

    states := operational.NewProviderStateStore()
    if err := states.Put(operational.ProviderState{
        ProviderName: "mock",
        Lifecycle: operational.LifecycleEnabled,
        Capabilities: []operational.Capability{operational.CapabilityPPOB},
    }); err != nil {
        t.Fatal(err)
    }

    router, err := NewWithState(registry, store, nil, states)
    if err != nil {
        t.Fatal(err)
    }

    if _, err := router.Select(context.Background(), Request{
        ProductCode: "xld10",
        Amount: 50000,
    }); !errors.Is(err, ErrNoProviderAvailable) {
        t.Fatalf("expected operational gates to remain authoritative, got %v", err)
    }
}
