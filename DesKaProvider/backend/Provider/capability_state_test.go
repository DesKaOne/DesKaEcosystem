package provider

import "testing"

func TestCapabilityStatusCanonicalState(t *testing.T) {
	cases := []struct {
		name   string
		status CapabilityStatus
		want   CapabilityState
	}{
		{"not implemented", CapabilityStatus{}, CapabilityNotImplemented},
		{"implemented but disabled", CapabilityStatus{AdapterImplemented: true, Enabled: false}, CapabilityDisabled},
		{"implemented", CapabilityStatus{AdapterImplemented: true, Enabled: true}, CapabilityImplemented},
		{"tested", CapabilityStatus{AdapterImplemented: true, Tested: true, Enabled: true}, CapabilityTested},
		{"live validated", CapabilityStatus{AdapterImplemented: true, Tested: true, Enabled: true, LiveTested: true}, CapabilityLiveValidated},
		{"live validated implies tested", CapabilityStatus{AdapterImplemented: true, Enabled: true, LiveTested: true}, CapabilityLiveValidated},
		{"disabled wins over stale live flag", CapabilityStatus{AdapterImplemented: true, Enabled: false, LiveTested: true}, CapabilityDisabled},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.status.State(); got != tc.want {
				t.Fatalf("State() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestCapabilityStatusVerificationDoesNotImplyImplementation(t *testing.T) {
	status := CapabilityStatus{Verified: true, Configured: true, Enabled: true}
	if got := status.State(); got != CapabilityNotImplemented {
		t.Fatalf("verification/configuration must not imply implementation, got %q", got)
	}
}

func TestCapabilityDescriptorSupportsRemainsSeparateFromState(t *testing.T) {
	d := CapabilityDescriptor{Capabilities: map[Capability]CapabilityStatus{
		CapabilityPPOB: {AdapterImplemented: true, Tested: true, Enabled: true},
	}}
	status, ok := d.Status(CapabilityPPOB)
	if !ok {
		t.Fatal("expected PPOB capability status")
	}
	if status.State() != CapabilityTested {
		t.Fatalf("unexpected canonical state: %q", status.State())
	}
	if !d.Supports(CapabilityPPOB) {
		t.Fatal("tested and enabled capability should remain routable")
	}
}
