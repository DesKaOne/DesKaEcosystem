package operational

import (
	"errors"
	"sort"
	"strings"

	provider "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider"
)

var (
	ErrProviderNotFound = errors.New("provider not found")
	ErrInvalidLifecycle = errors.New("invalid provider lifecycle")
	ErrCapabilityNotAvailable = errors.New("provider capability is not available")
)

// ProviderAdminService is the internal control-plane boundary for provider lifecycle changes.
// It deliberately does not mutate provider capabilities or health state.
type ProviderAdminService struct {
	states *ProviderStateStore
}

func NewProviderAdminService(states *ProviderStateStore) (*ProviderAdminService, error) {
	if states == nil {
		return nil, errors.New("provider state store is required")
	}
	return &ProviderAdminService{states: states}, nil
}

func (s *ProviderAdminService) SetLifecycle(name string, lifecycle Lifecycle) (ProviderState, error) {
	if s == nil || s.states == nil {
		return ProviderState{}, errors.New("provider state store is required")
	}
	switch lifecycle {
	case LifecycleEnabled, LifecycleDisabled:
	default:
		return ProviderState{}, ErrInvalidLifecycle
	}

	name = strings.TrimSpace(strings.ToLower(name))
	if name == "" {
		return ProviderState{}, ErrProviderNotFound
	}

	state, ok := s.states.Get(name)
	if !ok {
		return ProviderState{}, ErrProviderNotFound
	}
	state.Lifecycle = lifecycle
	if err := s.states.Put(state); err != nil {
		return ProviderState{}, err
	}
	updated, _ := s.states.Get(name)
	return updated, nil
}

func (s *ProviderAdminService) Enable(name string) (ProviderState, error) {
	return s.SetLifecycle(name, LifecycleEnabled)
}

func (s *ProviderAdminService) Disable(name string) (ProviderState, error) {
	return s.SetLifecycle(name, LifecycleDisabled)
}


func (s *ProviderAdminService) SetCapabilityEnabled(name string, capability Capability, enabled bool) (ProviderState, error) {
	if s == nil || s.states == nil {
		return ProviderState{}, errors.New("provider state store is required")
	}
	state, err := s.states.SetCapabilityEnabled(name, capability, enabled)
	if err != nil {
		if strings.TrimSpace(strings.ToLower(name)) == "" {
			return ProviderState{}, ErrProviderNotFound
		}
		if errors.Is(err, errors.New("provider not found")) {
			return ProviderState{}, ErrProviderNotFound
		}
		if errors.Is(err, errors.New("provider capability is not available")) {
			return ProviderState{}, ErrCapabilityNotAvailable
		}
		return ProviderState{}, err
	}
	return state, nil
}

func (s *ProviderAdminService) EnableCapability(name string, capability Capability) (ProviderState, error) {
	return s.SetCapabilityEnabled(name, capability, true)
}

func (s *ProviderAdminService) DisableCapability(name string, capability Capability) (ProviderState, error) {
	return s.SetCapabilityEnabled(name, capability, false)
}


func reconcileEnabledCapabilities(previous []Capability, descriptor provider.CapabilityDescriptor) []Capability {
	previousSet := make(map[Capability]struct{}, len(previous))
	for _, capability := range previous {
		previousSet[capability] = struct{}{}
	}
	result := make([]Capability, 0, len(descriptor.Capabilities))
	for capability, status := range descriptor.Capabilities {
		if !status.AdapterImplemented {
			continue
		}
		if _, ok := previousSet[capability]; ok {
			result = append(result, capability)
		}
	}
	if previous == nil {
		result = result[:0]
		for capability, status := range descriptor.Capabilities {
			if status.AdapterImplemented {
				result = append(result, capability)
			}
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}
