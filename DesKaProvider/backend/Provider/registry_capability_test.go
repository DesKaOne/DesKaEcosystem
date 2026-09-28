package provider

import (
	"context"
	"testing"
)

func TestRegistryCapabilitiesAreProviderScopedAndCopied(t *testing.T) {
	registry := NewRegistry()
	impl := &capabilityTestProvider{}
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


type capabilityTestProvider struct{}

func (capabilityTestProvider) GetProducts(context.Context, ProductRequest) ([]Product, error) { return nil, nil }
func (capabilityTestProvider) Inquiry(context.Context, InquiryRequest) (InquiryResult, error) { return InquiryResult{}, nil }
func (capabilityTestProvider) Purchase(context.Context, PurchaseRequest) (PurchaseResult, error) { return PurchaseResult{}, nil }
func (capabilityTestProvider) GetStatus(context.Context, StatusRequest) (PurchaseStatus, error) { return PurchaseStatus{}, nil }
func (capabilityTestProvider) HandleWebhook(context.Context, WebhookRequest) (WebhookEvent, error) { return WebhookEvent{}, nil }

func TestCapabilityStatusRejectsInvalidReadinessCombinations(t *testing.T) {
	cases := []struct {
		name string
		status CapabilityStatus
	}{
		{"enabled without implementation", CapabilityStatus{Tested: true, Enabled: true}},
		{"live tested without tested", CapabilityStatus{AdapterImplemented: true, Enabled: true, LiveTested: true}},
		{"production ready without verification", CapabilityStatus{Configured: true, AdapterImplemented: true, Tested: true, Enabled: true, LiveTested: true, ProductionReady: true}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.status.Validate(); err == nil {
				t.Fatal("expected invalid capability status to be rejected")
			}
		})
	}
}

func TestCapabilityDescriptorRejectsUnknownCapability(t *testing.T) {
	registry := NewRegistry()
	err := registry.RegisterWithCapabilities("demo", &capabilityTestProvider{}, CapabilityDescriptor{
		Capabilities: map[Capability]CapabilityStatus{
			Capability("provider_specific"): {AdapterImplemented: true, Tested: true},
		},
	})
	if err == nil {
		t.Fatal("expected unknown capability to be rejected")
	}
}

func TestCapabilityDescriptorSupportsOnlyTestedEnabledCapabilities(t *testing.T) {
	descriptor := CapabilityDescriptor{Capabilities: map[Capability]CapabilityStatus{
		CapabilityBalance: {AdapterImplemented: true, Tested: false, Enabled: true},
		CapabilityCatalog: {AdapterImplemented: true, Tested: true, Enabled: true},
	}}
	if descriptor.Supports(CapabilityBalance) {
		t.Fatal("untested capability must not be routable")
	}
	if !descriptor.Supports(CapabilityCatalog) {
		t.Fatal("tested and enabled capability should be routable")
	}
}

func TestRegisterCapabilityProviderRejectsInvalidMetadata(t *testing.T) {
	registry := NewRegistry()
	if err := registry.RegisterCapabilityProvider("demo", CapabilityBalance, struct{}{}, CapabilityStatus{
		AdapterImplemented: false,
		Enabled: true,
	}); err == nil {
		t.Fatal("expected invalid capability metadata to be rejected")
	}
	if err := registry.RegisterCapabilityProvider("demo", Capability("unknown"), struct{}{}, CapabilityStatus{
		AdapterImplemented: true,
		Tested: true,
	}); err == nil {
		t.Fatal("expected unknown capability to be rejected")
	}
}
