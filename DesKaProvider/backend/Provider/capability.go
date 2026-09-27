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
type CapabilityStatus struct {
	Verified          bool
	Configured        bool
	AdapterImplemented bool
	Enabled           bool
	LiveTested        bool
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
