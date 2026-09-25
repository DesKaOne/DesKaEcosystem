package operational

import (
	"errors"
	"sync"
	"testing"
)

func TestProviderAdminServiceMutatesLifecycleOnly(t *testing.T) {
	store := NewProviderStateStore()
	original := ProviderState{
		ProviderName: "mock",
		Lifecycle:    LifecycleDisabled,
		Capabilities: []Capability{CapabilityWebhook, CapabilityPPOB, CapabilityBalance},
	}
	if err := store.Put(original); err != nil {
		t.Fatal(err)
	}

	admin, err := NewProviderAdminService(store)
	if err != nil {
		t.Fatal(err)
	}

	enabled, err := admin.Enable(" MOCK ")
	if err != nil {
		t.Fatal(err)
	}
	if !enabled.Enabled() {
		t.Fatal("provider should be enabled")
	}
	if enabled.ProviderName != "mock" {
		t.Fatalf("unexpected provider name: %q", enabled.ProviderName)
	}
	if !enabled.Supports(CapabilityPPOB) || !enabled.Supports(CapabilityBalance) || !enabled.Supports(CapabilityWebhook) {
		t.Fatalf("lifecycle mutation must preserve capabilities: %#v", enabled.Capabilities)
	}

	disabled, err := admin.Disable("mock")
	if err != nil {
		t.Fatal(err)
	}
	if disabled.Enabled() {
		t.Fatal("provider should be disabled")
	}
	if len(disabled.Capabilities) != len(original.Capabilities) {
		t.Fatalf("capabilities changed during lifecycle mutation: %#v", disabled.Capabilities)
	}
	for _, capability := range original.Capabilities {
		if !disabled.Supports(capability) {
			t.Fatalf("capability %q was lost", capability)
		}
	}
}

func TestProviderAdminServiceRejectsUnknownProvider(t *testing.T) {
	admin, err := NewProviderAdminService(NewProviderStateStore())
	if err != nil {
		t.Fatal(err)
	}

	if _, err := admin.Enable("missing"); !errors.Is(err, ErrProviderNotFound) {
		t.Fatalf("expected ErrProviderNotFound, got %v", err)
	}
}

func TestProviderAdminServiceRejectsInvalidLifecycle(t *testing.T) {
	store := NewProviderStateStore()
	if err := store.Put(ProviderState{ProviderName: "mock", Lifecycle: LifecycleDisabled}); err != nil {
		t.Fatal(err)
	}
	admin, err := NewProviderAdminService(store)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := admin.SetLifecycle("mock", Lifecycle("maintenance")); !errors.Is(err, ErrInvalidLifecycle) {
		t.Fatalf("expected ErrInvalidLifecycle, got %v", err)
	}
}

func TestProviderAdminServiceConcurrentLifecycleMutation(t *testing.T) {
	store := NewProviderStateStore()
	if err := store.Put(ProviderState{
		ProviderName: "mock",
		Lifecycle:    LifecycleDisabled,
		Capabilities: []Capability{CapabilityPPOB},
	}); err != nil {
		t.Fatal(err)
	}
	admin, err := NewProviderAdminService(store)
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if i%2 == 0 {
				_, _ = admin.Enable("mock")
				return
			}
			_, _ = admin.Disable("mock")
		}(i)
	}
	wg.Wait()

	state, ok := store.Get("mock")
	if !ok {
		t.Fatal("provider state disappeared")
	}
	if !state.Supports(CapabilityPPOB) {
		t.Fatalf("capability lost after concurrent lifecycle mutation: %#v", state.Capabilities)
	}
	if state.Lifecycle != LifecycleEnabled && state.Lifecycle != LifecycleDisabled {
		t.Fatalf("invalid lifecycle after concurrent mutation: %q", state.Lifecycle)
	}
}
