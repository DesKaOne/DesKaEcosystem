package midtrans

import (
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	payment "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/internal/Payment"
	"github.com/DesKaOne/DesKaEcosystem/DesKaProvider/config"
)

func TestCreatePaymentMapsSnapTokenAndPendingState(t *testing.T) {
	serverKey := "server-key"
	var gotAuth, gotBody string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("{\"token\":\"snap-token-1\",\"redirect_url\":\"https://app.test/token\"}"))
	}))
	defer ts.Close()

	client, err := New(config.MidtransConfig{
		ServerKey: serverKey, SnapEndpoint: ts.URL + "/snap/v1/transactions", APIEndpoint: ts.URL,
	}, ts.Client())
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	result, err := client.CreatePayment(context.Background(), payment.PaymentRequest{
		ReferenceID: "ref-1", Amount: 10000, Currency: "IDR",
	})
	if err != nil {
		t.Fatalf("CreatePayment: %v", err)
	}
	if result.ReferenceID != "ref-1" || result.ProviderReference != "snap-token-1" || result.Status != payment.StatusPending {
		t.Fatalf("unexpected result: %+v", result)
	}
	wantAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte(serverKey+":"))
	if gotAuth != wantAuth {
		t.Fatalf("unexpected authorization: %q", gotAuth)
	}
	if !strings.Contains(gotBody, "\"order_id\":\"ref-1\"") || !strings.Contains(gotBody, "\"gross_amount\":10000") {
		t.Fatalf("unexpected request body: %s", gotBody)
	}
}

func TestGetPaymentStatusMapsSettlement(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v2/ref-1/status" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("{\"status_code\":\"200\",\"status_message\":\"Success\",\"transaction_id\":\"trx-1\",\"order_id\":\"ref-1\",\"transaction_status\":\"settlement\",\"gross_amount\":\"10000.00\"}"))
	}))
	defer ts.Close()

	client, err := New(config.MidtransConfig{
		ServerKey: "server-key", SnapEndpoint: ts.URL, APIEndpoint: ts.URL,
	}, ts.Client())
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	result, err := client.GetPaymentStatus(context.Background(), payment.StatusRequest{ReferenceID: "ref-1"})
	if err != nil {
		t.Fatalf("GetPaymentStatus: %v", err)
	}
	if result.Status != payment.StatusSuccess || result.Amount != 10000 || result.ProviderReference != "trx-1" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestGetPaymentStatusRejectsIdentityMismatch(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("{\"status_code\":\"200\",\"status_message\":\"Success\",\"transaction_id\":\"trx-1\",\"order_id\":\"other-ref\",\"transaction_status\":\"settlement\",\"gross_amount\":\"10000.00\"}"))
	}))
	defer ts.Close()

	client, err := New(config.MidtransConfig{
		ServerKey: "server-key", SnapEndpoint: ts.URL, APIEndpoint: ts.URL,
	}, ts.Client())
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if _, err := client.GetPaymentStatus(context.Background(), payment.StatusRequest{ReferenceID: "ref-1"}); err == nil {
		t.Fatal("expected identity mismatch error")
	}
}

func TestWebhookVerifiesSignatureAndNormalizesStatus(t *testing.T) {
	serverKey := "server-key"
	gross := "10000.00"
	sig := signature("ref-1", "200", gross, serverKey)
	payload := "{\"status_code\":\"200\",\"status_message\":\"settled\",\"signature_key\":\"" + sig + "\",\"transaction_id\":\"trx-1\",\"order_id\":\"ref-1\",\"transaction_status\":\"settlement\",\"gross_amount\":\"" + gross + "\"}"

	client, err := New(config.MidtransConfig{
		ServerKey: serverKey, SnapEndpoint: "https://example.test/snap", APIEndpoint: "https://example.test",
	}, nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	result, err := client.HandlePaymentWebhook(context.Background(), []byte(payload))
	if err != nil {
		t.Fatalf("HandlePaymentWebhook: %v", err)
	}
	if result.Status != payment.StatusSuccess || result.ReferenceID != "ref-1" || result.ProviderReference != "trx-1" {
		t.Fatalf("unexpected webhook result: %+v", result)
	}
}

func TestWebhookRejectsInvalidSignature(t *testing.T) {
	client, err := New(config.MidtransConfig{
		ServerKey: "server-key", SnapEndpoint: "https://example.test/snap", APIEndpoint: "https://example.test",
	}, nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	payload := "{\"status_code\":\"200\",\"signature_key\":\"bad\",\"transaction_id\":\"trx-1\",\"order_id\":\"ref-1\",\"transaction_status\":\"settlement\",\"gross_amount\":\"10000.00\"}"
	if _, err := client.HandlePaymentWebhook(context.Background(), []byte(payload)); err == nil {
		t.Fatal("expected invalid signature error")
	}
}

func TestCreatePaymentRejectsNonIDR(t *testing.T) {
	client, err := New(config.MidtransConfig{
		ServerKey: "server-key", SnapEndpoint: "https://example.test/snap", APIEndpoint: "https://example.test",
	}, nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	_, err = client.CreatePayment(context.Background(), payment.PaymentRequest{
		ReferenceID: "ref-1", Amount: 10000, Currency: "USD",
	})
	if err != payment.ErrUnsupported {
		t.Fatalf("expected ErrUnsupported, got %v", err)
	}
}

func TestNormalizeStatusRejectsUnknown(t *testing.T) {
	if _, err := normalizeStatus("unknown"); err == nil {
		t.Fatal("expected unknown status error")
	}
}

func TestWebhookRejectsSuccessWithUnacceptableFraudStatus(t *testing.T) {
	serverKey := "server-key"
	gross := "10000.00"
	sig := signature("ref-1", "201", gross, serverKey)
	payload := "{\"status_code\":\"201\",\"status_message\":\"challenge\",\"signature_key\":\"" + sig + "\",\"transaction_id\":\"trx-1\",\"order_id\":\"ref-1\",\"transaction_status\":\"capture\",\"gross_amount\":\"" + gross + "\",\"fraud_status\":\"challenge\"}"
	client, err := New(config.MidtransConfig{ServerKey: serverKey, SnapEndpoint: "https://example.test/snap", APIEndpoint: "https://example.test"}, nil)
	if err != nil { t.Fatalf("New: %v", err) }
	if _, err := client.HandlePaymentWebhook(context.Background(), []byte(payload)); err == nil {
		t.Fatal("expected success webhook with challenge fraud status to fail closed")
	}
}
