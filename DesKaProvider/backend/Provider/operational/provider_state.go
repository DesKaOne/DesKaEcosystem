package operational

import (
	provider "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider"
	"errors"
	"sort"
	"strings"
	"sync"
)

type Lifecycle string

const (
	LifecycleDisabled Lifecycle = "disabled"
	LifecycleEnabled  Lifecycle = "enabled"
)

type Capability = provider.Capability

const (
	CapabilityPayment = provider.CapabilityPayment
	CapabilityPPOB    = provider.CapabilityPPOB
	CapabilityPayout  = provider.CapabilityPayout
	CapabilityBalance = provider.CapabilityBalance
	CapabilityWebhook = provider.CapabilityWebhook
	CapabilityCatalog = provider.CapabilityCatalog
)

type ProviderState struct {
	ProviderName string
	Lifecycle    Lifecycle
	Capabilities []Capability
	// EnabledCapabilities is the operational capability gate. A nil value is
	// retained for legacy state and is interpreted as Capabilities until runtime
	// performs the explicit migration against current registry metadata.
	EnabledCapabilities []Capability
	CapabilityFingerprint string
}

func NewProviderState(name string) (ProviderState, error) {
	name = strings.TrimSpace(strings.ToLower(name))
	if name == "" {
		return ProviderState{}, errors.New("provider name is required")
	}
	return ProviderState{
		ProviderName: name,
		Lifecycle:    LifecycleDisabled,
	}, nil
}

func (s ProviderState) Enabled() bool {
	return s.Lifecycle == LifecycleEnabled
}

func (s ProviderState) Supports(capability Capability) bool {
	capabilities := s.Capabilities
	if s.EnabledCapabilities != nil {
		capabilities = s.EnabledCapabilities
	}
	for _, value := range capabilities {
		if value == capability {
			return true
		}
	}
	return false
}

type ProviderStateStore struct {
	mu          sync.RWMutex
	states      map[string]ProviderState
	persistence ProviderStatePersistence
}

type ProviderStatePersistence interface {
	Load() ([]ProviderState, error)
	Save([]ProviderState) error
}

func NewPersistentProviderStateStore(persistence ProviderStatePersistence) (*ProviderStateStore, error) {
	if persistence == nil {
		return nil, errors.New("provider state persistence is required")
	}
	store := &ProviderStateStore{states: make(map[string]ProviderState), persistence: persistence}
	states, err := persistence.Load()
	if err != nil {
		return nil, err
	}
	for _, state := range states {
		if err := store.putMemory(state); err != nil {
			return nil, err
		}
	}
	return store, nil
}

func NewProviderStateStore() *ProviderStateStore {
	return &ProviderStateStore{states: make(map[string]ProviderState)}
}

func (s *ProviderStateStore) Get(name string) (ProviderState, bool) {
	name = strings.TrimSpace(strings.ToLower(name))
	s.mu.RLock()
	defer s.mu.RUnlock()
	state, ok := s.states[name]
	state.Capabilities = append([]Capability(nil), state.Capabilities...)
	state.EnabledCapabilities = append([]Capability(nil), state.EnabledCapabilities...)
	return state, ok
}

func (s *ProviderStateStore) Update(name string, mutate func(*ProviderState) error) (ProviderState, error) {
	if s == nil {
		return ProviderState{}, errors.New("provider state store is required")
	}
	name = strings.TrimSpace(strings.ToLower(name))
	if name == "" {
		return ProviderState{}, errors.New("provider name is required")
	}
	if mutate == nil {
		return ProviderState{}, errors.New("provider state mutation is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	current, ok := s.states[name]
	if !ok {
		return ProviderState{}, errors.New("provider not found")
	}
	current.Capabilities = append([]Capability(nil), current.Capabilities...)
	current.EnabledCapabilities = append([]Capability(nil), current.EnabledCapabilities...)
	if err := mutate(&current); err != nil {
		return ProviderState{}, err
	}
	current.ProviderName = name
	if err := validateAndNormalizeState(&current); err != nil {
		return ProviderState{}, err
	}

	next := make(map[string]ProviderState, len(s.states))
	for key, value := range s.states {
		value.Capabilities = append([]Capability(nil), value.Capabilities...)
		value.EnabledCapabilities = append([]Capability(nil), value.EnabledCapabilities...)
		next[key] = value
	}
	next[name] = current
	if s.persistence != nil {
		states := make([]ProviderState, 0, len(next))
		for _, value := range next {
			value.Capabilities = append([]Capability(nil), value.Capabilities...)
			value.EnabledCapabilities = append([]Capability(nil), value.EnabledCapabilities...)
			states = append(states, value)
		}
		sort.Slice(states, func(i, j int) bool { return states[i].ProviderName < states[j].ProviderName })
		if err := s.persistence.Save(states); err != nil {
			return ProviderState{}, err
		}
	}
	s.states = next
	return current, nil
}

func validateAndNormalizeState(state *ProviderState) error {
	if state == nil {
		return errors.New("provider state is required")
	}
	state.ProviderName = strings.TrimSpace(strings.ToLower(state.ProviderName))
	if state.ProviderName == "" {
		return errors.New("provider name is required")
	}
	switch state.Lifecycle {
	case LifecycleEnabled, LifecycleDisabled:
	default:
		return errors.New("invalid provider lifecycle")
	}
	sort.Slice(state.Capabilities, func(i, j int) bool { return state.Capabilities[i] < state.Capabilities[j] })
	sort.Slice(state.EnabledCapabilities, func(i, j int) bool { return state.EnabledCapabilities[i] < state.EnabledCapabilities[j] })
	return nil
}

func (s *ProviderStateStore) putMemory(state ProviderState) error {
	state.ProviderName = strings.TrimSpace(strings.ToLower(state.ProviderName))
	if state.ProviderName == "" {
		return errors.New("provider name is required")
	}
	switch state.Lifecycle {
	case LifecycleEnabled, LifecycleDisabled:
	default:
		return errors.New("invalid provider lifecycle")
	}
	capabilities := append([]Capability(nil), state.Capabilities...)
	sort.Slice(capabilities, func(i, j int) bool { return capabilities[i] < capabilities[j] })
	state.Capabilities = capabilities
	enabledCapabilities := append([]Capability(nil), state.EnabledCapabilities...)
	sort.Slice(enabledCapabilities, func(i, j int) bool { return enabledCapabilities[i] < enabledCapabilities[j] })
	state.EnabledCapabilities = enabledCapabilities
	s.states[state.ProviderName] = state
	return nil
}

func (s *ProviderStateStore) Put(state ProviderState) error {
	state.ProviderName = strings.TrimSpace(strings.ToLower(state.ProviderName))
	if err := validateAndNormalizeState(&state); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	next := make(map[string]ProviderState, len(s.states)+1)
	for name, existing := range s.states {
		next[name] = existing
	}
	next[state.ProviderName] = state
	if s.persistence != nil {
		states := make([]ProviderState, 0, len(next))
		for _, value := range next {
			value.Capabilities = append([]Capability(nil), value.Capabilities...)
			value.EnabledCapabilities = append([]Capability(nil), value.EnabledCapabilities...)
			states = append(states, value)
		}
		sort.Slice(states, func(i, j int) bool { return states[i].ProviderName < states[j].ProviderName })
		if err := s.persistence.Save(states); err != nil {
			return err
		}
	}
	s.states = next
	return nil
}
