package integration

import (
    "errors"
    "net/url"
    "strings"
)

func EndpointAllowed(rawURL string) bool {
    u, err := url.Parse(strings.TrimSpace(rawURL))
    if err != nil || u.Scheme != "https" || u.Hostname() == "" {
        return false
    }
    if u.User != nil || u.Fragment != "" {
        return false
    }
    return HostAllowed(u.String())
}

func ValidateEndpoints(endpoints ...string) error {
    if len(endpoints) == 0 {
        return errors.New("at least one integration endpoint is required")
    }
    for _, endpoint := range endpoints {
        if !EndpointAllowed(endpoint) {
            return errors.New("integration endpoint is not explicitly allowlisted")
        }
    }
    return nil
}
