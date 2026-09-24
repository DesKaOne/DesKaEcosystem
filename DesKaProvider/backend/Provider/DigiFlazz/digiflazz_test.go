package digiflazz

import (
	"context"
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/DesKaProvider/config"
	"github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider"
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
	got, err := c.Purchase(context.Background(), Provider.PurchaseRequest{
		ProductCode: "xld10", CustomerNo: "087800001232", ReferenceID: "ref-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != Provider.StatusFailed || got.ProviderCode != "02" {
		t.Fatalf("unexpected result: %#v", got)
	}
}

func TestWebhookSignatureAndMapping(t *testing.T) {
	body := []byte(`{"data":{"ref_id":"ref-1","customer_no":"087800001233","buyer_sku_code":"xld10","message":"Transaksi Sukses","status":"Sukses","rc":"00","sn":"SN1","price":10000}}`)
	signature := newHMAC(body, "hooksecret")

	c, err := New(config.DigiFlazzConfig{Username: "buyer", APIKey: "secret"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	got, err := c.HandleWebhook(context.Background(), Provider.WebhookRequest{
		Body: body, Signature: "sha1=" + signature, SignatureSecret: "hooksecret",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != Provider.StatusSuccess || got.ProviderCode != "00" || got.SerialNumber != "SN1" {
		t.Fatalf("unexpected webhook: %#v", got)
	}

	_, err = c.HandleWebhook(context.Background(), Provider.WebhookRequest{
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
