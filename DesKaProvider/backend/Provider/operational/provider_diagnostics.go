package operational

import (
	"sort"
	"errors"
	"strings"

	provider "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider"
)

// ProviderDiagnostic is a provider-neutral administrative snapshot. It is
// observational only and never authorizes payment, purchase, or routing.
type ProviderDiagnostic struct {
	State ProviderState
	Drift CapabilityDrift
	Drifted bool
}

// Diagnose returns the current persisted operational state alongside
// deterministic registry capability drift. It does not mutate either source.
func (s *ProviderAdminService) Diagnose(name string, registry *provider.Registry) (ProviderDiagnostic, error) {
	if s == nil || s.states == nil {
		return ProviderDiagnostic{}, errors.New("provider state store is required")
	}
	if registry == nil {
		return ProviderDiagnostic{}, errors.New("provider registry is required")
	}
	name = strings.TrimSpace(strings.ToLower(name))
	if name == "" {
		return ProviderDiagnostic{}, ErrProviderNotFound
	}
	state, ok := s.states.Get(name)
	if !ok {
		return ProviderDiagnostic{}, ErrProviderNotFound
	}
	descriptor, err := registry.Capabilities(name)
	if err != nil {
		return ProviderDiagnostic{}, err
	}
	drift := DetectCapabilityDrift(state, descriptor)
	return ProviderDiagnostic{State: state, Drift: drift, Drifted: drift.Drifted()}, nil
}

// DiagnoseAll returns deterministic diagnostics for all persisted providers.
// The returned slice is independent from the underlying state store.
func (s *ProviderAdminService) DiagnoseAll(registry *provider.Registry) ([]ProviderDiagnostic, error) {
	if s == nil || s.states == nil {
		return nil, errors.New("provider state store is required")
	}
	if registry == nil {
		return nil, errors.New("provider registry is required")
	}
	states := s.states.All()
	result := make([]ProviderDiagnostic, 0, len(states))
	for _, state := range states {
		diagnostic, err := s.Diagnose(state.ProviderName, registry)
		if err != nil {
			return nil, err
		}
		result = append(result, diagnostic)
	}
	return result, nil
}

// ReconcileCapabilityState explicitly synchronizes persisted capability
// membership/fingerprint with registry metadata. Drift never enables a
// provider; an existing enabled lifecycle is disabled until an operator
// explicitly re-enables it after reconciliation.
func (s *ProviderAdminService) ReconcileCapabilityState(name string, registry *provider.Registry) (ProviderDiagnostic, error) {
	if s == nil || s.states == nil {
		return ProviderDiagnostic{}, errors.New("provider state store is required")
	}
	if registry == nil {
		return ProviderDiagnostic{}, errors.New("provider registry is required")
	}
	name = strings.TrimSpace(strings.ToLower(name))
	if name == "" {
		return ProviderDiagnostic{}, ErrProviderNotFound
	}
	if _, ok := s.states.Get(name); !ok {
		return ProviderDiagnostic{}, ErrProviderNotFound
	}
	descriptor, err := registry.Capabilities(name)
	if err != nil {
		return ProviderDiagnostic{}, err
	}
	_, err = s.states.Update(name, func(state *ProviderState) error {
		drift := DetectCapabilityDrift(*state, descriptor)
		if drift.Drifted() {
			state.Lifecycle = LifecycleDisabled
		}
		state.Capabilities = capabilitiesFromDescriptor(descriptor)
		state.CapabilityFingerprint = CapabilityMetadataFingerprint(descriptor)
		return nil
	})
	if err != nil {
		return ProviderDiagnostic{}, err
	}
	return s.Diagnose(name, registry)
}

func capabilitiesFromDescriptor(descriptor provider.CapabilityDescriptor) []Capability {
	result := make([]Capability, 0, len(descriptor.Capabilities))
	for capability, status := range descriptor.Capabilities {
		if status.AdapterImplemented {
			result = append(result, capability)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}
