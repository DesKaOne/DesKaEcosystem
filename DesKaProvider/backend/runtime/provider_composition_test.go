package runtime

import (
    "context"\n    "errors"\n    "net/http"
    "os"
    "testing"
    "reflect"

    "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider/operational"

    provider "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider"
    mock "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider/Mock"
)

func TestRegisterConfiguredProvidersDoesNotRequireDigiFlazz(t *testing.T) {
    t.Setenv("MIDTRANS_SERVER_KEY", "test-midtrans-key")
    registry := provider.NewRegistry()

    if err := registerConfiguredProviders(registry, nil, http.DefaultClient); err != nil {
        t.Fatal(err)
    }

    if _, err := registry.GetCapabilityProvider("midtrans", provider.CapabilityPayment); err != nil {
        t.Fatalf("Midtrans should register without DigiFlazz: %v", err)
    }
    if _, err := registry.Get("digiflazz"); err == nil {
        t.Fatal("DigiFlazz must remain unregistered when its credentials are absent")
    }
}

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


func TestRegisterConfiguredProvidersAlignsExplicitCapabilityMatrix(t *testing.T) {
	t.Setenv("MIDTRANS_SERVER_KEY", "test-midtrans-key")
	t.Setenv("IAK_USERNAME", "test-iak-user")
	t.Setenv("IAK_API_KEY", "test-iak-key")
	t.Setenv("XP_SINDONESIA_ID", "test-xp-id")
	t.Setenv("XP_SINDONESIA_KEY", "test-xp-key")
	t.Setenv("XP_SINDONESIA_API", "test-xp-api")

	registry := provider.NewRegistry()
	if err := registerConfiguredProviders(registry, mock.New(mock.Config{}), http.DefaultClient); err != nil {
		t.Fatal(err)
	}

	want := map[string][]provider.Capability{
		"midtrans":      {provider.CapabilityPayment, provider.CapabilityWebhook},
		"digiflazz":     {provider.CapabilityPPOB, provider.CapabilityBalance, provider.CapabilityWebhook, provider.CapabilityCatalog},
		"iak":            {provider.CapabilityPPOB, provider.CapabilityBalance, provider.CapabilityWebhook, provider.CapabilityCatalog},
		"xp-sindonesia": {provider.CapabilityPPOB, provider.CapabilityBalance, provider.CapabilityWebhook},
	}
	for name, capabilities := range want {
		descriptor, err := registry.Capabilities(name)
		if err != nil {
			t.Fatalf("%s capabilities: %v", name, err)
		}
		if len(descriptor.Capabilities) != len(capabilities) {
			t.Fatalf("%s capability count = %d, want %d: %#v", name, len(descriptor.Capabilities), len(capabilities), descriptor.Capabilities)
		}
		for _, capability := range capabilities {
			status, ok := descriptor.Status(capability)
			if !ok {
				t.Fatalf("%s missing capability %q", name, capability)
			}
			if !status.Configured || !status.AdapterImplemented || !status.Tested {
				t.Fatalf("%s capability %q has incomplete implementation metadata: %+v", name, capability, status)
			}
			if status.Enabled || status.LiveTested || status.ProductionReady {
				t.Fatalf("%s capability %q is unsafe by default: %+v", name, capability, status)
			}
		}
	}

	for _, unsupported := range []struct {
		providerName string
		capability   provider.Capability
	}{
		{"midtrans", provider.CapabilityPPOB},
		{"xp-sindonesia", provider.CapabilityCatalog},
	} {
		descriptor, err := registry.Capabilities(unsupported.providerName)
		if err != nil {
			t.Fatalf("%s capabilities: %v", unsupported.providerName, err)
		}
		if _, ok := descriptor.Status(unsupported.capability); ok {
			t.Fatalf("%s must not infer unsupported capability %q", unsupported.providerName, unsupported.capability)
		}
	}
}


func TestCapabilitiesFromDescriptorMatchesImplementedMetadataOnly(t *testing.T) {
	descriptor := provider.CapabilityDescriptor{Capabilities: map[provider.Capability]provider.CapabilityStatus{
		provider.CapabilityPPOB:    {AdapterImplemented: true},
		provider.CapabilityBalance: {AdapterImplemented: true},
		provider.CapabilityWebhook: {AdapterImplemented: false},
		provider.CapabilityPayout:  {AdapterImplemented: true},
	}}
	got := capabilitiesFromDescriptor(descriptor)
	want := []operational.Capability{operational.CapabilityBalance, operational.CapabilityPayout, operational.CapabilityPPOB}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("derived capabilities = %#v, want %#v", got, want)
	}
}

func TestRegisterConfiguredProvidersComposesPartialConfigurationIndependently(t *testing.T) {
	t.Setenv("MIDTRANS_SERVER_KEY", "test-midtrans-key")
	t.Setenv("IAK_USERNAME", "")
	t.Setenv("IAK_API_KEY", "")
	t.Setenv("XP_SINDONESIA_ID", "test-xp-id")
	t.Setenv("XP_SINDONESIA_KEY", "test-xp-key")
	t.Setenv("XP_SINDONESIA_API", "test-xp-api")
	t.Setenv("DIGIFLAZZ_USERNAME", "")
	t.Setenv("DIGIFLAZZ_API_KEY", "")

	registry := provider.NewRegistry()
	if err := registerConfiguredProviders(registry, nil, http.DefaultClient); err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"midtrans", "xp-sindonesia"} {
		if _, err := registry.Capabilities(name); err != nil {
			t.Fatalf("%s should compose independently: %v", name, err)
		}
	}
	for _, name := range []string{"iak", "digiflazz"} {
		if _, err := registry.Capabilities(name); err == nil {
			t.Fatalf("%s must remain absent without configuration", name)
		}
	}
}

func TestRegisterConfiguredProvidersRejectsPartialXPSindonesiaConfiguration(t *testing.T) {
	t.Setenv("MIDTRANS_SERVER_KEY", "")
	t.Setenv("IAK_USERNAME", "")
	t.Setenv("IAK_API_KEY", "")
	t.Setenv("XP_SINDONESIA_ID", "test-xp-id")
	t.Setenv("XP_SINDONESIA_KEY", "")
	t.Setenv("XP_SINDONESIA_API", "")

	registry := provider.NewRegistry()
	if err := registerConfiguredProviders(registry, nil, http.DefaultClient); err == nil {
		t.Fatal("expected incomplete XP SINDONESIA configuration to fail")
	}
	if _, err := registry.Capabilities("xp-sindonesia"); err == nil {
		t.Fatal("XP SINDONESIA must not be partially registered")
	}
}

func TestRuntimeCapabilityMatrixSnapshotIsDefensive(t *testing.T) {
	t.Setenv("MIDTRANS_SERVER_KEY", "test-midtrans-key")
	t.Setenv("IAK_USERNAME", "test-iak-user")
	t.Setenv("IAK_API_KEY", "test-iak-key")
	t.Setenv("XP_SINDONESIA_ID", "test-xp-id")
	t.Setenv("XP_SINDONESIA_KEY", "test-xp-key")
	t.Setenv("XP_SINDONESIA_API", "test-xp-api")

	registry := provider.NewRegistry()
	if err := registerConfiguredProviders(registry, nil, http.DefaultClient); err != nil {
		t.Fatal(err)
	}

	snapshot := registry.CapabilityMatrix()
	snapshot.Providers["midtrans"].Capabilities[provider.CapabilityPayment] = provider.CapabilityStatus{}
	xp := snapshot.Providers["xp-sindonesia"]\n\txp.Capabilities = nil\n\tsnapshot.Providers["xp-sindonesia"] = xp

	again := registry.CapabilityMatrix()
	midtransStatus, ok := again.Status("midtrans", provider.CapabilityPayment)
	if !ok || !midtransStatus.Configured || !midtransStatus.AdapterImplemented || !midtransStatus.Tested {
		t.Fatalf("registry snapshot mutation leaked into runtime metadata: %+v", midtransStatus)
	}
	if _, ok := again.Status("xp-sindonesia", provider.CapabilityPPOB); !ok {
		t.Fatal("registry snapshot mutation removed XP SINDONESIA metadata")
	}
}

func TestRuntimeCompositionDoesNotInferUnsupportedXPSindonesiaCapabilities(t *testing.T) {
	t.Setenv("XP_SINDONESIA_ID", "test-xp-id")
	t.Setenv("XP_SINDONESIA_KEY", "test-xp-key")
	t.Setenv("XP_SINDONESIA_API", "test-xp-api")

	registry := provider.NewRegistry()
	if err := registerConfiguredProviders(registry, nil, http.DefaultClient); err != nil {
		t.Fatal(err)
	}

	descriptor, err := registry.Capabilities("xp-sindonesia")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := descriptor.Status(provider.CapabilityCatalog); ok {
		t.Fatal("XP SINDONESIA must not infer Catalog from PPOBProvider presence")
	}
	client, err := registry.Get("xp-sindonesia")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.GetProducts(context.Background(), provider.ProductRequest{}); !errors.Is(err, provider.ErrUnsupportedOperation) {
		t.Fatalf("XP SINDONESIA Catalog method must remain unsupported, got %v", err)
	}
}

