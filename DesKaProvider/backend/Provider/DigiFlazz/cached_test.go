package digiflazz

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/DesKaOne/DesKaEcosystem/DesKaProvider/config"
	provider "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider"
)

func TestCachedClientAvoidsRepeatedPriceListRequestsWithinTTL(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{
			{"product_name": "XL 10K", "buyer_sku_code": "xld10", "buyer_product_status": true},
		}})
	}))
	defer server.Close()

	client, err := New(config.DigiFlazzConfig{
		Username: "buyer", APIKey: "secret", PriceListEndpoint: server.URL,
	}, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	cached, err := NewCachedClient(client, time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	req := provider.ProductRequest{Category: "Pulsa"}
	first, err := cached.GetProducts(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	second, err := cached.GetProducts(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 1 || len(second) != 1 || first[0] != second[0] {
		t.Fatalf("unexpected cached products: first=%#v second=%#v", first, second)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("expected one provider request within TTL, got %d", got)
	}
}

func TestCachedClientSeparatesActiveFilterCacheEntries(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		var got map[string]any
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatal(err)
		}
		active := true
		if value, ok := got["buyer_product_status"]; ok {
			active = value.(bool)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{
			{"product_name": "Active", "buyer_sku_code": "active", "buyer_product_status": active},
		}})
	}))
	defer server.Close()

	client, err := New(config.DigiFlazzConfig{
		Username: "buyer", APIKey: "secret", PriceListEndpoint: server.URL,
	}, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	cached, err := NewCachedClient(client, time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	active := true
	if _, err := cached.GetProducts(context.Background(), provider.ProductRequest{Category: "Pulsa", Active: &active}); err != nil {
		t.Fatal(err)
	}
	if _, err := cached.GetProducts(context.Background(), provider.ProductRequest{Category: "Pulsa", Active: &active}); err != nil {
		t.Fatal(err)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("expected active-filtered requests to share a cache entry, got %d provider requests", got)
	}
}
