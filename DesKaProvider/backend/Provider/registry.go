package provider

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
)

var ErrProviderNotFound = errors.New("provider not found")

type registryEntry struct {
	provider PPOBProvider
	descriptor CapabilityDescriptor
}

type Registry struct {
	mu        sync.RWMutex
	providers map[string]registryEntry
}

func NewRegistry() *Registry {
	return &Registry{providers: make(map[string]registryEntry)}
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
	r.providers[key] = registryEntry{provider: provider}
	return nil
}

func (r *Registry) Get(name string) (PPOBProvider, error) {
	key := normalizeName(name)
	r.mu.RLock()
	entry, ok := r.providers[key]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrProviderNotFound, key)
	}
	return entry.provider, nil
}

func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func normalizeName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}


func (r *Registry) RegisterWithCapabilities(name string, provider PPOBProvider, descriptor CapabilityDescriptor) error {
	key := normalizeName(name)
	if key == "" {
		return errors.New("provider name is required")
	}
	if provider == nil {
		return errors.New("provider implementation is required")
	}
	if descriptor.Capabilities == nil {
		descriptor.Capabilities = make(map[Capability]CapabilityStatus)
	}
	copied := make(map[Capability]CapabilityStatus, len(descriptor.Capabilities))
	for capability, status := range descriptor.Capabilities {
		copied[capability] = status
	}
	descriptor.Capabilities = copied

	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.providers[key]; exists {
		return fmt.Errorf("provider %q is already registered", key)
	}
	r.providers[key] = registryEntry{provider: provider, descriptor: descriptor}
	return nil
}

func (r *Registry) Capabilities(name string) (CapabilityDescriptor, error) {
	key := normalizeName(name)
	r.mu.RLock()
	entry, ok := r.providers[key]
	r.mu.RUnlock()
	if !ok {
		return CapabilityDescriptor{}, fmt.Errorf("%w: %s", ErrProviderNotFound, key)
	}
	copied := make(map[Capability]CapabilityStatus, len(entry.descriptor.Capabilities))
	for capability, status := range entry.descriptor.Capabilities {
		copied[capability] = status
	}
	return CapabilityDescriptor{Capabilities: copied}, nil
}
