package provider

import "testing"

func TestCapabilityDescriptorSupportsRequiresEnabledAndAdapterImplemented(t *testing.T) {
	d := CapabilityDescriptor{Capabilities: map[Capability]CapabilityStatus{
		CapabilityPPOB: {AdapterImplemented: true, Enabled: false},
	}}
	if d.Supports(CapabilityPPOB) {
		t.Fatal("disabled capability must not be supported")
	}
	d.Capabilities[CapabilityPPOB] = CapabilityStatus{AdapterImplemented: true, Enabled: true}
	if !d.Supports(CapabilityPPOB) {
		t.Fatal("implemented and enabled capability should be supported")
	}
	d.Capabilities[CapabilityPPOB] = CapabilityStatus{AdapterImplemented: false, Enabled: true}
	if d.Supports(CapabilityPPOB) {
		t.Fatal("enabled capability without an adapter must not be supported")
	}
}

func TestCapabilityDescriptorStatusPreservesIndependentFlags(t *testing.T) {
	status := CapabilityStatus{
		Verified: true, Configured: true, AdapterImplemented: true,
		Enabled: false, LiveTested: false,
	}
	d := CapabilityDescriptor{Capabilities: map[Capability]CapabilityStatus{CapabilityPPOB: status}}
	got, ok := d.Status(CapabilityPPOB)
	if !ok {
		t.Fatal("expected capability status")
	}
	if got != status {
		t.Fatalf("unexpected status: %#v", got)
	}
}

func TestCapabilityStatusValidateRejectsTestedWithoutAdapter(t *testing.T) {
	status := CapabilityStatus{Tested: true}
	if err := status.Validate(); err == nil {
		t.Fatal("tested capability without an implemented adapter must be rejected")
	}
}
