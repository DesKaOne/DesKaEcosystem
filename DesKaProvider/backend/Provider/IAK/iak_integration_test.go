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
    if os.Getenv("IAK_INTEGRATION") != "1" || !integration.Enabled("iak") {
        t.Skip("live integration requires the provider-specific opt-in and global DesKaProvider live gate")
    }
    if os.Getenv("IAK_USERNAME") == "" || os.Getenv("IAK_API_KEY") == "" {
        t.Skip("IAK runtime credentials are not configured")
    }

    cfg, err := config.LoadIAKConfig()
    if err != nil {
        t.Fatal(err)
    }
        if !integration.HostAllowed(cfg.BalanceEndpoint) {
        t.Skip("provider endpoint host is not explicitly allowlisted for live integration")
    }
client, err := New(cfg, nil)
    if err != nil {
        t.Fatal(err)
    }

    if _, err := client.GetBalance(context.Background()); err != nil {
        t.Fatalf("IAK live balance check failed: %v", err)
    }

    products, err := client.GetProducts(context.Background(), provider.ProductRequest{})
    if err != nil {
        t.Fatalf("IAK live price-list check failed: %v", err)
    }
    if products == nil {
        t.Fatal("IAK live price-list returned nil product slice")
    }
}
