package midtrans

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/DesKaOne/DesKaEcosystem/DesKaProvider/config"
	"github.com/DesKaOne/DesKaEcosystem/DesKaProvider/integration"
	payment "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/internal/Payment"
)

func TestSandboxIntegrationPaymentLifecycle(t *testing.T) {
	if !integration.Enabled("midtrans") {
		t.Skip("Midtrans sandbox integration disabled; set DESKAPROVIDER_LIVE_INTEGRATION=1 and DESKAPROVIDER_LIVE_INTEGRATION_PROVIDER=midtrans")
	}

	cfg, err := config.LoadMidtransConfig()
	if err != nil {
		t.Fatalf("LoadMidtransConfig: %v", err)
	}
	if err := integration.ValidateEndpoints(cfg.SnapEndpoint, cfg.APIEndpoint); err != nil {
		t.Fatalf("Midtrans sandbox endpoint validation: %v", err)
	}

	orderID := strings.TrimSpace(os.Getenv("MIDTRANS_INTEGRATION_ORDER_ID"))
	if orderID == "" {
		orderID = "deskaprovider-sandbox-" + time.Now().UTC().Format("20060102T150405.000000000Z")
	}
	amount := int64(10000)
	client, err := New(cfg, nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	created, err := client.CreatePayment(ctx, payment.PaymentRequest{
		ReferenceID: orderID,
		Amount:      amount,
		Currency:    "IDR",
		Description: "DesKaProvider Midtrans sandbox validation",
	})
	if err != nil {
		t.Fatalf("sandbox payment create: %v", err)
	}
	if created.ReferenceID != orderID || created.ProviderReference == "" || created.Status != payment.StatusPending {
		t.Fatalf("unexpected sandbox create result: %+v", created)
	}

	status, err := client.GetPaymentStatus(ctx, payment.StatusRequest{ReferenceID: orderID})
	if err != nil {
		t.Fatalf("sandbox payment status: %v", err)
	}
	if status.ReferenceID != orderID || status.ProviderReference == "" {
		t.Fatalf("unexpected sandbox status identity: %+v", status)
	}
	if status.Amount != amount || status.Currency != "IDR" {
		t.Fatalf("unexpected sandbox status amount/currency: %+v", status)
	}
	if status.Status != payment.StatusPending && status.Status != payment.StatusSuccess && status.Status != payment.StatusFailed {
		t.Fatalf("unexpected normalized sandbox status: %+v", status)
	}

	// Validate the webhook contract against the actual sandbox transaction
	// identity/status shape without causing a second payment request.
	statusCode := "200"
	grossAmount := "10000.00"
	midtransStatus := mapWebhookStatus(status.Status)
	if midtransStatus == "" {
		t.Fatalf("cannot map normalized status back to Midtrans webhook status: %q", status.Status)
	}
	signatureKey := signature(orderID, statusCode, grossAmount, cfg.ServerKey)
	payload, err := json.Marshal(map[string]string{
		"status_code":        statusCode,
		"status_message":     "sandbox contract validation",
		"signature_key":      signatureKey,
		"transaction_id":     status.ProviderReference,
		"order_id":           orderID,
		"transaction_status": midtransStatus,
		"gross_amount":       grossAmount,
	})
	if err != nil {
		t.Fatalf("marshal webhook validation payload: %v", err)
	}
	webhook, err := client.HandlePaymentWebhook(ctx, payload)
	if err != nil {
		t.Fatalf("sandbox webhook contract: %v", err)
	}
	if webhook.ReferenceID != orderID || webhook.ProviderReference != status.ProviderReference {
		t.Fatalf("unexpected webhook identity: %+v", webhook)
	}
}

func mapWebhookStatus(status payment.Status) string {
	switch status {
	case payment.StatusSuccess:
		return "settlement"
	case payment.StatusPending:
		return "pending"
	case payment.StatusFailed:
		return "deny"
	default:
		return ""
	}
}
