package operational

import (
	"time"
)

// HealthObservation is a provider-neutral point-in-time health observation.
// It is operational evidence only and never authorizes routing or financial action.
type HealthObservation struct {
	ProviderName       string
	Status             Health
	ObservedAt         time.Time
	ConsecutiveFailures int
	LastError          string
}

// HealthObservationFixture is deterministic test data for operational-state
// consumers. Fixtures model observation outcomes without enabling providers.
type HealthObservationFixture struct {
	Name       string
	Provider   string
	Snapshot   Snapshot
	Health     HealthObservation
	Freshness  Freshness
	Observation string
}

func NewHealthObservationFixture(provider string, status Health, observedAt time.Time, failures int, lastError string) HealthObservationFixture {
	snapshot := Snapshot{
		ProviderName:        provider,
		Health:              status,
		LastCheckedAt:       observedAt,
		ConsecutiveFailures: failures,
		LastError:           lastError,
	}
	return HealthObservationFixture{
		Name:     string(status),
		Provider: provider,
		Snapshot: snapshot,
		Health: HealthObservation{
			ProviderName:       provider,
			Status:             status,
			ObservedAt:         observedAt,
			ConsecutiveFailures: failures,
			LastError:          lastError,
		},
	}
}

// StandardHealthObservationFixtures covers the deterministic operational
// observation states used by v0.1: healthy, degraded, unhealthy, and unknown.
func StandardHealthObservationFixtures(provider string, now time.Time) []HealthObservationFixture {
	fixtures := []HealthObservationFixture{
		NewHealthObservationFixture(provider, HealthHealthy, now, 0, ""),
		NewHealthObservationFixture(provider, HealthHealthy, now.Add(-10*time.Minute), 0, ""),
		NewHealthObservationFixture(provider, HealthDegraded, now, 1, "provider unavailable"),
		NewHealthObservationFixture(provider, HealthUnhealthy, now, 3, "provider error"),
	}
	fixtures[0].Name, fixtures[0].Observation, fixtures[0].Freshness = "success-fresh", "success", FreshnessFresh
	fixtures[1].Name, fixtures[1].Observation, fixtures[1].Freshness = "success-stale", "stale", FreshnessStale
	fixtures[2].Name, fixtures[2].Observation, fixtures[2].Freshness = "unavailable", "unavailable", FreshnessFresh
	fixtures[3].Name, fixtures[3].Observation, fixtures[3].Freshness = "error", "error", FreshnessFresh
	return fixtures
}
