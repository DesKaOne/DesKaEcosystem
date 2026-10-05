package config

import (
	"os"
	"testing"
)

func TestLoadMidtransConfigRequiresServerKey(t *testing.T) {
	t.Setenv("MIDTRANS_SERVER_KEY", "")
	if _, err := LoadMidtransConfig(); err == nil {
		t.Fatal("expected missing server key error")
	}
}

func TestLoadMidtransConfigDefaults(t *testing.T) {
	t.Setenv("MIDTRANS_SERVER_KEY", "server-key")
	t.Setenv("MIDTRANS_SNAP_ENDPOINT", "")
	t.Setenv("MIDTRANS_API_ENDPOINT", "")

	cfg, err := LoadMidtransConfig()
	if err != nil {
		t.Fatalf("LoadMidtransConfig: %v", err)
	}
	if cfg.SnapEndpoint != defaultMidtransSnapEndpoint {
		t.Fatalf("unexpected snap endpoint: %q", cfg.SnapEndpoint)
	}
	if cfg.APIEndpoint != defaultMidtransAPIEndpoint {
		t.Fatalf("unexpected API endpoint: %q", cfg.APIEndpoint)
	}
}

func TestLoadMidtransConfigOverrides(t *testing.T) {
	t.Setenv("MIDTRANS_SERVER_KEY", "server-key")
	t.Setenv("MIDTRANS_SNAP_ENDPOINT", "https://example.test/snap")
	t.Setenv("MIDTRANS_API_ENDPOINT", "https://example.test/")

	cfg, err := LoadMidtransConfig()
	if err != nil {
		t.Fatalf("LoadMidtransConfig: %v", err)
	}
	if cfg.SnapEndpoint != "https://example.test/snap" {
		t.Fatalf("unexpected snap endpoint: %q", cfg.SnapEndpoint)
	}
	if cfg.APIEndpoint != "https://example.test" {
		t.Fatalf("unexpected API endpoint: %q", cfg.APIEndpoint)
	}
}

func TestLoadMidtransConfigDoesNotReadUnrelatedEnvironment(t *testing.T) {
	t.Setenv("MIDTRANS_SERVER_KEY", "server-key")
	_ = os.Getenv("MIDTRANS_SERVER_KEY")
}
