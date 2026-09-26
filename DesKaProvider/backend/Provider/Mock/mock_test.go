package mock

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	provider "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider"
)

func TestProviderDeterministicPurchaseAndStatus(t *testing.T) {
	p := New(Config{
		Products:       []provider.Product{{Code: "xld10", Name: "XL 10GB"}},
		PurchaseStatus: provider.StatusPending,
		ProviderCode:   "MOCK_PENDING",
		Message:        "mock pending",
		Price:          10000,
	})

	req := provider.PurchaseRequest{
		ProductCode: "xld10",
		CustomerNo:  "087800001232",
		ReferenceID: "ref-1",
	}
	first, err := p.Purchase(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	second, err := p.Purchase(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatalf("expected deterministic results, first=%#v second=%#v", first, second)
	}

	status, err := p.GetStatus(context.Background(), provider.StatusRequest{
		ProductCode: "xld10",
		CustomerNo:  "087800001232",
		ReferenceID: "ref-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if status.Status != provider.StatusPending || status.ProviderCode != "MOCK_PENDING" {
		t.Fatalf("unexpected status: %#v", status)
	}
}

func TestProviderUnknownProductAndWebhook(t *testing.T) {
	p := New(Config{Products: []provider.Product{{Code: "xld10", Name: "XL 10GB"}}})

	inquiry, err := p.Inquiry(context.Background(), provider.InquiryRequest{ProductCode: "unknown"})
	if err != nil {
		t.Fatal(err)
	}
	if inquiry.Status != provider.StatusFailed || inquiry.ProviderCode != "PRODUCT_NOT_FOUND" {
		t.Fatalf("unexpected inquiry: %#v", inquiry)
	}

	_, err = p.Purchase(context.Background(), provider.PurchaseRequest{ProductCode: "unknown", ReferenceID: "ref-2"})
	if !errors.Is(err, ErrProductNotFound) {
		t.Fatalf("expected ErrProductNotFound, got %v", err)
	}

	payload, err := json.Marshal(provider.WebhookEvent{
		ReferenceID:  "ref-3",
		CustomerNo:   "087800001233",
		ProductCode:  "xld10",
		Status:       provider.StatusSuccess,
		ProviderCode: "00",
		Message:      "mock success",
		Price:        10000,
	})
	if err != nil {
		t.Fatal(err)
	}
	event, err := p.HandleWebhook(context.Background(), provider.WebhookRequest{Body: payload})
	if err != nil {
		t.Fatal(err)
	}
	if event.ReferenceID != "ref-3" || event.Status != provider.StatusSuccess {
		t.Fatalf("unexpected webhook event: %#v", event)
	}
}
