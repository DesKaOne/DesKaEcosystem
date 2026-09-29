package operational

import (
	"testing"

	provider "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider"
)

func TestCapabilityMetadataFingerprintIsDeterministic(t *testing.T) {
	descriptorA := provider.CapabilityDescriptor{Capabilities: map[provider.Capability]provider.CapabilityStatus{
		provider.CapabilityWebhook: {AdapterImplemented: true, Tested: true},
		provider.CapabilityPPOB:    {AdapterImplemented: true, Tested: true},
	}}
	descriptorB := provider.CapabilityDescriptor{Capabilities: map[provider.Capability]provider.CapabilityStatus{
		provider.CapabilityPPOB:    {AdapterImplemented: true, Tested: true},
		provider.CapabilityWebhook: {AdapterImplemented: true, Tested: true},
	}}
	if got, want := CapabilityMetadataFingerprint(descriptorA), CapabilityMetadataFingerprint(descriptorB); got != want {
		t.Fatalf("fingerprint must ignore map iteration order: %q != %q", got, want)
	}
}

func TestDetectCapabilityDriftReportsAddedAndRemovedCapabilities(t *testing.T) {
	state := ProviderState{
		ProviderName: "mock",
		Capabilities: []Capability{CapabilityPPOB, CapabilityWebhook},
	}
	descriptor := provider.CapabilityDescriptor{Capabilities: map[provider.Capability]provider.CapabilityStatus{
		provider.CapabilityPPOB:    {AdapterImplemented: true},
		provider.CapabilityBalance: {AdapterImplemented: true},
	}}
	drift := DetectCapabilityDrift(state, descriptor)
	if !drift.Drifted() {
		t.Fatal("expected capability drift")
	}
	if len(drift.Added) != 1 || drift.Added[0] != CapabilityBalance {
		t.Fatalf("unexpected added capabilities: %#v", drift.Added)
	}
	if len(drift.Removed) != 1 || drift.Removed[0] != CapabilityWebhook {
		t.Fatalf("unexpected removed capabilities: %#v", drift.Removed)
	}
}

func TestDetectCapabilityDriftReportsMetadataChanges(t *testing.T) {
	descriptor := provider.CapabilityDescriptor{Capabilities: map[provider.Capability]provider.CapabilityStatus{
		provider.CapabilityPPOB: {AdapterImplemented: true, Tested: true, Enabled: false},
	}}
	state := ProviderState{
		ProviderName:          "mock",
		Capabilities:          []Capability{CapabilityPPOB},
		CapabilityFingerprint: CapabilityMetadataFingerprint(descriptor),
	}
	changed := descriptor
	status := changed.Capabilities[provider.CapabilityPPOB]
	status.Enabled = true
	changed.Capabilities[provider.CapabilityPPOB] = status

	drift := DetectCapabilityDrift(state, changed)
	if !drift.Drifted() || !drift.MetadataChanged {
		t.Fatalf("expected metadata drift, got %#v", drift)
	}
}

func TestDetectCapabilityDriftLegacyStateWithoutFingerprintUsesCapabilitySet(t *testing.T) {
	descriptor := provider.CapabilityDescriptor{Capabilities: map[provider.Capability]provider.CapabilityStatus{
		provider.CapabilityPPOB: {AdapterImplemented: true},
	}}
	state := ProviderState{ProviderName: "mock", Capabilities: []Capability{CapabilityPPOB}}
	drift := DetectCapabilityDrift(state, descriptor)
	if drift.Drifted() {
		t.Fatalf("matching legacy capability set should migrate without drift: %#v", drift)
	}
}
