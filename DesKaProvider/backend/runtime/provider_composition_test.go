package runtime

import (
    "net/http"
    "os"
    "testing"

    provider "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider"
    mock "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider/Mock"
)

func TestRegisterConfiguredProvidersRequiresExplicitCapabilityMetadata(t *testing.T) {
    registry := provider.NewRegistry()
    if err := registerConfiguredProviders(registry, mock.New(mock.Config{}), http.DefaultClient); err != nil {
        t.Fatal(err)
    }

    descriptor, err := registry.Capabilities("digiflazz")
    if err != nil {
        t.Fatal(err)
    }
    for _, capability := range []provider.Capability{
        provider.CapabilityPPOB,
        provider.CapabilityBalance,
        provider.CapabilityWebhook,
    } {
        status, ok := descriptor.Capabilities[capability]
        if !ok {
            t.Fatalf("missing DigiFlazz capability %q", capability)
        }
        if !status.Verified || !status.Configured || !status.AdapterImplemented {
            t.Fatalf("unexpected DigiFlazz %q status: %+v", capability, status)
        }
        if status.Enabled || status.LiveTested {
            t.Fatalf("unsafe default for DigiFlazz %q: %+v", capability, status)
        }
    }
}

func TestRegisterConfiguredProvidersDoesNotPartiallyRegisterIAK(t *testing.T) {
    t.Setenv("IAK_USERNAME", "configured")
    t.Setenv("IAK_API_KEY", "")
    registry := provider.NewRegistry()

    if err := registerConfiguredProviders(registry, mock.New(mock.Config{}), http.DefaultClient); err == nil {
        t.Fatal("expected incomplete IAK configuration to fail")
    }

    if _, err := registry.Get("iak"); err == nil {
        t.Fatal("IAK must not be registered after configuration failure")
    }
    if _, err := registry.Capabilities("iak"); err == nil {
        t.Fatal("IAK capability metadata must not exist after configuration failure")
    }
}

func TestRegisterConfiguredProvidersRegistersIAKOnlyWhenConfigured(t *testing.T) {
    t.Setenv("IAK_USERNAME", "")
    t.Setenv("IAK_API_KEY", "")
    registry := provider.NewRegistry()

    if err := registerConfiguredProviders(registry, mock.New(mock.Config{}), http.DefaultClient); err != nil {
        t.Fatal(err)
    }
    if _, err := registry.Get("iak"); err == nil {
        t.Fatal("IAK must not be registered without configuration")
    }

    _ = os.Unsetenv("IAK_USERNAME")
    _ = os.Unsetenv("IAK_API_KEY")
}
