package provider

import "errors"

// Capability identifies a provider-neutral capability that can be exposed to
// routing and operational state without leaking provider-specific protocols.
type Capability string

const (
	CapabilityPayment Capability = "payment"
	CapabilityPPOB    Capability = "ppob"
	CapabilityPayout  Capability = "payout"
	CapabilityBalance Capability = "balance"
	CapabilityWebhook Capability = "webhook"
	CapabilityCatalog Capability = "catalog"
)

// CapabilityStatus deliberately separates commercial/provider verification,
// runtime configuration, adapter implementation, operational enablement, and
// live validation. A capability must not be treated as transaction authority
// merely because one of these flags is true.
type CapabilityState string

const (
	CapabilityNotImplemented CapabilityState = "NOT_IMPLEMENTED"
	CapabilityImplemented    CapabilityState = "IMPLEMENTED"
	CapabilityTested         CapabilityState = "TESTED"
	CapabilityLiveValidated  CapabilityState = "LIVE_VALIDATED"
	CapabilityDisabled       CapabilityState = "DISABLED"
)

type CapabilityStatus struct {
	Verified           bool
	Configured         bool
	AdapterImplemented bool
	Tested             bool
	Enabled            bool
	LiveTested         bool
	ProductionReady    bool
}

// State returns the canonical externally-observable capability state.
//
// The state deliberately does not collapse provider verification/configuration
// into implementation readiness. Disabled is terminal for routing eligibility,
// while live validation is only reached after the adapter is implemented,
// tested, enabled, and actually validated against the provider.
func (s CapabilityStatus) State() CapabilityState {
	if !s.AdapterImplemented {
		return CapabilityNotImplemented
	}
	if !s.Enabled {
		return CapabilityDisabled
	}
	if s.LiveTested {
		return CapabilityLiveValidated
	}
	if s.Tested {
		return CapabilityTested
	}
	return CapabilityImplemented
}

// CapabilityDescriptor is the provider-neutral registry metadata for one
// provider. Missing capabilities are intentionally not inferred from the
// PPOBProvider interface.
type CapabilityDescriptor struct {
	Capabilities map[Capability]CapabilityStatus
}

func (d CapabilityDescriptor) Supports(capability Capability) bool {
	status, ok := d.Capabilities[capability]
	return ok && status.Enabled && status.AdapterImplemented
}

func (d CapabilityDescriptor) Status(capability Capability) (CapabilityStatus, bool) {
	status, ok := d.Capabilities[capability]
	return status, ok
}

var canonicalCapabilities = []Capability{
	CapabilityPayment,
	CapabilityPPOB,
	CapabilityPayout,
	CapabilityBalance,
	CapabilityWebhook,
	CapabilityCatalog,
}

// AllCapabilities returns the provider-neutral capability vocabulary used by
// the registry and capability matrix. Provider-specific protocols are not part
// of this vocabulary.
func AllCapabilities() []Capability {
	out := make([]Capability, len(canonicalCapabilities))
	copy(out, canonicalCapabilities)
	return out
}

// CapabilityMatrix is a provider-neutral snapshot of capability metadata.
// It deliberately contains no provider protocol, endpoint, credential, or
// provider-specific status code.
type CapabilityMatrix struct {
	Providers map[string]CapabilityDescriptor
}

func (m CapabilityMatrix) Status(providerName string, capability Capability) (CapabilityStatus, bool) {
	d, ok := m.Providers[normalizeName(providerName)]
	if !ok {
		return CapabilityStatus{}, false
	}
	return d.Status(capability)
}


// Validate enforces the monotonic readiness invariants used by registry
// metadata. Commercial verification, configuration, implementation, tests,
// enablement, live validation, and production readiness remain distinct.
func (s CapabilityStatus) Validate() error {
	if s.Enabled && !s.AdapterImplemented {
		return errors.New("enabled capability must be implemented")
	}
	if s.LiveTested && (!s.AdapterImplemented || !s.Tested || !s.Enabled) {
		return errors.New("live-tested capability must be implemented, tested, and enabled")
	}
	if s.ProductionReady && (!s.Verified || !s.Configured || !s.AdapterImplemented || !s.Tested || !s.Enabled || !s.LiveTested) {
		return errors.New("production-ready capability must be verified, configured, implemented, tested, enabled, and live-tested")
	}
	return nil
}
