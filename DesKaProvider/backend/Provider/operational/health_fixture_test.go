package operational

import (
	"testing"
	"time"
)

func TestStandardHealthObservationFixturesAreProviderNeutral(t *testing.T) {
	now := time.Date(2026, 9, 28, 10, 0, 0, 0, time.UTC)
	fixtures := StandardHealthObservationFixtures("digiflazz", now)
	if len(fixtures) != 4 {
		t.Fatalf("expected four standard fixtures, got %d", len(fixtures))
	}
	want := []struct{ status Health; freshness Freshness; observation string }{
		{HealthHealthy, FreshnessFresh, "success"},
		{HealthHealthy, FreshnessStale, "stale"},
		{HealthDegraded, FreshnessFresh, "unavailable"},
		{HealthUnhealthy, FreshnessFresh, "error"},
	}
	for i, fixture := range fixtures {
		if fixture.Provider != "digiflazz" || fixture.Health.ProviderName != "digiflazz" {
			t.Fatalf("fixture %d leaked an unexpected provider identity: %#v", i, fixture)
		}
		if fixture.Health.Status != want[i].status || fixture.Freshness != want[i].freshness || fixture.Observation != want[i].observation {
			t.Fatalf("fixture %d mismatch: %#v", i, fixture)
		}
	}
}

func TestStandardHealthObservationFixturesDoNotAuthorizeRouting(t *testing.T) {
	now := time.Date(2026, 9, 28, 10, 0, 0, 0, time.UTC)
	fixtures := StandardHealthObservationFixtures("iak", now)
	for _, fixture := range fixtures {
		if fixture.Snapshot.Balance != 0 {
			t.Fatalf("health fixture unexpectedly carried financial state: %#v", fixture)
		}
		if fixture.Health.Status == HealthHealthy && fixture.Health.ConsecutiveFailures != 0 {
			t.Fatalf("healthy fixture has failure state: %#v", fixture)
		}
	}
}
