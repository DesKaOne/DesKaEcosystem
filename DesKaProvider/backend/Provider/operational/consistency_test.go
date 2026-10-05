package operational

import (
	"errors"
	"testing"
	"time"
)

func validOperationalSnapshot() Snapshot {
	now := time.Date(2026, 9, 28, 10, 0, 0, 0, time.UTC)
	return Snapshot{
		ProviderName: "mock",
		Balance: 1000000,
		Currency: "IDR",
		Health: HealthHealthy,
		LastCheckedAt: now,
		LastSuccessAt: now,
		ConsecutiveFailures: 0,
	}
}

func TestValidateSnapshotAcceptsConsistentHealthStates(t *testing.T) {
	now := time.Date(2026, 9, 28, 10, 0, 0, 0, time.UTC)
	cases := []Snapshot{
		validOperationalSnapshot(),
		{ProviderName: "mock", Currency: "IDR", Health: HealthDegraded, LastCheckedAt: now, ConsecutiveFailures: 1, LastError: "temporary failure"},
		{ProviderName: "mock", Currency: "IDR", Health: HealthUnhealthy, LastCheckedAt: now, LastSuccessAt: now.Add(-time.Minute), ConsecutiveFailures: 3, LastError: "provider unavailable"},
		{ProviderName: "mock", Currency: "IDR", Health: HealthUnknown, LastCheckedAt: now},
	}
	for _, snapshot := range cases {
		if err := ValidateSnapshot(snapshot); err != nil {
			t.Fatalf("expected valid snapshot, got %v: %#v", err, snapshot)
		}
	}
}

func TestValidateSnapshotRejectsContradictoryOperationalState(t *testing.T) {
	now := time.Date(2026, 9, 28, 10, 0, 0, 0, time.UTC)
	cases := []struct {
		name string
		snapshot Snapshot
		want error
	}{
		{"missing provider", Snapshot{Currency: "IDR", Health: HealthUnknown, LastCheckedAt: now}, ErrInvalidOperationalSnapshot},
		{"missing currency", Snapshot{ProviderName: "mock", Health: HealthUnknown, LastCheckedAt: now}, ErrInvalidOperationalSnapshot},
		{"missing checked time", Snapshot{ProviderName: "mock", Currency: "IDR", Health: HealthUnknown}, ErrInvalidOperationalSnapshot},
		{"success after check", func() Snapshot { s := validOperationalSnapshot(); s.LastSuccessAt = now.Add(time.Minute); return s }(), ErrOperationalTimestampOrder},
		{"negative failures", func() Snapshot { s := validOperationalSnapshot(); s.ConsecutiveFailures = -1; return s }(), ErrOperationalFailureCount},
		{"healthy with failure evidence", func() Snapshot { s := validOperationalSnapshot(); s.ConsecutiveFailures = 1; s.LastError = "temporary failure"; return s }(), ErrOperationalHealthState},
		{"healthy without current success", func() Snapshot { s := validOperationalSnapshot(); s.LastSuccessAt = now.Add(-time.Minute); return s }(), ErrOperationalTimestampOrder},
		{"degraded without evidence", func() Snapshot { s := validOperationalSnapshot(); s.Health = HealthDegraded; return s }(), ErrOperationalHealthState},
		{"unhealthy without error", func() Snapshot { s := validOperationalSnapshot(); s.Health = HealthUnhealthy; s.ConsecutiveFailures = 2; return s }(), ErrOperationalHealthState},
		{"unknown with failure evidence", func() Snapshot { s := validOperationalSnapshot(); s.Health = HealthUnknown; s.LastError = "failure"; s.ConsecutiveFailures = 1; return s }(), ErrOperationalHealthState},
		{"unknown health value", func() Snapshot { s := validOperationalSnapshot(); s.Health = Health("maintenance"); return s }(), ErrInvalidOperationalSnapshot},
	}
	for _, tc := range cases {
		if err := ValidateSnapshot(tc.snapshot); !errors.Is(err, tc.want) {
			t.Errorf("%s: expected errors.Is(%v), got %v", tc.name, tc.want, err)
		}
	}
}

func TestValidateSnapshotIsObservationalOnly(t *testing.T) {
	if err := ValidateSnapshot(validOperationalSnapshot()); err != nil {
		t.Fatal(err)
	}
}
