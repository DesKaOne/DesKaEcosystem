package operational

import (
	"errors"
	"strings"
)

var (
	ErrProviderNotFound = errors.New("provider not found")
	ErrInvalidLifecycle = errors.New("invalid provider lifecycle")
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
	return s.states.Get(name)
}

func (s *ProviderAdminService) Enable(name string) (ProviderState, error) {
	return s.SetLifecycle(name, LifecycleEnabled)
}

func (s *ProviderAdminService) Disable(name string) (ProviderState, error) {
	return s.SetLifecycle(name, LifecycleDisabled)
}
