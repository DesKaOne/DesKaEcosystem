package provider

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"

	payment "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/internal/Payment"
)

var ErrProviderNotFound = errors.New("provider not found")

type registryEntry struct {
	provider             PPOBProvider
	capabilityProviders  map[Capability]any
	descriptor            CapabilityDescriptor
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
	r.providers[key] = registryEntry{provider: provider, capabilityProviders: make(map[Capability]any)}
	return nil
}

func (r *Registry) Get(name string) (PPOBProvider, error) {
	key := normalizeName(name)
	r.mu.RLock()
	entry, ok := r.providers[key]
	r.mu.RUnlock()
	if !ok || entry.provider == nil {
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
	r.providers[key] = registryEntry{provider: provider, capabilityProviders: make(map[Capability]any), descriptor: descriptor}
	return nil
}

// RegisterCapabilityProvider records a provider implementation for a single
// optional capability without requiring the provider to implement PPOBProvider.
// This is used for partial capability providers such as a balance-only adapter.
func (r *Registry) RegisterCapabilityProvider(name string, capability Capability, implementation any, status CapabilityStatus) error {
 key := normalizeName(name)
 if key == "" { return errors.New("provider name is required") }
 if implementation == nil { return errors.New("capability implementation is required") }
 if !status.AdapterImplemented { return errors.New("capability implementation must be marked implemented") }
 r.mu.Lock()
 defer r.mu.Unlock()
 entry, exists := r.providers[key]
 if !exists { entry = registryEntry{capabilityProviders: make(map[Capability]any)} }
 if entry.capabilityProviders == nil { entry.capabilityProviders = make(map[Capability]any) }
 if _, exists := entry.capabilityProviders[capability]; exists { return fmt.Errorf("provider %q capability %q is already registered", key, capability) }
 entry.capabilityProviders[capability] = implementation
 if entry.descriptor.Capabilities == nil { entry.descriptor.Capabilities = make(map[Capability]CapabilityStatus) }
 entry.descriptor.Capabilities[capability] = status
 r.providers[key] = entry
 return nil
}

// GetCapabilityProvider returns the implementation registered for one optional capability.
// GetPaymentProvider returns the provider-neutral payment implementation only when
// it has been explicitly registered. Capability status remains a separate routing
// gate; this accessor does not enable or authorize payment execution.
func (r *Registry) GetPaymentProvider(name string) (payment.Provider, error) {
	implementation, err := r.GetCapabilityProvider(name, CapabilityPayment)
	if err != nil {
		return nil, err
	}
	p, ok := implementation.(payment.Provider)
	if !ok {
		return nil, fmt.Errorf("provider %q payment implementation has invalid contract", normalizeName(name))
	}
	return p, nil
}

// GetPaymentWebhookProvider returns the optional provider-neutral payment
// webhook implementation registered for a provider. Capability status remains
// a separate routing gate; this accessor never enables payment processing.
func (r *Registry) GetPaymentWebhookProvider(name string) (payment.WebhookProvider, error) {
	implementation, err := r.GetCapabilityProvider(name, CapabilityPayment)
	if err != nil {
		return nil, err
	}
	p, ok := implementation.(payment.WebhookProvider)
	if !ok {
		return nil, fmt.Errorf("provider %q payment webhook implementation has invalid contract", normalizeName(name))
	}
	return p, nil
}

func (r *Registry) GetCapabilityProvider(name string, capability Capability) (any, error) {
 key := normalizeName(name)
 r.mu.RLock()
 entry, ok := r.providers[key]
 impl, capabilityOK := entry.capabilityProviders[capability]
 r.mu.RUnlock()
 if !ok { return nil, fmt.Errorf("%w: %s", ErrProviderNotFound, key) }
 if !capabilityOK { return nil, fmt.Errorf("%w: %s capability %s", ErrProviderNotFound, key, capability) }
 return impl, nil
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
