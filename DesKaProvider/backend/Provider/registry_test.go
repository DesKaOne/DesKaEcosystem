package provider

import (
	"context"
	
	payment "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/internal/Payment"
	"errors"
	"testing"
)

type registryTestProvider struct{}

func (registryTestProvider) GetProducts(context.Context, ProductRequest) ([]Product, error) { return nil, nil }
func (registryTestProvider) Inquiry(context.Context, InquiryRequest) (InquiryResult, error) { return InquiryResult{}, nil }
func (registryTestProvider) Purchase(context.Context, PurchaseRequest) (PurchaseResult, error) { return PurchaseResult{}, nil }
func (registryTestProvider) GetStatus(context.Context, StatusRequest) (PurchaseStatus, error) { return PurchaseStatus{}, nil }
func (registryTestProvider) HandleWebhook(context.Context, WebhookRequest) (WebhookEvent, error) { return WebhookEvent{}, nil }

func TestRegistryRegisterGetAndNames(t *testing.T) {
	r := NewRegistry()
	p := registryTestProvider{}
	if err := r.Register(" DigiFlazz ", p); err != nil {
		t.Fatal(err)
	}
	got, err := r.Get("digiflazz")
	if err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Fatal("expected registered provider")
	}
	if err := r.Register("DIGIFLAZZ", p); err == nil {
		t.Fatal("expected duplicate registration error")
	}
	if _, err := r.Get("unknown"); !errors.Is(err, ErrProviderNotFound) {
		t.Fatalf("expected ErrProviderNotFound, got %v", err)
	}
	if len(r.Names()) != 1 || r.Names()[0] != "digiflazz" {
		t.Fatalf("unexpected names: %v", r.Names())
	}
}


type registryTestPaymentProvider struct{}

func (registryTestPaymentProvider) CreatePayment(context.Context, payment.PaymentRequest) (payment.PaymentResult, error) {
	return payment.PaymentResult{}, nil
}
func (registryTestPaymentProvider) GetPaymentStatus(context.Context, payment.StatusRequest) (payment.StatusResult, error) {
	return payment.StatusResult{}, nil
}

func TestRegistryPaymentCapabilityIsExplicitAndTyped(t *testing.T) {
	r := NewRegistry()
	if err := r.RegisterCapabilityProvider("midtrans", CapabilityPayment, registryTestPaymentProvider{}, CapabilityStatus{
		AdapterImplemented: true, Tested: true, Enabled: false,
	}); err != nil {
		t.Fatal(err)
	}

	p, err := r.GetPaymentProvider("midtrans")
	if err != nil {
		t.Fatal(err)
	}
	if p == nil {
		t.Fatal("expected typed payment provider")
	}

	statuses, err := r.Capabilities("midtrans")
	if err != nil {
		t.Fatal(err)
	}
	status, ok := statuses.Status(CapabilityPayment)
	if !ok {
		t.Fatal("expected payment capability metadata")
	}
	if status.Enabled {
		t.Fatal("payment capability must remain disabled by default")
	}
	if status.State() != CapabilityDisabled {
		t.Fatalf("expected DISABLED state, got %s", status.State())
	}
}

func TestRegistryPaymentCapabilityRejectsWrongImplementation(t *testing.T) {
	r := NewRegistry()
	if err := r.RegisterCapabilityProvider("midtrans", CapabilityPayment, struct{}{}, CapabilityStatus{
		AdapterImplemented: true, Tested: true, Enabled: false,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := r.GetPaymentProvider("midtrans"); err == nil {
		t.Fatal("expected invalid payment contract error")
	}
}

func TestRegistryConfiguredCapabilityDoesNotAutoPromoteReadiness(t *testing.T) {
	r := NewRegistry()
	if err := r.RegisterCapabilityProvider("configured-only", CapabilityPPOB, registryTestProvider{}, CapabilityStatus{
		Verified: true,
		Configured: true,
		AdapterImplemented: true,
		Tested: true,
		Enabled: false,
		LiveTested: false,
	}); err != nil {
		t.Fatal(err)
	}

	statuses, err := r.Capabilities("configured-only")
	if err != nil {
		t.Fatal(err)
	}
	status, ok := statuses.Status(CapabilityPPOB)
	if !ok {
		t.Fatal("expected PPOB capability metadata")
	}
	if status.Enabled || status.LiveTested || status.ProductionReady {
		t.Fatalf("configuration must not auto-promote readiness: %#v", status)
	}
	if status.State() != CapabilityTested {
		t.Fatalf("expected TESTED state, got %s", status.State())
	}
}

func TestCapabilityStatusValidateRejectsInvalidReadinessPromotion(t *testing.T) {
	cases := []struct {
		name   string
		status CapabilityStatus
	}{
		{
			name: "enabled without implementation",
			status: CapabilityStatus{Enabled: true},
		},
		{
			name: "live-tested without tested and enabled",
			status: CapabilityStatus{AdapterImplemented: true, LiveTested: true},
		},
		{
			name: "production-ready without live validation",
			status: CapabilityStatus{
				Verified: true, Configured: true, AdapterImplemented: true,
				Tested: true, Enabled: true,
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.status.Validate(); err == nil {
				t.Fatal("expected readiness validation error")
			}
		})
	}
}
