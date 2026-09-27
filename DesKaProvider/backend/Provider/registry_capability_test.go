package provider

import "testing"

func TestRegistryCapabilitiesAreProviderScopedAndCopied(t *testing.T) {
	registry := NewRegistry()
	impl := &mockProvider{}
	descriptor := CapabilityDescriptor{Capabilities: map[Capability]CapabilityStatus{
		CapabilityPPOB: {Verified: true, AdapterImplemented: true, Enabled: true},
	}}
	if err := registry.RegisterWithCapabilities("Demo", impl, descriptor); err != nil {
		t.Fatalf("register: %v", err)
	}
	got, err := registry.Capabilities("demo")
	if err != nil {
		t.Fatalf("capabilities: %v", err)
	}
	status, ok := got.Status(CapabilityPPOB)
	if !ok || !status.Verified || !status.AdapterImplemented || !status.Enabled {
		t.Fatalf("unexpected capability status: %#v", status)
	}
	got.Capabilities[CapabilityPPOB] = CapabilityStatus{}
	again, err := registry.Capabilities("demo")
	if err != nil {
		t.Fatalf("capabilities after mutation: %v", err)
	}
	status, _ = again.Status(CapabilityPPOB)
	if !status.Verified || !status.AdapterImplemented || !status.Enabled {
		t.Fatal("registry capability metadata was not isolated from caller mutation")
	}
}
