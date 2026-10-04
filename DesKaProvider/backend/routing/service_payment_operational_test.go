package routing

import (
	"context"
	"errors"
	"testing"

	provider "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider"
	"github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider/operational"
	payment "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/internal/Payment"
)

type operationalGatePaymentProvider struct {
	createCalls int
}

func (p *operationalGatePaymentProvider) CreatePayment(ctx context.Context, req payment.PaymentRequest) (payment.PaymentResult, error) {
	p.createCalls++
	return payment.PaymentResult{
		ReferenceID: req.ReferenceID,
		ProviderReference: "provider-ref",
		Status: payment.StatusPending,
		Amount: req.Amount,
		Currency: req.Currency,
	}, nil
}

func (p *operationalGatePaymentProvider) GetPaymentStatus(context.Context, payment.StatusRequest) (payment.StatusResult, error) {
	return payment.StatusResult{}, nil
}

func (p *operationalGatePaymentProvider) HandlePaymentWebhook(context.Context, []byte) (payment.StatusResult, error) {
	return payment.StatusResult{}, nil
}

func TestSubmitPaymentRejectsDisabledOperationalLifecycleBeforeProviderCall(t *testing.T) {
	registry := provider.NewRegistry()
	adapter := &operationalGatePaymentProvider{}
	status := provider.CapabilityStatus{
		Verified:          true,
		Configured:        true,
		AdapterImplemented: true,
		Tested:            true,
		Enabled:           true,
	}
	descriptor := provider.CapabilityDescriptor{
		Capabilities: map[provider.Capability]provider.CapabilityStatus{
			provider.CapabilityPayment: status,
		},
	}
	if err := registry.RegisterCapabilityProvider("midtrans", provider.CapabilityPayment, adapter, status); err != nil {
		t.Fatal(err)
	}

	operationalStore := operational.NewMemoryStore()
	router, err := New(registry, operationalStore, nil)
	if err != nil {
		t.Fatal(err)
	}
	stateStore := operational.NewProviderStateStore()
	state := operational.ProviderState{
		ProviderName:          "midtrans",
		Lifecycle:             operational.LifecycleDisabled,
		Capabilities:          []operational.Capability{operational.CapabilityPayment},
		EnabledCapabilities:  []operational.Capability{operational.CapabilityPayment},
		CapabilityFingerprint: operational.CapabilityMetadataFingerprint(descriptor),
	}
	if err := stateStore.Put(state); err != nil {
		t.Fatal(err)
	}
	router.ProviderState = stateStore

	service, err := NewService(router)
	if err != nil {
		t.Fatal(err)
	}

	_, err = service.SubmitPayment(context.Background(), "midtrans", payment.PaymentRequest{
		ReferenceID: "payment-disabled-1",
		Amount:      10000,
		Currency:    "IDR",
		CustomerID:  "customer-1",
		Description: "test payment",
	})
	if err == nil {
		t.Fatal("expected disabled operational lifecycle to block payment submission")
	}
	if !errors.Is(err, ErrPaymentCapabilityDisabled) {
		t.Fatalf("expected payment capability disabled error, got %v", err)
	}
	if adapter.createCalls != 0 {
		t.Fatalf("disabled operational lifecycle must block provider side effect, got %d CreatePayment calls", adapter.createCalls)
	}
}
