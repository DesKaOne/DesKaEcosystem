package rcb

import (
	"context"
	"errors"
	"testing"

	provider "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider"
)

func TestFoundationRejectsUnverifiedOperations(t *testing.T) {
	c := New()
	ctx := context.Background()

	if _, err := c.GetProducts(ctx, provider.ProductRequest{}); !errors.Is(err, ErrNotImplemented) { t.Fatalf("GetProducts error = %v", err) }
	if _, err := c.Inquiry(ctx, provider.InquiryRequest{}); !errors.Is(err, ErrNotImplemented) { t.Fatalf("Inquiry error = %v", err) }
	if _, err := c.Purchase(ctx, provider.PurchaseRequest{}); !errors.Is(err, ErrNotImplemented) { t.Fatalf("Purchase error = %v", err) }
	if _, err := c.GetStatus(ctx, provider.StatusRequest{}); !errors.Is(err, ErrNotImplemented) { t.Fatalf("GetStatus error = %v", err) }
	if _, err := c.HandleWebhook(ctx, provider.WebhookRequest{}); !errors.Is(err, ErrNotImplemented) { t.Fatalf("HandleWebhook error = %v", err) }
}

func TestFoundationDoesNotClaimCapabilityReadiness(t *testing.T) {
	c := New()
	var _ provider.PPOBProvider = c
}
