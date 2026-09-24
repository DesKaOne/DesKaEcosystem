package provider

import (
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
