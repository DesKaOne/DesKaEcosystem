package operational

import (
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

type Capability string

const (
	CapabilityPayment  Capability = "payment"
	CapabilityPPOB     Capability = "ppob"
	CapabilityPayout   Capability = "payout"
	CapabilityBalance  Capability = "balance"
	CapabilityWebhook  Capability = "webhook"
)

type ProviderState struct {
	ProviderName string
	Lifecycle    Lifecycle
	Capabilities []Capability
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
	for _, value := range s.Capabilities {
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
	return state, ok
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
	s.states[state.ProviderName] = state
	return nil
}

func (s *ProviderStateStore) Put(state ProviderState) error {
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

	s.mu.Lock()
	defer s.mu.Unlock()
	state.ProviderName = strings.TrimSpace(strings.ToLower(state.ProviderName))
	capabilities := append([]Capability(nil), state.Capabilities...)
	sort.Slice(capabilities, func(i, j int) bool { return capabilities[i] < capabilities[j] })
	state.Capabilities = capabilities
	next := make(map[string]ProviderState, len(s.states)+1)
	for name, existing := range s.states {
		next[name] = existing
	}
	next[state.ProviderName] = state
	if s.persistence != nil {
		states := make([]ProviderState, 0, len(next))
		for _, value := range next {
			value.Capabilities = append([]Capability(nil), value.Capabilities...)
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

func (s *ProviderStateStore) allMemory() []ProviderState {
	return s.allMemory()
}

func (s *ProviderStateStore) All() []ProviderState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]ProviderState, 0, len(s.states))
	for _, state := range s.states {
		state.Capabilities = append([]Capability(nil), state.Capabilities...)
		result = append(result, state)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ProviderName < result[j].ProviderName })
	return result
}
