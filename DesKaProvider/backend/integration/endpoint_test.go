package integration

import "testing"

func TestEndpointAllowedRequiresGateIndependentOfHost(t *testing.T) {
	t.Setenv(LiveAllowedHostsEnv, "api.sandbox.midtrans.com")
	if !EndpointAllowed("https://api.sandbox.midtrans.com") {
		t.Fatal("allowlisted HTTPS endpoint should be accepted")
	}
}

func TestEndpointAllowedRejectsMalformedEndpoint(t *testing.T) {
	t.Setenv(LiveAllowedHostsEnv, "api.sandbox.midtrans.com")
	for _, endpoint := range []string{
		"not-a-url",
		"http://api.sandbox.midtrans.com",
		"https://",
		"https://user:pass@api.sandbox.midtrans.com",
		"https://api.sandbox.midtrans.com/#fragment",
	} {
		if EndpointAllowed(endpoint) {
			t.Fatalf("endpoint must be rejected: %q", endpoint)
		}
	}
}

func TestEndpointAllowedRejectsUnlistedHost(t *testing.T) {
	t.Setenv(LiveAllowedHostsEnv, "api.sandbox.midtrans.com")
	if EndpointAllowed("https://app.sandbox.midtrans.com/snap/v1/transactions") {
		t.Fatal("unlisted host must not be accepted")
	}
}

func TestValidateEndpointsRequiresExplicitAllowlist(t *testing.T) {
	t.Setenv(LiveAllowedHostsEnv, "api.sandbox.midtrans.com,app.sandbox.midtrans.com")
	if err := ValidateEndpoints(
		"https://api.sandbox.midtrans.com",
		"https://app.sandbox.midtrans.com/snap/v1/transactions",
	); err != nil {
		t.Fatalf("ValidateEndpoints: %v", err)
	}
	if err := ValidateEndpoints("https://api.sandbox.midtrans.com", "https://evil.example"); err == nil {
		t.Fatal("unallowlisted endpoint must fail validation")
	}
}


func TestEndpointAllowedWrongProviderGateDoesNotAuthorize(t *testing.T) {
    t.Setenv(LiveIntegrationEnv, "1")
    t.Setenv(LiveProviderEnv, "midtrans")
    t.Setenv(LiveAllowedHostsEnv, "prepaid.iak.id")
    if Enabled("iak") {
        t.Fatal("wrong provider selection must not enable IAK integration")
    }
    if !EndpointAllowed("https://prepaid.iak.id/api/check-balance") {
        t.Fatal("endpoint allowlist itself should remain independently testable")
    }
}

func TestEndpointAllowedMissingGateDoesNotAuthorize(t *testing.T) {
    t.Setenv(LiveIntegrationEnv, "0")
    t.Setenv(LiveProviderEnv, "iak")
    t.Setenv(LiveAllowedHostsEnv, "prepaid.iak.id")
    if Enabled("iak") {
        t.Fatal("disabled global gate must not enable IAK integration")
    }
}
