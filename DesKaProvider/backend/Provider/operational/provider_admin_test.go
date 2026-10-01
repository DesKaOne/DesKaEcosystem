package operational

import (
	"errors"
	"sync"
	"testing"

	provider "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider"
	mock "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider/Mock"
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


func TestProviderAdminServiceControlsCapabilitiesIndependentlyOfLifecycle(t *testing.T) {
	store := NewProviderStateStore()
	state := ProviderState{
		ProviderName: "mock",
		Lifecycle: LifecycleEnabled,
		Capabilities: []Capability{CapabilityPPOB, CapabilityBalance},
	}
	if err := store.Put(state); err != nil { t.Fatal(err) }
	admin, err := NewProviderAdminService(store)
	if err != nil { t.Fatal(err) }

	updated, err := admin.DisableCapability("mock", CapabilityPPOB)
	if err != nil { t.Fatal(err) }
	if !updated.Enabled() { t.Fatal("capability mutation must not change provider lifecycle") }
	if updated.Supports(CapabilityPPOB) { t.Fatal("disabled capability must not remain operationally enabled") }
	if !updated.Supports(CapabilityBalance) { t.Fatal("unrelated capability must remain enabled") }

	updated, err = admin.EnableCapability("mock", CapabilityPPOB)
	if err != nil { t.Fatal(err) }
	if !updated.Enabled() || !updated.Supports(CapabilityPPOB) { t.Fatal("explicit capability enable must restore only the requested capability") }
}

func TestProviderAdminServiceConcurrentCapabilityMutationPreservesBothUpdates(t *testing.T) {
	store := NewProviderStateStore()
	if err := store.Put(ProviderState{
		ProviderName: "mock",
		Lifecycle: LifecycleEnabled,
		Capabilities: []Capability{CapabilityPPOB, CapabilityBalance},
		EnabledCapabilities: []Capability{CapabilityPPOB, CapabilityBalance},
	}); err != nil { t.Fatal(err) }
	admin, err := NewProviderAdminService(store)
	if err != nil { t.Fatal(err) }

	errCh := make(chan error, 2)
	start := make(chan struct{})
	go func() {
		<-start
		_, errCh <- admin.DisableCapability("mock", CapabilityPPOB)
	}()
	go func() {
		<-start
		_, errCh <- admin.DisableCapability("mock", CapabilityBalance)
	}()
	close(start)
	for i := 0; i < 2; i++ {
		if err := <-errCh; err != nil { t.Fatal(err) }
	}

	state, ok := store.Get("mock")
	if !ok { t.Fatal("provider state missing") }
	if state.Supports(CapabilityPPOB) || state.Supports(CapabilityBalance) {
		t.Fatalf("concurrent capability disables must preserve both updates: %#v", state.EnabledCapabilities)
	}
	if !state.Enabled() {
		t.Fatalf("capability mutation must not alter provider lifecycle: %#v", state)
	}
}

func TestProviderAdminServiceRejectsUnavailableCapability(t *testing.T) {
	store := NewProviderStateStore()
	if err := store.Put(ProviderState{ProviderName: "mock", Lifecycle: LifecycleEnabled, Capabilities: []Capability{CapabilityPPOB}}); err != nil { t.Fatal(err) }
	admin, err := NewProviderAdminService(store)
	if err != nil { t.Fatal(err) }
	if _, err := admin.EnableCapability("mock", CapabilityPayment); !errors.Is(err, ErrCapabilityNotAvailable) {
		t.Fatalf("expected ErrCapabilityNotAvailable, got %v", err)
	}
}


func TestProviderAdminServiceReconcilePreservesExplicitCapabilityDisable(t *testing.T) {
	store := NewProviderStateStore()
	state := ProviderState{
		ProviderName: "mock",
		Lifecycle: LifecycleEnabled,
		Capabilities: []Capability{CapabilityPPOB, CapabilityBalance},
		EnabledCapabilities: []Capability{CapabilityBalance},
	}
	if err := store.Put(state); err != nil { t.Fatal(err) }
	admin, err := NewProviderAdminService(store)
	if err != nil { t.Fatal(err) }
	descriptor := provider.CapabilityDescriptor{Capabilities: map[provider.Capability]provider.CapabilityStatus{
		provider.CapabilityPPOB: {AdapterImplemented: true, Enabled: true},
		provider.CapabilityBalance: {AdapterImplemented: true, Enabled: true},
	}}
	updated, err := admin.ReconcileCapabilityState("mock", providerRegistryForTest(descriptor))
	if err != nil { t.Fatal(err) }
	if updated.State.Supports(CapabilityPPOB) {
		t.Fatal("reconciliation must not re-enable an explicitly disabled capability")
	}
	if !updated.State.Supports(CapabilityBalance) {
		t.Fatal("reconciliation must preserve unrelated enabled capability")
	}
}

func providerRegistryForTest(descriptor provider.CapabilityDescriptor) *provider.Registry {
	r := provider.NewRegistry()
	_ = r.RegisterWithCapabilities("mock", mock.New(mock.Config{}), descriptor)
	return r
}
