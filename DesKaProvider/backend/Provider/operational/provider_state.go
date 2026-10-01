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

func cloneCapabilities(values []Capability) []Capability {
	if values == nil {
		return nil
	}
	return append([]Capability{}, values...)
}

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

var ErrProviderStatePersistenceAmbiguous = errors.New("provider state persistence outcome is ambiguous")

func wrapProviderStatePersistenceAmbiguous(err error) error {
	if err == nil {
		return nil
	}
	return errors.Join(ErrProviderStatePersistenceAmbiguous, err)
}

func safeStateAfterAmbiguousPersistence(current, requested ProviderState) ProviderState {
	// Ambiguous persistence must fail closed: lifecycle enablement and
	// capability enablement are never promoted from an uncertain write.
	result := current
	if requested.Lifecycle == LifecycleDisabled {
		result.Lifecycle = LifecycleDisabled
	}
	if current.EnabledCapabilities != nil && requested.EnabledCapabilities != nil {
		allowed := make(map[Capability]struct{}, len(requested.EnabledCapabilities))
		for _, capability := range requested.EnabledCapabilities {
			allowed[capability] = struct{}{}
		}
		filtered := make([]Capability, 0, len(current.EnabledCapabilities))
		for _, capability := range current.EnabledCapabilities {
			if _, ok := allowed[capability]; ok {
				filtered = append(filtered, capability)
			}
		}
		result.EnabledCapabilities = filtered
	} else if current.EnabledCapabilities == nil && requested.EnabledCapabilities != nil {
		// Legacy nil means all implemented capabilities. A requested explicit
		// set is never broader than that effective set, so retaining it is safe
		// even when the persistence outcome is uncertain.
		result.EnabledCapabilities = cloneCapabilities(requested.EnabledCapabilities)
	}
	return result
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
	state.Capabilities = cloneCapabilities(state.Capabilities)
	state.EnabledCapabilities = cloneCapabilities(state.EnabledCapabilities)
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
	capabilities := cloneCapabilities(state.Capabilities)
	sort.Slice(capabilities, func(i, j int) bool { return capabilities[i] < capabilities[j] })
	state.Capabilities = capabilities
	enabledCapabilities := cloneCapabilities(state.EnabledCapabilities)
	sort.Slice(enabledCapabilities, func(i, j int) bool { return enabledCapabilities[i] < enabledCapabilities[j] })
	state.EnabledCapabilities = enabledCapabilities
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
	capabilities := cloneCapabilities(state.Capabilities)
	sort.Slice(capabilities, func(i, j int) bool { return capabilities[i] < capabilities[j] })
	state.Capabilities = capabilities
	enabledCapabilities := cloneCapabilities(state.EnabledCapabilities)
	sort.Slice(enabledCapabilities, func(i, j int) bool { return enabledCapabilities[i] < enabledCapabilities[j] })
	state.EnabledCapabilities = enabledCapabilities

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
			value.Capabilities = cloneCapabilities(value.Capabilities)
			value.EnabledCapabilities = cloneCapabilities(value.EnabledCapabilities)
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



func (s *ProviderStateStore) SetLifecycle(name string, lifecycle Lifecycle) (ProviderState, error) {
	name = strings.TrimSpace(strings.ToLower(name))
	if name == "" {
		return ProviderState{}, ErrProviderNotFound
	}
	switch lifecycle {
	case LifecycleEnabled, LifecycleDisabled:
	default:
		return ProviderState{}, ErrInvalidLifecycle
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	current, ok := s.states[name]
	if !ok {
		return ProviderState{}, ErrProviderNotFound
	}
	updated := current
	updated.Lifecycle = lifecycle

	next := make(map[string]ProviderState, len(s.states))
	for providerName, state := range s.states {
		next[providerName] = state
	}
	next[name] = updated
	if s.persistence != nil {
		states := make([]ProviderState, 0, len(next))
		for _, state := range next {
			state.Capabilities = cloneCapabilities(state.Capabilities)
			state.EnabledCapabilities = cloneCapabilities(state.EnabledCapabilities)
			states = append(states, state)
		}
		sort.Slice(states, func(i, j int) bool { return states[i].ProviderName < states[j].ProviderName })
		if err := s.persistence.Save(states); err != nil {
			if errors.Is(err, ErrProviderStatePersistenceAmbiguous) {
				s.states[name] = safeStateAfterAmbiguousPersistence(current, updated)
			}
			return ProviderState{}, err
		}
	}
	s.states = next
	updated.Capabilities = cloneCapabilities(updated.Capabilities)
	updated.EnabledCapabilities = cloneCapabilities(updated.EnabledCapabilities)
	return updated, nil
}

func (s *ProviderStateStore) SetCapabilityEnabled(name string, capability Capability, enabled bool) (ProviderState, error) {
	name = strings.TrimSpace(strings.ToLower(name))
	if name == "" {
		return ProviderState{}, errors.New("provider name is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	current, ok := s.states[name]
	if !ok {
		return ProviderState{}, ErrProviderNotFound
	}
	implemented := false
	for _, value := range current.Capabilities {
		if value == capability {
			implemented = true
			break
		}
	}
	if !implemented {
		return ProviderState{}, ErrCapabilityNotAvailable
	}

	updated := current
	updated.Capabilities = cloneCapabilities(current.Capabilities)
	updated.EnabledCapabilities = cloneCapabilities(current.EnabledCapabilities)
	if updated.EnabledCapabilities == nil {
		updated.EnabledCapabilities = cloneCapabilities(updated.Capabilities)
	}
	result := updated.EnabledCapabilities[:0]
	for _, value := range updated.EnabledCapabilities {
		if value != capability {
			result = append(result, value)
		}
	}
	if enabled {
		result = append(result, capability)
	}
	updated.EnabledCapabilities = result
	sort.Slice(updated.EnabledCapabilities, func(i, j int) bool { return updated.EnabledCapabilities[i] < updated.EnabledCapabilities[j] })

	next := make(map[string]ProviderState, len(s.states))
	for providerName, state := range s.states {
		next[providerName] = state
	}
	next[name] = updated
	if s.persistence != nil {
		states := make([]ProviderState, 0, len(next))
		for _, state := range next {
			state.Capabilities = cloneCapabilities(state.Capabilities)
			state.EnabledCapabilities = cloneCapabilities(state.EnabledCapabilities)
			states = append(states, state)
		}
		sort.Slice(states, func(i, j int) bool { return states[i].ProviderName < states[j].ProviderName })
		if err := s.persistence.Save(states); err != nil {
			if errors.Is(err, ErrProviderStatePersistenceAmbiguous) {
				s.states[name] = safeStateAfterAmbiguousPersistence(current, updated)
			}
			return ProviderState{}, err
		}
	}
	s.states = next
	updated.EnabledCapabilities = cloneCapabilities(updated.EnabledCapabilities)
	updated.Capabilities = cloneCapabilities(updated.Capabilities)
	return updated, nil
}


func (s *ProviderStateStore) All() []ProviderState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]ProviderState, 0, len(s.states))
	for _, state := range s.states {
		state.Capabilities = cloneCapabilities(state.Capabilities)
		state.EnabledCapabilities = cloneCapabilities(state.EnabledCapabilities)
		result = append(result, state)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ProviderName < result[j].ProviderName })
	return result
}
