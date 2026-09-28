package operational

import (
	"errors"
	"testing"
	"time"
)

func TestSnapshotEvaluateFreshness(t *testing.T) {
	now := time.Date(2026, 9, 28, 10, 0, 0, 0, time.UTC)
	window := time.Minute

	tests := []struct {
		name      string
		checkedAt time.Time
		want      Freshness
	}{
		{name: "unknown without check", want: FreshnessUnknown},
		{name: "fresh at boundary", checkedAt: now.Add(-window), want: FreshnessFresh},
		{name: "fresh before boundary", checkedAt: now.Add(-30 * time.Second), want: FreshnessFresh},
		{name: "stale after boundary", checkedAt: now.Add(-window - time.Second), want: FreshnessStale},
		{name: "future timestamp is fresh", checkedAt: now.Add(time.Second), want: FreshnessFresh},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			snapshot := Snapshot{LastCheckedAt: tt.checkedAt}
			got, err := snapshot.EvaluateFreshness(now, window)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("expected freshness %q, got %q", tt.want, got)
			}
		})
	}
}

func TestSnapshotEvaluateFreshnessRejectsInvalidWindow(t *testing.T) {
	_, err := (Snapshot{}).EvaluateFreshness(time.Now(), 0)
	if !errors.Is(err, ErrInvalidFreshnessWindow) {
		t.Fatalf("expected invalid freshness window, got %v", err)
	}
}

func TestReadOperationalSnapshotDistinguishesFreshnessWithoutAuthorizingBalance(t *testing.T) {
	registry := NewRegistryForFreshnessTest("mock", 2500000)
	store := NewMemoryStore()
	svc, err := NewSyncService(registry, store, "IDR", 3)
	if err != nil {
		t.Fatal(err)
	}

	now := time.Date(2026, 9, 28, 10, 0, 0, 0, time.UTC)
	svc.Now = func() time.Time { return now }

	if _, err := svc.SyncProvider(t.Context(), "mock"); err != nil {
		t.Fatal(err)
	}

	snapshot, freshness, err := svc.ReadOperationalSnapshot("mock", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Balance != 2500000 || freshness != FreshnessFresh {
		t.Fatalf("unexpected operational read: snapshot=%#v freshness=%q", snapshot, freshness)
	}

	now = now.Add(2 * time.Minute)
	_, freshness, err = svc.ReadOperationalSnapshot("mock", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if freshness != FreshnessStale {
		t.Fatalf("expected stale snapshot, got %q", freshness)
	}
}

func TestReadFreshOperationalSnapshotRejectsStaleObservation(t *testing.T) {
	registry := NewRegistryForFreshnessTest("mock", 2500000)
	store := NewMemoryStore()
	svc, err := NewSyncService(registry, store, "IDR", 3)
	if err != nil {
		t.Fatal(err)
	}

	now := time.Date(2026, 9, 28, 10, 0, 0, 0, time.UTC)
	svc.Now = func() time.Time { return now }
	if _, err := svc.SyncProvider(t.Context(), "mock"); err != nil {
		t.Fatal(err)
	}

	now = now.Add(2 * time.Minute)
	_, err = svc.ReadFreshOperationalSnapshot("mock", time.Minute)
	if !errors.Is(err, ErrStaleOperationalSnapshot) {
		t.Fatalf("expected stale snapshot error, got %v", err)
	}
}

func TestReadOperationalSnapshotRejectsMissingProvider(t *testing.T) {
	svc, err := NewSyncService(NewRegistryForFreshnessTest("mock", 1), NewMemoryStore(), "IDR", 3)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = svc.ReadOperationalSnapshot("missing", time.Minute)
	if !errors.Is(err, ErrOperationalSnapshotNotFound) {
		t.Fatalf("expected missing snapshot error, got %v", err)
	}
}

func NewRegistryForFreshnessTest(name string, balance int64) *Registry {
	registry := NewRegistry()
	_ = registry.Register(name, balanceStub{balance: balance})
	return registry
}
