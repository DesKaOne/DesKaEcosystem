package provider

import (
    "context"
    "testing"
)

type balanceOnlyProvider struct{}

func (balanceOnlyProvider) GetBalance(context.Context) (int64, error) { return 123, nil }

func TestRegisterCapabilityProviderWithoutPPOBProvider(t *testing.T) {
    r := NewRegistry()
    impl := balanceOnlyProvider{}
    status := CapabilityStatus{
        Verified: true, Configured: true, AdapterImplemented: true,
        Enabled: true, LiveTested: false,
    }
    if err := r.RegisterCapabilityProvider("xp-sindonesia", CapabilityBalance, impl, status); err != nil {
        t.Fatal(err)
    }
    got, err := r.GetCapabilityProvider("xp-sindonesia", CapabilityBalance)
    if err != nil {
        t.Fatal(err)
    }
    balance, ok := got.(BalanceProvider)
    if !ok {
        t.Fatalf("registered capability has type %T, want BalanceProvider", got)
    }
    if v, err := balance.GetBalance(context.Background()); err != nil || v != 123 {
        t.Fatalf("balance=%d err=%v", v, err)
    }

    descriptor, err := r.Capabilities("xp-sindonesia")
    if err != nil {
        t.Fatal(err)
    }
    if !descriptor.Supports(CapabilityBalance) {
        t.Fatal("expected balance capability to be supported only after explicit enablement")
    }
}

func TestRegisterCapabilityProviderDoesNotInferPPOB(t *testing.T) {
    r := NewRegistry()
    status := CapabilityStatus{AdapterImplemented: true, Enabled: true}
    if err := r.RegisterCapabilityProvider("xp-sindonesia", CapabilityBalance, balanceOnlyProvider{}, status); err != nil {
        t.Fatal(err)
    }
    if _, err := r.Get("xp-sindonesia"); err == nil {
        t.Fatal("partial capability registration must not create a PPOB provider")
    }
    descriptor, err := r.Capabilities("xp-sindonesia")
    if err != nil {
        t.Fatal(err)
    }
    if descriptor.Supports(CapabilityPPOB) {
        t.Fatal("balance registration must not imply PPOB")
    }
}

func TestRegisterCapabilityProviderRejectsDuplicateCapability(t *testing.T) {
    r := NewRegistry()
    status := CapabilityStatus{AdapterImplemented: true}
    if err := r.RegisterCapabilityProvider("xp-sindonesia", CapabilityBalance, balanceOnlyProvider{}, status); err != nil {
        t.Fatal(err)
    }
    if err := r.RegisterCapabilityProvider("xp-sindonesia", CapabilityBalance, balanceOnlyProvider{}, status); err == nil {
        t.Fatal("expected duplicate capability registration to fail")
    }
}


func TestCapabilitiesConcurrentWithCapabilityRegistration(t *testing.T) {
    r := NewRegistry()
    status := CapabilityStatus{AdapterImplemented: true}
    if err := r.RegisterCapabilityProvider("xp-sindonesia", CapabilityBalance, balanceOnlyProvider{}, status); err != nil {
        t.Fatal(err)
    }

    const iterations = 200
    done := make(chan struct{})
    go func() {
        defer close(done)
        for i := 0; i < iterations; i++ {
            // Each registration targets a distinct canonical capability so the
            // registry lock protects descriptor/map access under concurrent reads.
            capability := CapabilityPPOB
            if i%2 == 0 {
                capability = CapabilityCatalog
            }
            name := "provider-" + string(rune('a'+(i%26)))
            _ = r.RegisterCapabilityProvider(name, capability, balanceOnlyProvider{}, status)
        }
    }()

    for i := 0; i < iterations; i++ {
        if _, err := r.Capabilities("xp-sindonesia"); err != nil {
            t.Fatal(err)
        }
    }
    <-done
}
