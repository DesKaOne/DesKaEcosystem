package config

import (
	"os"
	"testing"
)

func TestLoadUsesDesKaProviderBoundary(t *testing.T) {
	t.Setenv("DESKACASH_HTTP_ADDR", ":9090")
	t.Setenv("DESKAPROVIDER_URL", "http://deskaprovider.internal")
	t.Setenv("DESKAPROVIDER_API_KEY", "test-key")
	t.Setenv("INDOCHAIN_RPC_URL", "http://must-not-be-read")

	cfg := Load()
	if cfg.HTTPAddr != ":9090" {
		t.Fatalf("expected HTTP address :9090, got %q", cfg.HTTPAddr)
	}
	if cfg.DesKaProviderURL != "http://deskaprovider.internal" {
		t.Fatalf("expected DesKaProvider URL, got %q", cfg.DesKaProviderURL)
	}
	if cfg.DesKaProviderAPIKey != "test-key" {
		t.Fatalf("expected DesKaProvider API key, got %q", cfg.DesKaProviderAPIKey)
	}
}

func TestLoadDoesNotExposeIndoChainRPCConfiguration(t *testing.T) {
	_ = os.Unsetenv("INDOCHAIN_RPC_URL")
	cfg := Load()
	if cfg.DesKaProviderURL != os.Getenv("DESKAPROVIDER_URL") {
		t.Fatalf("unexpected provider configuration drift")
	}
}
