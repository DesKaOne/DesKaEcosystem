package iak

import (
    "context"
    "os"
    "testing"

    "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/config"
    "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/integration"
    provider "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider"
)

func TestLiveReadOnly(t *testing.T) {
    if !integration.Enabled("iak") {
        t.Skip("live integration requires DESKAPROVIDER_LIVE_INTEGRATION=1 and provider=iak")
    }
    if os.Getenv("IAK_INTEGRATION") != "1" {
        t.Skip("IAK integration requires IAK_INTEGRATION=1")
    }
    if os.Getenv("IAK_USERNAME") == "" || os.Getenv("IAK_API_KEY") == "" {
        t.Fatal("IAK runtime credentials are required when the explicit integration gate is enabled")
    }

    cfg, err := config.LoadIAKConfig()
    if err != nil {
        t.Fatal(err)
    }
    if err := integration.ValidateEndpoints(cfg.PriceListEndpoint, cfg.BalanceEndpoint); err != nil {
        t.Fatalf("IAK read-only endpoints failed explicit allowlist validation: %v", err)
    }

    client, err := New(cfg, nil)
    if err != nil {
        t.Fatal(err)
    }

    balance, err := client.GetBalance(context.Background())
    if err != nil {
        t.Fatalf("IAK live balance check failed: %v", err)
    }
    if balance < 0 {
        t.Fatalf("IAK balance must not be negative: %d", balance)
    }

    products, err := client.GetProducts(context.Background(), provider.ProductRequest{})
    if err != nil {
        t.Fatalf("IAK live price-list check failed: %v", err)
    }
    if products == nil {
        t.Fatal("IAK live price-list returned nil product slice")
    }
}
