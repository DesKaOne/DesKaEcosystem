package xpsindonesia

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/DesKaProvider/integration"
	"github.com/DesKaOne/DesKaEcosystem/DesKaProvider/config"
)

func TestLiveReadOnlyBalance(t *testing.T) {
	if !integration.Enabled("xp_sindonesia") || os.Getenv("XP_SINDONESIA_INTEGRATION") != "1" {
		t.Skip("XP SINDONESIA read-only integration disabled")
	}

	cfg, err := config.LoadXPSindonesiaConfig()
	if err != nil {
		t.Fatal(err)
	}
	if err := integration.ValidateEndpoints(cfg.SaldoEndpoint); err != nil {
		t.Fatal(err)
	}

	client, err := New(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	balance, err := client.GetBalance(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if balance < 0 {
		t.Fatalf("XP SINDONESIA balance must not be negative: %d", balance)
	}
}

func TestLiveReadOnlyBalanceRequiresExplicitProviderGate(t *testing.T) {
	t.Setenv("DESKAPROVIDER_LIVE_INTEGRATION", "1")
	t.Setenv("DESKAPROVIDER_LIVE_INTEGRATION_PROVIDER", "midtrans")
	t.Setenv("XP_SINDONESIA_INTEGRATION", "1")
	if integration.Enabled("xp_sindonesia") {
		t.Fatal("wrong provider gate must not authorize XP SINDONESIA")
	}
}

func TestLiveReadOnlyBalanceRejectsUnallowlistedEndpoint(t *testing.T) {
	t.Setenv("DESKAPROVIDER_LIVE_INTEGRATION", "1")
	t.Setenv("DESKAPROVIDER_LIVE_INTEGRATION_PROVIDER", "xp_sindonesia")
	t.Setenv("DESKAPROVIDER_LIVE_INTEGRATION_ALLOWED_HOSTS", "other.example")
	t.Setenv("XP_SINDONESIA_SALDO_ENDPOINT", "https://xp.sindonesia.net/api/saldo.php")
	if err := integration.ValidateEndpoints(os.Getenv("XP_SINDONESIA_SALDO_ENDPOINT")); err == nil {
		t.Fatal("expected unallowlisted XP endpoint to be rejected")
	}
}

func TestLiveReadOnlyBalanceAllowlistUsesHostOnly(t *testing.T) {
	t.Setenv("DESKAPROVIDER_LIVE_INTEGRATION_ALLOWED_HOSTS", "xp.sindonesia.net")
	if !integration.EndpointAllowed("https://xp.sindonesia.net/api/saldo.php") {
		t.Fatal("expected XP balance endpoint to be allowed")
	}
	if integration.EndpointAllowed("https://evil.example/xp.sindonesia.net/api/saldo.php") {
		t.Fatal("path text must not authorize an unrelated host")
	}
	if !strings.HasPrefix("https://xp.sindonesia.net/api/saldo.php", "https://") {
		t.Fatal("sanity check")
	}
}
