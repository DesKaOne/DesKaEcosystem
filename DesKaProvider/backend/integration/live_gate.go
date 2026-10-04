package integration

import (
	"net/url"
	"os"
	"strings"
)

const (
	LiveIntegrationEnv   = "DESKAPROVIDER_LIVE_INTEGRATION"
	LiveProviderEnv      = "DESKAPROVIDER_LIVE_INTEGRATION_PROVIDER"
	LiveAllowedHostsEnv  = "DESKAPROVIDER_LIVE_INTEGRATION_ALLOWED_HOSTS"
)

// Enabled returns true only when the global live-integration switch is explicit
// and the requested provider is the explicitly selected provider.
func Enabled(provider string) bool {
	return os.Getenv(LiveIntegrationEnv) == "1" &&
		strings.EqualFold(strings.TrimSpace(os.Getenv(LiveProviderEnv)), strings.TrimSpace(provider))
}

// HostAllowed requires the live-test host to be explicitly allowlisted.
// An empty allowlist never authorizes a live request.
func HostAllowed(rawURL string) bool {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || u.Hostname() == "" {
		return false
	}
	host := strings.ToLower(strings.TrimSpace(u.Hostname()))
	for _, allowed := range strings.Split(os.Getenv(LiveAllowedHostsEnv), ",") {
		if strings.EqualFold(strings.TrimSpace(allowed), host) {
			return true
		}
	}
	return false
}
