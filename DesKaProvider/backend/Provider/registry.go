package provider

import (
	"errors"
	"fmt"
	"strings"
	"sync"
)

var ErrProviderNotFound = errors.New("provider not found")

type Registry struct {
	mu        sync.RWMutex
	providers map[string]PPOBProvider
}

func NewRegistry() *Registry {
	return &Registry{providers: make(map[string]PPOBProvider)}
}

func (r *Registry) Register(name string, provider PPOBProvider) error {
	key := normalizeName(name)
	if key == "" {
		return errors.New("provider name is required")
	}
	if provider == nil {
		return errors.New("provider implementation is required")
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.providers[key]; exists {
		return fmt.Errorf("provider %q is already registered", key)
	}
	r.providers[key] = provider
	return nil
}

func (r *Registry) Get(name string) (PPOBProvider, error) {
	key := normalizeName(name)
	r.mu.RLock()
	provider, ok := r.providers[key]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrProviderNotFound, key)
	}
	return provider, nil
}

func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}
	return names
}

func normalizeName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}
