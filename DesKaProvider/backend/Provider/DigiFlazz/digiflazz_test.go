package digiflazz

import (
	"context"
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/DesKaProvider/config"
	provider "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider"
)

func TestPurchaseBuildsOfficialBuyerRequestAndMapsResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var got map[string]any
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatal(err)
		}
		if got["username"] != "buyer" || got["buyer_sku_code"] != "xld10" || got["customer_no"] != "087800001232" || got["ref_id"] != "ref-1" {
			t.Fatalf("unexpected request: %#v", got)
		}
		h := md5.Sum([]byte("buyersecretref-1"))
		if got["sign"] != hex.EncodeToString(h[:]) {
			t.Fatalf("unexpected signature: %v", got["sign"])
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"ref_id": "ref-1", "customer_no": "087800001232", "buyer_sku_code": "xld10",
				"message": "Transaksi Gagal", "status": "Gagal", "rc": "02", "price": 10000,
			},
		})
	}))
	defer server.Close()

	c, err := New(config.DigiFlazzConfig{Username: "buyer", APIKey: "secret", Endpoint: server.URL}, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	got, err := c.Purchase(context.Background(), provider.PurchaseRequest{
		ProductCode: "xld10", CustomerNo: "087800001232", ReferenceID: "ref-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != provider.StatusFailed || got.ProviderCode != "02" {
		t.Fatalf("unexpected result: %#v", got)
	}
}


func TestPurchaseRejectsMissingRequiredPrice(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
			"ref_id": "ref-no-price", "customer_no": "087800001232", "buyer_sku_code": "xld10",
			"message": "Transaksi Sukses", "status": "Sukses", "rc": "00",
		}})
	}))
	defer server.Close()
	c, err := New(config.DigiFlazzConfig{Username: "buyer", APIKey: "secret", Endpoint: server.URL}, server.Client())
	if err != nil { t.Fatal(err) }
	_, err = c.Purchase(context.Background(), provider.PurchaseRequest{ProductCode: "xld10", CustomerNo: "087800001232", ReferenceID: "ref-no-price"})
	if err == nil || !strings.Contains(err.Error(), "missing required price") { t.Fatalf("expected missing required price rejection, got %v", err) }
}

func TestPurchaseMapsStructuredProviderErrorFromHTTP400(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"ref_id": "ref-ip-block",
				"customer_no": "087800001232",
				"buyer_sku_code": "xld10",
				"message": "IP Anda tidak kami kenali",
				"status": "Gagal",
				"rc": "45",
				"price": 10000,
			},
		})
	}))
	defer server.Close()

	c, err := New(config.DigiFlazzConfig{
		Username: "buyer",
		APIKey: "secret",
		Endpoint: server.URL,
	}, server.Client())
	if err != nil {
		t.Fatal(err)
	}

	got, err := c.Purchase(context.Background(), provider.PurchaseRequest{
		ProductCode: "xld10",
		CustomerNo:  "087800001232",
		ReferenceID: "ref-ip-block",
		Testing:     true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != provider.StatusFailed || got.ProviderCode != "45" {
		t.Fatalf("unexpected structured HTTP 400 mapping: %#v", got)
	}
	if got.Message == "" {
		t.Fatal("expected provider error message to be preserved")
	}
}

func TestDigiFlazzGetBalanceUsesOfficialDepositEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/cek-saldo" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		var got map[string]any
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatal(err)
		}
		if got["cmd"] != "deposit" || got["username"] != "buyer" {
			t.Fatalf("unexpected balance request: %#v", got)
		}
		h := md5.Sum([]byte("buyersecretdepo"))
		if got["sign"] != hex.EncodeToString(h[:]) {
			t.Fatalf("unexpected balance signature: %v", got["sign"])
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"deposit": 1250000}})
	}))
	defer server.Close()

	c, err := New(config.DigiFlazzConfig{Username: "buyer", APIKey: "secret", Endpoint: server.URL + "/v1/transaction", BalanceEndpoint: server.URL + "/v1/cek-saldo"}, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	balance, err := c.GetBalance(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if balance != 1250000 {
		t.Fatalf("unexpected balance: %d", balance)
	}
}

func TestWebhookSignatureAndMapping(t *testing.T) {
	body := []byte(`{"data":{"ref_id":"ref-1","customer_no":"087800001233","buyer_sku_code":"xld10","message":"Transaksi Sukses","status":"Sukses","rc":"00","sn":"SN1","price":10000}}`)
	signature := newHMAC(body, "hooksecret")

	c, err := New(config.DigiFlazzConfig{Username: "buyer", APIKey: "secret"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	got, err := c.HandleWebhook(context.Background(), provider.WebhookRequest{
		Body: body, Signature: "sha1=" + signature, SignatureSecret: "hooksecret",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != provider.StatusSuccess || got.ProviderCode != "00" || got.SerialNumber != "SN1" {
		t.Fatalf("unexpected webhook: %#v", got)
	}

	_, err = c.HandleWebhook(context.Background(), provider.WebhookRequest{
		Body: body, Signature: "sha1=bad", SignatureSecret: "hooksecret",
	})
	if err == nil {
		t.Fatal("expected invalid signature error")
	}
}

func TestWebhookRejectsNonPrepaidMetadata(t *testing.T) {
	body := []byte(`{"data":{"ref_id":"ref-meta","customer_no":"087800001233","buyer_sku_code":"xld10","message":"Transaksi Sukses","status":"Sukses","rc":"00","sn":"SN1","price":10000}}`)
	c, err := New(config.DigiFlazzConfig{Username: "buyer", APIKey: "secret"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name      string
		event     string
		userAgent string
	}{
		{name: "postpaid user agent", event: "create", userAgent: "Digiflazz-Pasca-Hookshot"},
		{name: "unsupported event", event: "resend", userAgent: "Digiflazz-Hookshot"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := c.HandleWebhook(context.Background(), provider.WebhookRequest{
				Body: body, Event: tc.event, UserAgent: tc.userAgent,
			})
			if err == nil {
				t.Fatal("expected non-prepaid webhook metadata to be rejected")
			}
		})
	}
}

func TestWebhookRejectsStatusRCConflict(t *testing.T) {
	body := []byte(`{"data":{"ref_id":"ref-conflict","customer_no":"087800001233","buyer_sku_code":"xld10","message":"Konflik","status":"Sukses","rc":"03","price":10000}}`)
	c, err := New(config.DigiFlazzConfig{Username: "buyer", APIKey: "secret"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.HandleWebhook(context.Background(), provider.WebhookRequest{Body: body})
	if err == nil || !strings.Contains(err.Error(), "conflicts with rc") {
		t.Fatalf("expected webhook status/RC conflict rejection, got %v", err)
	}
}

func newHMAC(body []byte, secret string) string {
	h := hmac.New(sha1.New, []byte(secret))
	_, _ = h.Write(body)
	return hex.EncodeToString(h.Sum(nil))
}

func TestDigiFlazzGetProductsUsesOfficialPriceListEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/price-list" { t.Fatalf("unexpected path: %s", r.URL.Path) }
		var got map[string]any
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil { t.Fatal(err) }
		if got["cmd"] != "prepaid" || got["username"] != "buyer" || got["category"] != "Pulsa" { t.Fatalf("unexpected request: %#v", got) }
		h := md5.Sum([]byte("buyersecretpricelist"))
		if got["sign"] != hex.EncodeToString(h[:]) { t.Fatalf("unexpected signature: %v", got["sign"]) }
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{
			{"product_name":"XL 10K","buyer_sku_code":"xld10","buyer_product_status":true,"seller_product_status":true},
			{"product_name":"XL 25K","buyer_sku_code":"xld25","buyer_product_status":false,"seller_product_status":true},
			{"product_name":"XL 50K","buyer_sku_code":"xld50","buyer_product_status":true,"seller_product_status":false},
		}})
	}))
	defer server.Close()

	c, err := New(config.DigiFlazzConfig{Username:"buyer", APIKey:"secret", PriceListEndpoint:server.URL + "/v1/price-list"}, server.Client())
	if err != nil { t.Fatal(err) }
	active := true
	got, err := c.GetProducts(context.Background(), provider.ProductRequest{Category:"Pulsa", Active:&active})
	if err != nil { t.Fatal(err) }
	if len(got) != 2 || got[0].Code != "xld10" || got[0].Name != "XL 10K" || got[1].Code != "xld50" || got[1].Name != "XL 50K" {
		t.Fatalf("unexpected products: %#v", got)
	}
}

func TestDigiFlazzGetProductsRejectsIncompleteProduct(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{
			{"product_name":"XL 10K","buyer_sku_code":"xld10","buyer_product_status":true},
			{"product_name":"","buyer_sku_code":"xld25","buyer_product_status":true},
		}})
	}))
	defer server.Close()

	c, err := New(config.DigiFlazzConfig{Username:"buyer", APIKey:"secret", PriceListEndpoint:server.URL}, server.Client())
	if err != nil { t.Fatal(err) }
	_, err = c.GetProducts(context.Background(), provider.ProductRequest{})
	if err == nil || !strings.Contains(err.Error(), "missing buyer SKU code or product name") {
		t.Fatalf("expected incomplete price-list product rejection, got %v", err)
	}
}

func TestDigiFlazzInquiryPLNUsesOfficialEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/inquiry-pln" { t.Fatalf("unexpected path: %s", r.URL.Path) }
		var got map[string]any
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil { t.Fatal(err) }
		if got["username"] != "buyer" || got["customer_no"] != "1234554321" { t.Fatalf("unexpected request: %#v", got) }
		h := md5.Sum([]byte("buyersecret1234554321"))
		if got["sign"] != hex.EncodeToString(h[:]) { t.Fatalf("unexpected signature: %v", got["sign"]) }
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"message":"Transaksi Sukses","status":"Sukses","rc":"00"}})
	}))
	defer server.Close()
	c, err := New(config.DigiFlazzConfig{Username:"buyer", APIKey:"secret", InquiryPLNEndpoint:server.URL + "/v1/inquiry-pln"}, server.Client())
	if err != nil { t.Fatal(err) }
	got, err := c.Inquiry(context.Background(), provider.InquiryRequest{ProductCode:"pln", CustomerNo:"1234554321", ReferenceID:"ref-1"})
	if err != nil { t.Fatal(err) }
	if got.Status != provider.StatusSuccess || got.ProviderCode != "00" { t.Fatalf("unexpected inquiry result: %#v", got) }
}
func TestDigiFlazzInquiryRejectsUnsupportedProduct(t *testing.T) {
	c, err := New(config.DigiFlazzConfig{Username:"buyer", APIKey:"secret"}, nil)
	if err != nil { t.Fatal(err) }
	_, err = c.Inquiry(context.Background(), provider.InquiryRequest{ProductCode:"xld10", CustomerNo:"123"})
	if !errors.Is(err, provider.ErrUnsupportedOperation) { t.Fatalf("expected unsupported operation, got %v", err) }
}


func TestGetStatusFailsClosedWithoutResubmission(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("DigiFlazz status must not resubmit the transaction endpoint")
	}))
	defer server.Close()
	c, err := New(config.DigiFlazzConfig{Username: "buyer", APIKey: "secret", Endpoint: server.URL}, server.Client())
	if err != nil { t.Fatal(err) }
	_, err = c.GetStatus(context.Background(), provider.StatusRequest{ProductCode: "xld10", CustomerNo: "087800001232", ReferenceID: "ref-1"})
	if !errors.Is(err, provider.ErrUnsupportedOperation) { t.Fatalf("expected unsupported status operation, got %v", err) }
}

func TestUnknownProviderStatusFailsClosed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"ref_id": "ref-unknown", "customer_no": "087800001232", "buyer_sku_code": "xld10", "message": "unknown", "status": "Menunggu", "rc": "999", "price": 10000}})
	}))
	defer server.Close()
	c, err := New(config.DigiFlazzConfig{Username: "buyer", APIKey: "secret", Endpoint: server.URL}, server.Client())
	if err != nil { t.Fatal(err) }
	_, err = c.Purchase(context.Background(), provider.PurchaseRequest{ProductCode: "xld10", CustomerNo: "087800001232", ReferenceID: "ref-unknown"})
	if !errors.Is(err, ErrUnknownResponseCode) { t.Fatalf("expected unknown status error, got %v", err) }
}

func TestPurchaseRejectsResponseIdentityMismatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"ref_id": "other-ref", "customer_no": "087800001232", "buyer_sku_code": "xld10", "message": "Transaksi Sukses", "status": "Sukses", "rc": "00", "price": 10000}})
	}))
	defer server.Close()
	c, err := New(config.DigiFlazzConfig{Username: "buyer", APIKey: "secret", Endpoint: server.URL}, server.Client())
	if err != nil { t.Fatal(err) }
	_, err = c.Purchase(context.Background(), provider.PurchaseRequest{ProductCode: "xld10", CustomerNo: "087800001232", ReferenceID: "ref-1"})
	if err == nil || !strings.Contains(err.Error(), "identity mismatch") { t.Fatalf("expected response identity mismatch, got %v", err) }
}

func TestDigiFlazzDefaultHTTPClientHasTimeout(t *testing.T) {
	c, err := New(config.DigiFlazzConfig{Username: "buyer", APIKey: "secret"}, nil)
	if err != nil { t.Fatal(err) }
	if c.httpClient.Timeout <= 0 { t.Fatalf("expected default HTTP timeout, got %s", c.httpClient.Timeout) }
}

func TestDigiFlazzHTTPTimeoutCancelsRequest(t *testing.T) {
	transport := roundTripFunc(func(*http.Request) (*http.Response, error) {
		<-time.After(100 * time.Millisecond)
		return nil, context.DeadlineExceeded
	})
	c, err := New(config.DigiFlazzConfig{Username: "buyer", APIKey: "secret", Endpoint: "http://example.invalid", HTTPTimeout: 10 * time.Millisecond}, &http.Client{Transport: transport})
	if err != nil { t.Fatal(err) }
	start := time.Now()
	_, err = c.Purchase(context.Background(), provider.PurchaseRequest{ProductCode: "xld10", CustomerNo: "087800001232", ReferenceID: "ref-timeout"})
	if err == nil || time.Since(start) > time.Second { t.Fatalf("expected bounded HTTP timeout, err=%v elapsed=%s", err, time.Since(start)) }
}

type roundTripFunc func(*http.Request) (*http.Response, error)
func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestMapResponseCodeCoversDocumentedBuyerCodes(t *testing.T) {
	cases := map[string]provider.TransactionStatus{
		"00": provider.StatusSuccess,
		"01": provider.StatusFailed, "02": provider.StatusFailed, "03": provider.StatusPending,
		"40": provider.StatusFailed, "41": provider.StatusFailed, "42": provider.StatusFailed,
		"43": provider.StatusFailed, "44": provider.StatusFailed, "45": provider.StatusFailed,
		"47": provider.StatusFailed, "49": provider.StatusFailed, "50": provider.StatusFailed,
		"51": provider.StatusFailed, "52": provider.StatusFailed, "53": provider.StatusFailed,
		"54": provider.StatusFailed, "55": provider.StatusFailed, "56": provider.StatusFailed,
		"57": provider.StatusFailed, "58": provider.StatusFailed, "59": provider.StatusFailed,
		"60": provider.StatusFailed, "61": provider.StatusFailed, "62": provider.StatusFailed,
		"63": provider.StatusFailed, "64": provider.StatusFailed, "65": provider.StatusFailed,
		"66": provider.StatusFailed, "67": provider.StatusFailed, "68": provider.StatusFailed,
		"69": provider.StatusFailed, "70": provider.StatusFailed, "71": provider.StatusFailed,
		"72": provider.StatusFailed, "73": provider.StatusFailed, "74": provider.StatusFailed,
		"80": provider.StatusFailed, "81": provider.StatusFailed, "82": provider.StatusFailed,
		"83": provider.StatusFailed, "84": provider.StatusFailed, "85": provider.StatusFailed,
		"86": provider.StatusFailed, "87": provider.StatusFailed, "88": provider.StatusFailed,
		"99": provider.StatusPending,
	}
	for rc,want:=range cases {
		t.Run(rc,func(t *testing.T){ got,err:=mapResponseCode(rc); if err!=nil||got!=want{t.Fatalf("rc %s => %q,%v; want %q",rc,got,err,want)} })
	}
}
func TestMapResponseCodeRejectsUnknown(t *testing.T) {
	if _,err:=mapResponseCode("999"); !errors.Is(err,ErrUnknownResponseCode){t.Fatalf("expected unknown response code, got %v",err)}
}
func TestMapResponseStatusRejectsStatusRCConflict(t *testing.T) {
	if _,err:=mapResponseStatus("Sukses","03"); err == nil {
		t.Fatal("expected status/RC conflict to fail closed")
	}
}
func TestMapResponseStatusAcceptsConsistentStatusRC(t *testing.T) {
	cases := []struct{status,rc string; want provider.TransactionStatus}{
		{"Sukses","00",provider.StatusSuccess},
		{"Pending","03",provider.StatusPending},
		{"Gagal","02",provider.StatusFailed},
	}
	for _,tc:=range cases {
		t.Run(tc.status,func(t *testing.T){
			got,err:=mapResponseStatus(tc.status,tc.rc)
			if err!=nil||got!=tc.want{t.Fatalf("got %q,%v; want %q",got,err,tc.want)}
		})
	}
}
