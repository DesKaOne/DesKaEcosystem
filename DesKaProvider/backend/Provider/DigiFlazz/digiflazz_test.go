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
			{"product_name":"XL 10K","buyer_sku_code":"xld10","buyer_product_status":true},
			{"product_name":"XL 25K","buyer_sku_code":"xld25","buyer_product_status":false},
		}})
	}))
	defer server.Close()

	c, err := New(config.DigiFlazzConfig{Username:"buyer", APIKey:"secret", PriceListEndpoint:server.URL + "/v1/price-list"}, server.Client())
	if err != nil { t.Fatal(err) }
	active := true
	got, err := c.GetProducts(context.Background(), provider.ProductRequest{Category:"Pulsa", Active:&active})
	if err != nil { t.Fatal(err) }
	if len(got) != 1 || got[0].Code != "xld10" || got[0].Name != "XL 10K" { t.Fatalf("unexpected products: %#v", got) }
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
