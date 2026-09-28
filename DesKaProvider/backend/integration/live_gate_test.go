package integration

import "testing"

func TestEnabledRequiresExplicitProviderSelection(t *testing.T) {
	t.Setenv(LiveIntegrationEnv, "1")
	t.Setenv(LiveProviderEnv, "iak")

	if !Enabled("IAK") {
		t.Fatal("expected explicitly selected IAK provider to be enabled")
	}
	if Enabled("Midtrans") {
		t.Fatal("a different provider must not be enabled")
	}
}

func TestEnabledIsOffByDefault(t *testing.T) {
	t.Setenv(LiveIntegrationEnv, "")
	t.Setenv(LiveProviderEnv, "iak")
	if Enabled("iak") {
		t.Fatal("live integration must remain disabled without the explicit global switch")
	}
}

func TestHostAllowedRequiresExplicitAllowlist(t *testing.T) {
	t.Setenv(LiveAllowedHostsEnv, "")
	if HostAllowed("https://api.example.test") {
		t.Fatal("empty allowlist must not authorize a live host")
	}

	t.Setenv(LiveAllowedHostsEnv, "api.example.test, sandbox.example.test")
	if !HostAllowed("https://api.example.test/path") {
		t.Fatal("explicitly allowlisted host should be authorized")
	}
	if HostAllowed("https://api.other.test/path") {
		t.Fatal("unlisted host must not be authorized")
	}
}

func TestHostAllowedRejectsMalformedURL(t *testing.T) {
	t.Setenv(LiveAllowedHostsEnv, "api.example.test")
	if HostAllowed("not-a-url") {
		t.Fatal("malformed URL must not be authorized")
	}
}
