package digiflazz

import (
	"context"
	"os"
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/DesKaProvider/config"
    "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/integration"
	provider "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider"
)

func TestLiveCSFailureCase(t *testing.T) {
	if os.Getenv("DIGIFLAZZ_INTEGRATION") != "1" || !integration.Enabled("digiflazz") {
		t.Skip("live integration requires the provider-specific opt-in and global DesKaProvider live gate")
	}
	if os.Getenv("DIGIFLAZZ_USERNAME") == "" || os.Getenv("DIGIFLAZZ_API_KEY") == "" {
		t.Skip("DigiFlazz runtime credentials are not configured")
	}

	cfg, err := config.LoadDigiFlazzConfig()
	if err != nil {
		t.Fatal(err)
	}
		if !integration.HostAllowed(cfg.Endpoint) {
		t.Skip("provider endpoint host is not explicitly allowlisted for live integration")
	}
client, err := New(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}

	result, err := client.Purchase(context.Background(), provider.PurchaseRequest{
		ProductCode: "xld10",
		CustomerNo:  "087800001232",
		ReferenceID: "deskaprovider-ci-cs-failure",
		Testing:     true,
	})
	if err != nil {
		t.Fatal(err)
	}

	if result.Status != provider.StatusFailed {
		t.Fatalf("expected failed status for official CS test tuple, got %#v", result)
	}
	if result.ProviderCode != "02" {
		t.Fatalf("expected provider code 02 for official CS test tuple, got %#v", result)
	}
}
