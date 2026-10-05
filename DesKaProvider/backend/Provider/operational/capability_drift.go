package operational

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"

	provider "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider"
)

// CapabilityDrift describes deterministic divergence between persisted
// operational capability state and the current registry metadata.
type CapabilityDrift struct {
	ProviderName          string
	Added                 []Capability
	Removed               []Capability
	MetadataChanged       bool
	FingerprintUnavailable bool
}

// Drifted reports whether the persisted state must not be treated as
// synchronized with the current registry capability metadata.
func (d CapabilityDrift) Drifted() bool {
	return len(d.Added) > 0 || len(d.Removed) > 0 || d.MetadataChanged || d.FingerprintUnavailable
}

// CapabilityMetadataFingerprint returns a deterministic fingerprint of the
// complete provider-neutral capability metadata. Map iteration order is never
// part of the fingerprint.
func CapabilityMetadataFingerprint(descriptor provider.CapabilityDescriptor) string {
	capabilities := make([]provider.Capability, 0, len(descriptor.Capabilities))
	for capability := range descriptor.Capabilities {
		capabilities = append(capabilities, capability)
	}
	sort.Slice(capabilities, func(i, j int) bool { return capabilities[i] < capabilities[j] })

	h := sha256.New()
	for _, capability := range capabilities {
		status := descriptor.Capabilities[capability]
		fmt.Fprintf(h, "%s|%t|%t|%t|%t|%t|%t|%t\n",
			capability,
			status.Verified,
			status.Configured,
			status.AdapterImplemented,
			status.Tested,
			status.Enabled,
			status.LiveTested,
			status.ProductionReady,
		)
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

// DetectCapabilityDrift compares the persisted operational capability set and
// fingerprint with the current registry metadata. It does not mutate state.
func DetectCapabilityDrift(state ProviderState, descriptor provider.CapabilityDescriptor) CapabilityDrift {
	drift := CapabilityDrift{ProviderName: state.ProviderName}
	current := make(map[Capability]struct{}, len(descriptor.Capabilities))
	for capability, status := range descriptor.Capabilities {
		if status.AdapterImplemented {
			current[capability] = struct{}{}
		}
	}

	persisted := make(map[Capability]struct{}, len(state.Capabilities))
	for _, capability := range state.Capabilities {
		persisted[capability] = struct{}{}
	}

	for capability := range current {
		if _, ok := persisted[capability]; !ok {
			drift.Added = append(drift.Added, capability)
		}
	}
	for capability := range persisted {
		if _, ok := current[capability]; !ok {
			drift.Removed = append(drift.Removed, capability)
		}
	}
	sort.Slice(drift.Added, func(i, j int) bool { return drift.Added[i] < drift.Added[j] })
	sort.Slice(drift.Removed, func(i, j int) bool { return drift.Removed[i] < drift.Removed[j] })

	if strings.TrimSpace(state.CapabilityFingerprint) == "" {
		// Legacy state can still be classified safely from the persisted
		// capability set. The first synchronized runtime writes a fingerprint
		// so subsequent metadata changes become observable.
		drift.FingerprintUnavailable = len(drift.Added) > 0 || len(drift.Removed) > 0
	} else {
		drift.MetadataChanged = state.CapabilityFingerprint != CapabilityMetadataFingerprint(descriptor)
	}
	return drift
}
