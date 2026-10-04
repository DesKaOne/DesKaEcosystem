package digiflazz

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/DesKaOne/DesKaEcosystem/DesKaProvider/config"
	"github.com/DesKaOne/DesKaEcosystem/DesKaProvider/integration"
	provider "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider"
)

func TestLiveCSFailureCase(t *testing.T) {
	if os.Getenv("DIGIFLAZZ_INTEGRATION") != "1" || !integration.Enabled("digiflazz") {
		t.Skip("live integration requires DIGIFLAZZ_INTEGRATION=1 and the global DigiFlazz provider gate")
	}
	if os.Getenv("DIGIFLAZZ_USERNAME") == "" || os.Getenv("DIGIFLAZZ_API_KEY") == "" {
		t.Skip("DigiFlazz runtime credentials are not configured")
	}

	cfg, err := config.LoadDigiFlazzConfig()
	if err != nil {
		t.Fatal(err)
	}
	if err := integration.ValidateEndpoints(cfg.Endpoint); err != nil {
		t.Fatalf("DigiFlazz transaction endpoint failed explicit allowlist validation: %v", err)
	}

	client, err := New(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}

	refID := strings.TrimSpace(os.Getenv("DIGIFLAZZ_INTEGRATION_REFERENCE_ID"))
	if refID == "" {
		refID = "deskaprovider-ci-cs-failure-" + time.Now().UTC().Format("20060102T150405.000000000Z")
	}

	result, err := client.Purchase(context.Background(), provider.PurchaseRequest{
		ProductCode: "xld10",
		CustomerNo:  "087800001232",
		ReferenceID: refID,
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

func TestLiveReadOnlyBalance(t *testing.T) {
	if os.Getenv("DIGIFLAZZ_INTEGRATION") != "1" || !integration.Enabled("digiflazz") {
		t.Skip("live integration requires DIGIFLAZZ_INTEGRATION=1 and the global DigiFlazz provider gate")
	}
	if os.Getenv("DIGIFLAZZ_USERNAME") == "" || os.Getenv("DIGIFLAZZ_API_KEY") == "" {
		t.Skip("DigiFlazz runtime credentials are not configured")
	}

	cfg, err := config.LoadDigiFlazzConfig()
	if err != nil {
		t.Fatal(err)
	}
	if err := integration.ValidateEndpoints(cfg.BalanceEndpoint); err != nil {
		t.Fatalf("DigiFlazz balance endpoint failed explicit allowlist validation: %v", err)
	}

	client, err := New(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	balance, err := client.GetBalance(context.Background())
	if err != nil {
		t.Fatalf("DigiFlazz live balance check failed: %v", err)
	}
	if balance < 0 {
		t.Fatalf("DigiFlazz balance must not be negative: %d", balance)
	}
}
