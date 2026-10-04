package operational

import (
	"errors"
	"time"
)

type Freshness string

const (
	FreshnessUnknown Freshness = "unknown"
	FreshnessFresh   Freshness = "fresh"
	FreshnessStale   Freshness = "stale"
)

var (
	ErrInvalidFreshnessWindow    = errors.New("freshness window must be greater than zero")
	ErrOperationalSnapshotNotFound = errors.New("provider operational snapshot not found")
	ErrStaleOperationalSnapshot  = errors.New("provider operational snapshot is stale")
)

// EvaluateFreshness derives observational freshness from the last operational
// check. Freshness does not authorize payment, payout, routing, funding, or
// any other financial side effect.
func (s Snapshot) EvaluateFreshness(now time.Time, maxAge time.Duration) (Freshness, error) {
	if maxAge <= 0 {
		return FreshnessUnknown, ErrInvalidFreshnessWindow
	}
	if s.LastCheckedAt.IsZero() {
		return FreshnessUnknown, nil
	}
	if now.Before(s.LastCheckedAt) || now.Sub(s.LastCheckedAt) <= maxAge {
		return FreshnessFresh, nil
	}
	return FreshnessStale, nil
}

// ReadOperationalSnapshot returns the stored operational observation together
// with its derived freshness. It is observational only and must not be used
// as transaction authorization.
func (s *SyncService) ReadOperationalSnapshot(name string, maxAge time.Duration) (Snapshot, Freshness, error) {
	if s == nil || s.Store == nil {
		return Snapshot{}, FreshnessUnknown, errors.New("operational store is required")
	}
	snapshot, found, err := getSnapshot(s.Store, name)
	if err != nil {
		return Snapshot{}, FreshnessUnknown, err
	}
	if !found {
		return Snapshot{}, FreshnessUnknown, ErrOperationalSnapshotNotFound
	}
	freshness, err := snapshot.EvaluateFreshness(s.Now(), maxAge)
	if err != nil {
		return Snapshot{}, FreshnessUnknown, err
	}
	return snapshot, freshness, nil
}

// ReadFreshOperationalSnapshot is a convenience read for operational
// consumers that explicitly require a fresh observation. It still does not
// authorize a financial side effect or imply provider routing eligibility.
func (s *SyncService) ReadFreshOperationalSnapshot(name string, maxAge time.Duration) (Snapshot, error) {
	snapshot, freshness, err := s.ReadOperationalSnapshot(name, maxAge)
	if err != nil {
		return Snapshot{}, err
	}
	if freshness != FreshnessFresh {
		return Snapshot{}, ErrStaleOperationalSnapshot
	}
	return snapshot, nil
}
