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

func TestAllCapabilitiesIsCanonicalAndDefensive(t *testing.T) {
	got := AllCapabilities()
	want := []Capability{CapabilityPayment, CapabilityPPOB, CapabilityPayout, CapabilityBalance, CapabilityWebhook, CapabilityCatalog}
	if len(got) != len(want) {
		t.Fatalf("unexpected capability count: got=%d want=%d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("capability[%d] = %q, want %q", i, got[i], want[i])
		}
	}
	got[0] = "mutated"
	if AllCapabilities()[0] != CapabilityPayment {
		t.Fatal("AllCapabilities must return a defensive copy")
	}
}

func TestCapabilityStatusProductionReadyDoesNotImplyLiveValidation(t *testing.T) {
	status := CapabilityStatus{AdapterImplemented: true, Enabled: true, ProductionReady: true}
	if status.State() != CapabilityImplemented {
		t.Fatalf("production readiness must not imply live validation, got %q", status.State())
	}
}

func TestCapabilityMatrixIsProviderNeutralSnapshot(t *testing.T) {
	r := NewRegistry()
	if err := r.RegisterWithCapabilities("Midtrans", registryTestProvider{}, CapabilityDescriptor{
		Capabilities: map[Capability]CapabilityStatus{
			CapabilityPayment: {AdapterImplemented: true, Tested: true, Enabled: false},
			CapabilityWebhook: {AdapterImplemented: true, Tested: true, Enabled: false},
		},
	}); err != nil {
		t.Fatal(err)
	}
	matrix := r.CapabilityMatrix()
	status, ok := matrix.Status("midtrans", CapabilityPayment)
	if !ok || !status.AdapterImplemented || !status.Tested || status.Enabled {
		t.Fatalf("unexpected matrix status: %#v ok=%v", status, ok)
	}
	status.Enabled = true
	again, _ := r.CapabilityMatrix().Status("midtrans", CapabilityPayment)
	if again.Enabled {
		t.Fatal("matrix must be a defensive snapshot")
	}
}
