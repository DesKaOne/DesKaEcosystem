package provider

// Capability identifies a provider-neutral capability that can be exposed to
// routing and operational state without leaking provider-specific protocols.
type Capability string

const (
	CapabilityPayment Capability = "payment"
	CapabilityPPOB    Capability = "ppob"
	CapabilityPayout  Capability = "payout"
	CapabilityBalance Capability = "balance"
	CapabilityWebhook Capability = "webhook"
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
